package demandflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/runtime/agent"
	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
	"github.com/jesseedcp/devflow-agent/internal/runtime/llm"
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
	"github.com/jesseedcp/devflow-agent/internal/runtime/tools"
)

type RuntimeRunner struct {
	ConfigPath     string
	PermissionMode permissions.PermissionMode
	MaxIterations  int
}

func runtimeRegistry(protocol string) *tools.Registry {
	registry := tools.CreateDefaultRegistry()
	registry.Register(&tools.ToolSearchTool{Registry: registry, Protocol: protocol})
	return registry
}

func permissionModeFor(req RunnerRequest, explicit permissions.PermissionMode) (permissions.PermissionMode, error) {
	switch req.Stage {
	case StageImplementation:
		switch explicit {
		case permissions.ModeAcceptEdits, permissions.ModeBypass:
			return explicit, nil
		default:
			return "", fmt.Errorf("implementation stage requires an explicit permission mode (acceptEdits or bypassPermissions)")
		}
	case StageRequirements, StagePlan, StageVerification, StageCloseout:
		return permissions.ModePlan, nil
	default:
		return "", fmt.Errorf("stage %s does not have an agent permission mode", req.Stage)
	}
}

func runtimePermissionResponse(req RunnerRequest, mode permissions.PermissionMode, ev agent.PermissionRequestEvent) agent.PermissionResponse {
	switch req.Stage {
	case StageRequirements, StagePlan, StageVerification, StageCloseout:
		if isRuntimeReadOnlyTool(ev.ToolName) {
			return agent.PermAllow
		}
		return agent.PermDeny
	case StageImplementation:
		switch mode {
		case permissions.ModeBypass:
			return agent.PermAllow
		case permissions.ModeAcceptEdits:
			if isRuntimeReadOnlyTool(ev.ToolName) || ev.ToolName == "WriteFile" || ev.ToolName == "EditFile" {
				return agent.PermAllow
			}
			return agent.PermDeny
		default:
			return agent.PermDeny
		}
	default:
		return agent.PermDeny
	}
}

func isRuntimeReadOnlyTool(toolName string) bool {
	switch toolName {
	case "ReadFile", "Glob", "Grep", "ToolSearch":
		return true
	default:
		return false
	}
}

func (r RuntimeRunner) Run(ctx context.Context, req RunnerRequest) (RunnerResponse, error) {
	mode, err := permissionModeFor(req, r.PermissionMode)
	if err != nil {
		return RunnerResponse{}, err
	}

	cfg, err := config.LoadConfig(r.ConfigPath)
	if err != nil {
		return RunnerResponse{}, fmt.Errorf("load devflow config: %w", err)
	}
	if len(cfg.Providers) == 0 {
		return RunnerResponse{}, fmt.Errorf("no providers configured in devflow config")
	}
	provider := &cfg.Providers[0]

	systemPrompt := "You are Devflow, the backend demand delivery agent. Follow the stage prompt exactly. Return a complete markdown artifact body only. Never return an empty answer. If blocked, still return the required headings and describe the blocker under the relevant section."
	client, err := llm.NewClient(provider, systemPrompt)
	if err != nil {
		return RunnerResponse{}, fmt.Errorf("create llm client: %w", err)
	}

	registry := runtimeRegistry(provider.Protocol)
	ag := agent.New(client, registry, provider.Protocol)
	ag.WorkDir = req.Root
	maxIterations := r.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 20
	}
	ag.MaxIterations = maxIterations
	ag.Checker = permissions.NewChecker(
		permissions.NewPathSandbox(req.Root),
		&permissions.RuleEngine{LocalPath: filepath.Join(req.Root, ".devflow", "permissions.local.yaml")},
		mode,
	)

	conv := conversation.NewManager()
	conv.AddUserMessage(req.Prompt)

	var textParts []string
	var toolSummary []string
	var traces []RuntimeToolTrace
	toolDescs := map[string]string{}
	var agentErr error
	maxIterationsHit := false
	var finalizedText string
	var finalizedRuntime RuntimeSummary
	for ev := range ag.Run(ctx, conv) {
		switch e := ev.(type) {
		case agent.StreamText:
			textParts = append(textParts, e.Text)
		case agent.ToolUseEvent:
			collectRuntimeTraceUse(toolDescs, e)
		case agent.ToolResultEvent:
			toolSummary = append(toolSummary, e.ToolName)
			traces = append(traces, collectRuntimeTraceResult(toolDescs, e))
		case agent.PermissionRequestEvent:
			e.ResponseCh <- runtimePermissionResponse(req, mode, e)
		case agent.ErrorEvent:
			if strings.Contains(e.Message, "maximum iterations") {
				maxIterationsHit = true
			}
			if body, summary, ok := maybeFinalizeRuntimeError(req, provider.Model, maxIterations, traces, e.Message); ok {
				finalizedText = body
				finalizedRuntime = summary
				agentErr = nil
				continue
			}
			agentErr = runtimeAgentError(req.Stage, provider.Model, maxIterations, toolSummary, e.Message)
		case agent.LoopComplete:
		}
	}
	if strings.TrimSpace(finalizedText) != "" {
		return RunnerResponse{
			Text:        strings.TrimSpace(finalizedText),
			ToolSummary: toolSummary,
			Runtime:     finalizedRuntime,
		}, nil
	}
	if agentErr != nil {
		return RunnerResponse{}, agentErr
	}

	text := strings.TrimSpace(strings.Join(textParts, ""))
	if text == "" {
		return RunnerResponse{}, runtimeEmptyOutputError(req.Stage, provider.Model, maxIterations)
	}

	return RunnerResponse{
		Text:        text,
		ToolSummary: toolSummary,
		Runtime: summarizeRuntimeTraces(
			req.Stage,
			provider.Model,
			RuntimeCompletionModelText,
			maxIterationsHit,
			traces,
			changedFilesFromRuntimeTraces(req.Root, traces),
		),
	}, nil
}

func runtimeEmptyOutputError(stage Stage, model string, maxIterations int) error {
	return fmt.Errorf("runtime runner produced no artifact text after %d iterations for stage %s with model %s; retry with a stronger model, inspect provider compatibility, or write the artifact manually and continue through the review gate", maxIterations, stage, model)
}

func runtimeAgentError(stage Stage, model string, maxIterations int, toolSummary []string, message string) error {
	if len(toolSummary) == 0 {
		return fmt.Errorf("runtime runner failed for stage %s with model %s after up to %d iterations: %s; no tool results were observed", stage, model, maxIterations, message)
	}
	return fmt.Errorf(
		"runtime runner failed for stage %s with model %s after up to %d iterations: %s; tool calls=%d last tools=%s",
		stage,
		model,
		maxIterations,
		message,
		len(toolSummary),
		strings.Join(lastRuntimeTools(toolSummary, 5), ","),
	)
}

func lastRuntimeTools(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func maybeFinalizeRuntimeError(req RunnerRequest, model string, maxIterations int, traces []RuntimeToolTrace, message string) (string, RuntimeSummary, bool) {
	if !strings.Contains(message, "maximum iterations") {
		return "", RuntimeSummary{}, false
	}
	if !shouldFinalizeImplementationAfterMaxIterations(req, traces) {
		return "", RuntimeSummary{}, false
	}
	changedFiles := changedFilesFromRuntimeTraces(req.Root, traces)
	body := renderImplementationRuntimeFinalizer(model, maxIterations, traces, changedFiles)
	summary := summarizeRuntimeTraces(
		req.Stage,
		model,
		RuntimeCompletionDeterministicFinalizer,
		true,
		traces,
		changedFiles,
	)
	return body, summary, true
}

func collectRuntimeTraceUse(descs map[string]string, ev agent.ToolUseEvent) {
	desc := ""
	switch ev.ToolName {
	case "Bash":
		desc, _ = ev.Args["command"].(string)
	case "ReadFile", "WriteFile", "EditFile":
		desc, _ = ev.Args["file_path"].(string)
	case "Glob", "Grep":
		desc, _ = ev.Args["pattern"].(string)
	case "ToolSearch":
		desc, _ = ev.Args["query"].(string)
	}
	if strings.TrimSpace(desc) != "" {
		descs[ev.ToolID] = strings.TrimSpace(desc)
	}
}

func collectRuntimeTraceResult(descs map[string]string, ev agent.ToolResultEvent) RuntimeToolTrace {
	return RuntimeToolTrace{
		ToolID:   ev.ToolID,
		ToolName: ev.ToolName,
		Desc:     descs[ev.ToolID],
		Output:   ev.Output,
		IsError:  ev.IsError,
		Elapsed:  ev.Elapsed,
	}
}

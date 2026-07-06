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
	var agentErr error
	for ev := range ag.Run(ctx, conv) {
		switch e := ev.(type) {
		case agent.StreamText:
			textParts = append(textParts, e.Text)
		case agent.ToolResultEvent:
			toolSummary = append(toolSummary, e.ToolName)
		case agent.PermissionRequestEvent:
			e.ResponseCh <- agent.PermDeny
		case agent.ErrorEvent:
			agentErr = fmt.Errorf("agent error: %s", e.Message)
		case agent.LoopComplete:
		}
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
	}, nil
}

func runtimeEmptyOutputError(stage Stage, model string, maxIterations int) error {
	return fmt.Errorf("runtime runner produced no artifact text after %d iterations for stage %s with model %s; retry with a stronger model, inspect provider compatibility, or write the artifact manually and continue through the review gate", maxIterations, stage, model)
}

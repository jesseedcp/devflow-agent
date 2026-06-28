package demandflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type Engine struct {
	Store artifacts.Store
	Gate  quality.Gate
	root  string
}

func NewEngine(root string) Engine {
	return Engine{
		Store: artifacts.NewStore(root),
		Gate:  quality.Gate{},
		root:  root,
	}
}

func (e Engine) Run(ctx context.Context, opts Options) error {
	if opts.Runner == nil {
		return fmt.Errorf("runner is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return e.Store.WithDemandLock(opts.DemandID, func() error {
		switch opts.Stage {
		case StageRequirements:
			return e.runRequirements(ctx, opts)
		case StagePlan:
			return e.runPlan(ctx, opts)
		case StageImplementation:
			return e.runImplementation(ctx, opts)
		case StageVerification:
			return e.runVerification(ctx, opts)
		case StageCloseout:
			return e.runCloseout(ctx, opts)
		default:
			return fmt.Errorf("unsupported stage %q", opts.Stage)
		}
	})
}

func (e Engine) advance(demand *artifacts.Demand, next workflow.State) error {
	current := workflow.State(demand.State)
	advanced, err := workflow.Advance(current, next)
	if err != nil {
		return err
	}
	demand.State = string(advanced)
	return e.Store.SaveDemand(*demand)
}

func (e Engine) runRequirements(ctx context.Context, opts Options) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}

	current := workflow.State(demand.State)
	if current != workflow.Created && current != workflow.ContextLoaded {
		return fmt.Errorf("requirements stage requires state created or context_loaded, got %s", current)
	}
	if current == workflow.Created {
		if err := e.advance(&demand, workflow.ContextLoaded); err != nil {
			return err
		}
	}
	if err := e.advance(&demand, workflow.RequirementsDrafting); err != nil {
		return err
	}

	snapshot, err := newContextLoader(e.root).Load(opts.DemandID)
	if err != nil {
		return err
	}
	prompt, _, err := BuildPrompt(StageRequirements, snapshot)
	if err != nil {
		return err
	}
	resp, err := opts.Runner.Run(ctx, RunnerRequest{
		Stage:    StageRequirements,
		Root:     opts.Root,
		DemandID: opts.DemandID,
		Prompt:   prompt,
		Context:  snapshot,
		ToolMode: ToolModeReadOnly,
	})
	if err != nil {
		return err
	}

	if err := e.Store.WriteArtifact(opts.DemandID, artifacts.RequirementsFile, resp.Text); err != nil {
		return err
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "requirements.drafted",
		Message: "requirements drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.RequirementsReview)
}

func (e Engine) runPlan(ctx context.Context, opts Options) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.PlanDrafting {
		return fmt.Errorf("plan stage requires state plan_drafting, got %s", demand.State)
	}

	snapshot, err := newContextLoader(e.root).Load(opts.DemandID)
	if err != nil {
		return err
	}
	prompt, _, err := BuildPrompt(StagePlan, snapshot)
	if err != nil {
		return err
	}
	resp, err := opts.Runner.Run(ctx, RunnerRequest{
		Stage:    StagePlan,
		Root:     opts.Root,
		DemandID: opts.DemandID,
		Prompt:   prompt,
		Context:  snapshot,
		ToolMode: ToolModeReadOnly,
	})
	if err != nil {
		return err
	}

	if err := e.Store.WriteArtifact(opts.DemandID, artifacts.PlanFile, resp.Text); err != nil {
		return err
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "plan.drafted",
		Message: "plan drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.PlanReview)
}

func (e Engine) runImplementation(ctx context.Context, opts Options) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.Implementation {
		return fmt.Errorf("implementation stage requires state implementation, got %s", demand.State)
	}

	snapshot, err := newContextLoader(e.root).Load(opts.DemandID)
	if err != nil {
		return err
	}
	prompt, toolMode, err := BuildPrompt(StageImplementation, snapshot)
	if err != nil {
		return err
	}
	resp, err := opts.Runner.Run(ctx, RunnerRequest{
		Stage:    StageImplementation,
		Root:     opts.Root,
		DemandID: opts.DemandID,
		Prompt:   prompt,
		Context:  snapshot,
		ToolMode: toolMode,
	})
	if err != nil {
		return err
	}

	progress := strings.TrimSpace(resp.Text)
	for _, tool := range resp.ToolSummary {
		progress += "\n- " + tool
	}
	if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, progress+"\n\n"); err != nil {
		return err
	}

	if len(opts.QualityCommands) > 0 {
		result := e.Gate.Run(ctx, opts.Root, opts.QualityCommands...)
		if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, renderQualityEvidence(result)); err != nil {
			return err
		}
		if !result.Passed {
			if err := e.advance(&demand, workflow.FailedQualityGate); err != nil {
				return err
			}
			return fmt.Errorf("quality gate failed: %s", summarizeQuality(result))
		}
	}

	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "implementation.completed",
		Message: "implementation completed and quality gate passed",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.MRReview)
}

func (e Engine) runVerification(ctx context.Context, opts Options) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.Verification {
		return fmt.Errorf("verification stage requires state verification, got %s", demand.State)
	}

	snapshot, err := newContextLoader(e.root).Load(opts.DemandID)
	if err != nil {
		return err
	}
	prompt, _, err := BuildPrompt(StageVerification, snapshot)
	if err != nil {
		return err
	}
	resp, err := opts.Runner.Run(ctx, RunnerRequest{
		Stage:    StageVerification,
		Root:     opts.Root,
		DemandID: opts.DemandID,
		Prompt:   prompt,
		Context:  snapshot,
		ToolMode: ToolModeReadOnly,
	})
	if err != nil {
		return err
	}

	body := strings.TrimSpace(resp.Text)
	if len(opts.QualityCommands) > 0 {
		result := e.Gate.Run(ctx, opts.Root, opts.QualityCommands...)
		body += "\n\n" + renderQualityEvidence(result)
	}
	if err := e.Store.WriteArtifact(opts.DemandID, artifacts.VerificationFile, body+"\n"); err != nil {
		return err
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "verification.drafted",
		Message: "verification drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.Verification)
}

func (e Engine) runCloseout(ctx context.Context, opts Options) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.Closeout {
		return fmt.Errorf("closeout stage requires state closeout, got %s", demand.State)
	}

	snapshot, err := newContextLoader(e.root).Load(opts.DemandID)
	if err != nil {
		return err
	}
	prompt, _, err := BuildPrompt(StageCloseout, snapshot)
	if err != nil {
		return err
	}
	resp, err := opts.Runner.Run(ctx, RunnerRequest{
		Stage:    StageCloseout,
		Root:     opts.Root,
		DemandID: opts.DemandID,
		Prompt:   prompt,
		Context:  snapshot,
		ToolMode: ToolModeReadOnly,
	})
	if err != nil {
		return err
	}

	const memoryMarker = "---DEVFLOW-MEMORY-CANDIDATES---"
	closeoutBody := strings.TrimSpace(resp.Text)
	memoryBody := ""
	if parts := strings.SplitN(resp.Text, memoryMarker, 2); len(parts) == 2 {
		closeoutBody = strings.TrimSpace(parts[0])
		memoryBody = strings.TrimSpace(parts[1])
	} else {
		memoryBody = strings.TrimSpace(fmt.Sprintf("# Memory Candidates: %s\n\n## 稳定知识候选\n\n- (no stable candidates were generated)\n", demand.Title))
	}

	if err := e.Store.WriteArtifact(opts.DemandID, artifacts.CloseoutFile, closeoutBody+"\n"); err != nil {
		return err
	}
	if err := e.Store.WriteArtifact(opts.DemandID, artifacts.MemoryCandidatesFile, memoryBody+"\n"); err != nil {
		return err
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "closeout.drafted",
		Message: "closeout and memory candidates drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.Closeout)
}

func renderQualityEvidence(result quality.GateResult) string {
	var b strings.Builder
	b.WriteString("## 质量门禁结果\n\n")
	for _, r := range result.Results {
		command := r.Command
		if len(r.Args) > 0 {
			command += " " + strings.Join(r.Args, " ")
		}
		fmt.Fprintf(&b, "- %s: exit %d\n", command, r.ExitCode)
		for _, line := range strings.Split(strings.TrimSpace(r.Stdout), "\n") {
			if line == "" {
				continue
			}
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		for _, line := range strings.Split(strings.TrimSpace(r.Stderr), "\n") {
			if line == "" {
				continue
			}
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

func summarizeQuality(result quality.GateResult) string {
	var failing []string
	for _, r := range result.Results {
		if r.ExitCode == 0 {
			continue
		}
		command := r.Command
		if len(r.Args) > 0 {
			command += " " + strings.Join(r.Args, " ")
		}
		failing = append(failing, fmt.Sprintf("%s (exit %d)", command, r.ExitCode))
	}
	if len(failing) == 0 {
		return "quality gate failed"
	}
	return strings.Join(failing, "; ")
}

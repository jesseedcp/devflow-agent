package demandflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"

	"strconv"
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

func qualityRoot(opts Options) string {
	if strings.TrimSpace(opts.QualityRoot) != "" {
		return opts.QualityRoot
	}
	return opts.Root
}

func runnerRoot(opts Options) string {
	if strings.TrimSpace(opts.RunnerRoot) != "" {
		return opts.RunnerRoot
	}
	return opts.Root
}

func (e Engine) Run(ctx context.Context, opts Options) error {
	_, err := e.RunDetailed(ctx, opts)
	return err
}

func (e Engine) RunDetailed(ctx context.Context, opts Options) (RunResult, error) {
	if opts.Runner == nil {
		return RunResult{}, fmt.Errorf("runner is required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	var result RunResult
	err := e.Store.WithDemandLock(opts.DemandID, func() error {
		demand, loadErr := e.Store.LoadDemand(opts.DemandID)
		if loadErr != nil {
			return loadErr
		}
		result = RunResult{
			DemandID:      opts.DemandID,
			Stage:         opts.Stage,
			PreviousState: workflow.State(demand.State),
		}
		switch opts.Stage {
		case StageRequirements:
			return e.runRequirements(ctx, opts, &result)
		case StagePlan:
			return e.runPlan(ctx, opts, &result)
		case StageImplementation:
			return e.runImplementation(ctx, opts, &result)
		case StageMRReview:
			return e.runMRReview(ctx, opts, &result)
		case StageVerification:
			return e.runVerification(ctx, opts, &result)
		case StageCloseout:
			return e.runCloseout(ctx, opts, &result)
		default:
			return fmt.Errorf("unsupported stage %q", opts.Stage)
		}
	})
	if err != nil {
		if result.DemandID != "" {
			if demand, loadErr := e.Store.LoadDemand(opts.DemandID); loadErr == nil {
				result.CurrentState = workflow.State(demand.State)
				result.NextActions = NextActions(result.CurrentState, opts.DemandID)
			}
		}
		return result, err
	}

	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return result, err
	}
	result.CurrentState = workflow.State(demand.State)
	result.NextActions = NextActions(result.CurrentState, opts.DemandID)
	return result, nil
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

func (e Engine) runRequirements(ctx context.Context, opts Options, result *RunResult) error {
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
		Root:     runnerRoot(opts),
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
	result.Artifacts = append(result.Artifacts, artifacts.RequirementsFile)
	result.Message = "requirements drafted by demand runner"
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "requirements.drafted",
		Message: "requirements drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.RequirementsReview)
}

func (e Engine) runPlan(ctx context.Context, opts Options, result *RunResult) error {
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
		Root:     runnerRoot(opts),
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
	result.Artifacts = append(result.Artifacts, artifacts.PlanFile)
	result.Message = "plan drafted by demand runner"
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "plan.drafted",
		Message: "plan drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.PlanReview)
}

func (e Engine) runImplementation(ctx context.Context, opts Options, result *RunResult) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}
	current := workflow.State(demand.State)
	if current == workflow.FailedQualityGate {
		if err := e.advance(&demand, workflow.Implementation); err != nil {
			return err
		}
		if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
			Time:    opts.Now(),
			Type:    "implementation.retry",
			Message: "implementation retried after failed quality gate",
		}); err != nil {
			return err
		}
	} else if current != workflow.Implementation {
		return fmt.Errorf("implementation stage requires state implementation or failed_quality_gate, got %s", current)
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
		Root:     runnerRoot(opts),
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
	result.Artifacts = append(result.Artifacts, artifacts.ProgressFile)

	if len(opts.QualityCommands) > 0 {
		gateResult := e.Gate.Run(ctx, qualityRoot(opts), opts.QualityCommands...)
		passed := gateResult.Passed
		result.QualityPassed = &passed
		if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, renderQualityEvidence(gateResult)); err != nil {
			return err
		}
		if !gateResult.Passed {
			if err := e.advance(&demand, workflow.FailedQualityGate); err != nil {
				return err
			}
			result.Message = "quality gate failed: " + summarizeQuality(gateResult)
			return fmt.Errorf("quality gate failed: %s", summarizeQuality(gateResult))
		}
	}

	if opts.MergeRequest.Adapter != nil {
		if err := e.syncMergeRequest(ctx, opts, &demand); err != nil {
			return err
		}
	}

	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "implementation.completed",
		Message: "implementation completed and quality gate passed",
	}); err != nil {
		return err
	}
	result.Message = "implementation completed and quality gate passed"
	return e.advance(&demand, workflow.MRReview)
}

func (e Engine) runVerification(ctx context.Context, opts Options, result *RunResult) error {
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
		Root:     runnerRoot(opts),
		DemandID: opts.DemandID,
		Prompt:   prompt,
		Context:  snapshot,
		ToolMode: ToolModeReadOnly,
	})
	if err != nil {
		return err
	}

	body := strings.TrimSpace(resp.Text)
	qualityFailed := false
	qualitySummary := ""
	if len(opts.QualityCommands) > 0 {
		gateResult := e.Gate.Run(ctx, qualityRoot(opts), opts.QualityCommands...)
		passed := gateResult.Passed
		result.QualityPassed = &passed
		body += "\n\n" + renderQualityEvidence(gateResult)
		if !gateResult.Passed {
			qualityFailed = true
			qualitySummary = summarizeQuality(gateResult)
		}
	}
	if err := e.Store.WriteArtifact(opts.DemandID, artifacts.VerificationFile, body+"\n"); err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, artifacts.VerificationFile)
	if qualityFailed {
		if err := e.advance(&demand, workflow.FailedQualityGate); err != nil {
			return err
		}
		result.Message = "quality gate failed: " + qualitySummary
		return fmt.Errorf("quality gate failed: %s", qualitySummary)
	}
	result.Message = "verification drafted by demand runner"
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "verification.drafted",
		Message: "verification drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.Verification)
}

func (e Engine) runCloseout(ctx context.Context, opts Options, result *RunResult) error {
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
		Root:     runnerRoot(opts),
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
	result.Artifacts = append(result.Artifacts, artifacts.CloseoutFile, artifacts.MemoryCandidatesFile)
	result.Message = "closeout and memory candidates drafted by demand runner"
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "closeout.drafted",
		Message: "closeout and memory candidates drafted by demand runner",
	}); err != nil {
		return err
	}
	return e.advance(&demand, workflow.Closeout)
}

func (e Engine) runMRReview(ctx context.Context, opts Options, result *RunResult) error {
	demand, err := e.Store.LoadDemand(opts.DemandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.MRReview {
		return fmt.Errorf("mr-review stage requires state mr_review, got %s", demand.State)
	}
	if opts.Review.Adapter == nil {
		return fmt.Errorf("mr-review stage requires a review adapter")
	}

	comments, err := opts.Review.Adapter.ListUnresolved(ctx, opts.Review.Ref)
	if err != nil {
		return fmt.Errorf("list unresolved review comments: %w", err)
	}

	if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, renderReviewSummary(comments)); err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, artifacts.ProgressFile)

	actionPlan := BuildReviewActionPlan(comments)
	if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, RenderReviewActionPlan(actionPlan)); err != nil {
		return err
	}

	if actionPlan.NextState != workflow.Verification {
		if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
			Time:    opts.Now(),
			Type:    "mr_review.action_required",
			Message: actionPlan.Message,
			Data: map[string]string{
				"next_state": string(actionPlan.NextState),
			},
		}); err != nil {
			return err
		}
		if err := e.advance(&demand, actionPlan.NextState); err != nil {
			return err
		}
		result.Message = actionPlan.Message
		return errors.New(actionPlan.Message)
	}

	if opts.MergeRequest.Adapter != nil {
		if err := e.syncMergeRequest(ctx, opts, &demand); err != nil {
			return err
		}
	}

	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "mr_review.cleared",
		Message: "mr review cleared, no blocking unresolved comments",
	}); err != nil {
		return err
	}
	result.Message = "mr review cleared, no blocking unresolved comments"
	return e.advance(&demand, workflow.Verification)
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

func renderMergeRequestEvidence(result adapters.MergeRequestResult) string {
	verb := "Reused"
	if result.WasCreated {
		verb = "Created"
	}
	return fmt.Sprintf("\n## 合并请求\n\n%s !%d\n\n- **Title:** %s\n- **State:** %s\n- **URL:** %s\n- **Action:** %s\n\n",
		verb, result.IID, result.Title, result.State, result.WebURL, verb)
}

func (e Engine) syncMergeRequest(ctx context.Context, opts Options, demand *artifacts.Demand) error {
	adapter := opts.MergeRequest.Adapter
	if adapter == nil {
		return nil
	}
	result, err := adapter.EnsureMergeRequest(ctx, opts.MergeRequest.Spec)
	if err != nil {
		if advanceErr := e.advance(demand, workflow.BlockedNeedPlatform); advanceErr != nil {
			return fmt.Errorf("merge request sync failed: %w (block state: %v)", err, advanceErr)
		}
		eventErr := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
			Time:    opts.Now(),
			Type:    "merge_request.sync_failed",
			Message: "merge request sync failed: " + err.Error(),
			Data:    map[string]string{"blocked_need_platform": err.Error()},
		})
		if eventErr != nil {
			return fmt.Errorf("mr sync failed: %w (event: %v)", err, eventErr)
		}
		return fmt.Errorf("merge request sync failed (blocked_need_platform): %w", err)
	}
	evidence := renderMergeRequestEvidence(result)
	if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, evidence); err != nil {
		return err
	}
	eventData := map[string]string{
		"mr_iid":    strconv.Itoa(result.IID),
		"mr_url":    result.WebURL,
		"mr_action": "reused",
	}
	if result.WasCreated {
		eventData["mr_action"] = "created"
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "merge_request.synced",
		Message: fmt.Sprintf("merge request !%d %s", result.IID, eventData["mr_action"]),
		Data:    eventData,
	}); err != nil {
		return err
	}
	return nil
}

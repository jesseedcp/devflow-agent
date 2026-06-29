package dogfood

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type LiveOptions struct {
	Root          string
	ConfigPath    string
	Review        adapters.ReviewRef
	UseGitLab     bool
	Now           func() time.Time
	Timeout       time.Duration
	MaxIterations int
}

type LiveResult struct {
	Root       string
	RepoRoot   string
	DemandRoot string
	DemandID   string
	FinalState workflow.State
	ReportPath string
	Steps      []Step
}

const liveSandboxDemandID = "live-dogfood-coupon-eligibility"

func RunLiveSandbox(ctx context.Context, opts LiveOptions) (LiveResult, error) {
	if os.Getenv("DEVFLOW_LIVE_DOGFOOD") != "1" {
		return LiveResult{}, fmt.Errorf("live dogfood requires DEVFLOW_LIVE_DOGFOOD=1")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	sandbox, err := CreateLiveSandbox(opts.Root)
	if err != nil {
		return LiveResult{}, err
	}
	store := artifacts.NewStore(sandbox.DemandRoot)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          liveSandboxDemandID,
		Title:       "Live dogfood coupon eligibility",
		Description: "Implement coupon eligibility in the generated sandbox repo so go test ./... passes",
		Source:      "live-dogfood",
		State:       string(workflow.Created),
	}); err != nil {
		return LiveResult{}, fmt.Errorf("create live dogfood demand: %w", err)
	}

	maxIterations := opts.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30
	}
	runner := demandflow.RuntimeRunner{
		ConfigPath:     opts.ConfigPath,
		PermissionMode: permissions.ModeAcceptEdits,
		MaxIterations:  maxIterations,
	}
	engine := demandflow.NewEngine(sandbox.DemandRoot)
	result := LiveResult{
		Root:       sandbox.Root,
		RepoRoot:   sandbox.RepoRoot,
		DemandRoot: sandbox.DemandRoot,
		DemandID:   liveSandboxDemandID,
	}

	runStage := func(name string, stage demandflow.Stage, configure func(*demandflow.Options)) error {
		runOpts := demandflow.Options{
			Root:        sandbox.DemandRoot,
			RunnerRoot:  sandbox.RepoRoot,
			QualityRoot: sandbox.RepoRoot,
			DemandID:    liveSandboxDemandID,
			Stage:       stage,
			Runner:      runner,
			Now:         opts.Now,
		}
		if configure != nil {
			configure(&runOpts)
		}
		detail, err := engine.RunDetailed(ctx, runOpts)
		state := detail.CurrentState
		if state == "" {
			if demand, loadErr := store.LoadDemand(liveSandboxDemandID); loadErr == nil {
				state = workflow.State(demand.State)
			}
		}
		result.Steps = append(result.Steps, Step{Name: name, State: state, Output: detail.Message})
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	}

	confirm := func(stage, summary string) error {
		confirmation, err := demandflow.Confirm(demandflow.ConfirmOptions{
			Root:     sandbox.DemandRoot,
			DemandID: liveSandboxDemandID,
			Stage:    stage,
			By:       "devflow live dogfood",
			Summary:  summary,
			Now:      opts.Now,
		})
		result.Steps = append(result.Steps, Step{Name: "confirm " + stage, State: confirmation.CurrentState, Output: summary})
		if err != nil {
			return fmt.Errorf("confirm %s: %w", stage, err)
		}
		return nil
	}

	if err := runStage("requirements", demandflow.StageRequirements, nil); err != nil {
		return result, err
	}
	if err := confirm("requirements", "live requirements accepted for sandbox dogfood"); err != nil {
		return result, err
	}
	if err := runStage("plan", demandflow.StagePlan, nil); err != nil {
		return result, err
	}
	if err := confirm("plan", "live plan accepted for sandbox dogfood"); err != nil {
		return result, err
	}
	if err := runStage("implementation", demandflow.StageImplementation, func(o *demandflow.Options) {
		o.QualityCommands = sandbox.QualityCommands
	}); err != nil {
		return result, err
	}
	if opts.UseGitLab {
		if err := runStage("mr-review", demandflow.StageMRReview, func(o *demandflow.Options) {
			o.Review = demandflow.ReviewOptions{Adapter: adapters.GitLabReviewAdapter{}, Ref: opts.Review}
		}); err != nil {
			return result, err
		}
	} else {
		if err := runStage("mr-review", demandflow.StageMRReview, func(o *demandflow.Options) {
			o.Review = demandflow.ReviewOptions{Adapter: offlineReviewAdapter{}, Ref: adapters.ReviewRef{Project: "live-dogfood/offline", MergeRequest: "1"}}
		}); err != nil {
			return result, err
		}
	}
	if err := runStage("verification", demandflow.StageVerification, func(o *demandflow.Options) {
		o.QualityCommands = sandbox.QualityCommands
	}); err != nil {
		return result, err
	}
	if err := confirm("verification", "live sandbox verification passed"); err != nil {
		return result, err
	}
	if err := runStage("closeout", demandflow.StageCloseout, nil); err != nil {
		return result, err
	}
	if err := confirm("closeout", "live sandbox closeout accepted"); err != nil {
		return result, err
	}

	demand, err := store.LoadDemand(liveSandboxDemandID)
	if err != nil {
		return result, fmt.Errorf("load final live demand: %w", err)
	}
	result.FinalState = workflow.State(demand.State)
	reportPath := filepath.Join(store.DemandDir(liveSandboxDemandID), "live-dogfood-report.md")
	if err := os.WriteFile(reportPath, []byte(renderLiveReport(result)), 0o644); err != nil {
		return result, fmt.Errorf("write live dogfood report: %w", err)
	}
	result.ReportPath = reportPath
	return result, nil
}

func renderLiveReport(result LiveResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Live Dogfood Report: %s\n\n", result.DemandID)
	fmt.Fprintf(&b, "Root: `%s`\n\n", result.Root)
	fmt.Fprintf(&b, "RepoRoot: `%s`\n\n", result.RepoRoot)
	fmt.Fprintf(&b, "DemandRoot: `%s`\n\n", result.DemandRoot)
	fmt.Fprintf(&b, "FinalState: `%s`\n\n", result.FinalState)
	b.WriteString("## Steps\n\n")
	for _, step := range result.Steps {
		fmt.Fprintf(&b, "- `%s` -> `%s`: %s\n", step.Name, step.State, step.Output)
	}
	return b.String()
}

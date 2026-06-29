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
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type Options struct {
	Root            string
	QualityRoot     string
	ScenarioName    string
	QualityCommands []quality.Command
	Now             func() time.Time
}

type Result struct {
	Root        string
	QualityRoot string
	DemandID    string
	FinalState  workflow.State
	ReportPath  string
	Steps       []Step
}

type Step struct {
	Name   string
	State  workflow.State
	Output string
}

func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		temp, err := os.MkdirTemp("", "devflow-dogfood-*")
		if err != nil {
			return Result{}, fmt.Errorf("create dogfood root: %w", err)
		}
		root = temp
	}
	qualityRoot := strings.TrimSpace(opts.QualityRoot)
	if qualityRoot == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Result{}, fmt.Errorf("get quality root: %w", err)
		}
		qualityRoot = wd
	}
	scenario, ok := ScenarioByName(opts.ScenarioName)
	if !ok {
		return Result{}, fmt.Errorf("unsupported dogfood scenario %q", opts.ScenarioName)
	}
	if len(opts.QualityCommands) == 0 {
		opts.QualityCommands = []quality.Command{{Name: "go", Args: []string{"test", "./...", "-count=1", "-timeout", "5m"}}}
	}

	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          scenario.DemandID,
		Title:       scenario.Title,
		Description: scenario.Description,
		Source:      "dogfood:" + scenario.Name,
		State:       string(workflow.Created),
	}); err != nil {
		return Result{}, fmt.Errorf("create dogfood demand: %w", err)
	}

	engine := demandflow.NewEngine(root)
	runner := &demandflow.StaticRunner{Responses: scenario.Responses}
	result := Result{Root: root, QualityRoot: qualityRoot, DemandID: scenario.DemandID}

	runStage := func(name string, stage demandflow.Stage, configure func(*demandflow.Options)) error {
		runOpts := demandflow.Options{
			Root:        root,
			QualityRoot: qualityRoot,
			DemandID:    scenario.DemandID,
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
			if demand, loadErr := store.LoadDemand(scenario.DemandID); loadErr == nil {
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
			Root:     root,
			DemandID: scenario.DemandID,
			Stage:    stage,
			By:       "devflow dogfood",
			Summary:  summary,
			Now:      opts.Now,
		})
		state := confirmation.CurrentState
		result.Steps = append(result.Steps, Step{Name: "confirm " + stage, State: state, Output: summary})
		if err != nil {
			return fmt.Errorf("confirm %s: %w", stage, err)
		}
		return nil
	}

	if err := runStage("requirements", demandflow.StageRequirements, nil); err != nil {
		return result, err
	}
	if err := confirm("requirements", "deterministic dogfood requirements accepted"); err != nil {
		return result, err
	}
	if err := runStage("plan", demandflow.StagePlan, nil); err != nil {
		return result, err
	}
	if err := confirm("plan", "deterministic dogfood plan accepted"); err != nil {
		return result, err
	}
	if err := runStage("implementation", demandflow.StageImplementation, func(o *demandflow.Options) {
		o.MergeRequest = demandflow.MergeRequestOptions{
			Adapter: offlineMergeRequestAdapter{},
			Spec: adapters.MergeRequestSpec{
				SourceBranch: "dogfood/test",
				TargetBranch: "main",
				Title:        "Dogfood MR sync",
			},
		}
		o.QualityCommands = opts.QualityCommands
	}); err != nil {
		return result, err
	}
	if err := runStage("mr-review", demandflow.StageMRReview, func(o *demandflow.Options) {
		o.Review = demandflow.ReviewOptions{
			Adapter: offlineReviewAdapter{},
			Ref:     adapters.ReviewRef{Project: "dogfood/offline", MergeRequest: "1"},
		}
	}); err != nil {
		return result, err
	}
	if err := runStage("verification", demandflow.StageVerification, func(o *demandflow.Options) {
		o.QualityCommands = opts.QualityCommands
	}); err != nil {
		return result, err
	}
	if err := confirm("verification", "deterministic dogfood verification accepted"); err != nil {
		return result, err
	}
	if err := runStage("closeout", demandflow.StageCloseout, nil); err != nil {
		return result, err
	}
	if err := confirm("closeout", "deterministic dogfood closeout accepted"); err != nil {
		return result, err
	}

	demand, err := store.LoadDemand(scenario.DemandID)
	if err != nil {
		return result, fmt.Errorf("load final demand: %w", err)
	}
	result.FinalState = workflow.State(demand.State)
	reportPath := filepath.Join(store.DemandDir(scenario.DemandID), "dogfood-report.md")
	if err := os.WriteFile(reportPath, []byte(renderReport(result, store.DemandDir(scenario.DemandID))), 0o644); err != nil {
		return result, fmt.Errorf("write dogfood report: %w", err)
	}
	result.ReportPath = reportPath
	return result, nil
}

type offlineReviewAdapter struct{}

func (offlineReviewAdapter) ListUnresolved(context.Context, adapters.ReviewRef) ([]adapters.ReviewComment, error) {
	return nil, nil
}

func (offlineReviewAdapter) Reply(context.Context, adapters.ReviewRef, string, string) error {
	return nil
}

type offlineMergeRequestAdapter struct{}

func (offlineMergeRequestAdapter) EnsureMergeRequest(_ context.Context, _ adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	return adapters.MergeRequestResult{IID: 1, WebURL: "https://offline/-/1", Title: "Dogfood MR sync", State: "opened", WasCreated: true}, nil
}

func renderReport(result Result, demandDir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Dogfood Report: %s\n\n", result.DemandID)
	fmt.Fprintf(&b, "Root: `%s`\n\n", result.Root)
	fmt.Fprintf(&b, "QualityRoot: `%s`\n\n", result.QualityRoot)
	fmt.Fprintf(&b, "FinalState: `%s`\n\n", result.FinalState)
	b.WriteString("## Steps\n\n")
	for _, step := range result.Steps {
		fmt.Fprintf(&b, "- `%s` -> `%s`: %s\n", step.Name, step.State, step.Output)
	}
	b.WriteString("\n## Artifacts\n\n")
	for _, name := range []string{
		artifacts.RequirementsFile,
		artifacts.PlanFile,
		artifacts.ProgressFile,
		artifacts.VerificationFile,
		artifacts.CloseoutFile,
		artifacts.MemoryCandidatesFile,
		artifacts.EventsFile,
	} {
		fmt.Fprintf(&b, "- `%s`\n", filepath.Join(demandDir, name))
	}
	return b.String()
}

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

type OperatorOptions struct {
	Root            string
	QualityRoot     string
	ScenarioName    string
	QualityCommands []quality.Command
	Now             func() time.Time
}

type OperatorResult struct {
	Root        string
	QualityRoot string
	DemandID    string
	FinalState  workflow.State
	ReportPath  string
	Steps       []OperatorStep
}

type OperatorStep struct {
	Name       string
	State      workflow.State
	Attention  string
	Drive      string
	Evaluation string
	Output     string
}

func RunOperator(ctx context.Context, opts OperatorOptions) (OperatorResult, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	root, err := operatorRoot(opts.Root)
	if err != nil {
		return OperatorResult{}, err
	}
	qualityRoot, err := operatorQualityRoot(opts.QualityRoot)
	if err != nil {
		return OperatorResult{}, err
	}
	scenario, ok := ScenarioByName(opts.ScenarioName)
	if !ok {
		return OperatorResult{}, fmt.Errorf("unsupported dogfood scenario %q", opts.ScenarioName)
	}
	if len(opts.QualityCommands) == 0 {
		opts.QualityCommands = []quality.Command{{Name: "go", Args: []string{"test", "./...", "-count=1", "-timeout", "5m"}}}
	}

	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          scenario.DemandID,
		Title:       scenario.Title,
		Description: scenario.Description,
		Source:      "operator-dogfood:" + scenario.Name,
		State:       string(workflow.Created),
	}); err != nil {
		return OperatorResult{}, fmt.Errorf("create operator dogfood demand: %w", err)
	}

	if strings.TrimSpace(scenario.Codemap) != "" {
		if err := store.WriteArtifact(scenario.DemandID, artifacts.CodemapFile, scenario.Codemap); err != nil {
			return OperatorResult{}, fmt.Errorf("write operator dogfood codemap: %w", err)
		}
	}

	engine := demandflow.NewEngine(root)
	runner := &demandflow.StaticRunner{Responses: scenario.Responses}
	result := OperatorResult{Root: root, QualityRoot: qualityRoot, DemandID: scenario.DemandID}
	record := func(name, output string) error {
		step, err := inspectOperatorStep(root, scenario.DemandID, name, output)
		result.Steps = append(result.Steps, step)
		return err
	}
	runStage := func(name string, stage demandflow.Stage, configure func(*demandflow.Options)) error {
		if err := record("before "+name, "operator inspected next action"); err != nil {
			return err
		}
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
		if err != nil {
			_ = record("failed "+name, err.Error())
			return fmt.Errorf("%s: %w", name, err)
		}
		return record(name, detail.Message)
	}
	confirm := func(stage, summary string) error {
		if err := record("before confirm "+stage, "operator reached human gate"); err != nil {
			return err
		}
		confirmation, err := demandflow.Confirm(demandflow.ConfirmOptions{
			Root:     root,
			DemandID: scenario.DemandID,
			Stage:    stage,
			By:       "devflow operator dogfood",
			Summary:  summary,
			Now:      opts.Now,
		})
		if err != nil {
			_ = record("failed confirm "+stage, err.Error())
			return fmt.Errorf("confirm %s: %w", stage, err)
		}
		result.Steps = append(result.Steps, OperatorStep{Name: "confirm " + stage, State: confirmation.CurrentState, Attention: "confirmed", Drive: "human_confirmation", Output: summary})
		return nil
	}

	if err := runStage("requirements", demandflow.StageRequirements, nil); err != nil {
		return result, err
	}
	if err := confirm("requirements", "operator dogfood requirements accepted"); err != nil {
		return result, err
	}
	if err := runStage("plan", demandflow.StagePlan, nil); err != nil {
		return result, err
	}
	if err := confirm("plan", "operator dogfood plan accepted"); err != nil {
		return result, err
	}
	if err := runStage("implementation", demandflow.StageImplementation, func(o *demandflow.Options) {
		o.MergeRequest = demandflow.MergeRequestOptions{Adapter: offlineMergeRequestAdapter{}, Spec: adapters.MergeRequestSpec{SourceBranch: "operator-dogfood/test", TargetBranch: "main", Title: "Operator dogfood MR sync"}}
		o.QualityCommands = opts.QualityCommands
	}); err != nil {
		return result, err
	}
	if err := runStage("mr-review", demandflow.StageMRReview, func(o *demandflow.Options) {
		o.Review = demandflow.ReviewOptions{Adapter: offlineReviewAdapter{}, Ref: adapters.ReviewRef{Project: "dogfood/offline", MergeRequest: "1"}}
	}); err != nil {
		return result, err
	}
	if err := runStage("verification", demandflow.StageVerification, func(o *demandflow.Options) {
		o.QualityCommands = opts.QualityCommands
	}); err != nil {
		return result, err
	}
	if err := store.AppendEvent(scenario.DemandID, artifacts.Event{
		Time:    opts.Now().UTC(),
		Type:    "verification.recorded",
		Message: "operator dogfood verification passed",
		Data: map[string]string{
			"status":        "PASS",
			"command":       operatorQualityCommandText(opts.QualityCommands),
			"evidence_file": artifacts.VerificationFile,
		},
	}); err != nil {
		return result, fmt.Errorf("record operator verification: %w", err)
	}
	if err := record("record verification", "operator dogfood recorded PASS verification evidence"); err != nil {
		return result, err
	}
	if err := store.AppendEvent(scenario.DemandID, artifacts.Event{
		Time:    opts.Now().UTC(),
		Type:    "verification.evidence_recorded",
		Message: "operator dogfood recorded manual acceptance evidence",
		Data: map[string]string{
			"status":        "pass",
			"type":          "manual",
			"criterion":     "Dogfood operator records acceptance evidence",
			"summary":       "Operator loop recorded manual evidence for the full dogfood workflow.",
			"by":            "operator",
			"evidence_file": artifacts.VerificationFile,
		},
	}); err != nil {
		return result, fmt.Errorf("record operator manual evidence: %w", err)
	}
	if err := confirm("verification", "operator dogfood verification accepted"); err != nil {
		return result, err
	}
	if err := runStage("closeout", demandflow.StageCloseout, nil); err != nil {
		return result, err
	}
	if err := confirm("closeout", "operator dogfood closeout accepted"); err != nil {
		return result, err
	}

	demand, err := store.LoadDemand(scenario.DemandID)
	if err != nil {
		return result, fmt.Errorf("load final demand: %w", err)
	}
	result.FinalState = workflow.State(demand.State)
	reportPath := filepath.Join(store.DemandDir(scenario.DemandID), "operator-dogfood-report.md")
	if err := os.WriteFile(reportPath, []byte(renderOperatorReport(result, store.DemandDir(scenario.DemandID))), 0o644); err != nil {
		return result, fmt.Errorf("write operator dogfood report: %w", err)
	}
	result.ReportPath = reportPath
	return result, nil
}

func operatorRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root != "" {
		return root, nil
	}
	temp, err := os.MkdirTemp("", "devflow-operator-dogfood-*")
	if err != nil {
		return "", fmt.Errorf("create operator dogfood root: %w", err)
	}
	return temp, nil
}

func operatorQualityRoot(root string) (string, error) {
	root = strings.TrimSpace(root)
	if root != "" {
		return root, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get quality root: %w", err)
	}
	return wd, nil
}

func inspectOperatorStep(root, demandID, name, output string) (OperatorStep, error) {
	summary, err := demandflow.InspectConsole(root, demandID)
	if err != nil {
		return OperatorStep{Name: name, Output: output}, err
	}
	decision := demandflow.DecideDriveStop(summary, 0, 5)
	evaluation, evalErr := demandflow.EvaluateDemand(root, demandID)
	evaluationText := "unavailable"
	if evalErr == nil {
		evaluationText = string(evaluation.Overall)
	}
	driveText := string(decision.Reason)
	if !decision.ShouldStop {
		driveText = "runnable:" + string(decision.Action.Stage)
	}
	return OperatorStep{
		Name:       name,
		State:      summary.Workspace.State,
		Attention:  summary.Workspace.Attention,
		Drive:      driveText,
		Evaluation: evaluationText,
		Output:     output,
	}, nil
}

func renderOperatorReport(result OperatorResult, demandDir string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Operator Dogfood Report: %s\n\n", result.DemandID)
	fmt.Fprintf(&builder, "Root: `%s`\n\n", result.Root)
	fmt.Fprintf(&builder, "QualityRoot: `%s`\n\n", result.QualityRoot)
	fmt.Fprintf(&builder, "FinalState: `%s`\n\n", result.FinalState)
	builder.WriteString("## Operator Steps\n\n")
	builder.WriteString("| Step | State | Attention | Drive | Evaluation |\n")
	builder.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, step := range result.Steps {
		fmt.Fprintf(&builder, "| %s | %s | %s | %s | %s |\n", escapeReportCell(step.Name), step.State, escapeReportCell(step.Attention), escapeReportCell(step.Drive), escapeReportCell(step.Evaluation))
	}
	builder.WriteString("\n## Workbench Snapshot\n\n")
	builder.WriteString("```text\n")
	builder.WriteString(renderOperatorWorkbenchSnapshot(result))
	builder.WriteString("```\n\n")
	builder.WriteString("## Artifacts\n\n")
	for _, name := range []string{artifacts.RequirementsFile, artifacts.PlanFile, artifacts.ProgressFile, artifacts.VerificationFile, artifacts.CloseoutFile, artifacts.MemoryCandidatesFile, artifacts.EventsFile} {
		fmt.Fprintf(&builder, "- `%s`\n", filepath.Join(demandDir, name))
	}
	return builder.String()
}

func renderOperatorWorkbenchSnapshot(result OperatorResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "workbench snapshot for %s\n", result.DemandID)
	for _, step := range result.Steps {
		fmt.Fprintf(&builder, "%s %s %s\n", step.Name, step.State, step.Attention)
	}
	return builder.String()
}

func operatorQualityCommandText(commands []quality.Command) string {
	var rendered []string
	for _, command := range commands {
		text := strings.TrimSpace(command.Name)
		if len(command.Args) > 0 {
			text = strings.TrimSpace(text + " " + strings.Join(command.Args, " "))
		}
		if text != "" {
			rendered = append(rendered, text)
		}
	}
	if len(rendered) == 0 {
		return "operator dogfood quality gate"
	}
	return strings.Join(rendered, "; ")
}

func escapeReportCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}

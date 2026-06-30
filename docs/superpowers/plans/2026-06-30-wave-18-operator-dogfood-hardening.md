# Wave 18 Operator Dogfood Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an operator-facing dogfood loop that validates `drive`, `evaluate`, and `workbench` behavior on a deterministic full demand flow.

**Architecture:** Keep the deterministic workflow runner in `internal/dogfood`, and keep operator display behavior in `internal/cli`. Add a read-only `workbench --snapshot` mode for CI and reports, then add `dogfood.RunOperator` to exercise console summaries, drive decisions, evaluation results, manual confirmations, and report generation without external APIs.

**Tech Stack:** Go, existing `internal/artifacts`, `internal/demandflow`, `internal/dogfood`, `internal/cli`, PowerShell release-readiness script, standard Go tests.

---

## File Structure

- Modify `internal/cli/workbench.go` to parse `--snapshot` and `--demand`.
- Modify `internal/cli/workbench_model.go` or create focused helpers in `internal/cli/workbench_snapshot.go` for snapshot rendering.
- Modify `internal/cli/workbench_test.go` for snapshot tests.
- Create `internal/dogfood/operator.go` for deterministic operator dogfood.
- Create `internal/dogfood/operator_test.go` for operator dogfood behavior.
- Modify `internal/cli/dogfood.go` to route `--operator-loop`.
- Modify `internal/cli/dogfood_test.go` for CLI dispatch.
- Modify `scripts/release-readiness.ps1` to run operator dogfood.
- Modify `docs/user-guide/backend-demand-loop.md` and `docs/release/v0.1.md`.

## Task 1: Add Workbench Snapshot Mode

**Files:**
- Modify: `internal/cli/workbench.go`
- Create: `internal/cli/workbench_snapshot.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Write failing snapshot list test**

Append to `internal/cli/workbench_test.go`:

```go
func TestWorkbenchSnapshotRendersDemandList(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "snapshot-demand", Title: "Snapshot demand", Description: "Snapshot", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Devflow Workbench Snapshot", "snapshot-demand", "verification"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot output missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Write failing selected-demand snapshot test**

Append to `internal/cli/workbench_test.go`:

```go
func TestWorkbenchSnapshotSelectedDemandShowsDetail(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "snapshot-detail", Title: "Snapshot detail", Description: "Snapshot", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Summary", "State:", "Attention:", "Quality:", "Next:", "Run-ready:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot detail missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 3: Write failing missing-demand test**

Append to `internal/cli/workbench_test.go`:

```go
func TestWorkbenchSnapshotMissingDemandReturnsError(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "known", Title: "Known", Source: "test", State: string(workflow.Created)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", "missing"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `demand "missing" not found`) {
		t.Fatalf("err = %v, want missing demand error", err)
	}
}
```

- [ ] **Step 4: Run tests and verify failure**

Run:

```powershell
go test ./internal/cli -run "TestWorkbenchSnapshot" -count=1
```

Expected: FAIL because `--snapshot` and `--demand` are not defined on `workbench`.

- [ ] **Step 5: Extend workbench options and parser**

Modify `internal/cli/workbench.go`:

```go
type workbenchOptions struct {
	root           string
	configPath     string
	noAltScreen    bool
	snapshot       bool
	demandID       string
	qualityCommand stringSliceFlag
}
```

Update `runWorkbench`:

```go
func runWorkbench(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseWorkbenchArgs(args, stderr)
	if err != nil {
		return err
	}
	if opts.snapshot {
		text, err := renderWorkbenchSnapshot(opts)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(stdout, text)
		return err
	}
	return runWorkbenchProgram(opts)
}
```

Add `fmt` import to `internal/cli/workbench.go`.

Update `parseWorkbenchArgs`:

```go
fs.BoolVar(&opts.snapshot, "snapshot", false, "render a non-interactive workbench snapshot")
fs.StringVar(&opts.demandID, "demand", "", "selected demand id for snapshot")
```

After parsing:

```go
opts.demandID = strings.TrimSpace(opts.demandID)
```

- [ ] **Step 6: Implement snapshot renderer**

Create `internal/cli/workbench_snapshot.go`:

```go
package cli

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func renderWorkbenchSnapshot(opts workbenchOptions) (string, error) {
	summaries, err := demandflow.ListConsole(opts.root)
	if err != nil {
		return "", err
	}
	selected := 0
	if opts.demandID != "" {
		selected = -1
		for index, summary := range summaries {
			if summary.Workspace.Demand.ID == opts.demandID {
				selected = index
				break
			}
		}
		if selected == -1 {
			return "", fmt.Errorf("demand %q not found", opts.demandID)
		}
	}

	var builder strings.Builder
	builder.WriteString("Devflow Workbench Snapshot\n\n")
	if len(summaries) == 0 {
		builder.WriteString("No demands found\n")
		return builder.String(), nil
	}
	for index, summary := range summaries {
		cursor := " "
		if index == selected {
			cursor = ">"
		}
		fmt.Fprintf(&builder, "%s %-24s %-22s %s\n", cursor, summary.Workspace.Demand.ID, summary.Workspace.State, summary.Workspace.Attention)
	}

	detail := summaries[selected]
	fmt.Fprintln(&builder, "\nSummary")
	fmt.Fprintf(&builder, "State: %s\n", detail.Workspace.State)
	fmt.Fprintf(&builder, "Attention: %s\n", detail.Workspace.Attention)
	fmt.Fprintln(&builder, "Quality:")
	evaluation, err := demandflow.EvaluateDemand(opts.root, detail.Workspace.Demand.ID)
	if err != nil {
		fmt.Fprintf(&builder, "  unavailable: %v\n", err)
	} else {
		for _, stage := range evaluation.Stages {
			fmt.Fprintf(&builder, "  %-14s %s", stage.Stage, stage.Status)
			if stage.Blockers > 0 || stage.Warnings > 0 {
				fmt.Fprintf(&builder, " blockers=%d warnings=%d", stage.Blockers, stage.Warnings)
			}
			fmt.Fprintln(&builder)
		}
	}
	fmt.Fprintln(&builder, "Next:")
	renderWorkbenchAction(&builder, detail.PrimaryAction)
	fmt.Fprintln(&builder, "Run-ready:")
	if detail.RunReadyAction.Runnable {
		renderWorkbenchAction(&builder, detail.RunReadyAction)
	} else {
		fmt.Fprintf(&builder, "  %s\n", detail.RunReadyAction.BlockReason)
	}
	return builder.String(), nil
}
```

- [ ] **Step 7: Run snapshot tests**

Run:

```powershell
gofmt -w internal/cli/workbench.go internal/cli/workbench_snapshot.go internal/cli/workbench_test.go
go test ./internal/cli -run "TestWorkbenchSnapshot" -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit snapshot mode**

Run:

```powershell
git add internal/cli/workbench.go internal/cli/workbench_snapshot.go internal/cli/workbench_test.go
git commit -m "Expose workbench snapshot mode" -m "Operator dogfood needs a non-interactive way to inspect the workbench view, so workbench can now render a read-only snapshot for CI, reports, and quick local checks." -m "Constraint: Snapshot mode must not execute workbench actions." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run \"TestWorkbenchSnapshot\" -count=1"
```

## Task 2: Add Operator Dogfood Runner

**Files:**
- Create: `internal/dogfood/operator.go`
- Create: `internal/dogfood/operator_test.go`

- [ ] **Step 1: Write failing operator dogfood test**

Create `internal/dogfood/operator_test.go`:

```go
package dogfood

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestRunOperatorCompletesAndWritesEvidence(t *testing.T) {
	root := t.TempDir()
	qualityRoot := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	result, err := RunOperator(context.Background(), OperatorOptions{
		Root:        root,
		QualityRoot: qualityRoot,
		QualityCommands: []quality.Command{{
			Name: testHelperExecutable(t),
			Args: []string{"-test.run=^TestDogfoodHelper$"},
		}},
		Now: fixedDogfoodNow,
	})
	if err != nil {
		t.Fatalf("RunOperator returned error: %v", err)
	}
	if result.FinalState != workflow.Completed {
		t.Fatalf("FinalState = %s, want completed", result.FinalState)
	}
	if result.ReportPath == "" {
		t.Fatal("ReportPath is empty")
	}
	report, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{"Operator Dogfood Report", "Drive", "Evaluation", "Workbench Snapshot", "human_confirmation", "completed"} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("report missing %q:\n%s", want, reportText)
		}
	}
	demandDir := filepath.Join(root, ".devflow", "demands", result.DemandID)
	for _, name := range []string{artifacts.RequirementsFile, artifacts.PlanFile, artifacts.VerificationFile, artifacts.CloseoutFile, "operator-dogfood-report.md"} {
		if _, err := os.Stat(filepath.Join(demandDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```powershell
go test ./internal/dogfood -run TestRunOperatorCompletesAndWritesEvidence -count=1
```

Expected: FAIL because `RunOperator` and `OperatorOptions` are undefined.

- [ ] **Step 3: Implement operator dogfood types and helpers**

Create `internal/dogfood/operator.go`:

```go
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
```

- [ ] **Step 4: Add root and inspection helpers**

Append to `internal/dogfood/operator.go`:

```go
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

func escapeReportCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}
```

- [ ] **Step 5: Run operator dogfood test**

Run:

```powershell
gofmt -w internal/dogfood/operator.go internal/dogfood/operator_test.go
go test ./internal/dogfood -run TestRunOperatorCompletesAndWritesEvidence -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit operator dogfood runner**

Run:

```powershell
git add internal/dogfood/operator.go internal/dogfood/operator_test.go
git commit -m "Add deterministic operator dogfood runner" -m "Wave 18 needs a dogfood loop that validates the operator-facing semantics around console summaries, drive decisions, stage evaluation, and manual gates without calling external services." -m "Constraint: Operator dogfood remains deterministic and offline." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/dogfood -run TestRunOperatorCompletesAndWritesEvidence -count=1"
```

## Task 3: Expose `devflow dogfood --operator-loop`

**Files:**
- Modify: `internal/cli/dogfood.go`
- Modify: `internal/cli/dogfood_test.go`

- [ ] **Step 1: Write failing CLI dispatch test**

Append to `internal/cli/dogfood_test.go`:

```go
func TestDogfoodOperatorLoopCompletes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	helper := testCLIExecutable(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"dogfood",
		"--operator-loop",
		"--root", root,
		"--quality-root", t.TempDir(),
		"--quality-command", `"` + helper + `" -test.run=^TestCLICommandHelper$`,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dogfood operator loop: %v\nstderr:\n%s", err, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"operator dogfood completed", "state: completed", "report:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```powershell
go test ./internal/cli -run TestDogfoodOperatorLoopCompletes -count=1
```

Expected: FAIL because `--operator-loop` is not defined.

- [ ] **Step 3: Add dogfood flag and route**

Modify `internal/cli/dogfood.go`.

Add flag variable:

```go
var operatorLoop bool
```

Register it:

```go
fs.BoolVar(&operatorLoop, "operator-loop", false, "run operator-facing dogfood through console, drive, evaluate, and workbench evidence")
```

Replace the single `dogfood.Run` call with:

```go
if operatorLoop {
	result, err := dogfood.RunOperator(context.Background(), dogfood.OperatorOptions{
		Root:            root,
		QualityRoot:     qualityRoot,
		ScenarioName:    scenario,
		QualityCommands: commands,
		Now:             time.Now,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "operator dogfood completed for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "state: %s\n", result.FinalState)
	fmt.Fprintf(stdout, "root: %s\n", result.Root)
	fmt.Fprintf(stdout, "quality-root: %s\n", result.QualityRoot)
	fmt.Fprintf(stdout, "report: %s\n", result.ReportPath)
	return nil
}
```

Keep the existing deterministic `dogfood.Run` path unchanged after this block.

- [ ] **Step 4: Run CLI dogfood tests**

Run:

```powershell
gofmt -w internal/cli/dogfood.go internal/cli/dogfood_test.go
go test ./internal/cli -run "TestDogfood" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit CLI route**

Run:

```powershell
git add internal/cli/dogfood.go internal/cli/dogfood_test.go
git commit -m "Expose operator dogfood from the CLI" -m "The dogfood command can now run the operator-facing loop while preserving the existing deterministic engine loop as the default." -m "Constraint: Operator dogfood must remain opt-in at the CLI command level." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run \"TestDogfood\" -count=1"
```

## Task 4: Add Operator Dogfood to Release Readiness

**Files:**
- Modify: `scripts/release-readiness.ps1`

- [ ] **Step 1: Add release-readiness assertion test by script scan**

There is no PowerShell unit test harness in this repo. Add a deterministic content check command to the task verification:

```powershell
rg -n "operator dogfood|--operator-loop" scripts\release-readiness.ps1
```

Expected before implementation: no match.

- [ ] **Step 2: Update release-readiness script**

In `scripts/release-readiness.ps1`, after deterministic dogfood:

```powershell
Invoke-Step "deterministic dogfood" { powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\dogfood-local.ps1') -Version $Version }
Invoke-Step "operator dogfood" { .\dist\devflow-windows-amd64.exe dogfood --operator-loop --root (Join-Path $readinessRoot 'operator-dogfood') --quality-root $repoRoot --quality-command "go test ./... -count=1 -timeout 5m" }
Invoke-Step "git diff check" { git diff --check }
```

- [ ] **Step 3: Verify script scan**

Run:

```powershell
rg -n "operator dogfood|--operator-loop" scripts\release-readiness.ps1
```

Expected: both strings appear in the operator dogfood invocation.

- [ ] **Step 4: Commit release readiness change**

Run:

```powershell
git add scripts/release-readiness.ps1
git commit -m "Run operator dogfood during release readiness" -m "Release readiness now verifies the operator-facing loop in addition to the deterministic engine dogfood, so drive, evaluate, and workbench evidence are exercised before release." -m "Constraint: Operator dogfood must run offline with no GitLab or live provider credentials." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: rg -n \"operator dogfood|--operator-loop\" scripts/release-readiness.ps1"
```

## Task 5: Document Wave 18

**Files:**
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Update user guide**

In `docs/user-guide/backend-demand-loop.md`, after the Workbench TUI section, add:

````markdown
### Operator dogfood

Use operator dogfood before relying on the workflow for real delivery. It runs the deterministic backend-demand loop while collecting console, drive, evaluate, and workbench evidence.

```powershell
devflow dogfood --operator-loop
```

The command writes `operator-dogfood-report.md` under the generated demand directory. The report is the quickest way to inspect whether the operator-facing loop is still coherent after changes.
````

- [ ] **Step 2: Update release notes**

In `docs/release/v0.1.md`, after Wave 17, add:

```markdown
### Wave 18 - Operator Dogfood Hardening

- Adds `devflow dogfood --operator-loop` for offline operator-facing dogfood.
- Adds `devflow workbench --snapshot` for non-interactive workbench evidence.
- Runs operator dogfood during release readiness.
```

- [ ] **Step 3: Verify docs contain commands**

Run:

```powershell
rg -n "operator-loop|workbench --snapshot|Operator Dogfood" docs\user-guide\backend-demand-loop.md docs\release\v0.1.md
```

Expected: all three concepts are present.

- [ ] **Step 4: Commit docs**

Run:

```powershell
git add docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document operator dogfood hardening" -m "Wave 18 adds an operator-facing dogfood loop and workbench snapshot mode, so the docs now explain how to run and inspect them." -m "Constraint: Documentation must keep live provider and GitLab gates opt-in." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: rg -n \"operator-loop|workbench --snapshot|Operator Dogfood\" docs/user-guide/backend-demand-loop.md docs/release/v0.1.md"
```

## Task 6: Full Verification and PR

**Files:**
- All modified files from Tasks 1-5.

- [ ] **Step 1: Run focused package tests**

Run:

```powershell
go test ./internal/cli ./internal/dogfood -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 3: Run operator dogfood manually**

Run:

```powershell
go run ./cmd/devflow dogfood --operator-loop --quality-root . --quality-command "go test ./internal/dogfood -count=1"
```

Expected output includes:

```text
operator dogfood completed
state: completed
report:
```

- [ ] **Step 4: Inspect worktree**

Run:

```powershell
git status --short --branch
git log --oneline -8
```

Expected: branch contains Wave 18 commits and no unstaged changes.

- [ ] **Step 5: Push and open PR**

Run:

```powershell
git push -u origin <wave-18-branch>
gh pr create --base main --head <wave-18-branch> --title "Wave 18 operator dogfood hardening" --body "## Summary

- Add workbench snapshot mode.
- Add offline operator dogfood loop.
- Run operator dogfood during release readiness.

## Verification

- go test ./internal/cli ./internal/dogfood -count=1
- go test ./... -count=1 -timeout 5m
- go vet ./...
- go build ./cmd/devflow
- git diff --check
- go run ./cmd/devflow dogfood --operator-loop --quality-root . --quality-command \"go test ./internal/dogfood -count=1\""
```

- [ ] **Step 6: Wait for CI**

Run:

```powershell
gh pr checks --watch --fail-fast
```

Expected: Ubuntu and Windows Go verification pass.

## Acceptance Checklist

- `devflow workbench --snapshot` prints a non-interactive workbench view.
- `devflow workbench --snapshot --demand <id>` prints selected-demand detail.
- `devflow dogfood --operator-loop` completes to `completed`.
- Operator dogfood report includes drive, evaluation, and workbench evidence.
- Release readiness includes operator dogfood.
- Full local verification passes.
- PR CI passes.

# Wave 15 Guided Autopilot Drive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `devflow drive --demand <id>` to repeatedly execute runner-safe stages until the next manual gate or failure.

**Architecture:** Reuse Wave 14 `ConsoleSummary` and `ConsoleAction` as the safety model. Add deterministic drive decisions in `internal/demandflow`, then implement the CLI loop in `internal/cli` by reusing the existing console runner helper and reloading state after every step.

**Tech Stack:** Go standard library, existing `internal/demandflow`, existing `internal/cli`, existing artifact event/progress files, PowerShell verification commands.

---

## File Structure

- Create `internal/demandflow/drive.go` for `DriveReport`, `DriveStep`, `DriveStopReason`, and `DecideDriveStop`.
- Create `internal/demandflow/drive_test.go` for stop-reason tests.
- Create `internal/cli/drive.go` for command parsing, dry-run, execution loop, report rendering, and progress evidence.
- Create `internal/cli/drive_test.go` for CLI behavior with runner stubs.
- Modify `internal/cli/cli.go` to add help and dispatch.
- Modify `docs/user-guide/backend-demand-loop.md` and `docs/release/v0.1.md`.

## Task 1: Demandflow Drive Decisions

**Files:**
- Create: `internal/demandflow/drive.go`
- Create: `internal/demandflow/drive_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/demandflow/drive_test.go`:

```go
package demandflow

import (
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestDecideDriveStopForManualGates(t *testing.T) {
	cases := []struct {
		name string
		action ConsoleAction
		want DriveStopReason
	}{
		{"human confirmation", ConsoleAction{Kind: ConsoleActionHumanConfirmation, Label: "Confirm verification"}, DriveStopHumanConfirmation},
		{"memory review", ConsoleAction{Kind: ConsoleActionMemoryReview, Label: "Review memory"}, DriveStopMemoryGate},
		{"memory decision", ConsoleAction{Kind: ConsoleActionMemoryDecision, Label: "Promote memory"}, DriveStopMemoryGate},
		{"mr flags", ConsoleAction{Kind: ConsoleActionMRReview, Label: "Check MR review"}, DriveStopMRFlagsRequired},
		{"manual", ConsoleAction{Kind: ConsoleActionManual, Label: "Inspect manually"}, DriveStopManualAction},
		{"none completed", ConsoleAction{Kind: ConsoleActionNone, Label: "No action"}, DriveStopComplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := ConsoleSummary{Workspace: WorkspaceSummary{Demand: artifacts.Demand{ID: "drive"}, State: workflow.Completed}, PrimaryAction: tc.action}
			got := DecideDriveStop(summary, 0, 5)
			if got.Reason != tc.want {
				t.Fatalf("Reason = %s, want %s", got.Reason, tc.want)
			}
		})
	}
}

func TestDecideDriveStopAllowsRunnableAction(t *testing.T) {
	summary := ConsoleSummary{
		Workspace: WorkspaceSummary{Demand: artifacts.Demand{ID: "drive"}, State: workflow.Created},
		PrimaryAction: ConsoleAction{Kind: ConsoleActionAgentStage, Stage: StageRequirements, Runnable: true, Label: "Draft requirements"},
	}
	got := DecideDriveStop(summary, 0, 5)
	if got.ShouldStop {
		t.Fatalf("ShouldStop = true, want false: %#v", got)
	}
}

func TestDecideDriveStopMaxSteps(t *testing.T) {
	summary := ConsoleSummary{
		Workspace: WorkspaceSummary{Demand: artifacts.Demand{ID: "drive"}, State: workflow.Implementation},
		PrimaryAction: ConsoleAction{Kind: ConsoleActionAgentStage, Stage: StageImplementation, Runnable: true, Label: "Run implementation"},
	}
	got := DecideDriveStop(summary, 5, 5)
	if !got.ShouldStop || got.Reason != DriveStopMaxStepsReached {
		t.Fatalf("decision = %#v, want max steps stop", got)
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/demandflow -run TestDecideDriveStop -count=1
```

Expected: compile failure for missing drive types.

- [ ] **Step 3: Implement drive model**

Create `internal/demandflow/drive.go`:

```go
package demandflow

import "github.com/jesseedcp/devflow-agent/internal/workflow"

type DriveStopReason string

const (
	DriveStopNone              DriveStopReason = ""
	DriveStopHumanConfirmation DriveStopReason = "human_confirmation"
	DriveStopMemoryGate        DriveStopReason = "memory_gate"
	DriveStopMRFlagsRequired   DriveStopReason = "mr_flags_required"
	DriveStopRunnerFailed      DriveStopReason = "runner_failed"
	DriveStopMaxStepsReached   DriveStopReason = "max_steps_reached"
	DriveStopComplete          DriveStopReason = "complete"
	DriveStopManualAction      DriveStopReason = "manual_action"
)

type DriveStep struct {
	Number        int
	Action        ConsoleAction
	PreviousState workflow.State
	CurrentState  workflow.State
	Message       string
}

type DriveReport struct {
	DemandID    string
	Steps       []DriveStep
	StopReason DriveStopReason
	StopAction ConsoleAction
	Error       string
}

type DriveDecision struct {
	ShouldStop bool
	Reason     DriveStopReason
	Action     ConsoleAction
}

func DecideDriveStop(summary ConsoleSummary, stepsCompleted int, maxSteps int) DriveDecision {
	action := summary.PrimaryAction
	if maxSteps > 0 && stepsCompleted >= maxSteps {
		return DriveDecision{ShouldStop: true, Reason: DriveStopMaxStepsReached, Action: action}
	}
	if action.Runnable {
		return DriveDecision{ShouldStop: false, Action: action}
	}
	switch action.Kind {
	case ConsoleActionHumanConfirmation:
		return DriveDecision{ShouldStop: true, Reason: DriveStopHumanConfirmation, Action: action}
	case ConsoleActionMemoryReview, ConsoleActionMemoryDecision:
		return DriveDecision{ShouldStop: true, Reason: DriveStopMemoryGate, Action: action}
	case ConsoleActionMRReview:
		return DriveDecision{ShouldStop: true, Reason: DriveStopMRFlagsRequired, Action: action}
	case ConsoleActionNone:
		return DriveDecision{ShouldStop: true, Reason: DriveStopComplete, Action: action}
	default:
		return DriveDecision{ShouldStop: true, Reason: DriveStopManualAction, Action: action}
	}
}
```

- [ ] **Step 4: Verify and commit**

Run:

```powershell
gofmt -w internal/demandflow/drive.go internal/demandflow/drive_test.go
go test ./internal/demandflow -run TestDecideDriveStop -count=1
go test ./internal/demandflow -count=1
git add internal/demandflow/drive.go internal/demandflow/drive_test.go
git commit -m "Model guided drive stop decisions" -m "Wave 15 needs deterministic stop reasons before CLI code can loop runner-safe actions. This adds a drive report and decision model on top of ConsoleSummary." -m "Constraint: Demandflow drive decisions are read-only and do not execute stages." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -count=1"
```

## Task 2: Add `devflow drive --dry-run`

**Files:**
- Create: `internal/cli/drive.go`
- Create: `internal/cli/drive_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write failing CLI tests**

Create `internal/cli/drive_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestDriveDryRunPrintsFirstSafeAction(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-dry", Title: "Drive dry", Description: "Dry run", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"drive", "--root", root, "--demand", demand.ID, "--dry-run"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive dry-run returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Drive: drive-dry", "dry-run", "Draft requirements", "devflow run --demand drive-dry --stage requirements"} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, got)
		}
	}
}

func TestDriveRequiresDemand(t *testing.T) {
	err := Run([]string{"drive"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v, want --demand is required", err)
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/cli -run TestDrive -count=1
```

Expected: unknown command `drive`.

- [ ] **Step 3: Add CLI help and dispatch**

In `internal/cli/cli.go`, add usage:

```text
  devflow drive --demand <id> [--dry-run]
```

Add command description:

```text
  drive    Run safe demand stages until the next manual gate
```

Add dispatch:

```go
case "drive":
	return runDrive(args[1:], stdout, stderr)
```

- [ ] **Step 4: Implement dry-run**

Create `internal/cli/drive.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

type driveArgs struct {
	root           string
	demandID       string
	maxSteps       int
	dryRun         bool
	runnerRoot     string
	qualityRoot    string
	configPath     string
	permissionMode string
	gitlabProject  string
	gitlabMR       string
	gitlabBaseURL  string
	qualityCommand stringSliceFlag
}

func runDrive(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDriveArgs(args, stderr)
	if err != nil {
		return err
	}
	summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Drive: %s\n\n", opts.demandID)
	if opts.dryRun {
		fmt.Fprintln(stdout, "dry-run")
		printDriveAction(stdout, summary.PrimaryAction)
		return nil
	}
	return runDriveLoop(opts, stdout, stderr)
}

func parseDriveArgs(args []string, stderr io.Writer) (driveArgs, error) {
	fs := flag.NewFlagSet("drive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts driveArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.IntVar(&opts.maxSteps, "max-steps", 5, "maximum runner-safe steps")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "show planned next action without executing")
	fs.StringVar(&opts.runnerRoot, "runner-root", "", "working directory for agent tools; defaults to --root")
	fs.StringVar(&opts.qualityRoot, "quality-root", "", "working directory for quality commands; defaults to --root")
	fs.StringVar(&opts.configPath, "config", "", "devflow config path")
	fs.StringVar(&opts.permissionMode, "permission-mode", "", "permission mode for implementation")
	fs.StringVar(&opts.gitlabProject, "gitlab-project", "", "gitlab project path for mr-review")
	fs.StringVar(&opts.gitlabMR, "gitlab-mr", "", "gitlab merge request iid for mr-review")
	fs.StringVar(&opts.gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.Var(&opts.qualityCommand, "quality-command", "quality command as a quoted program and args (repeatable)")
	if err := fs.Parse(args); err != nil {
		return driveArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.demandID == "" {
		return driveArgs{}, fmt.Errorf("--demand is required")
	}
	if opts.maxSteps < 1 || opts.maxSteps > 20 {
		return driveArgs{}, fmt.Errorf("--max-steps must be between 1 and 20")
	}
	return opts, nil
}

func printDriveAction(stdout io.Writer, action demandflow.ConsoleAction) {
	if action.Label != "" {
		fmt.Fprintf(stdout, "action: %s\n", action.Label)
	}
	if action.Command != "" {
		fmt.Fprintf(stdout, "command: %s\n", action.Command)
	}
	if action.BlockReason != "" && !action.Runnable {
		fmt.Fprintf(stdout, "blocked: %s\n", action.BlockReason)
	}
}

func runDriveLoop(opts driveArgs, stdout io.Writer, stderr io.Writer) error {
	return fmt.Errorf("drive execution loop is not wired")
}
```

- [ ] **Step 5: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/drive.go internal/cli/drive_test.go internal/cli/cli.go
go test ./internal/cli -run TestDrive -count=1
git add internal/cli/drive.go internal/cli/drive_test.go internal/cli/cli.go
git commit -m "Add drive dry-run command" -m "The drive command starts with a read-only dry-run mode so users can inspect the next safe action before the execution loop is wired." -m "Constraint: Dry-run must not mutate demand artifacts." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run TestDrive -count=1"
```

## Task 3: Implement Drive Loop

**Files:**
- Modify: `internal/cli/drive.go`
- Modify: `internal/cli/drive_test.go`

- [ ] **Step 1: Add failing loop tests**

Append:

```go
func TestDriveRunsUntilHumanGate(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-loop", Title: "Drive loop", Description: "Loop", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var calls int
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		calls++
		return store.UpdateState(demand.ID, string(workflow.RequirementsReview))
	}

	var stdout bytes.Buffer
	if err := Run([]string{"drive", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("runner calls = %d, want 1", calls)
	}
	if !strings.Contains(stdout.String(), "reason: human_confirmation") {
		t.Fatalf("drive output missing human stop:\n%s", stdout.String())
	}
}

func TestDriveStopsOnRunnerError(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-error", Title: "Drive error", Description: "Error", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		return fmt.Errorf("quality gate failed")
	}

	var stdout bytes.Buffer
	err := Run([]string{"drive", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "quality gate failed") {
		t.Fatalf("err = %v, want quality gate failed", err)
	}
	if !strings.Contains(stdout.String(), "reason: runner_failed") {
		t.Fatalf("drive output missing runner_failed:\n%s", stdout.String())
	}
}
```

Imports must include `fmt` and `io`.

- [ ] **Step 2: Run failing loop tests**

Run:

```powershell
go test ./internal/cli -run "TestDriveRunsUntilHumanGate|TestDriveStopsOnRunnerError" -count=1
```

Expected: `drive execution loop is not wired`.

- [ ] **Step 3: Implement loop**

Replace `runDriveLoop`:

```go
func runDriveLoop(opts driveArgs, stdout io.Writer, stderr io.Writer) error {
	for step := 0; step < opts.maxSteps; step++ {
		summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
		if err != nil {
			return err
		}
		decision := demandflow.DecideDriveStop(summary, step, opts.maxSteps)
		if decision.ShouldStop {
			printDriveStop(stdout, decision)
			return nil
		}
		fmt.Fprintf(stdout, "Step %d\n", step+1)
		printDriveAction(stdout, decision.Action)
		consoleOpts := consoleArgs{
			root:           opts.root,
			demandID:       opts.demandID,
			runnerRoot:     opts.runnerRoot,
			qualityRoot:    opts.qualityRoot,
			configPath:     opts.configPath,
			permissionMode: opts.permissionMode,
			gitlabProject:  opts.gitlabProject,
			gitlabMR:       opts.gitlabMR,
			gitlabBaseURL:  opts.gitlabBaseURL,
			qualityCommand: opts.qualityCommand,
		}
		action := consoleRunnableAction(consoleOpts, decision.Action)
		if !action.Runnable {
			printDriveStop(stdout, demandflow.DriveDecision{ShouldStop: true, Reason: demandflow.DriveStopMRFlagsRequired, Action: action})
			return nil
		}
		if err := runConsoleStageAction(consoleOpts, action, stdout, stderr); err != nil {
			printDriveStop(stdout, demandflow.DriveDecision{ShouldStop: true, Reason: demandflow.DriveStopRunnerFailed, Action: action})
			return err
		}
	}
	summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	printDriveStop(stdout, demandflow.DriveDecision{ShouldStop: true, Reason: demandflow.DriveStopMaxStepsReached, Action: summary.PrimaryAction})
	return nil
}

func printDriveStop(stdout io.Writer, decision demandflow.DriveDecision) {
	fmt.Fprintln(stdout, "\nStopped")
	fmt.Fprintf(stdout, "reason: %s\n", decision.Reason)
	if decision.Action.Label != "" {
		fmt.Fprintf(stdout, "next: %s\n", decision.Action.Label)
	}
	if decision.Action.Command != "" {
		fmt.Fprintf(stdout, "command: %s\n", decision.Action.Command)
	}
}
```

- [ ] **Step 4: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/drive.go internal/cli/drive_test.go
go test ./internal/cli -run TestDrive -count=1
go test ./internal/cli -count=1
git add internal/cli/drive.go internal/cli/drive_test.go
git commit -m "Drive demands until the next manual gate" -m "The drive loop now repeatedly executes runner-safe console actions, reloads demand state after each step, and stops with explicit reasons for manual gates, runner failures, and max-step protection." -m "Constraint: Drive reuses console runner safety checks and does not execute arbitrary command strings." -m "Rejected: Auto-confirm manual gates | human approval remains a product boundary." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -count=1"
```

## Task 4: Document And Verify

**Files:**
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Document drive**

Add:

```markdown
### Guided drive

Use `devflow drive` to run safe agent stages until the next manual gate.

```powershell
devflow drive --demand add-coupon-check
devflow drive --demand add-coupon-check --dry-run
```

Drive never confirms stages, promotes memory, rejects memory, or merges MRs. It stops with an explicit reason when the next step needs a human.
```

- [ ] **Step 2: Release note**

Add:

```markdown
### Wave 15 - Guided Autopilot Drive

- Adds `devflow drive --demand <id>` to execute runner-safe stages until a manual gate.
- Adds `--dry-run` and max-step protection.
- Keeps human confirmation and memory decisions manual.
```

- [ ] **Step 3: Full verification**

Run:

```powershell
gofmt -w internal/demandflow/drive.go internal/demandflow/drive_test.go internal/cli/drive.go internal/cli/drive_test.go internal/cli/cli.go
go test ./internal/demandflow ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

- [ ] **Step 4: Commit docs**

Run:

```powershell
git add docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document guided demand drive" -m "Wave 15 adds a guided autopilot command, so the user guide and release notes now explain dry-run, safe execution, and manual stop boundaries." -m "Constraint: Documentation must state that drive never performs human confirmation or memory decisions." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check"
```

## Final Checklist

- `devflow drive --demand <id>` exists.
- `--dry-run` does not mutate artifacts.
- loop stops at human confirmation.
- loop stops on memory gate.
- loop stops on runner error.
- loop enforces max steps.
- full verification passes.

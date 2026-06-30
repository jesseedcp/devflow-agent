# Wave 14 Operator Console Agent Runner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `devflow console` as an operator-focused demand workbench and `devflow console --demand <id> --run-next` as a safe entry into existing agent runner stages.

**Architecture:** Keep Wave 13 `WorkspaceSummary` as the evidence source, add a thin `ConsoleSummary`/`ConsoleAction` layer in `internal/demandflow`, then render and optionally execute safe actions from `internal/cli`. Execution must reuse the existing `runDemandStage` path; console must not bypass human confirmation, memory decisions, or GitLab flag requirements.

**Tech Stack:** Go standard library, existing `internal/demandflow`, existing `internal/cli`, existing `internal/workflow`, existing test stubs, PowerShell verification commands.

---

## Current Baseline

Wave 13 is merged and fixed on `main`. The relevant code is:

- `internal/demandflow/workspace.go`
  - `WorkspaceSummary`
  - `ListWorkspaces(root)`
  - `InspectWorkspace(root, demandID)`
  - `WorkspaceNextActions(summary)`
- `internal/demandflow/status.go`
  - compatibility wrapper `InspectStatus`
  - state fallback `NextActions`
- `internal/cli/status.go`
  - `runStatus`
  - `runNext`
  - detail and list rendering
- `internal/cli/run.go`
  - `runDemandStage(args, stdout, stderr)`
  - `newDemandRunner`, `newReviewAdapter`, `newMergeRequestAdapter` stubs
- `internal/cli/cli.go`
  - command dispatch and help text

Wave 14 must not change workflow state rules. It only adds an operator command and a typed action layer around existing summaries and runner execution.

## File Structure

Create or modify these files:

- Create `internal/demandflow/console.go`
  - Defines `ConsoleSummary`, `ConsoleAction`, `ConsoleActionKind`, `InspectConsole`, `ListConsole`, and `BuildConsoleAction`.
- Create `internal/demandflow/console_test.go`
  - Tests action classification, run-ready behavior, memory/human gates, and list ordering.
- Create `internal/cli/console.go`
  - Parses `devflow console` flags, renders list/detail, and dispatches safe `--run-next` actions.
- Create `internal/cli/console_test.go`
  - Tests console list/detail output and safe execution/refusal behavior with a runner stub.
- Modify `internal/cli/cli.go`
  - Add help text and dispatch for `console`.
- Modify `docs/user-guide/backend-demand-loop.md`
  - Document console usage after the status section.
- Modify `docs/release/v0.1.md`
  - Add Wave 14 release note.

## Design Contract

Action kinds are stable strings:

```go
const (
	ConsoleActionNone              ConsoleActionKind = "none"
	ConsoleActionAgentStage        ConsoleActionKind = "agent_stage"
	ConsoleActionHumanConfirmation ConsoleActionKind = "human_confirmation"
	ConsoleActionMemoryReview      ConsoleActionKind = "memory_review"
	ConsoleActionMemoryDecision    ConsoleActionKind = "memory_decision"
	ConsoleActionMRReview          ConsoleActionKind = "mr_review"
	ConsoleActionManual            ConsoleActionKind = "manual"
)
```

Runnable means console may execute the action without bypassing a human gate:

- `agent_stage`: runnable for requirements, plan, implementation, verification, closeout.
- `mr_review`: not runnable from demandflow alone; CLI can run it only when `--gitlab-project` and `--gitlab-mr` are provided.
- `human_confirmation`: not runnable.
- `memory_review`: not runnable.
- `memory_decision`: not runnable.
- `manual`: not runnable.
- `none`: not runnable.

## Task 1: Add Demandflow Console Model

**Files:**
- Create: `internal/demandflow/console.go`
- Create: `internal/demandflow/console_test.go`

- [ ] **Step 1: Write failing console model tests**

Create `internal/demandflow/console_test.go`:

```go
package demandflow

import (
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestInspectConsoleRequirementsIsRunReady(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-req", Title: "Console req", Description: "Draft requirements", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	console, err := InspectConsole(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectConsole returned error: %v", err)
	}
	if console.PrimaryAction.Kind != ConsoleActionAgentStage {
		t.Fatalf("PrimaryAction.Kind = %q, want %q", console.PrimaryAction.Kind, ConsoleActionAgentStage)
	}
	if console.PrimaryAction.Stage != StageRequirements {
		t.Fatalf("PrimaryAction.Stage = %q, want %q", console.PrimaryAction.Stage, StageRequirements)
	}
	if !console.PrimaryAction.Runnable {
		t.Fatalf("PrimaryAction should be runnable: %#v", console.PrimaryAction)
	}
	if console.RunReadyAction.Label != console.PrimaryAction.Label {
		t.Fatalf("RunReadyAction = %#v, want primary action", console.RunReadyAction)
	}
}

func TestInspectConsoleVerificationPassRequiresHumanConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-pass", Title: "Console pass", Description: "Confirm verification", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: fixedConsoleTime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	console, err := InspectConsole(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectConsole returned error: %v", err)
	}
	if console.PrimaryAction.Kind != ConsoleActionHumanConfirmation {
		t.Fatalf("PrimaryAction.Kind = %q, want human confirmation", console.PrimaryAction.Kind)
	}
	if console.PrimaryAction.Runnable {
		t.Fatalf("human confirmation must not be runnable: %#v", console.PrimaryAction)
	}
	if console.PrimaryAction.BlockReason == "" {
		t.Fatalf("human confirmation action needs block reason: %#v", console.PrimaryAction)
	}
	if console.RunReadyAction.Kind != ConsoleActionNone {
		t.Fatalf("RunReadyAction = %#v, want none", console.RunReadyAction)
	}
}

func TestInspectConsoleMemoryPendingIsManual(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-memory", Title: "Console memory", Description: "Review memory", Source: "test", State: string(workflow.Completed)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.MemoryCandidatesFile, "# Memory Candidates: Console memory\n\n## 稳定知识候选\n\n- Reuse operator loop\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	console, err := InspectConsole(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectConsole returned error: %v", err)
	}
	if console.PrimaryAction.Kind != ConsoleActionMemoryReview {
		t.Fatalf("PrimaryAction.Kind = %q, want memory review", console.PrimaryAction.Kind)
	}
	if console.PrimaryAction.Runnable {
		t.Fatalf("memory review must not be runnable: %#v", console.PrimaryAction)
	}
	if console.RunReadyAction.Kind != ConsoleActionNone {
		t.Fatalf("RunReadyAction = %#v, want none", console.RunReadyAction)
	}
}

func TestListConsolePreservesWorkspacePriority(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	for _, demand := range []artifacts.Demand{
		{ID: "z-complete", Title: "Z complete", Description: "Done", Source: "test", State: string(workflow.Completed)},
		{ID: "a-failed", Title: "A failed", Description: "Failed", Source: "test", State: string(workflow.FailedQualityGate)},
		{ID: "b-verify", Title: "B verify", Description: "Verify", Source: "test", State: string(workflow.Verification)},
	} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
		}
	}

	summaries, err := ListConsole(root)
	if err != nil {
		t.Fatalf("ListConsole returned error: %v", err)
	}
	got := []string{summaries[0].Workspace.Demand.ID, summaries[1].Workspace.Demand.ID, summaries[2].Workspace.Demand.ID}
	want := []string{"a-failed", "b-verify", "z-complete"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %#v, want %#v", got, want)
		}
	}
}

func TestBuildConsoleActionClassifiesKnownCommands(t *testing.T) {
	summary := WorkspaceSummary{Demand: artifacts.Demand{ID: "known"}, State: workflow.Implementation}
	cases := []struct {
		name     string
		action   NextAction
		kind     ConsoleActionKind
		stage    Stage
		runnable bool
	}{
		{
			name:     "implementation stage",
			action:   NextAction{Label: "Run implementation", Command: `devflow run --demand known --stage implementation --permission-mode acceptEdits --quality-command "go test ./..."`},
			kind:     ConsoleActionAgentStage,
			stage:    StageImplementation,
			runnable: true,
		},
		{
			name:     "human confirmation",
			action:   NextAction{Label: "Confirm plan", Command: "devflow confirm --demand known --stage plan --by <name> --summary <summary>"},
			kind:     ConsoleActionHumanConfirmation,
			runnable: false,
		},
		{
			name:     "memory promote",
			action:   NextAction{Label: "Promote memory candidate", Command: "devflow memory promote --demand known --candidate <index> --by <name>"},
			kind:     ConsoleActionMemoryDecision,
			runnable: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildConsoleAction(summary, tc.action)
			if got.Kind != tc.kind || got.Stage != tc.stage || got.Runnable != tc.runnable {
				t.Fatalf("BuildConsoleAction = %#v, want kind=%s stage=%s runnable=%v", got, tc.kind, tc.stage, tc.runnable)
			}
		})
	}
}

func fixedConsoleTime() time.Time {
	return time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
}
```

- [ ] **Step 2: Run the failing demandflow tests**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectConsole|TestListConsole|TestBuildConsoleAction" -count=1
```

Expected result:

```text
FAIL
undefined: InspectConsole
undefined: ConsoleActionAgentStage
```

- [ ] **Step 3: Implement the console model**

Create `internal/demandflow/console.go`:

```go
package demandflow

import (
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ConsoleActionKind string

const (
	ConsoleActionNone              ConsoleActionKind = "none"
	ConsoleActionAgentStage        ConsoleActionKind = "agent_stage"
	ConsoleActionHumanConfirmation ConsoleActionKind = "human_confirmation"
	ConsoleActionMemoryReview      ConsoleActionKind = "memory_review"
	ConsoleActionMemoryDecision    ConsoleActionKind = "memory_decision"
	ConsoleActionMRReview          ConsoleActionKind = "mr_review"
	ConsoleActionManual            ConsoleActionKind = "manual"
)

type ConsoleSummary struct {
	Workspace      WorkspaceSummary
	PrimaryAction  ConsoleAction
	RunReadyAction ConsoleAction
}

type ConsoleAction struct {
	Label       string
	Command     string
	Reason      string
	Kind        ConsoleActionKind
	Stage       Stage
	Runnable    bool
	BlockReason string
}

func InspectConsole(root, demandID string) (ConsoleSummary, error) {
	workspace, err := InspectWorkspace(root, demandID)
	if err != nil {
		return ConsoleSummary{}, err
	}
	return buildConsoleSummary(workspace), nil
}

func ListConsole(root string) ([]ConsoleSummary, error) {
	workspaces, err := ListWorkspaces(root)
	if err != nil {
		return nil, err
	}
	out := make([]ConsoleSummary, 0, len(workspaces))
	for _, workspace := range workspaces {
		out = append(out, buildConsoleSummary(workspace))
	}
	return out, nil
}

func buildConsoleSummary(workspace WorkspaceSummary) ConsoleSummary {
	primary := ConsoleAction{Kind: ConsoleActionNone, BlockReason: "no recommended action"}
	if len(workspace.Actions) > 0 {
		primary = BuildConsoleAction(workspace, workspace.Actions[0])
	}
	runReady := ConsoleAction{Kind: ConsoleActionNone, BlockReason: "no safe runner action"}
	if primary.Runnable {
		runReady = primary
	}
	return ConsoleSummary{
		Workspace:      workspace,
		PrimaryAction:  primary,
		RunReadyAction: runReady,
	}
}

func BuildConsoleAction(summary WorkspaceSummary, action NextAction) ConsoleAction {
	out := ConsoleAction{
		Label:       action.Label,
		Command:     action.Command,
		Reason:      action.Reason,
		Kind:        ConsoleActionManual,
		BlockReason: "manual action",
	}
	command := strings.TrimSpace(action.Command)
	if command == "" {
		out.Kind = ConsoleActionNone
		out.BlockReason = "no command available"
		return out
	}
	if strings.HasPrefix(command, "devflow confirm ") {
		out.Kind = ConsoleActionHumanConfirmation
		out.BlockReason = "human confirmation is required"
		return out
	}
	if strings.HasPrefix(command, "devflow memory list ") {
		out.Kind = ConsoleActionMemoryReview
		out.BlockReason = "memory review is a manual gate"
		return out
	}
	if strings.HasPrefix(command, "devflow memory promote ") || strings.HasPrefix(command, "devflow memory reject ") {
		out.Kind = ConsoleActionMemoryDecision
		out.BlockReason = "memory decisions require a human"
		return out
	}
	if strings.HasPrefix(command, "devflow run ") {
		stage, ok := stageFromDevflowRunCommand(command)
		if !ok {
			out.Kind = ConsoleActionManual
			out.BlockReason = "run command does not include a recognized stage"
			return out
		}
		out.Stage = stage
		if stage == StageMRReview {
			out.Kind = ConsoleActionMRReview
			out.BlockReason = "MR review requires explicit GitLab flags"
			return out
		}
		if isConsoleRunnableStage(stage) {
			out.Kind = ConsoleActionAgentStage
			out.Runnable = true
			out.BlockReason = ""
			return out
		}
	}
	out.Kind = ConsoleActionManual
	out.BlockReason = "manual action"
	if summary.State == workflow.Completed {
		out.Kind = ConsoleActionNone
		out.BlockReason = "demand is complete"
	}
	return out
}

func isConsoleRunnableStage(stage Stage) bool {
	switch stage {
	case StageRequirements, StagePlan, StageImplementation, StageVerification, StageCloseout:
		return true
	default:
		return false
	}
}

func stageFromDevflowRunCommand(command string) (Stage, bool) {
	fields := strings.Fields(command)
	for index := 0; index < len(fields)-1; index++ {
		if fields[index] != "--stage" {
			continue
		}
		stage, err := ParseStage(fields[index+1])
		return stage, err == nil
	}
	return "", false
}
```

- [ ] **Step 4: Run demandflow console tests**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectConsole|TestListConsole|TestBuildConsoleAction" -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] **Step 5: Run all demandflow tests**

Run:

```powershell
go test ./internal/demandflow -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] **Step 6: Commit Task 1**

Run:

```powershell
git add internal/demandflow/console.go internal/demandflow/console_test.go
git commit -m "Classify console actions for Wave 14" -m "The operator console needs typed action semantics instead of executing copied command strings. This adds ConsoleSummary and ConsoleAction on top of Wave 13 WorkspaceSummary so CLI surfaces can distinguish safe runner stages from human and memory gates." -m "Constraint: Console classification is read-only and does not change workflow state." -m "Rejected: Execute NextAction.Command through a shell | unsafe and too dependent on presentation strings." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -count=1"
```

## Task 2: Add Read-Only `devflow console`

**Files:**
- Create: `internal/cli/console.go`
- Create: `internal/cli/console_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write failing CLI console read-only tests**

Create `internal/cli/console_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestConsoleListRendersDemandAttention(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	for _, demand := range []artifacts.Demand{
		{ID: "z-complete", Title: "Z complete", Description: "Done", Source: "test", State: string(workflow.Completed)},
		{ID: "a-failed", Title: "A failed", Description: "Failed", Source: "test", State: string(workflow.FailedQualityGate)},
	} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
		}
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Demand Console", "a-failed", "quality gate failed", "z-complete", "Next:", "devflow console --demand a-failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
}

func TestConsoleDetailRendersOperatorView(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-detail", Title: "Console detail", Description: "Show detail", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console detail returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"Demand Console: console-detail",
		"State: verification",
		"Attention: ready to confirm verification",
		"Stages:",
		"Evidence:",
		"verification   PASS go test ./...",
		"Recommended:",
		"Confirm verification",
		"Run-ready:",
		"no safe runner action",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("console detail output missing %q:\n%s", want, got)
		}
	}
}

func TestConsoleHelpIncludesCommand(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow console [--demand <id>] [--run-next]", "console   Show the operator console and optionally run the next safe stage"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run failing CLI console tests**

Run:

```powershell
go test ./internal/cli -run "TestConsoleListRendersDemandAttention|TestConsoleDetailRendersOperatorView|TestConsoleHelpIncludesCommand" -count=1
```

Expected result:

```text
FAIL
unknown command "console"
```

- [ ] **Step 3: Add CLI dispatch and help**

In `internal/cli/cli.go`, add to usage:

```text
  devflow console [--demand <id>] [--run-next]
```

Add to command descriptions:

```text
  console   Show the operator console and optionally run the next safe stage
```

Add dispatch:

```go
case "console":
	return runConsole(args[1:], stdout, stderr)
```

- [ ] **Step 4: Implement read-only console CLI**

Create `internal/cli/console.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

type consoleArgs struct {
	root           string
	demandID       string
	runNext        bool
	runnerRoot     string
	qualityRoot    string
	configPath     string
	permissionMode string
	gitlabProject  string
	gitlabMR       string
	gitlabBaseURL  string
	qualityCommand stringSliceFlag
}

func runConsole(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseConsoleArgs(args, stderr)
	if err != nil {
		return err
	}
	if opts.runNext {
		return runConsoleNext(opts, stdout, stderr)
	}
	if opts.demandID != "" {
		summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
		if err != nil {
			return err
		}
		printConsoleDetail(stdout, summary)
		return nil
	}
	summaries, err := demandflow.ListConsole(opts.root)
	if err != nil {
		return err
	}
	printConsoleList(stdout, summaries)
	return nil
}

func parseConsoleArgs(args []string, stderr io.Writer) (consoleArgs, error) {
	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts consoleArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.BoolVar(&opts.runNext, "run-next", false, "run the next safe agent stage")
	fs.StringVar(&opts.runnerRoot, "runner-root", "", "working directory for agent tools; defaults to --root")
	fs.StringVar(&opts.qualityRoot, "quality-root", "", "working directory for quality commands; defaults to --root")
	fs.StringVar(&opts.configPath, "config", "", "devflow config path")
	fs.StringVar(&opts.permissionMode, "permission-mode", "", "permission mode for implementation")
	fs.StringVar(&opts.gitlabProject, "gitlab-project", "", "gitlab project path for mr-review")
	fs.StringVar(&opts.gitlabMR, "gitlab-mr", "", "gitlab merge request iid for mr-review")
	fs.StringVar(&opts.gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.Var(&opts.qualityCommand, "quality-command", "quality command as a quoted program and args (repeatable)")
	if err := fs.Parse(args); err != nil {
		return consoleArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.runNext && opts.demandID == "" {
		return consoleArgs{}, fmt.Errorf("--demand is required for --run-next")
	}
	return opts, nil
}

func printConsoleList(stdout io.Writer, summaries []demandflow.ConsoleSummary) {
	fmt.Fprintln(stdout, "Demand Console")
	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "\nNo demands found")
		return
	}
	fmt.Fprintln(stdout)
	for _, summary := range summaries {
		workspace := summary.Workspace
		fmt.Fprintf(stdout, "  %-24s %-22s %s\n", workspace.Demand.ID, workspace.State, workspace.Attention)
	}
	fmt.Fprintln(stdout, "\nNext:")
	fmt.Fprintf(stdout, "  devflow console --demand %s\n", summaries[0].Workspace.Demand.ID)
}

func printConsoleDetail(stdout io.Writer, summary demandflow.ConsoleSummary) {
	workspace := summary.Workspace
	fmt.Fprintf(stdout, "Demand Console: %s\n", workspace.Demand.ID)
	fmt.Fprintf(stdout, "State: %s\n", workspace.State)
	fmt.Fprintf(stdout, "Attention: %s\n\n", workspace.Attention)

	fmt.Fprintln(stdout, "Stages:")
	for _, stage := range workspace.Stages {
		fmt.Fprintf(stdout, "  %-14s %s\n", stage.Name, humanStatus(stage.Status))
	}

	fmt.Fprintln(stdout, "\nEvidence:")
	printConsoleEvidence(stdout, workspace)

	fmt.Fprintln(stdout, "\nRecommended:")
	printConsoleAction(stdout, summary.PrimaryAction)

	fmt.Fprintln(stdout, "\nRun-ready:")
	if summary.RunReadyAction.Runnable {
		printConsoleAction(stdout, summary.RunReadyAction)
	} else {
		fmt.Fprintf(stdout, "  %s\n", summary.RunReadyAction.BlockReason)
	}
}

func printConsoleEvidence(stdout io.Writer, workspace demandflow.WorkspaceSummary) {
	switch workspace.Verification.Status {
	case "pass":
		fmt.Fprintf(stdout, "  %-14s PASS %s\n", "verification", workspace.Verification.Command)
	case "fail":
		fmt.Fprintf(stdout, "  %-14s FAIL %s\n", "verification", workspace.Verification.Command)
	default:
		fmt.Fprintf(stdout, "  %-14s none\n", "verification")
	}
	fmt.Fprintf(stdout, "  %-14s %d pending, %d promoted, %d rejected\n", "memory", workspace.Memory.Pending, workspace.Memory.Promoted, workspace.Memory.Rejected)
	mr := humanStatus(workspace.MergeRequest.Status)
	if workspace.MergeRequest.Reference != "" {
		mr = workspace.MergeRequest.Reference + " " + mr
	}
	fmt.Fprintf(stdout, "  %-14s %s\n", "mr", mr)
}

func printConsoleAction(stdout io.Writer, action demandflow.ConsoleAction) {
	if action.Label != "" {
		fmt.Fprintf(stdout, "  %s\n", action.Label)
	}
	if strings.TrimSpace(action.Command) != "" {
		fmt.Fprintf(stdout, "  %s\n", action.Command)
	}
	if action.BlockReason != "" && !action.Runnable {
		fmt.Fprintf(stdout, "  blocked: %s\n", action.BlockReason)
	}
}

func runConsoleNext(opts consoleArgs, stdout io.Writer, stderr io.Writer) error {
	summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	action := summary.PrimaryAction
	if !action.Runnable {
		fmt.Fprintf(stdout, "next action is not runner-safe: %s\n", action.Label)
		if action.BlockReason != "" {
			fmt.Fprintf(stdout, "%s\n", action.BlockReason)
		}
		if strings.TrimSpace(action.Command) != "" {
			fmt.Fprintf(stdout, "manual command:\n%s\n", action.Command)
		}
		return nil
	}
	return runConsoleStageAction(opts, action, stdout, stderr)
}

func runConsoleStageAction(opts consoleArgs, action demandflow.ConsoleAction, stdout io.Writer, stderr io.Writer) error {
	return fmt.Errorf("console runner execution is not wired")
}
```

The last function intentionally fails for now; Task 3 wires execution with tests.

- [ ] **Step 5: Run read-only CLI tests**

Run:

```powershell
go test ./internal/cli -run "TestConsoleListRendersDemandAttention|TestConsoleDetailRendersOperatorView|TestConsoleHelpIncludesCommand" -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 6: Commit Task 2**

Run:

```powershell
git add internal/cli/cli.go internal/cli/console.go internal/cli/console_test.go
git commit -m "Add read-only operator console" -m "Wave 14 starts by exposing the workspace summary as a console surface before wiring execution. The command lists demands, renders one demand's operator evidence, and keeps runner execution behind a separate tested step." -m "Constraint: Read-only console must not mutate demand artifacts." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run \"TestConsole\" -count=1"
```

## Task 3: Wire `--run-next` To Safe Runner Stages

**Files:**
- Modify: `internal/cli/console.go`
- Modify: `internal/cli/console_test.go`

- [ ] **Step 1: Add failing execution tests**

Append to `internal/cli/console_test.go`:

```go
func TestConsoleRunNextCallsRunnerForAgentStage(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-run", Title: "Console run", Description: "Run requirements", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var gotArgs []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		fmt.Fprintln(stdout, "stub runner called")
		return nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "stub runner called") {
		t.Fatalf("stdout = %q, want stub runner output", stdout.String())
	}
	wantArgs := []string{"--root", root, "--demand", demand.ID, "--stage", "requirements"}
	for _, want := range wantArgs {
		if !containsString(gotArgs, want) {
			t.Fatalf("runner args = %#v, missing %q", gotArgs, want)
		}
	}
}

func TestConsoleRunNextRefusesHumanConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-confirm", Title: "Console confirm", Description: "Confirm", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 1, 0, 0, time.UTC), Type: "verification.recorded", Message: "pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	var called bool
	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		called = true
		return nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	if called {
		t.Fatal("runner was called for human confirmation")
	}
	if !strings.Contains(stdout.String(), "next action is not runner-safe: Confirm verification") {
		t.Fatalf("stdout = %q, want runner-safe refusal", stdout.String())
	}
}

func TestConsoleRunNextPassesQualityAndRunnerFlags(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-implementation", Title: "Console implementation", Description: "Implement", Source: "test", State: string(workflow.Implementation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var gotArgs []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next", "--runner-root", root, "--quality-root", root, "--config", "devflow.yaml", "--permission-mode", "acceptEdits", "--quality-command", "go test ./..."}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	for _, want := range []string{"--runner-root", root, "--quality-root", root, "--config", "devflow.yaml", "--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(gotArgs, want) {
			t.Fatalf("runner args = %#v, missing %q", gotArgs, want)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

Add imports to `internal/cli/console_test.go`:

```go
import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run failing execution tests**

Run:

```powershell
go test ./internal/cli -run "TestConsoleRunNext" -count=1
```

Expected result before wiring:

```text
FAIL
undefined: runConsoleDemandStage
```

or:

```text
FAIL
console runner execution is not wired
```

- [ ] **Step 3: Wire runner execution**

In `internal/cli/console.go`, add near the top:

```go
var runConsoleDemandStage = runDemandStage
```

Replace `runConsoleStageAction` with:

```go
func runConsoleStageAction(opts consoleArgs, action demandflow.ConsoleAction, stdout io.Writer, stderr io.Writer) error {
	if action.Stage == "" {
		return fmt.Errorf("console action %q has no runnable stage", action.Label)
	}
	args := []string{
		"--root", opts.root,
		"--demand", opts.demandID,
		"--stage", string(action.Stage),
	}
	if strings.TrimSpace(opts.runnerRoot) != "" {
		args = append(args, "--runner-root", strings.TrimSpace(opts.runnerRoot))
	}
	if strings.TrimSpace(opts.qualityRoot) != "" {
		args = append(args, "--quality-root", strings.TrimSpace(opts.qualityRoot))
	}
	if strings.TrimSpace(opts.configPath) != "" {
		args = append(args, "--config", strings.TrimSpace(opts.configPath))
	}
	if strings.TrimSpace(opts.permissionMode) != "" {
		args = append(args, "--permission-mode", strings.TrimSpace(opts.permissionMode))
	}
	for _, command := range opts.qualityCommand {
		if strings.TrimSpace(command) != "" {
			args = append(args, "--quality-command", strings.TrimSpace(command))
		}
	}
	if action.Stage == demandflow.StageMRReview {
		if strings.TrimSpace(opts.gitlabProject) == "" || strings.TrimSpace(opts.gitlabMR) == "" {
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required for mr-review")
		}
		args = append(args, "--gitlab-project", strings.TrimSpace(opts.gitlabProject), "--gitlab-mr", strings.TrimSpace(opts.gitlabMR))
		if strings.TrimSpace(opts.gitlabBaseURL) != "" {
			args = append(args, "--gitlab-base-url", strings.TrimSpace(opts.gitlabBaseURL))
		}
	}
	return runConsoleDemandStage(args, stdout, stderr)
}
```

- [ ] **Step 4: Run execution tests**

Run:

```powershell
go test ./internal/cli -run "TestConsoleRunNext" -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 5: Run all CLI tests**

Run:

```powershell
go test ./internal/cli -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 6: Commit Task 3**

Run:

```powershell
git add internal/cli/console.go internal/cli/console_test.go
git commit -m "Run safe console actions through the stage runner" -m "The operator console can now execute the next runner-safe agent stage by calling the existing devflow run path with typed stage metadata, while refusing human confirmation and memory gates." -m "Constraint: Console execution must not shell-execute rendered command strings." -m "Rejected: Auto-confirm verification after PASS evidence | human gates remain explicit product semantics." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -count=1"
```

## Task 4: Document Wave 14 Operator Loop

**Files:**
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Update user guide**

In `docs/user-guide/backend-demand-loop.md`, add after the "Demand Workspace Status" section:

```markdown
### Operator console

Use `devflow console` when you want an operator view rather than a material audit.

```powershell
devflow console
devflow console --demand add-coupon-check
devflow console --demand add-coupon-check --run-next
```

`console` is built on the same local workspace evidence as `status`, but it separates the recommended action from the run-ready action. `--run-next` only executes runner-safe agent stages such as requirements, plan, implementation, verification, and closeout. It does not auto-confirm human gates, promote memory, reject memory, or merge MRs.
```

- [ ] **Step 2: Update release notes**

In `docs/release/v0.1.md`, add:

```markdown
### Wave 14 - Operator Console And Runner Entry

- Adds `devflow console` for an operator-oriented view over local demand workspaces.
- Adds `devflow console --demand <id> --run-next` to execute the next runner-safe agent stage through the existing stage runner.
- Keeps human confirmation, memory decisions, and MR review requirements explicit.
```

- [ ] **Step 3: Run documentation checks**

Run:

```powershell
rg -n "Operator console|Wave 14|--run-next" docs
git diff --check
```

Expected result:

```text
docs/user-guide/backend-demand-loop.md:<line>:### Operator console
docs/release/v0.1.md:<line>:### Wave 14 - Operator Console And Runner Entry
```

- [ ] **Step 4: Commit Task 4**

Run:

```powershell
git add docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document the Wave 14 operator console" -m "The user guide now explains when to use console instead of status and clarifies that run-next only executes runner-safe agent stages." -m "Constraint: Documentation must preserve the manual confirmation and memory decision boundaries." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: rg -n \"Operator console|Wave 14|--run-next\" docs; git diff --check"
```

## Task 5: Full Verification And PR

**Files:**
- No planned source edits except possible `gofmt`.

- [ ] **Step 1: Format Go files**

Run:

```powershell
gofmt -w internal/demandflow/console.go internal/demandflow/console_test.go internal/cli/console.go internal/cli/console_test.go internal/cli/cli.go
```

- [ ] **Step 2: Run targeted tests**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectConsole|TestListConsole|TestBuildConsoleAction" -count=1
go test ./internal/cli -run "TestConsole" -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 3: Run package tests**

Run:

```powershell
go test ./internal/demandflow ./internal/cli -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 4: Run full verification**

Run:

```powershell
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

Expected result:

```text
go vet exits 0
go build exits 0
go test exits 0
git diff --check exits 0
```

- [ ] **Step 5: Manual smoke test**

Run:

```powershell
$tmp = Join-Path $env:TEMP ("devflow-wave14-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null
go run ./cmd/devflow start --root $tmp --title "Wave 14 smoke" --description "Check console"
go run ./cmd/devflow console --root $tmp
$id = (Get-ChildItem (Join-Path $tmp ".devflow\demands") -Directory | Select-Object -First 1).Name
go run ./cmd/devflow console --root $tmp --demand $id
go run ./cmd/devflow next --root $tmp --demand $id
```

Expected output contains:

```text
Demand Console
Demand Console:
Run-ready:
devflow run --demand
```

- [ ] **Step 6: Review diff**

Run:

```powershell
git diff --stat main...HEAD
git diff main...HEAD -- internal/demandflow internal/cli docs
```

Check:

- No workflow state transition was added.
- Console does not shell-execute `NextAction.Command`.
- `--run-next` refuses human confirmation and memory gates.
- `console` output does not print secrets.
- `status` and `next` behavior remains compatible.

- [ ] **Step 7: Push and open PR**

Run:

```powershell
git push -u origin HEAD
$branch = git branch --show-current
gh pr create --base main --head $branch --title "Wave 14 operator console and runner entry" --body "## Summary`n- Adds ConsoleSummary and typed console actions`n- Adds devflow console list/detail views`n- Adds console --run-next for runner-safe agent stages`n- Documents the operator console loop`n`n## Verification`n- go vet ./...`n- go build ./cmd/devflow`n- go test ./... -count=1 -timeout 5m`n- git diff --check"
```

- [ ] **Step 8: Merge only after CI passes**

After Ubuntu and Windows CI pass, use `superpowers:finishing-a-development-branch`.

Recommended merge path:

```powershell
git switch main
git pull --ff-only origin main
git merge --no-ff <wave-14-branch>
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git push origin main
```

## Self-Review Checklist

- Spec coverage:
  - `devflow console`: Task 2.
  - `devflow console --demand <id>`: Task 2.
  - typed actions: Task 1.
  - safe runner execution: Task 3.
  - refusal for human and memory gates: Task 1 and Task 3.
  - docs: Task 4.
- Type consistency:
  - `ConsoleSummary`, `ConsoleAction`, and `ConsoleActionKind` are defined before CLI usage.
  - `runConsoleDemandStage` uses the existing `runDemandStage` signature.
  - `ConsoleAction.Stage` uses `demandflow.Stage`.
- Scope:
  - No Web UI, Bubble Tea TUI, Eino, auto-confirmation, auto-memory decisions, or workflow state changes.

## Execution Notes

Use an isolated branch or worktree:

```powershell
git switch main
git pull --ff-only origin main
git switch -c feature/devflow-wave-14
```

If using a worktree:

```powershell
git worktree add .worktrees/devflow-wave-14 -b feature/devflow-wave-14 main
Set-Location .worktrees/devflow-wave-14
```

Commit after each task. Keep each commit small and use the repository Lore Commit Protocol.

# Wave 6 Usability And Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the merged Wave 1-5 backend-demand Agent from a working engine into a user-operable product loop with clear next-step guidance, safer recovery, configuration setup, diagnostics, and documentation.

**Architecture:** Keep `internal/demandflow` as the product workflow authority and add a read-only advisory layer for status and next actions. Keep `internal/cli` as a thin command adapter for `status`, `next`, `init`, and `doctor`, while preserving the existing `run`, `start`, `confirm`, `verify`, `closeout`, `chat`, and `tui` commands. Add documentation and examples without introducing new runtime dependencies.

**Tech Stack:** Go 1.25.0, existing Devflow `.devflow` artifact store, `internal/demandflow`, `internal/cli`, `internal/runtime/config`, standard-library filesystem/process/env APIs, GitHub/GitLab-free local tests, and optional manual live provider smoke tests.

---

## Current Environment

Wave 5 has been merged into `main` through PR #1:

```text
41dcebf Merge Devflow Agent v0.1 runtime and demand loop
```

Wave 6 worktree:

```powershell
$target = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-wave-6'
Set-Location $target
git status --short --branch
```

Expected starting branch:

```text
## feature/devflow-wave-6
```

Known local build artifact in the main checkout:

```text
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\devflow.exe
```

Do not commit generated binaries or local cache directories.

Baseline already verified in the Wave 6 worktree:

```powershell
go test ./... -count=1 -timeout 5m
```

## Wave 6 Scope

In scope:

- Add `devflow status --demand <id>` to show demand state, artifact paths, and available next actions.
- Add `devflow next --demand <id>` to print the next recommended command.
- Make `devflow run` output more useful after each stage.
- Allow safe recovery from `failed_quality_gate` by rerunning implementation.
- Add `devflow init` to create a no-secret `.devflow/config.yaml`.
- Add `devflow doctor` to diagnose config, env, git, and GitLab readiness without printing secrets.
- Add example config files and a user guide for the backend-demand loop.
- Add an optional live smoke procedure documented for Ark/OpenAI-compatible providers.

Out of scope:

- New frontend UI.
- New external dependencies.
- Automatic GitLab MR creation.
- Automatic merge to main.
- Storing API keys in generated files.
- Rewriting the Wave 5 demandflow engine.

## File Map

Create:

```text
internal/demandflow/status.go
internal/demandflow/status_test.go
internal/demandflow/result.go
internal/cli/status.go
internal/cli/status_test.go
internal/cli/init.go
internal/cli/init_test.go
internal/cli/doctor.go
internal/cli/doctor_test.go
docs/examples/config.openai-compat.yaml
docs/examples/config.anthropic.yaml
docs/user-guide/backend-demand-loop.md
```

Modify:

```text
README.md
internal/cli/cli.go
internal/cli/cli_test.go
internal/cli/run.go
internal/cli/run_test.go
internal/demandflow/engine.go
internal/demandflow/engine_test.go
internal/demandflow/runtime_runner.go
internal/demandflow/runtime_runner_test.go
```

No `go.mod` or `go.sum` changes should be needed.

## Task 0: Preflight

**Files:** none

- [ ] Check branch and worktree status.

```powershell
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-6
```

- [ ] Verify clean baseline.

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

No commit for Task 0.

## Task 1: Demand Status And Next-Action Model

**Files:**

- Create: `internal/demandflow/status.go`
- Create: `internal/demandflow/status_test.go`

- [ ] Add status types in `internal/demandflow/status.go`.

```go
package demandflow

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/jesseedcp/devflow-agent/internal/artifacts"
    "github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ArtifactInfo struct {
    Name   string
    Path   string
    Exists bool
    Size   int64
}

type NextAction struct {
    Label   string
    Command string
    Reason  string
}

type StatusReport struct {
    Demand    artifacts.Demand
    State     workflow.State
    DemandDir string
    Artifacts []ArtifactInfo
    Actions   []NextAction
}
```

- [ ] Add `InspectStatus`.

```go
func InspectStatus(root, demandID string) (StatusReport, error) {
    store := artifacts.NewStore(root)
    demand, err := store.LoadDemand(demandID)
    if err != nil {
        return StatusReport{}, err
    }

    demandDir := store.DemandDir(demandID)
    report := StatusReport{
        Demand:    demand,
        State:     workflow.State(demand.State),
        DemandDir: demandDir,
        Artifacts: inspectArtifacts(demandDir),
    }
    report.Actions = NextActions(report.State, demand.ID)
    return report, nil
}
```

- [ ] Add artifact inspection.

```go
func inspectArtifacts(demandDir string) []ArtifactInfo {
    names := []string{
        artifacts.RequirementsFile,
        artifacts.PlanFile,
        artifacts.ProgressFile,
        artifacts.VerificationFile,
        artifacts.CloseoutFile,
        artifacts.MemoryCandidatesFile,
        artifacts.EventsFile,
    }
    infos := make([]ArtifactInfo, 0, len(names))
    for _, name := range names {
        path := filepath.Join(demandDir, name)
        info := ArtifactInfo{Name: name, Path: path}
        if stat, err := os.Stat(path); err == nil {
            info.Exists = true
            info.Size = stat.Size()
        }
        infos = append(infos, info)
    }
    return infos
}
```

- [ ] Add `NextActions`.

```go
func NextActions(state workflow.State, demandID string) []NextAction {
    idArg := shellQuote(demandID)
    switch state {
    case workflow.Created, workflow.ContextLoaded:
        return []NextAction{{
            Label:   "Draft requirements",
            Command: "devflow run --demand " + idArg + " --stage requirements",
            Reason:  "The demand needs requirements before human review.",
        }}
    case workflow.RequirementsDrafting:
        return []NextAction{{
            Label:   "Continue requirements",
            Command: "devflow run --demand " + idArg + " --stage requirements",
            Reason:  "Requirements drafting is in progress.",
        }}
    case workflow.RequirementsReview:
        return []NextAction{{
            Label:   "Confirm requirements",
            Command: "devflow confirm --demand " + idArg + " --stage requirements --by <name> --summary <summary>",
            Reason:  "Requirements need human confirmation before planning.",
        }}
    case workflow.PlanDrafting:
        return []NextAction{{
            Label:   "Draft plan",
            Command: "devflow run --demand " + idArg + " --stage plan",
            Reason:  "The confirmed requirements are ready for planning.",
        }}
    case workflow.PlanReview:
        return []NextAction{{
            Label:   "Confirm plan",
            Command: "devflow confirm --demand " + idArg + " --stage plan --by <name> --summary <summary>",
            Reason:  "The technical plan needs human confirmation before implementation.",
        }}
    case workflow.Implementation:
        return []NextAction{{
            Label:   "Run implementation",
            Command: "devflow run --demand " + idArg + " --stage implementation --permission-mode acceptEdits --quality-command \"go test ./...\"",
            Reason:  "Implementation can now edit code and run quality gates.",
        }}
    case workflow.FailedQualityGate:
        return []NextAction{{
            Label:   "Retry implementation",
            Command: "devflow run --demand " + idArg + " --stage implementation --permission-mode acceptEdits --quality-command \"go test ./...\"",
            Reason:  "The previous quality gate failed; rerun implementation after addressing failures.",
        }}
    case workflow.MRReview:
        return []NextAction{{
            Label:   "Check MR review",
            Command: "devflow run --demand " + idArg + " --stage mr-review --gitlab-project <group/project> --gitlab-mr <iid>",
            Reason:  "MR review must be clear before verification.",
        }}
    case workflow.Verification:
        return []NextAction{
            {
                Label:   "Draft verification",
                Command: "devflow run --demand " + idArg + " --stage verification --quality-command \"go test ./...\"",
                Reason:  "Verification evidence should be generated or refreshed.",
            },
            {
                Label:   "Confirm verification",
                Command: "devflow confirm --demand " + idArg + " --stage verification --by <name> --summary <summary>",
                Reason:  "Human confirmation advances verification to closeout.",
            },
        }
    case workflow.Closeout:
        return []NextAction{
            {
                Label:   "Draft closeout",
                Command: "devflow run --demand " + idArg + " --stage closeout",
                Reason:  "Closeout and memory candidates should be generated or refreshed.",
            },
            {
                Label:   "Confirm closeout",
                Command: "devflow confirm --demand " + idArg + " --stage closeout --by <name> --summary <summary>",
                Reason:  "Human confirmation completes the demand.",
            },
        }
    case workflow.Completed:
        return []NextAction{{
            Label:   "No action",
            Command: "",
            Reason:  "The demand is complete.",
        }}
    default:
        return []NextAction{{
            Label:   "Inspect manually",
            Command: "devflow status --demand " + idArg,
            Reason:  fmt.Sprintf("State %s has no automated recommendation.", state),
        }}
    }
}
```

- [ ] Add quoting helper.

```go
func shellQuote(value string) string {
    if value == "" {
        return `""`
    }
    if strings.ContainsAny(value, " \t\r\n\"'") {
        return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
    }
    return value
}
```

- [ ] Add tests in `internal/demandflow/status_test.go`.

Test cases:

```go
func TestNextActionsMapStatesToCommands(t *testing.T) {
    cases := []struct {
        state   workflow.State
        want    string
        command string
    }{
        {workflow.Created, "Draft requirements", "devflow run --demand add-coupon-check --stage requirements"},
        {workflow.RequirementsReview, "Confirm requirements", "devflow confirm --demand add-coupon-check --stage requirements"},
        {workflow.PlanDrafting, "Draft plan", "devflow run --demand add-coupon-check --stage plan"},
        {workflow.FailedQualityGate, "Retry implementation", "devflow run --demand add-coupon-check --stage implementation"},
        {workflow.MRReview, "Check MR review", "devflow run --demand add-coupon-check --stage mr-review"},
        {workflow.Completed, "No action", ""},
    }
    for _, tc := range cases {
        actions := NextActions(tc.state, "add-coupon-check")
        if len(actions) == 0 {
            t.Fatalf("%s: no actions", tc.state)
        }
        if actions[0].Label != tc.want {
            t.Fatalf("%s: label = %q, want %q", tc.state, actions[0].Label, tc.want)
        }
        if tc.command != "" && !strings.Contains(actions[0].Command, tc.command) {
            t.Fatalf("%s: command = %q, want contains %q", tc.state, actions[0].Command, tc.command)
        }
    }
}
```

Also test `InspectStatus` with a temp demand workspace and assert:

- state is loaded;
- demand directory is under `.devflow/demands/<id>`;
- `requirements.md` is reported as existing;
- actions are populated.

- [ ] Verify and commit.

```powershell
gofmt -w internal/demandflow/status.go internal/demandflow/status_test.go
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow/status.go internal/demandflow/status_test.go
git commit -m @'
Explain demand state with next actions

Wave 6 starts by making workflow state inspectable. The advisory layer
keeps state decisions in demandflow and gives CLI commands a shared
source for what the user should do next.

Constraint: Existing workflow states remain authoritative
Rejected: Duplicating next-step rules inside CLI handlers | would drift from demandflow behavior
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 2: Add `devflow status` And `devflow next`

**Files:**

- Create: `internal/cli/status.go`
- Create: `internal/cli/status_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] Update help text in `internal/cli/cli.go`.

Add usage lines:

```text
  devflow status --demand <id>
  devflow next --demand <id>
```

Add command descriptions:

```text
  status    Show demand state, artifacts, and next actions
  next      Print the next recommended command for a demand
```

Add dispatch:

```go
case "status":
    return runStatus(args[1:], stdout)
case "next":
    return runNext(args[1:], stdout)
```

- [ ] Create `internal/cli/status.go`.

```go
package cli

import (
    "flag"
    "fmt"
    "io"
    "strings"

    "github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runStatus(args []string, stdout io.Writer) error {
    opts, err := parseDemandLookupArgs("status", args)
    if err != nil {
        return err
    }
    report, err := demandflow.InspectStatus(opts.root, opts.demandID)
    if err != nil {
        return err
    }
    fmt.Fprintf(stdout, "Demand: %s\n", report.Demand.ID)
    fmt.Fprintf(stdout, "Title: %s\n", report.Demand.Title)
    fmt.Fprintf(stdout, "State: %s\n", report.State)
    fmt.Fprintf(stdout, "Directory: %s\n\n", report.DemandDir)
    fmt.Fprintln(stdout, "Artifacts:")
    for _, artifact := range report.Artifacts {
        status := "missing"
        if artifact.Exists {
            status = fmt.Sprintf("%d bytes", artifact.Size)
        }
        fmt.Fprintf(stdout, "  - %s: %s\n", artifact.Name, status)
    }
    fmt.Fprintln(stdout, "\nNext actions:")
    for _, action := range report.Actions {
        fmt.Fprintf(stdout, "  - %s: %s\n", action.Label, action.Reason)
        if strings.TrimSpace(action.Command) != "" {
            fmt.Fprintf(stdout, "    %s\n", action.Command)
        }
    }
    return nil
}

func runNext(args []string, stdout io.Writer) error {
    opts, err := parseDemandLookupArgs("next", args)
    if err != nil {
        return err
    }
    report, err := demandflow.InspectStatus(opts.root, opts.demandID)
    if err != nil {
        return err
    }
    if len(report.Actions) == 0 || strings.TrimSpace(report.Actions[0].Command) == "" {
        fmt.Fprintf(stdout, "No next command for %s in state %s\n", report.Demand.ID, report.State)
        return nil
    }
    fmt.Fprintln(stdout, report.Actions[0].Command)
    return nil
}

type demandLookupArgs struct {
    root     string
    demandID string
}

func parseDemandLookupArgs(name string, args []string) (demandLookupArgs, error) {
    fs := flag.NewFlagSet(name, flag.ContinueOnError)
    fs.SetOutput(io.Discard)
    var opts demandLookupArgs
    fs.StringVar(&opts.root, "root", ".", "root directory")
    fs.StringVar(&opts.demandID, "demand", "", "demand id")
    if err := fs.Parse(args); err != nil {
        return demandLookupArgs{}, err
    }
    opts.root = strings.TrimSpace(opts.root)
    if opts.root == "" {
        opts.root = "."
    }
    opts.demandID = strings.TrimSpace(opts.demandID)
    if opts.demandID == "" {
        return demandLookupArgs{}, fmt.Errorf("--demand is required")
    }
    return opts, nil
}
```

- [ ] Add `internal/cli/status_test.go`.

Tests:

- `status` prints demand ID, state, artifact names, and next actions;
- `next` prints the first recommended command only;
- missing `--demand` returns `--demand is required`;
- help output includes `status` and `next`.

- [ ] Verify and commit.

```powershell
gofmt -w internal/cli/cli.go internal/cli/status.go internal/cli/status_test.go internal/cli/cli_test.go
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/cli
git commit -m @'
Show users what to do next for a demand

The status and next commands make the workflow usable without reading
state files by hand, while keeping all state advice sourced from
demandflow.

Constraint: CLI remains a thin adapter over demandflow
Rejected: Asking users to inspect demand.json manually | not acceptable for product use
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 3: Return Detailed Run Results And Improve `devflow run` Output

**Files:**

- Create: `internal/demandflow/result.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/engine_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

- [ ] Create `internal/demandflow/result.go`.

```go
package demandflow

import "github.com/jesseedcp/devflow-agent/internal/workflow"

type RunResult struct {
    DemandID       string
    Stage          Stage
    PreviousState  workflow.State
    CurrentState   workflow.State
    Artifacts      []string
    QualityPassed  *bool
    Message        string
    NextActions    []NextAction
}
```

- [ ] Add `RunDetailed` while keeping `Run`.

In `internal/demandflow/engine.go`, change `Run` to wrap `RunDetailed`:

```go
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
```

Update private stage methods to accept `result *RunResult` and fill:

- `Artifacts`
- `Message`
- `QualityPassed` for implementation and verification when quality commands run.

- [ ] Improve `runDemandStage`.

Change the engine call in `internal/cli/run.go`:

```go
result, err := engine.RunDetailed(context.Background(), opts)
if err != nil {
    if result.DemandID != "" {
        printRunResult(stdout, result)
    }
    return err
}
printRunResult(stdout, result)
return nil
```

Add:

```go
func printRunResult(stdout io.Writer, result demandflow.RunResult) {
    fmt.Fprintf(stdout, "stage %s completed for %s\n", result.Stage, result.DemandID)
    if result.PreviousState != "" || result.CurrentState != "" {
        fmt.Fprintf(stdout, "state: %s -> %s\n", result.PreviousState, result.CurrentState)
    }
    if result.Message != "" {
        fmt.Fprintf(stdout, "%s\n", result.Message)
    }
    if len(result.Artifacts) > 0 {
        fmt.Fprintln(stdout, "artifacts:")
        for _, artifact := range result.Artifacts {
            fmt.Fprintf(stdout, "  - %s\n", artifact)
        }
    }
    if len(result.NextActions) > 0 {
        fmt.Fprintln(stdout, "next:")
        action := result.NextActions[0]
        fmt.Fprintf(stdout, "  %s\n", action.Label)
        if strings.TrimSpace(action.Command) != "" {
            fmt.Fprintf(stdout, "  %s\n", action.Command)
        }
    }
}
```

- [ ] Update tests.

Add assertions:

- requirements run output includes `state: created -> requirements_review`;
- output lists `requirements.md`;
- output includes `next:` and a confirmation command;
- failed quality gate prints state ending in `failed_quality_gate`.

- [ ] Verify and commit.

```powershell
gofmt -w internal/demandflow/*.go internal/cli/run.go internal/cli/run_test.go
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow internal/cli
git commit -m @'
Report demand run results with next steps

RunDetailed preserves the existing engine API while giving the CLI
state transitions, artifact writes, and next-action guidance after each
stage.

Constraint: Existing Engine.Run callers must keep working
Rejected: Parsing events after the run to infer output | weaker than returning structured results
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 4: Quality-Gate Recovery

**Files:**

- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/engine_test.go`
- Modify: `internal/cli/status_test.go`

- [ ] Allow rerunning implementation from `failed_quality_gate`.

In `runImplementation`, change state check:

```go
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
```

- [ ] Add tests.

Add a demand in `failed_quality_gate`, rerun implementation with passing quality, and assert:

- state becomes `mr_review`;
- `progress.md` contains retry output;
- `events.jsonl` contains `implementation.retry`.

- [ ] Verify and commit.

```powershell
gofmt -w internal/demandflow/engine.go internal/demandflow/engine_test.go internal/cli/status_test.go
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow internal/cli
git commit -m @'
Let failed quality gates recover through implementation

Quality failures now lead to a clear retry path instead of trapping the
demand in failed_quality_gate with no runnable next command.

Constraint: Recovery must use the existing workflow transition from failed_quality_gate to implementation
Rejected: Manually editing demand.json to resume | unsafe and not user-operable
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/demandflow -count=1; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 5: `devflow init` For No-Secret Config Setup

**Files:**

- Create: `internal/cli/init.go`
- Create: `internal/cli/init_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Create: `docs/examples/config.openai-compat.yaml`
- Create: `docs/examples/config.anthropic.yaml`

- [ ] Add help and dispatch.

Usage:

```text
  devflow init --provider <openai-compat|openai|anthropic>
```

Description:

```text
  init      Create a no-secret .devflow/config.yaml
```

Dispatch:

```go
case "init":
    return runInit(args[1:], stdout)
```

- [ ] Create `internal/cli/init.go`.

```go
package cli

import (
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
)

func runInit(args []string, stdout io.Writer) error {
    fs := flag.NewFlagSet("init", flag.ContinueOnError)
    fs.SetOutput(io.Discard)

    var root, provider, baseURL, model string
    var force bool
    fs.StringVar(&root, "root", ".", "root directory")
    fs.StringVar(&provider, "provider", "openai-compat", "provider protocol")
    fs.StringVar(&baseURL, "base-url", "", "provider base url")
    fs.StringVar(&model, "model", "", "provider model")
    fs.BoolVar(&force, "force", false, "overwrite existing config")

    if err := fs.Parse(args); err != nil {
        return err
    }

    provider = strings.TrimSpace(provider)
    if provider == "" {
        provider = "openai-compat"
    }
    cfg, err := renderInitialConfig(provider, baseURL, model)
    if err != nil {
        return err
    }

    cfgDir := filepath.Join(root, ".devflow")
    cfgPath := filepath.Join(cfgDir, "config.yaml")
    if _, err := os.Stat(cfgPath); err == nil && !force {
        return fmt.Errorf("%s already exists; use --force to overwrite", cfgPath)
    } else if err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("inspect config: %w", err)
    }
    if err := os.MkdirAll(cfgDir, 0o755); err != nil {
        return fmt.Errorf("create .devflow directory: %w", err)
    }
    if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
        return fmt.Errorf("write config: %w", err)
    }
    fmt.Fprintf(stdout, "wrote %s\n", cfgPath)
    fmt.Fprintln(stdout, "Set the required API key in your environment before running devflow chat or devflow run.")
    return nil
}
```

- [ ] Add config renderer.

```go
func renderInitialConfig(provider, baseURL, model string) (string, error) {
    switch provider {
    case "openai-compat":
        if strings.TrimSpace(baseURL) == "" {
            baseURL = "https://ark.cn-beijing.volces.com/api/coding/v3"
        }
        if strings.TrimSpace(model) == "" {
            model = "ark-code-latest"
        }
        return fmt.Sprintf(`providers:
  - name: ark
    protocol: openai-compat
    base_url: %s
    model: %s
    context_window: 128000
    max_output_tokens: 8192
permission_mode: default
`, baseURL, model), nil
    case "openai":
        if strings.TrimSpace(baseURL) == "" {
            baseURL = "https://api.openai.com/v1"
        }
        if strings.TrimSpace(model) == "" {
            model = "gpt-5.4"
        }
        return fmt.Sprintf(`providers:
  - name: openai
    protocol: openai
    base_url: %s
    model: %s
    context_window: 128000
    max_output_tokens: 8192
permission_mode: default
`, baseURL, model), nil
    case "anthropic":
        if strings.TrimSpace(baseURL) == "" {
            baseURL = "https://api.anthropic.com"
        }
        if strings.TrimSpace(model) == "" {
            model = "claude-sonnet-4-5"
        }
        return fmt.Sprintf(`providers:
  - name: anthropic
    protocol: anthropic
    base_url: %s
    model: %s
    context_window: 200000
    max_output_tokens: 8192
permission_mode: default
`, baseURL, model), nil
    default:
        return "", fmt.Errorf("unsupported provider %q", provider)
    }
}
```

The generated config must not contain literal API key values.

- [ ] Add tests.

Tests:

- default init writes `.devflow/config.yaml`;
- default config contains `openai-compat`, Ark base URL, and `ark-code-latest`;
- config does not contain `api_key`;
- existing config fails without `--force`;
- `--force` overwrites;
- unsupported provider returns error.

- [ ] Add docs examples.

`docs/examples/config.openai-compat.yaml`:

```yaml
providers:
  - name: ark
    protocol: openai-compat
    base_url: https://ark.cn-beijing.volces.com/api/coding/v3
    model: ark-code-latest
    context_window: 128000
    max_output_tokens: 8192
permission_mode: default
```

`docs/examples/config.anthropic.yaml`:

```yaml
providers:
  - name: anthropic
    protocol: anthropic
    base_url: https://api.anthropic.com
    model: claude-sonnet-4-5
    context_window: 200000
    max_output_tokens: 8192
permission_mode: default
```

- [ ] Verify and commit.

```powershell
gofmt -w internal/cli/cli.go internal/cli/init.go internal/cli/init_test.go internal/cli/cli_test.go
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/cli docs/examples
git commit -m @'
Create no-secret Devflow configuration from the CLI

The init command gives users a safe first-run path by generating
.devflow/config.yaml without embedding API keys.

Constraint: API keys must stay in the user's environment or private config
Rejected: Prompting for and writing API keys | increases secret leakage risk
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 6: `devflow doctor`

**Files:**

- Create: `internal/cli/doctor.go`
- Create: `internal/cli/doctor_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] Add help and dispatch.

Usage:

```text
  devflow doctor
```

Description:

```text
  doctor    Diagnose config, environment, git, and GitLab readiness
```

Dispatch:

```go
case "doctor":
    return runDoctor(args[1:], stdout)
```

- [ ] Create `internal/cli/doctor.go`.

```go
package cli

import (
    "context"
    "flag"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"

    "github.com/jesseedcp/devflow-agent/internal/runtime/config"
)

type doctorCheck struct {
    Name    string
    OK      bool
    Message string
}

func runDoctor(args []string, stdout io.Writer) error {
    fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
    fs.SetOutput(io.Discard)
    var configPath string
    fs.StringVar(&configPath, "config", "", "config path")
    if err := fs.Parse(args); err != nil {
        return err
    }
    checks := runDoctorChecks(context.Background(), configPath)
    failed := false
    for _, check := range checks {
        mark := "OK"
        if !check.OK {
            mark = "FAIL"
            failed = true
        }
        fmt.Fprintf(stdout, "[%s] %s: %s\n", mark, check.Name, check.Message)
    }
    if failed {
        return fmt.Errorf("doctor found failing checks")
    }
    return nil
}
```

- [ ] Add check implementation.

```go
func runDoctorChecks(ctx context.Context, configPath string) []doctorCheck {
    checks := []doctorCheck{
        checkGit(ctx),
        checkConfig(configPath),
        checkGitLabToken(),
    }
    return checks
}

func checkGit(ctx context.Context) doctorCheck {
    cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
    out, err := cmd.Output()
    if err != nil {
        return doctorCheck{Name: "git", OK: false, Message: "not inside a git repository"}
    }
    return doctorCheck{Name: "git", OK: true, Message: strings.TrimSpace(string(out))}
}

func checkConfig(configPath string) doctorCheck {
    cfg, err := config.LoadConfig(configPath)
    if err != nil {
        return doctorCheck{Name: "config", OK: false, Message: err.Error()}
    }
    if len(cfg.Providers) == 0 {
        return doctorCheck{Name: "config", OK: false, Message: "no providers configured"}
    }
    provider := cfg.Providers[0]
    if provider.ResolveAPIKey() == "" {
        return doctorCheck{Name: "config", OK: false, Message: "provider " + provider.Name + " has no API key in config or environment"}
    }
    return doctorCheck{Name: "config", OK: true, Message: "loaded provider " + provider.Name + " without printing secrets"}
}

func checkGitLabToken() doctorCheck {
    if os.Getenv("GITLAB_TOKEN") == "" {
        return doctorCheck{Name: "gitlab", OK: false, Message: "GITLAB_TOKEN is not set; mr-review requires it unless a token is passed by adapter code"}
    }
    return doctorCheck{Name: "gitlab", OK: true, Message: "GITLAB_TOKEN is set"}
}
```

- [ ] Add tests.

Tests:

- `doctor` reports config failure when no config exists;
- `doctor --config <file>` reports OK when provider contains test `api_key`;
- output does not contain the test API key string;
- `checkGitLabToken` reports missing token without printing environment values.

Use `t.Setenv("GITLAB_TOKEN", "")` and temporary config files.

- [ ] Verify and commit.

```powershell
gofmt -w internal/cli/cli.go internal/cli/doctor.go internal/cli/doctor_test.go internal/cli/cli_test.go
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/cli
git commit -m @'
Diagnose Devflow setup without exposing secrets

The doctor command gives users an explicit readiness check for config,
git, and GitLab while keeping token values out of terminal output.

Constraint: Diagnostics must never print API keys or tokens
Rejected: Letting provider startup failures be the only diagnostic path | too opaque for first-run setup
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 7: Optional Live Smoke Command

**Files:**

- Create: `internal/cli/smoke.go`
- Create: `internal/cli/smoke_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

This command is a guided local smoke, not a CI live test. It creates a temporary demand and runs the requirements stage with the configured provider only when the user explicitly runs it.

- [ ] Add help and dispatch.

Usage:

```text
  devflow smoke --title <title> --description <text>
```

Description:

```text
  smoke    Run an explicit local requirements-stage smoke test
```

Dispatch:

```go
case "smoke":
    return runSmoke(args[1:], stdout, stderr)
```

- [ ] Implement `internal/cli/smoke.go`.

Behavior:

- flags: `--root`, `--title`, `--description`, `--config`;
- require title and description;
- create demand using existing `runStart` logic or `artifacts.Store.CreateDemand`;
- run `demandflow.StageRequirements` with `RuntimeRunner`;
- print demand ID and artifact path;
- do not run automatically in tests.

Use package variables to stub the runner in tests:

```go
var newSmokeRunner = func(configPath string) demandflow.Runner {
    return demandflow.RuntimeRunner{ConfigPath: configPath, MaxIterations: 8}
}
```

- [ ] Add tests.

Tests:

- missing title errors;
- smoke with stub runner creates demand and requirements file;
- output includes `requirements.md`;
- generated demand remains in `requirements_review`.

- [ ] Verify and commit.

```powershell
gofmt -w internal/cli/cli.go internal/cli/smoke.go internal/cli/smoke_test.go internal/cli/cli_test.go
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/cli
git commit -m @'
Add an explicit requirements-stage smoke path

The smoke command lets users validate provider configuration against a
temporary demand without making live network calls part of CI.

Constraint: Live provider checks must remain opt-in
Rejected: Adding live API tests to go test | would make verification environment-dependent
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 8: User Guide And README Update

**Files:**

- Create: `docs/user-guide/backend-demand-loop.md`
- Modify: `README.md`

- [ ] Create `docs/user-guide/backend-demand-loop.md`.

Content must include:

````markdown
# Backend Demand Loop User Guide

## 1. Initialize Configuration

Run:

```powershell
devflow init --provider openai-compat
```

Set the provider key in the environment. For Ark/OpenAI-compatible usage:

```powershell
$env:OPENAI_API_KEY = '<your-key>'
```

Do not commit `.devflow/config.local.yaml` or files containing API keys.

## 2. Create A Demand

```powershell
devflow start --title "Add coupon eligibility check" --description "Only active members can claim coupons"
```

## 3. Check Status

```powershell
devflow status --demand add-coupon-eligibility-check
devflow next --demand add-coupon-eligibility-check
```

## 4. Run Requirements

```powershell
devflow run --demand add-coupon-eligibility-check --stage requirements
```

Confirm:

```powershell
devflow confirm --demand add-coupon-eligibility-check --stage requirements --by dd --summary "requirements look correct"
```

## 5. Run Plan

```powershell
devflow run --demand add-coupon-eligibility-check --stage plan
devflow confirm --demand add-coupon-eligibility-check --stage plan --by dd --summary "plan approved"
```

## 6. Run Implementation

```powershell
devflow run --demand add-coupon-eligibility-check --stage implementation --permission-mode acceptEdits --quality-command "go test ./..."
```

If the quality gate fails, fix the reported problem and rerun the same implementation command.

## 7. Run MR Review Gate

```powershell
$env:GITLAB_TOKEN = '<your-token>'
devflow run --demand add-coupon-eligibility-check --stage mr-review --gitlab-project "group/project" --gitlab-mr "123"
```

## 8. Run Verification And Closeout

```powershell
devflow run --demand add-coupon-eligibility-check --stage verification --quality-command "go test ./..."
devflow confirm --demand add-coupon-eligibility-check --stage verification --by dd --summary "verification passed"
devflow run --demand add-coupon-eligibility-check --stage closeout
devflow confirm --demand add-coupon-eligibility-check --stage closeout --by dd --summary "closeout accepted"
```

## 9. Diagnostics

```powershell
devflow doctor
```

The doctor command reports whether config, git, and GitLab token setup are ready without printing secret values.
````

- [ ] Update README.

Add links to:

- `docs/user-guide/backend-demand-loop.md`
- `docs/examples/config.openai-compat.yaml`
- `docs/examples/config.anthropic.yaml`

Add a short Wave 6 command list:

```text
devflow init
devflow status
devflow next
devflow doctor
devflow smoke
```

- [ ] Verify docs.

```powershell
git diff --check
```

- [ ] Commit.

```powershell
git add README.md docs/user-guide/backend-demand-loop.md
git commit -m @'
Document the backend demand loop for first users

The user guide gives a complete command path from config setup through
closeout so Wave 6 usability work is discoverable outside tests.

Constraint: Documentation must avoid real secret values
Rejected: Keeping usage only in implementation plans | not visible to product users
Confidence: high
Scope-risk: narrow
Tested: git diff --check
'@
```

## Task 9: Final Verification And PR

**Files:** none unless final fixes are needed

- [ ] Run full verification.

```powershell
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-6
```

Only local untracked build artifacts are acceptable.

- [ ] Push branch.

```powershell
git push -u origin feature/devflow-wave-6
```

- [ ] Create PR.

```powershell
gh pr create --base main --head feature/devflow-wave-6 --title "Wave 6: usability and production hardening" --body @'
## Summary
- Adds demand status and next-action guidance.
- Improves run output and failed quality-gate recovery.
- Adds init, doctor, smoke, config examples, and backend-demand user guide.

## Test Plan
- [ ] go test ./internal/demandflow -count=1
- [ ] go test ./internal/cli -count=1
- [ ] go test ./... -count=1 -timeout 5m
- [ ] go vet ./...
- [ ] go build ./cmd/devflow
- [ ] git diff --check
'@
```

Do not merge the PR until verification and review are complete.

## Definition Of Done

Wave 6 is complete when:

- `devflow status --demand <id>` prints state, artifact summary, and next actions.
- `devflow next --demand <id>` prints the first recommended command.
- `devflow run` prints state transition, artifact writes, and next action after successful stages.
- `failed_quality_gate` can recover by rerunning implementation.
- `devflow init` writes a no-secret `.devflow/config.yaml`.
- `devflow doctor` reports config/git/GitLab readiness without printing secrets.
- `devflow smoke` offers an explicit opt-in requirements-stage live smoke path.
- README links to examples and the backend-demand user guide.
- Full verification passes:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- A PR from `feature/devflow-wave-6` to `main` is open.

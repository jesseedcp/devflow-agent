# Wave 8 Real Demand End-To-End Dogfood Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Wave 7's local dogfood from a first-step smoke into a deterministic full backend-demand loop that runs requirements, plan, implementation, MR review, verification, closeout, human gates, quality gates, and an evidence report without live provider credentials.

**Architecture:** Keep `internal/demandflow` as the workflow authority, but make two reusable seams explicit: confirmation gates become a demandflow service instead of CLI-only logic, and quality gates can run in a separate repository root from the demand artifact root. Add `internal/dogfood` as a small orchestration package that composes existing demandflow engine, static runner responses, fake MR review, real quality commands, and report generation; expose it through `devflow dogfood` and update the PowerShell dogfood script to call that command.

**Tech Stack:** Go 1.25.0, existing `.devflow` artifact store, existing `internal/demandflow`, existing `internal/quality`, existing CLI pattern in `internal/cli`, PowerShell scripts, no new Go dependencies, no live model/API credentials in deterministic dogfood.

---

## Current Environment

Repository:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
```

Wave 8 worktree:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-wave-8
```

Branch:

```text
feature/devflow-wave-8
```

Starting point:

```text
2a5692d Merge Wave 7 release readiness into main
```

Baseline already verified in this worktree:

```powershell
go mod download
go test ./... -count=1 -timeout 5m
```

Observed result:

```text
all Go packages passed
```

## Wave 8 Product Thesis

Wave 7 proved that the binary can be built and that a demand can be created. It did not prove the product loop can complete. Wave 8 should make this true:

```text
devflow dogfood
  -> creates an isolated demand workspace
  -> runs requirements
  -> records a human requirements confirmation
  -> runs plan
  -> records a human plan confirmation
  -> runs implementation with real quality command
  -> clears an offline MR review gate
  -> runs verification with real quality command
  -> records a human verification confirmation
  -> runs closeout
  -> records a human closeout confirmation
  -> ends at completed
  -> writes dogfood-report.md with commands, states, artifacts, and quality evidence
```

The loop is deterministic. It uses static agent responses and an offline MR review adapter so it can run in CI and on a laptop without API keys. Optional live provider dogfood remains a later wave.

## Scope

In scope:

- Separate demand artifact root from quality command working directory.
- Extract human confirmation behavior from `internal/cli` into reusable `internal/demandflow`.
- Add a deterministic `internal/dogfood` package that runs a full coupon-eligibility scenario.
- Add `devflow dogfood` CLI.
- Update `scripts/dogfood-local.ps1` to build the binary and run `devflow dogfood`.
- Write dogfood evidence to `dogfood-report.md`.
- Update user docs and release notes to describe full deterministic dogfood.

Out of scope:

- Live model provider end-to-end dogfood.
- Real GitLab MR creation or real GitLab comments.
- Frontend, PD Agent, or test Agent.
- Rewriting the workflow state machine.
- Adding new dependencies.

## File Map

Create:

```text
internal/demandflow/confirmation.go
internal/demandflow/confirmation_test.go
internal/dogfood/scenario.go
internal/dogfood/runner.go
internal/dogfood/runner_test.go
internal/cli/dogfood.go
internal/cli/dogfood_test.go
docs/user-guide/full-loop-dogfood.md
docs/examples/dogfood/coupon-eligibility-report.md
```

Modify:

```text
README.md
docs/release/v0.1.md
docs/user-guide/dogfood-smoke.md
internal/cli/cli.go
internal/cli/cli_test.go
internal/cli/run.go
internal/cli/run_test.go
internal/cli/confirm-related code in internal/cli/cli.go
internal/demandflow/engine.go
internal/demandflow/engine_test.go
internal/demandflow/e2e_test.go
internal/demandflow/types.go
scripts/dogfood-local.ps1
```

No `go.mod` or `go.sum` changes should be needed.

## Task 0: Preflight

**Files:** none

- [ ] Confirm branch and clean state.

```powershell
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-8
```

- [ ] Confirm baseline.

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

No commit for Task 0.

## Task 1: Let Quality Gates Run From A Separate Repository Root

**Files:**

- Modify: `internal/demandflow/types.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/engine_test.go`
- Modify: `internal/demandflow/e2e_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

### Why

Current `demandflow.Options.Root` is both the demand artifact root and the quality command working directory. That blocks real dogfood from keeping artifacts in a temporary workspace while running `go test ./...` against the actual repository checkout.

- [ ] Add `QualityRoot` to `internal/demandflow/types.go`.

Find `type Options struct` and add:

```go
type Options struct {
	Root            string
	QualityRoot     string
	DemandID        string
	Stage           Stage
	QualityCommands []quality.Command
	Runner          Runner
	Review          ReviewOptions
	Now             func() time.Time
}
```

If the field order differs, preserve existing fields and insert `QualityRoot string` directly after `Root string`.

- [ ] Add helper in `internal/demandflow/engine.go`.

Add near `NewEngine`:

```go
func qualityRoot(opts Options) string {
	if strings.TrimSpace(opts.QualityRoot) != "" {
		return opts.QualityRoot
	}
	return opts.Root
}
```

`engine.go` already imports `strings`, so no new import should be required.

- [ ] Use `qualityRoot(opts)` for quality gates.

In `runImplementation`, change:

```go
gateResult := e.Gate.Run(ctx, opts.Root, opts.QualityCommands...)
```

to:

```go
gateResult := e.Gate.Run(ctx, qualityRoot(opts), opts.QualityCommands...)
```

In `runVerification`, make the same change:

```go
gateResult := e.Gate.Run(ctx, qualityRoot(opts), opts.QualityCommands...)
```

- [ ] Add CLI flag in `internal/cli/run.go`.

Add variable:

```go
var root, qualityRoot, demandID, stage, configPath, permissionMode, gitlabProject, gitlabMR, gitlabBaseURL string
```

Add flag:

```go
fs.StringVar(&qualityRoot, "quality-root", "", "working directory for quality commands; defaults to --root")
```

Pass through options:

```go
opts := demandflow.Options{
	Root:            root,
	QualityRoot:     strings.TrimSpace(qualityRoot),
	DemandID:        demandID,
	Stage:           parsedStage,
	QualityCommands: commands,
	Runner:          newDemandRunner(configPath, permissions.PermissionMode(permissionMode)),
	Now:             time.Now,
}
```

- [ ] Add demandflow test proving quality root separation.

In `internal/demandflow/engine_test.go`, add a quality runner that records working directory if one does not already exist:

```go
type recordingQualityRunner struct {
	root string
}

func (r *recordingQualityRunner) Run(_ context.Context, root string, command quality.Command) quality.Result {
	r.root = root
	return quality.Result{Command: command.Name, Args: command.Args, ExitCode: 0, Stdout: "ok"}
}
```

If `quality.Runner` has a different method signature in `internal/quality/gate.go`, match the existing fake quality runner pattern already used in tests.

Add test:

```go
func TestImplementationQualityGateUsesQualityRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	repoRoot := t.TempDir()
	store := artifacts.NewStore(artifactRoot)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "quality-root-check",
		Title:       "Quality root check",
		Description: "Quality commands should run in repo root",
		Source:      "test",
		State:       string(workflow.Implementation),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	runner := &recordingQualityRunner{}
	engine := NewEngine(artifactRoot)
	engine.Gate = quality.Gate{Runner: runner}

	err := engine.Run(context.Background(), Options{
		Root:        artifactRoot,
		QualityRoot: repoRoot,
		DemandID:    "quality-root-check",
		Stage:       StageImplementation,
		Runner: &StaticRunner{Responses: map[Stage]RunnerResponse{
			StageImplementation: {Text: "implementation body"},
		}},
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test", "./..."}}},
		Now:             fixedNow,
	})
	if err != nil {
		t.Fatalf("implementation: %v", err)
	}
	if runner.root != repoRoot {
		t.Fatalf("quality root = %q, want %q", runner.root, repoRoot)
	}
}
```

- [ ] Add CLI test in `internal/cli/run_test.go`.

Use the existing `newDemandRunner` stub pattern and a temporary demand in `implementation` state. The test should call:

```go
err := Run([]string{
	"run",
	"--root", artifactRoot,
	"--quality-root", repoRoot,
	"--demand", "quality-root-check",
	"--stage", "implementation",
	"--quality-command", helperCommand,
}, &stdout, &stderr)
```

Assert:

```go
if err != nil {
	t.Fatalf("run implementation: %v", err)
}
if !strings.Contains(stdout.String(), "quality gate passed") {
	t.Fatalf("stdout = %q", stdout.String())
}
```

Use the repo's existing CLI helper command pattern for quality commands instead of adding a new helper executable.

- [ ] Format and verify.

```powershell
gofmt -w internal/demandflow/types.go internal/demandflow/engine.go internal/demandflow/engine_test.go internal/demandflow/e2e_test.go internal/cli/run.go internal/cli/run_test.go
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] Commit.

```powershell
git add internal/demandflow internal/cli
git commit -m @'
Run quality gates from the repository under test

Dogfood needs demand artifacts in an isolated workspace while quality
commands run against the actual repository checkout. QualityRoot makes
that boundary explicit without changing existing callers.

Constraint: Existing devflow run behavior must keep using --root when --quality-root is omitted
Rejected: Writing dogfood artifacts directly into the repo root | repeat runs collide and clutter .devflow
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

## Task 2: Extract Human Confirmation Into Demandflow

**Files:**

- Create: `internal/demandflow/confirmation.go`
- Create: `internal/demandflow/confirmation_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

### Why

Full dogfood must pass the same human gates as a user. Today confirmation behavior lives inside CLI internals, so non-CLI orchestration either has to duplicate it or manually edit state. Wave 8 should make confirmation a reusable product operation.

- [ ] Create `internal/demandflow/confirmation.go`.

```go
package demandflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ConfirmOptions struct {
	Root     string
	DemandID string
	Stage    string
	By       string
	Summary  string
	Now      func() time.Time
}

type ConfirmResult struct {
	DemandID      string
	Stage         string
	Label         string
	PreviousState workflow.State
	CurrentState  workflow.State
	Artifact      string
}

func Confirm(opts ConfirmOptions) (ConfirmResult, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	demandID := strings.TrimSpace(opts.DemandID)
	stage := strings.TrimSpace(opts.Stage)
	by := NormalizeConfirmationText(opts.By)
	summary := NormalizeConfirmationText(opts.Summary)
	if demandID == "" || stage == "" || by == "" || summary == "" {
		return ConfirmResult{}, fmt.Errorf("--demand, --stage, --by, and --summary are required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	artifactName, requiredCurrent, nextState, label, err := ConfirmationTarget(stage)
	if err != nil {
		return ConfirmResult{}, err
	}

	store := artifacts.NewStore(root)
	var result ConfirmResult
	err = store.WithDemandLock(demandID, func() error {
		demand, err := store.LoadDemand(demandID)
		if err != nil {
			return err
		}

		current := workflow.State(demand.State)
		result = ConfirmResult{
			DemandID:      demandID,
			Stage:         stage,
			Label:         label,
			PreviousState: current,
			Artifact:      artifactName,
		}
		if current != requiredCurrent {
			return fmt.Errorf("confirmation stage %q requires current state %s, got %s", stage, requiredCurrent, current)
		}

		advanced, err := workflow.Advance(current, nextState)
		if err != nil {
			return err
		}

		confirmedAt := opts.Now().UTC()
		cycleToken := demand.UpdatedAt.UTC().Format(time.RFC3339Nano)
		confirmationID := ConfirmationID(demandID, stage, cycleToken, by, summary)
		record := fmt.Sprintf("- %s confirmed by %s at %s: %s\n", label, by, confirmedAt.Format(time.RFC3339), summary)
		if err := store.EnsureConfirmationEvidence(demandID, artifactName, confirmationID, record, artifacts.Event{
			Time:    confirmedAt,
			Type:    "stage.confirmed",
			Message: label + " confirmed",
			Data: map[string]string{
				"by":              by,
				"stage":           stage,
				"summary":         summary,
				"confirmation_id": confirmationID,
			},
		}); err != nil {
			return err
		}

		demand.State = string(advanced)
		if err := store.SaveDemand(demand); err != nil {
			return err
		}
		result.CurrentState = advanced
		return nil
	})
	return result, err
}

func ConfirmationTarget(stage string) (artifact string, requiredCurrent workflow.State, next workflow.State, label string, err error) {
	switch stage {
	case "requirements":
		return artifacts.RequirementsFile, workflow.RequirementsReview, workflow.PlanDrafting, "requirements", nil
	case "plan":
		return artifacts.PlanFile, workflow.PlanReview, workflow.Implementation, "plan", nil
	case "verification":
		return artifacts.VerificationFile, workflow.Verification, workflow.Closeout, "verification", nil
	case "closeout":
		return artifacts.CloseoutFile, workflow.Closeout, workflow.Completed, "closeout", nil
	default:
		return "", "", "", "", fmt.Errorf("unsupported confirmation stage %q", stage)
	}
}

func NormalizeConfirmationText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func ConfirmationID(demandID, stage, cycleToken, by, summary string) string {
	normalizedDemandID := strings.ToLower(strings.TrimSpace(demandID))
	normalizedStage := strings.TrimSpace(stage)
	normalizedCycleToken := strings.TrimSpace(cycleToken)
	normalizedBy := NormalizeConfirmationText(by)
	normalizedSummary := NormalizeConfirmationText(summary)

	hash := sha256.Sum256([]byte(normalizedDemandID + "\x00" + normalizedStage + "\x00" + normalizedCycleToken + "\x00" + normalizedBy + "\x00" + normalizedSummary))
	return hex.EncodeToString(hash[:8])
}
```

- [ ] Add tests in `internal/demandflow/confirmation_test.go`.

```go
package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestConfirmAdvancesRequirementsAndWritesEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "confirm-requirements",
		Title:       "Confirm requirements",
		Description: "Confirm requirements",
		Source:      "test",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	setDemandState(t, store, "confirm-requirements", workflow.RequirementsReview)

	result, err := Confirm(ConfirmOptions{
		Root:     root,
		DemandID: "confirm-requirements",
		Stage:    "requirements",
		By:       "alice\nadmin",
		Summary:  "looks\nright",
		Now:      fixedNow,
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if result.PreviousState != workflow.RequirementsReview || result.CurrentState != workflow.PlanDrafting {
		t.Fatalf("states = %s -> %s, want requirements_review -> plan_drafting", result.PreviousState, result.CurrentState)
	}
	demand, err := store.LoadDemand("confirm-requirements")
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("state = %q, want %q", demand.State, workflow.PlanDrafting)
	}
	body, err := os.ReadFile(filepath.Join(store.DemandDir("confirm-requirements"), artifacts.RequirementsFile))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "requirements confirmed by alice admin") {
		t.Fatalf("requirements evidence missing normalized confirmer:\n%s", text)
	}
	if !strings.Contains(text, "looks right") {
		t.Fatalf("requirements evidence missing normalized summary:\n%s", text)
	}
}

func TestConfirmRejectsWrongStateWithoutMutating(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "wrong-state",
		Title:       "Wrong state",
		Description: "Wrong state",
		Source:      "test",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	_, err := Confirm(ConfirmOptions{Root: root, DemandID: "wrong-state", Stage: "plan", By: "alice", Summary: "ok", Now: fixedNow})
	if err == nil || !strings.Contains(err.Error(), "requires current state plan_review") {
		t.Fatalf("err = %v, want wrong-state error", err)
	}
	demand, loadErr := store.LoadDemand("wrong-state")
	if loadErr != nil {
		t.Fatalf("load demand: %v", loadErr)
	}
	if demand.State != string(workflow.Created) {
		t.Fatalf("state mutated to %q", demand.State)
	}
}

func TestConfirmationIDIsStableAndSensitiveToCycleToken(t *testing.T) {
	left := ConfirmationID("demand", "requirements", "cycle-one", "alice", "ok")
	right := ConfirmationID("demand", "requirements", "cycle-two", "alice", "ok")
	if left == right {
		t.Fatalf("confirmation ids should differ across cycles: %s", left)
	}
	if again := ConfirmationID("demand", "requirements", "cycle-one", "alice", "ok"); again != left {
		t.Fatalf("confirmation id not stable: %s then %s", left, again)
	}
}
```

- [ ] Refactor `internal/cli/cli.go` to use demandflow confirmation.

Add import:

```go
"github.com/jesseedcp/devflow-agent/internal/demandflow"
```

Replace `runConfirm` body after flag parsing with:

```go
result, err := demandflow.Confirm(demandflow.ConfirmOptions{
	Root:     root,
	DemandID: demandID,
	Stage:    stage,
	By:       by,
	Summary:  summary,
	Now:      time.Now,
})
if err != nil {
	return err
}
_, err = fmt.Fprintf(stdout, "%s confirmed for %s\n", result.Label, demandID)
return err
```

Remove these CLI-local helpers if no longer used:

```go
confirmationTarget
normalizeConfirmationText
confirmationID
```

Update any tests that call the old CLI-local helpers to call:

```go
demandflow.NormalizeConfirmationText(...)
demandflow.ConfirmationID(...)
```

or move expected helper logic into tests.

- [ ] Format and verify.

```powershell
gofmt -w internal/demandflow/confirmation.go internal/demandflow/confirmation_test.go internal/cli/cli.go internal/cli/cli_test.go
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] Commit.

```powershell
git add internal/demandflow internal/cli
git commit -m @'
Make human confirmations reusable outside the CLI

The full dogfood runner needs to pass the same confirmation gates as a
user without manually editing demand state. Moving confirmation into
demandflow keeps the CLI thin and gives orchestration code one shared
gate implementation.

Constraint: Confirmation evidence format must remain compatible with existing demand artifacts
Rejected: Manually setting workflow states in dogfood | would skip the product's human gates
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

## Task 3: Add Deterministic Full-Loop Dogfood Orchestrator

**Files:**

- Create: `internal/dogfood/scenario.go`
- Create: `internal/dogfood/runner.go`
- Create: `internal/dogfood/runner_test.go`

- [ ] Create `internal/dogfood/scenario.go`.

```go
package dogfood

import "github.com/jesseedcp/devflow-agent/internal/demandflow"

type Scenario struct {
	Name        string
	DemandID    string
	Title       string
	Description string
	Responses   map[demandflow.Stage]demandflow.RunnerResponse
}

func CouponEligibilityScenario() Scenario {
	return Scenario{
		Name:        "coupon-eligibility",
		DemandID:    "dogfood-coupon-eligibility",
		Title:       "Dogfood coupon eligibility",
		Description: "Only active members can claim coupons once",
		Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageRequirements: {
				Text: "# Requirements: Dogfood coupon eligibility\n\n## Business Rules\n\n- Active members can claim an unexpired coupon once.\n- Inactive users are rejected with a clear reason.\n- Missing coupons are rejected with a clear reason.\n- Expired coupons are rejected with a clear reason.\n- Duplicate claims are rejected with a clear reason.\n",
			},
			demandflow.StagePlan: {
				Text: "# Technical Plan: Dogfood coupon eligibility\n\n## Implementation Shape\n\n- Add an eligibility service that checks user status, coupon existence, expiration, and duplicate claims.\n- Cover each rejection reason with focused unit tests.\n- Keep persistence behind repository interfaces so policy can be tested without a database.\n",
			},
			demandflow.StageImplementation: {
				Text:        "## Implementation Summary\n\nDeterministic dogfood records the intended backend implementation path without editing source files.\n",
				ToolSummary: []string{"quality gate executed against repository root"},
			},
			demandflow.StageVerification: {
				Text: "# Verification: Dogfood coupon eligibility\n\n## Evidence\n\n- Requirements, plan, implementation notes, MR review gate, quality gate, and closeout were exercised by the deterministic dogfood runner.\n",
			},
			demandflow.StageCloseout: {
				Text: "# Closeout: Dogfood coupon eligibility\n\n## Demand Result\n\nThe full backend-demand workflow completed deterministically for the coupon eligibility scenario.\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: Dogfood coupon eligibility\n\n## Stable Knowledge Candidates\n\n- Deterministic dogfood should cover every workflow gate before a release is considered ready.\n",
			},
		},
	}
}

func ScenarioByName(name string) (Scenario, bool) {
	switch name {
	case "", "coupon-eligibility":
		return CouponEligibilityScenario(), true
	default:
		return Scenario{}, false
	}
}
```

- [ ] Create `internal/dogfood/runner.go`.

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

type Options struct {
	Root            string
	QualityRoot     string
	ScenarioName    string
	QualityCommands []quality.Command
	Now             func() time.Time
}

type Result struct {
	Root       string
	QualityRoot string
	DemandID   string
	FinalState workflow.State
	ReportPath string
	Steps      []Step
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
```

- [ ] Create `internal/dogfood/runner_test.go`.

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

func TestRunCompletesFullDeterministicLoop(t *testing.T) {
	root := t.TempDir()
	qualityRoot := t.TempDir()
	result, err := Run(context.Background(), Options{
		Root:        root,
		QualityRoot: qualityRoot,
		QualityCommands: []quality.Command{{
			Name: testHelperExecutable(t),
			Args: []string{"-test.run=^TestDogfoodHelper$"},
		}},
		Now: fixedDogfoodNow,
	})
	if err != nil {
		t.Fatalf("dogfood run: %v", err)
	}
	if result.FinalState != workflow.Completed {
		t.Fatalf("final state = %s, want completed", result.FinalState)
	}
	if result.ReportPath == "" {
		t.Fatal("report path is empty")
	}
	report, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{"requirements", "confirm requirements", "plan", "implementation", "mr-review", "verification", "closeout", "completed"} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("report missing %q:\n%s", want, reportText)
		}
	}

	demandDir := filepath.Join(root, ".devflow", "demands", "dogfood-coupon-eligibility")
	for _, name := range []string{
		artifacts.RequirementsFile,
		artifacts.PlanFile,
		artifacts.ProgressFile,
		artifacts.VerificationFile,
		artifacts.CloseoutFile,
		artifacts.MemoryCandidatesFile,
		"dogfood-report.md",
	} {
		if _, err := os.Stat(filepath.Join(demandDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}
}

func TestRunRejectsUnknownScenario(t *testing.T) {
	_, err := Run(context.Background(), Options{Root: t.TempDir(), QualityRoot: t.TempDir(), ScenarioName: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported dogfood scenario") {
		t.Fatalf("err = %v, want unsupported scenario", err)
	}
}

func testHelperExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return exe
}
```

Add helper test in the same file:

```go
func TestDogfoodHelper(t *testing.T) {
	if os.Getenv("DEVFLOW_DOGFOOD_HELPER") != "1" {
		return
	}
	os.Exit(0)
}
```

Before running `Run`, set:

```go
t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
```

in `TestRunCompletesFullDeterministicLoop`.

If Go re-enters the full test binary instead of only helper, use the existing CLI helper pattern from `internal/cli/cli_test.go`.

- [ ] Format and verify.

```powershell
gofmt -w internal/dogfood/scenario.go internal/dogfood/runner.go internal/dogfood/runner_test.go
go test ./internal/dogfood -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] Commit.

```powershell
git add internal/dogfood
git commit -m @'
Run the backend demand loop as deterministic dogfood

The dogfood package composes existing demandflow stages, static agent
outputs, real quality gates, offline MR review, human confirmations, and
a report so the full product loop can be exercised without provider
credentials.

Constraint: Deterministic dogfood must run without network or API keys
Rejected: Extending the smoke command only to requirements | still leaves most workflow gates untested
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/dogfood -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

## Task 4: Expose Full Dogfood Through `devflow dogfood`

**Files:**

- Create: `internal/cli/dogfood.go`
- Create: `internal/cli/dogfood_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

- [ ] Update help in `internal/cli/cli.go`.

Add usage:

```text
  devflow dogfood [--scenario coupon-eligibility] [--quality-command <command>]
```

Add command description:

```text
  dogfood  Run a deterministic full backend-demand loop
```

Add dispatch:

```go
case "dogfood":
	return runDogfood(args[1:], stdout, stderr)
```

- [ ] Create `internal/cli/dogfood.go`.

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/dogfood"
	"github.com/jesseedcp/devflow-agent/internal/quality"
)

func runDogfood(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("dogfood", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var root, qualityRoot, scenario string
	var qualityCommands stringSliceFlag
	fs.StringVar(&root, "root", "", "demand artifact root; defaults to a new temp directory")
	fs.StringVar(&qualityRoot, "quality-root", ".", "working directory for quality commands")
	fs.StringVar(&scenario, "scenario", "coupon-eligibility", "dogfood scenario")
	fs.Var(&qualityCommands, "quality-command", "quality command as a quoted program and args (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var commands []quality.Command
	for _, raw := range qualityCommands {
		parts, err := parseCommandLine(raw)
		if err != nil {
			return fmt.Errorf("parse --quality-command %q: %w", raw, err)
		}
		if len(parts) == 0 {
			continue
		}
		commands = append(commands, quality.Command{Name: parts[0], Args: parts[1:]})
	}
	if strings.TrimSpace(qualityRoot) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		qualityRoot = wd
	}

	result, err := dogfood.Run(context.Background(), dogfood.Options{
		Root:            root,
		QualityRoot:     qualityRoot,
		ScenarioName:    scenario,
		QualityCommands: commands,
		Now:             time.Now,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "dogfood completed for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "state: %s\n", result.FinalState)
	fmt.Fprintf(stdout, "root: %s\n", result.Root)
	fmt.Fprintf(stdout, "quality-root: %s\n", result.QualityRoot)
	fmt.Fprintf(stdout, "report: %s\n", result.ReportPath)
	return nil
}
```

- [ ] Create `internal/cli/dogfood_test.go`.

```go
package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDogfoodCommandCompletesFullLoop(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	helper := testCLIExecutable(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"dogfood",
		"--root", root,
		"--quality-root", t.TempDir(),
		"--quality-command", `"` + helper + `" -test.run=^TestCLICommandHelper$`,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dogfood: %v\nstderr:\n%s", err, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"dogfood completed", "state: completed", "report:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestDogfoodHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"devflow dogfood", "dogfood  Run a deterministic full backend-demand loop"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}

func TestDogfoodRejectsUnknownScenario(t *testing.T) {
	err := Run([]string{"dogfood", "--root", t.TempDir(), "--scenario", "unknown"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported dogfood scenario") {
		t.Fatalf("err = %v, want unsupported scenario", err)
	}
}

func TestCLICommandHelper(t *testing.T) {
	if os.Getenv("DEVFLOW_DOGFOOD_HELPER") == "1" {
		os.Exit(0)
	}
}
```

If `TestCLICommandHelper` already exists in `cli_test.go`, do not add a duplicate; reuse the existing helper and only set the env var expected by it.

- [ ] Format and verify.

```powershell
gofmt -w internal/cli/cli.go internal/cli/dogfood.go internal/cli/dogfood_test.go internal/cli/cli_test.go
go test ./internal/cli -count=1
go test ./internal/dogfood -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] Commit.

```powershell
git add internal/cli
git commit -m @'
Expose full-loop dogfood from the CLI

The dogfood command gives release checks one command that exercises the
complete backend-demand workflow without model or GitLab credentials.

Constraint: CLI dogfood must remain deterministic and local by default
Rejected: Requiring devflow smoke plus manual commands | too easy to skip gates or miss artifacts
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/cli -count=1; go test ./internal/dogfood -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

## Task 5: Update PowerShell Dogfood Script To Run The Full Loop

**Files:**

- Modify: `scripts/dogfood-local.ps1`

- [ ] Replace the start/status/next section with `devflow dogfood`.

Keep the build and `-UseExistingBinary` behavior from Wave 7. After `& $binary version`, replace the `init`, `start`, `status`, and `next` calls with:

```powershell
$dogfoodRoot = Join-Path $rootPath 'artifacts'
New-Item -ItemType Directory -Force -Path $dogfoodRoot | Out-Null

& $binary dogfood --root $dogfoodRoot --quality-root $repoRoot --quality-command "go test ./... -count=1 -timeout 5m"
if ($LASTEXITCODE -ne 0) { throw "devflow dogfood failed" }

$report = Join-Path $dogfoodRoot '.devflow\demands\dogfood-coupon-eligibility\dogfood-report.md'
if (-not (Test-Path -LiteralPath $report)) {
    throw "dogfood report missing: $report"
}

Write-Host "dogfood root: $dogfoodRoot"
Write-Host "dogfood report: $report"
```

Do not delete `$rootPath` safety checks. The script should still remove only roots inside the system temp directory.

- [ ] Run script.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave8
```

Expected output includes:

```text
version: 0.1.0-wave8
dogfood completed for dogfood-coupon-eligibility
state: completed
dogfood report:
```

- [ ] Verify.

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] Commit.

```powershell
git add scripts/dogfood-local.ps1
git commit -m @'
Make local dogfood exercise the complete workflow

The PowerShell dogfood script now validates the full deterministic
backend-demand loop instead of stopping after demand creation and next
action guidance.

Constraint: Script cleanup must remain confined to the temp directory
Rejected: Keeping dogfood-local as a start/status smoke | no longer proves Wave 8 release readiness
Confidence: high
Scope-risk: narrow
Tested: powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\dogfood-local.ps1 -Version 0.1.0-wave8; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

## Task 6: Document Full-Loop Dogfood Evidence

**Files:**

- Create: `docs/user-guide/full-loop-dogfood.md`
- Create: `docs/examples/dogfood/coupon-eligibility-report.md`
- Modify: `docs/user-guide/dogfood-smoke.md`
- Modify: `docs/release/v0.1.md`
- Modify: `README.md`

- [ ] Create `docs/user-guide/full-loop-dogfood.md`.

````markdown
# Full-Loop Dogfood Guide

Run this before a release branch is merged:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-dev
```

This command rebuilds the CLI, creates an isolated dogfood workspace under the system temp directory, runs `devflow dogfood`, and prints the generated `dogfood-report.md` path.

## What It Proves

- A demand can be created from a realistic backend requirement.
- Requirements and plan stages can be drafted.
- Human confirmation gates can be recorded without manually editing state.
- Implementation and verification can run real quality commands against the repository checkout.
- MR review can be represented by an offline no-blocker adapter.
- Closeout and memory candidates can be drafted.
- The final demand state reaches `completed`.
- A report captures state transitions, artifact paths, and quality evidence.

## What It Does Not Prove

- Live model provider quality.
- Real GitLab MR API behavior.
- That generated implementation code is correct for production.

Those remain separate live dogfood or integration checks.
````

- [ ] Create `docs/examples/dogfood/coupon-eligibility-report.md`.

Use a sanitized sample based on expected report shape:

```markdown
# Dogfood Report: dogfood-coupon-eligibility

Root: `<temp>\devflow-dogfood-local\artifacts`

QualityRoot: `<repo>\devflow-agent`

FinalState: `completed`

## Steps

- `requirements` -> `requirements_review`: requirements drafted by demand runner
- `confirm requirements` -> `plan_drafting`: deterministic dogfood requirements accepted
- `plan` -> `plan_review`: plan drafted by demand runner
- `confirm plan` -> `implementation`: deterministic dogfood plan accepted
- `implementation` -> `mr_review`: implementation completed and quality gate passed
- `mr-review` -> `verification`: mr review cleared, no blocking unresolved comments
- `verification` -> `verification`: verification drafted by demand runner
- `confirm verification` -> `closeout`: deterministic dogfood verification accepted
- `closeout` -> `closeout`: closeout and memory candidates drafted by demand runner
- `confirm closeout` -> `completed`: deterministic dogfood closeout accepted

## Artifacts

- `requirements.md`
- `plan.md`
- `progress.md`
- `verification.md`
- `closeout.md`
- `memory-candidates.md`
- `events.jsonl`
```

- [ ] Update `docs/user-guide/dogfood-smoke.md`.

Add a top note:

```markdown
For release readiness, prefer the full-loop dogfood guide. This page remains for first-step smoke and optional live provider checks.

- [Full-loop dogfood guide](full-loop-dogfood.md)
```

- [ ] Update `docs/release/v0.1.md`.

Add `devflow dogfood` to the feature list:

```markdown
- Full deterministic dogfood through `devflow dogfood`.
```

Update verification commands:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0
```

Add a note:

```markdown
The dogfood script must finish with `state: completed`.
```

- [ ] Update `README.md`.

Add command to Wave 7/Wave 8 command list:

```text
devflow dogfood
```

Add docs link:

```markdown
- [Full-loop dogfood guide](docs/user-guide/full-loop-dogfood.md)
```

- [ ] Verify docs.

```powershell
git diff --check
```

Expected: exit 0.

- [ ] Commit.

```powershell
git add README.md docs/user-guide/full-loop-dogfood.md docs/user-guide/dogfood-smoke.md docs/release/v0.1.md docs/examples/dogfood/coupon-eligibility-report.md
git commit -m @'
Document the full-loop dogfood release gate

Wave 8 changes dogfood from a first-step smoke into a release-readiness
gate, so docs now describe what it proves, what it does not prove, and
where to find the evidence report.

Constraint: Documentation must keep live-provider and deterministic dogfood distinct
Rejected: Calling the old smoke guide sufficient for release | it does not cover full workflow completion
Confidence: high
Scope-risk: narrow
Tested: git diff --check
'@
```

## Task 7: Final Verification And PR

**Files:** none unless final fixes are needed

- [ ] Run full verification.

```powershell
go test ./internal/demandflow -count=1
go test ./internal/dogfood -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-wave8 -Output dist\devflow-windows-amd64.exe
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave8
git diff --check
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-8
```

Ignored local `dist/`, `.devflow/`, and temp dogfood outputs should not appear.

- [ ] Push branch.

```powershell
git push -u origin feature/devflow-wave-8
```

- [ ] Create PR.

```powershell
gh pr create --base main --head feature/devflow-wave-8 --title "Wave 8: full-loop deterministic dogfood" --body @'
## Summary
- Separates demand artifact root from quality command root.
- Moves human confirmation gates into reusable demandflow code.
- Adds deterministic full-loop dogfood and `devflow dogfood`.
- Updates local dogfood script and release docs to require completed-state evidence.

## Test Plan
- [ ] go test ./internal/demandflow -count=1
- [ ] go test ./internal/dogfood -count=1
- [ ] go test ./internal/cli -count=1
- [ ] go test ./... -count=1 -timeout 5m
- [ ] go vet ./...
- [ ] go build ./cmd/devflow
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-wave8 -Output dist\devflow-windows-amd64.exe
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave8
- [ ] git diff --check
'@
```

Do not merge until local verification and CI pass.

## Definition Of Done

Wave 8 is complete when:

- `devflow dogfood` exists and runs the deterministic coupon-eligibility scenario.
- Dogfood artifacts are written under an isolated root while quality commands run against the repository root.
- The full workflow reaches `completed`.
- Requirements, plan, progress, verification, closeout, memory candidates, events, and dogfood report are generated.
- Human confirmations are recorded through shared demandflow code, not manual state edits.
- `scripts/dogfood-local.ps1` runs full-loop dogfood and prints `dogfood-report.md`.
- Docs explain full-loop dogfood and distinguish it from live provider smoke.
- Full verification passes:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave8
git diff --check
```

- A PR from `feature/devflow-wave-8` to `main` is open.

## Self-Review Notes

Spec coverage:

- Demand artifact root vs quality root is covered by Task 1.
- Human confirmation reuse is covered by Task 2.
- Full deterministic workflow orchestration is covered by Task 3.
- User-facing CLI command is covered by Task 4.
- PowerShell release dogfood path is covered by Task 5.
- Documentation and sample evidence are covered by Task 6.
- Final verification and PR are covered by Task 7.

Placeholder scan:

- No step uses unfinished-marker wording or open-ended implementation instructions.
- Commands use concrete paths, versions, scenario names, states, and expected output.
- Live provider work remains explicitly out of scope.

Type consistency:

- `demandflow.Options.QualityRoot` is introduced before any dogfood package uses it.
- `demandflow.Confirm` is introduced before `internal/dogfood` uses confirmation gates.
- `dogfood.Options` and `dogfood.Result` are defined before CLI uses them.
- `scripts/dogfood-local.ps1` calls `devflow dogfood` only after the CLI command is added.

# Wave 9 Live Sandbox And GitLab Release Gates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in live release-readiness layer that proves Devflow can use a real provider in a safe sandbox, run real quality gates against editable code, and check real GitLab MR review gates without weakening the deterministic dogfood path.

**Architecture:** Keep Wave 8 deterministic dogfood as the default release gate. Add a separate live sandbox path that uses a temporary Go fixture repo as the editable agent workspace and a separate `.devflow` artifact root for demand materials. Add a direct GitLab review-gate CLI command so real MR readiness can be checked without constructing a full demand state by hand. All live checks must be opt-in and must skip or fail clearly when credentials are absent; CI remains deterministic.

**Tech Stack:** Go 1.25.0, existing `internal/demandflow`, existing `internal/runtime` MewCode-derived agent stack, existing `internal/adapters.GitLabReviewAdapter`, PowerShell scripts, no new Go dependencies.

---

## Current Environment

Repository:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
```

Base branch:

```text
main
```

Recommended Wave 9 worktree:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-wave-9
```

Recommended branch:

```text
feature/devflow-wave-9
```

Starting point after Wave 8:

```text
0d96347 Merge Wave 8 deterministic dogfood into main
```

## Product Thesis

Wave 8 proves the workflow shape deterministically. Wave 9 should prove the next risky boundary:

```text
devflow live-dogfood
  -> creates a temporary code sandbox containing a tiny Go coupon service
  -> creates demand artifacts in a separate artifact root
  -> uses the configured real provider through RuntimeRunner
  -> drafts requirements and plan
  -> confirms requirements and plan
  -> lets the agent edit only the sandbox code root during implementation
  -> runs go test ./... against the sandbox repo
  -> optionally checks a real GitLab MR for unresolved blocking comments
  -> drafts verification and closeout
  -> writes live-dogfood-report.md
```

The live path is intentionally separate from deterministic dogfood:

- Deterministic dogfood remains the default release gate and CI-safe.
- Live provider dogfood is opt-in through `DEVFLOW_LIVE_DOGFOOD=1`.
- Live GitLab gate is opt-in through `DEVFLOW_LIVE_GITLAB=1` or explicit CLI invocation.
- No secrets are written to disk, logs, reports, docs, or committed files.

## Scope

In scope:

- Separate demand artifact root from agent runner working directory.
- Add `devflow review-gate` for direct GitLab MR unresolved-comment checks.
- Add a tiny live sandbox Go fixture that can be edited safely by the agent.
- Add `internal/dogfood.RunLiveSandbox` using real RuntimeRunner when explicitly enabled.
- Add `devflow live-dogfood` CLI.
- Add PowerShell release-readiness wrapper with deterministic checks and optional live checks.
- Add docs explaining deterministic vs live gates and exact credential setup.

Out of scope:

- Automatically creating GitLab MRs.
- Running live provider checks in CI by default.
- Editing the real Devflow repository during live dogfood implementation.
- Frontend, PD Agent, or Test Agent.
- New provider SDKs or new Go dependencies.

## File Map

Create:

```text
internal/cli/review_gate.go
internal/cli/review_gate_test.go
internal/cli/live_dogfood.go
internal/cli/live_dogfood_test.go
internal/dogfood/sandbox.go
internal/dogfood/sandbox_test.go
internal/dogfood/live.go
internal/dogfood/live_test.go
scripts/release-readiness.ps1
docs/user-guide/live-dogfood.md
docs/examples/dogfood/live-sandbox-report.md
```

Modify:

```text
README.md
docs/release/v0.1.md
docs/user-guide/backend-demand-loop.md
docs/user-guide/dogfood-smoke.md
internal/cli/cli.go
internal/cli/run.go
internal/cli/run_test.go
internal/demandflow/engine.go
internal/demandflow/engine_test.go
internal/demandflow/types.go
internal/dogfood/runner.go
internal/dogfood/runner_test.go
```

No `go.mod` or `go.sum` changes should be needed.

---

## Task 0: Worktree And Baseline

**Files:** none

- [ ] From the main repo, fetch and create the isolated worktree.

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin main
git worktree add .worktrees\devflow-wave-9 -b feature/devflow-wave-9 origin/main
cd .worktrees\devflow-wave-9
```

Expected:

```text
Preparing worktree (new branch 'feature/devflow-wave-9')
HEAD is now at 0d96347 Merge Wave 8 deterministic dogfood into main
```

- [ ] Download modules and verify baseline.

```powershell
go mod download
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-9
```

No commit for Task 0.

---

## Task 1: Separate Agent Runner Root From Demand Artifact Root

**Files:**

- Modify: `internal/demandflow/types.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/engine_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

### Why

Wave 8 added `QualityRoot`, but live dogfood needs one more boundary:

- demand artifacts should live under an isolated `.devflow` artifact root;
- quality commands should run in the sandbox repo;
- the RuntimeRunner agent should edit the sandbox repo, not the artifact root.

That requires `RunnerRoot`.

- [ ] Add `RunnerRoot` to `internal/demandflow/types.go`.

Add the field directly after `Root`:

```go
type Options struct {
	Root            string
	RunnerRoot      string
	QualityRoot     string
	DemandID        string
	Stage           Stage
	QualityCommands []quality.Command
	Runner          Runner
	Review          ReviewOptions
	Now             func() time.Time
}
```

- [ ] Add helper in `internal/demandflow/engine.go`.

Place near `qualityRoot`:

```go
func runnerRoot(opts Options) string {
	if strings.TrimSpace(opts.RunnerRoot) != "" {
		return opts.RunnerRoot
	}
	return opts.Root
}
```

- [ ] Replace all `RunnerRequest{Root: opts.Root}` values with `runnerRoot(opts)`.

In these methods:

```text
runRequirements
runPlan
runImplementation
runVerification
runCloseout
```

Each request should look like:

```go
resp, err := opts.Runner.Run(ctx, RunnerRequest{
	Stage:    StageImplementation,
	Root:     runnerRoot(opts),
	DemandID: opts.DemandID,
	Prompt:   prompt,
	Context:  snapshot,
	ToolMode: toolMode,
})
```

Use the matching stage and tool mode for each method.

- [ ] Add test in `internal/demandflow/engine_test.go`.

Add this fake runner:

```go
type recordingDemandRunner struct {
	root string
}

func (r *recordingDemandRunner) Run(_ context.Context, req RunnerRequest) (RunnerResponse, error) {
	r.root = req.Root
	return RunnerResponse{Text: "# Requirements\n\nrunner root recorded\n"}, nil
}
```

Add this test:

```go
func TestRequirementsRunnerUsesRunnerRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	codeRoot := t.TempDir()
	store := artifacts.NewStore(artifactRoot)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "runner-root-check",
		Title:       "Runner root check",
		Description: "Agent should run in code root",
		Source:      "test",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	runner := &recordingDemandRunner{}
	engine := NewEngine(artifactRoot)
	err := engine.Run(context.Background(), Options{
		Root:       artifactRoot,
		RunnerRoot: codeRoot,
		DemandID:   "runner-root-check",
		Stage:      StageRequirements,
		Runner:     runner,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("requirements: %v", err)
	}
	if runner.root != codeRoot {
		t.Fatalf("runner root = %q, want %q", runner.root, codeRoot)
	}
}
```

- [ ] Add CLI flag in `internal/cli/run.go`.

Change the variable declaration:

```go
var root, runnerRoot, qualityRoot, demandID, stage, configPath, permissionMode, gitlabProject, gitlabMR, gitlabBaseURL string
```

Add flag:

```go
fs.StringVar(&runnerRoot, "runner-root", "", "working directory for agent tools; defaults to --root")
```

Pass through:

```go
opts := demandflow.Options{
	Root:            root,
	RunnerRoot:      strings.TrimSpace(runnerRoot),
	QualityRoot:     strings.TrimSpace(qualityRoot),
	DemandID:        demandID,
	Stage:           parsedStage,
	QualityCommands: commands,
	Runner:          newDemandRunner(configPath, permissions.PermissionMode(permissionMode)),
	Now:             time.Now,
}
```

- [ ] Add CLI test in `internal/cli/run_test.go`.

Add a fake runner local to the file if one does not already exist:

```go
type cliRecordingRunner struct {
	root string
}

func (r *cliRecordingRunner) Run(_ context.Context, req demandflow.RunnerRequest) (demandflow.RunnerResponse, error) {
	r.root = req.Root
	return demandflow.RunnerResponse{Text: "# Requirements\n\ncli runner root recorded\n"}, nil
}
```

Add test:

```go
func TestRunUsesRunnerRootForDemandRunner(t *testing.T) {
	artifactRoot := t.TempDir()
	codeRoot := t.TempDir()
	createDemandAtState(t, artifactRoot, workflow.Created)

	recorder := &cliRecordingRunner{}
	original := newDemandRunner
	defer func() { newDemandRunner = original }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return recorder
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"run",
		"--root", artifactRoot,
		"--runner-root", codeRoot,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if recorder.root != codeRoot {
		t.Fatalf("runner root = %q, want %q", recorder.root, codeRoot)
	}
}
```

Ensure `internal/cli/run_test.go` imports `context` if needed.

- [ ] Format and verify.

```powershell
gofmt -w internal/demandflow/types.go internal/demandflow/engine.go internal/demandflow/engine_test.go internal/cli/run.go internal/cli/run_test.go
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/demandflow internal/cli
git commit -m @'
Run agent tools from an explicit runner root

Live dogfood needs demand materials and editable code to live in separate
directories. RunnerRoot gives RuntimeRunner a code workspace while keeping
artifact storage under the demand root.

Constraint: Existing devflow run behavior must keep using --root when --runner-root is omitted
Rejected: Reusing QualityRoot for agent tools | quality commands and agent editing have different safety boundaries
Confidence: high
Scope-risk: moderate
Directive: Use Root for demand artifacts, RunnerRoot for agent tools, and QualityRoot for verification commands
Tested: go test ./internal/demandflow -count=1; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 2: Add Direct GitLab Review Gate CLI

**Files:**

- Create: `internal/cli/review_gate.go`
- Create: `internal/cli/review_gate_test.go`
- Modify: `internal/cli/cli.go`

### Why

The workflow `mr-review` stage requires a demand already in `mr_review`. Release readiness also needs a direct way to check a real MR. `devflow review-gate` should call the existing GitLab adapter and exit nonzero when unresolved blocking comments remain.

- [ ] Update help and dispatch in `internal/cli/cli.go`.

Add usage line:

```text
  devflow review-gate --gitlab-project <project> --gitlab-mr <iid>
```

Add command description:

```text
  review-gate Check unresolved GitLab MR comments directly
```

Add switch case:

```go
case "review-gate":
	return runReviewGate(args[1:], stdout, stderr)
```

- [ ] Create `internal/cli/review_gate.go`.

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func runReviewGate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("review-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var project, mr, baseURL string
	fs.StringVar(&project, "gitlab-project", "", "GitLab project path or id")
	fs.StringVar(&mr, "gitlab-mr", "", "GitLab merge request iid")
	fs.StringVar(&baseURL, "gitlab-base-url", "", "GitLab base url override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project = strings.TrimSpace(project)
	mr = strings.TrimSpace(mr)
	if project == "" || mr == "" {
		return fmt.Errorf("--gitlab-project and --gitlab-mr are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	comments, err := newReviewAdapter().ListUnresolved(ctx, adapters.ReviewRef{
		Project:      project,
		MergeRequest: mr,
		BaseURL:      baseURL,
	})
	if err != nil {
		return err
	}
	if len(comments) == 0 {
		_, err := fmt.Fprintf(stdout, "review gate passed for %s!%s: no unresolved blocking comments\n", project, mr)
		return err
	}

	fmt.Fprintf(stdout, "review gate blocked for %s!%s: %d unresolved blocking comment(s)\n", project, mr, len(comments))
	for _, comment := range comments {
		location := comment.FilePath
		if comment.Line > 0 {
			location = fmt.Sprintf("%s:%d", comment.FilePath, comment.Line)
		}
		if strings.TrimSpace(location) == "" {
			location = "(no file location)"
		}
		fmt.Fprintf(stdout, "- %s by %s: %s\n", location, comment.Author, strings.TrimSpace(comment.Body))
	}
	return fmt.Errorf("review gate blocked by unresolved GitLab comments")
}
```

- [ ] Create `internal/cli/review_gate_test.go`.

```go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

type fakeReviewGateAdapter struct {
	comments []adapters.ReviewComment
	ref      adapters.ReviewRef
}

func (f *fakeReviewGateAdapter) ListUnresolved(_ context.Context, ref adapters.ReviewRef) ([]adapters.ReviewComment, error) {
	f.ref = ref
	return f.comments, nil
}

func (f *fakeReviewGateAdapter) Reply(context.Context, adapters.ReviewRef, string, string) error {
	return nil
}

func TestReviewGatePassesWithoutUnresolvedComments(t *testing.T) {
	adapter := &fakeReviewGateAdapter{}
	original := newReviewAdapter
	defer func() { newReviewAdapter = original }()
	newReviewAdapter = func() adapters.ReviewAdapter { return adapter }

	var stdout bytes.Buffer
	err := Run([]string{"review-gate", "--gitlab-project", "group/project", "--gitlab-mr", "123", "--gitlab-base-url", "https://gitlab.example.com"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("review gate: %v", err)
	}
	if adapter.ref.Project != "group/project" || adapter.ref.MergeRequest != "123" || adapter.ref.BaseURL != "https://gitlab.example.com" {
		t.Fatalf("ref = %#v", adapter.ref)
	}
	if !strings.Contains(stdout.String(), "review gate passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestReviewGateBlocksOnUnresolvedComments(t *testing.T) {
	adapter := &fakeReviewGateAdapter{comments: []adapters.ReviewComment{{
		ID:       "discussion:1",
		Author:   "reviewer",
		Body:     "fix nil handling",
		FilePath: "internal/service.go",
		Line:     42,
		Blocking: true,
	}}}
	original := newReviewAdapter
	defer func() { newReviewAdapter = original }()
	newReviewAdapter = func() adapters.ReviewAdapter { return adapter }

	var stdout bytes.Buffer
	err := Run([]string{"review-gate", "--gitlab-project", "group/project", "--gitlab-mr", "123"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "review gate blocked") {
		t.Fatalf("err = %v, want blocked", err)
	}
	output := stdout.String()
	for _, want := range []string{"review gate blocked", "internal/service.go:42", "fix nil handling"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestReviewGateRequiresGitLabRef(t *testing.T) {
	err := Run([]string{"review-gate", "--gitlab-project", "group/project"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--gitlab-project and --gitlab-mr are required") {
		t.Fatalf("err = %v, want required ref", err)
	}
}

func TestReviewGateHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"devflow review-gate", "review-gate Check unresolved GitLab MR comments directly"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] Format and verify.

```powershell
gofmt -w internal/cli/cli.go internal/cli/review_gate.go internal/cli/review_gate_test.go
go test ./internal/cli -count=1
go test ./internal/adapters -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/cli
git commit -m @'
Expose GitLab MR review gates as a direct CLI check

Release readiness needs a simple way to verify a real MR has no unresolved
blocking review comments without manually driving a demand into mr_review.

Constraint: The command must reuse the existing GitLab review adapter and never print token values
Rejected: Only documenting devflow run --stage mr-review | it requires demand setup before the MR can be checked
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/cli -count=1; go test ./internal/adapters -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 3: Add A Safe Editable Live Sandbox Fixture

**Files:**

- Create: `internal/dogfood/sandbox.go`
- Create: `internal/dogfood/sandbox_test.go`

### Why

Live provider implementation should never edit the Devflow repository during release dogfood. It should edit a tiny generated Go repository that is safe to delete.

- [ ] Create `internal/dogfood/sandbox.go`.

```go
package dogfood

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jesseedcp/devflow-agent/internal/quality"
)

const liveSandboxModule = "devflow-live-dogfood"

type Sandbox struct {
	Root            string
	RepoRoot        string
	DemandRoot      string
	QualityCommands []quality.Command
}

func CreateLiveSandbox(root string) (Sandbox, error) {
	if root == "" {
		temp, err := os.MkdirTemp("", "devflow-live-dogfood-*")
		if err != nil {
			return Sandbox{}, fmt.Errorf("create live dogfood root: %w", err)
		}
		root = temp
	}
	root = filepath.Clean(root)
	repoRoot := filepath.Join(root, "repo")
	demandRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(repoRoot, "coupon"), 0o755); err != nil {
		return Sandbox{}, fmt.Errorf("create sandbox repo: %w", err)
	}
	if err := os.MkdirAll(demandRoot, 0o755); err != nil {
		return Sandbox{}, fmt.Errorf("create sandbox artifact root: %w", err)
	}
	files := map[string]string{
		filepath.Join(repoRoot, "go.mod"):                    sandboxGoMod(),
		filepath.Join(repoRoot, "coupon", "eligibility.go"):  sandboxEligibility(),
		filepath.Join(repoRoot, "coupon", "eligibility_test.go"): sandboxEligibilityTest(),
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return Sandbox{}, fmt.Errorf("write sandbox file %s: %w", path, err)
		}
	}
	return Sandbox{
		Root:       root,
		RepoRoot:   repoRoot,
		DemandRoot: demandRoot,
		QualityCommands: []quality.Command{{
			Name: "go",
			Args: []string{"test", "./...", "-count=1", "-timeout", "2m"},
		}},
	}, nil
}

func sandboxGoMod() string {
	return "module " + liveSandboxModule + "\n\ngo 1.25\n"
}

func sandboxEligibility() string {
	return `package coupon

import "time"

type User struct {
	ID     string
	Active bool
}

type Coupon struct {
	ID        string
	ExpiresAt time.Time
}

type Claim struct {
	UserID   string
	CouponID string
}

func Eligible(user User, coupon Coupon, existing []Claim, now time.Time) (bool, string) {
	return false, "not implemented"
}
`
}

func sandboxEligibilityTest() string {
	return `package coupon

import (
	"testing"
	"time"
)

func TestEligibleAllowsActiveUserWithFreshCoupon(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	ok, reason := Eligible(User{ID: "u1", Active: true}, Coupon{ID: "c1", ExpiresAt: now.Add(time.Hour)}, nil, now)
	if !ok || reason != "eligible" {
		t.Fatalf("Eligible active user = (%v, %q), want (true, eligible)", ok, reason)
	}
}

func TestEligibleRejectsInactiveUser(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	ok, reason := Eligible(User{ID: "u1", Active: false}, Coupon{ID: "c1", ExpiresAt: now.Add(time.Hour)}, nil, now)
	if ok || reason != "inactive user" {
		t.Fatalf("Eligible inactive user = (%v, %q), want (false, inactive user)", ok, reason)
	}
}

func TestEligibleRejectsExpiredCoupon(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	ok, reason := Eligible(User{ID: "u1", Active: true}, Coupon{ID: "c1", ExpiresAt: now.Add(-time.Minute)}, nil, now)
	if ok || reason != "expired coupon" {
		t.Fatalf("Eligible expired coupon = (%v, %q), want (false, expired coupon)", ok, reason)
	}
}

func TestEligibleRejectsDuplicateClaim(t *testing.T) {
	now := time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC)
	claims := []Claim{{UserID: "u1", CouponID: "c1"}}
	ok, reason := Eligible(User{ID: "u1", Active: true}, Coupon{ID: "c1", ExpiresAt: now.Add(time.Hour)}, claims, now)
	if ok || reason != "already claimed" {
		t.Fatalf("Eligible duplicate claim = (%v, %q), want (false, already claimed)", ok, reason)
	}
}
`
}
```

- [ ] Create `internal/dogfood/sandbox_test.go`.

```go
package dogfood

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateLiveSandboxWritesEditableRepoAndArtifactRoot(t *testing.T) {
	sandbox, err := CreateLiveSandbox(t.TempDir())
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}
	for _, path := range []string{
		filepath.Join(sandbox.RepoRoot, "go.mod"),
		filepath.Join(sandbox.RepoRoot, "coupon", "eligibility.go"),
		filepath.Join(sandbox.RepoRoot, "coupon", "eligibility_test.go"),
		sandbox.DemandRoot,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	if sandbox.RepoRoot == sandbox.DemandRoot {
		t.Fatal("repo root and demand root must be separate")
	}
	if len(sandbox.QualityCommands) != 1 || sandbox.QualityCommands[0].Name != "go" {
		t.Fatalf("quality commands = %#v", sandbox.QualityCommands)
	}
}
```

- [ ] Format and verify.

```powershell
gofmt -w internal/dogfood/sandbox.go internal/dogfood/sandbox_test.go
go test ./internal/dogfood -run TestCreateLiveSandbox -count=1 -v
go test ./internal/dogfood -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/dogfood/sandbox.go internal/dogfood/sandbox_test.go
git commit -m @'
Create an editable sandbox for live dogfood

Live implementation checks need a disposable codebase that the agent can edit
without touching the Devflow repository. The sandbox separates code, demand
artifacts, and quality commands.

Constraint: Live dogfood must not mutate the Devflow checkout
Rejected: Letting RuntimeRunner edit the real repo during release verification | too risky for a smoke gate
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/dogfood -run TestCreateLiveSandbox -count=1 -v; go test ./internal/dogfood -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 4: Add Live Sandbox Dogfood Runner

**Files:**

- Create: `internal/dogfood/live.go`
- Create: `internal/dogfood/live_test.go`
- Modify: `internal/dogfood/runner.go`
- Modify: `internal/dogfood/runner_test.go`

### Why

The dogfood package should own orchestration. CLI and scripts should only parse flags and print results.

- [ ] Create `internal/dogfood/live.go`.

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
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type LiveOptions struct {
	Root         string
	ConfigPath   string
	Review       adapters.ReviewRef
	UseGitLab    bool
	Now          func() time.Time
	Timeout      time.Duration
	MaxIterations int
}

type LiveResult struct {
	Root        string
	RepoRoot    string
	DemandRoot  string
	DemandID    string
	FinalState  workflow.State
	ReportPath  string
	Steps       []Step
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
		ConfigPath:      opts.ConfigPath,
		PermissionMode:  permissions.ModeAcceptEdits,
		MaxIterations:   maxIterations,
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
```

- [ ] Create non-live tests in `internal/dogfood/live_test.go`.

```go
package dogfood

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRunLiveSandboxRequiresExplicitEnvironmentOptIn(t *testing.T) {
	t.Setenv("DEVFLOW_LIVE_DOGFOOD", "")
	_, err := RunLiveSandbox(context.Background(), LiveOptions{Root: t.TempDir(), Now: fixedDogfoodNow})
	if err == nil || !strings.Contains(err.Error(), "DEVFLOW_LIVE_DOGFOOD=1") {
		t.Fatalf("err = %v, want live opt-in error", err)
	}
}

func TestRenderLiveReportIncludesRootsAndSteps(t *testing.T) {
	report := renderLiveReport(LiveResult{
		Root:       "root",
		RepoRoot:   "root/repo",
		DemandRoot: "root/artifacts",
		DemandID:   "live-dogfood-coupon-eligibility",
		Steps:      []Step{{Name: "requirements", State: "requirements_review", Output: "drafted"}},
	})
	for _, want := range []string{"Live Dogfood Report", "RepoRoot", "DemandRoot", "requirements"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}

func TestLiveSandboxDogfood(t *testing.T) {
	if os.Getenv("DEVFLOW_LIVE_DOGFOOD") != "1" {
		t.Skip("set DEVFLOW_LIVE_DOGFOOD=1 with provider credentials to run live sandbox dogfood")
	}
	result, err := RunLiveSandbox(context.Background(), LiveOptions{
		Root: t.TempDir(),
		Now:  fixedDogfoodNow,
	})
	if err != nil {
		t.Fatalf("live sandbox dogfood: %v", err)
	}
	if result.FinalState != "completed" {
		t.Fatalf("final state = %s, want completed", result.FinalState)
	}
	if result.ReportPath == "" {
		t.Fatal("report path is empty")
	}
}
```

- [ ] Verify and note live test behavior.

```powershell
go test ./internal/dogfood -run 'TestRunLiveSandboxRequiresExplicitEnvironmentOptIn|TestRenderLiveReportIncludesRootsAndSteps' -count=1 -v
go test ./internal/dogfood -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

Expected: non-live tests pass; `TestLiveSandboxDogfood` is skipped unless `DEVFLOW_LIVE_DOGFOOD=1`.

- [ ] Optional live verification, only when credentials are available in the private shell.

```powershell
$env:DEVFLOW_LIVE_DOGFOOD = "1"
$env:OPENAI_API_KEY = "<private key in shell only>"
go test ./internal/dogfood -run TestLiveSandboxDogfood -count=1 -v -timeout 15m
```

Expected when the provider works:

```text
--- PASS: TestLiveSandboxDogfood
```

- [ ] Commit.

```powershell
git add internal/dogfood
git commit -m @'
Add opt-in live sandbox dogfood orchestration

The live dogfood runner exercises RuntimeRunner against a disposable Go
sandbox while preserving deterministic dogfood as the default release gate.

Constraint: Live dogfood must require an explicit environment opt-in
Rejected: Running live provider dogfood in normal tests | credentials and model behavior are intentionally external
Confidence: medium
Scope-risk: moderate
Directive: Keep live checks isolated from the Devflow checkout and skipped unless DEVFLOW_LIVE_DOGFOOD=1
Tested: go test ./internal/dogfood -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
Not-tested: Live provider dogfood unless DEVFLOW_LIVE_DOGFOOD credentials were available
'@
```

---

## Task 5: Expose `devflow live-dogfood`

**Files:**

- Create: `internal/cli/live_dogfood.go`
- Create: `internal/cli/live_dogfood_test.go`
- Modify: `internal/cli/cli.go`

- [ ] Update `internal/cli/cli.go` help and dispatch.

Add usage:

```text
  devflow live-dogfood [--root <path>] [--config <path>] [--with-gitlab]
```

Add description:

```text
  live-dogfood Run opt-in live provider sandbox dogfood
```

Add dispatch:

```go
case "live-dogfood":
	return runLiveDogfood(args[1:], stdout, stderr)
```

- [ ] Create `internal/cli/live_dogfood.go`.

```go
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/dogfood"
)

var runLiveDogfoodFunc = dogfood.RunLiveSandbox

func runLiveDogfood(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("live-dogfood", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var root, configPath, gitlabProject, gitlabMR, gitlabBaseURL string
	var withGitLab bool
	var timeoutSeconds int
	var maxIterations int
	fs.StringVar(&root, "root", "", "live dogfood root; defaults to a new temp directory")
	fs.StringVar(&configPath, "config", "", "devflow config path")
	fs.BoolVar(&withGitLab, "with-gitlab", false, "use real GitLab review gate instead of offline adapter")
	fs.StringVar(&gitlabProject, "gitlab-project", "", "GitLab project path or id")
	fs.StringVar(&gitlabMR, "gitlab-mr", "", "GitLab merge request iid")
	fs.StringVar(&gitlabBaseURL, "gitlab-base-url", "", "GitLab base url override")
	fs.IntVar(&timeoutSeconds, "timeout-seconds", 600, "live dogfood timeout in seconds")
	fs.IntVar(&maxIterations, "max-iterations", 30, "maximum RuntimeRunner iterations per stage")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if withGitLab && (gitlabProject == "" || gitlabMR == "") {
		return fmt.Errorf("--gitlab-project and --gitlab-mr are required with --with-gitlab")
	}
	if timeoutSeconds <= 0 {
		return fmt.Errorf("--timeout-seconds must be positive")
	}
	if maxIterations <= 0 {
		return fmt.Errorf("--max-iterations must be positive")
	}

	result, err := runLiveDogfoodFunc(context.Background(), dogfood.LiveOptions{
		Root:          root,
		ConfigPath:    configPath,
		UseGitLab:     withGitLab,
		Review:        adapters.ReviewRef{Project: gitlabProject, MergeRequest: gitlabMR, BaseURL: gitlabBaseURL},
		Timeout:       time.Duration(timeoutSeconds) * time.Second,
		MaxIterations: maxIterations,
		Now:           time.Now,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "live dogfood completed for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "state: %s\n", result.FinalState)
	fmt.Fprintf(stdout, "root: %s\n", result.Root)
	fmt.Fprintf(stdout, "repo-root: %s\n", result.RepoRoot)
	fmt.Fprintf(stdout, "demand-root: %s\n", result.DemandRoot)
	fmt.Fprintf(stdout, "report: %s\n", result.ReportPath)
	fmt.Fprintf(stdout, "steps: %s\n", strconv.Itoa(len(result.Steps)))
	return nil
}
```

- [ ] Create `internal/cli/live_dogfood_test.go`.

```go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/dogfood"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestLiveDogfoodCommandPrintsResult(t *testing.T) {
	original := runLiveDogfoodFunc
	defer func() { runLiveDogfoodFunc = original }()
	runLiveDogfoodFunc = func(_ context.Context, opts dogfood.LiveOptions) (dogfood.LiveResult, error) {
		if opts.Timeout <= 0 || opts.MaxIterations != 12 {
			t.Fatalf("opts = %#v", opts)
		}
		return dogfood.LiveResult{
			Root:       "root",
			RepoRoot:   "root/repo",
			DemandRoot: "root/artifacts",
			DemandID:   "live-dogfood-coupon-eligibility",
			FinalState: workflow.Completed,
			ReportPath: "root/artifacts/report.md",
			Steps:      []dogfood.Step{{Name: "requirements", State: workflow.RequirementsReview, Output: "ok"}},
		}, nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"live-dogfood", "--timeout-seconds", "30", "--max-iterations", "12"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("live-dogfood: %v", err)
	}
	for _, want := range []string{"live dogfood completed", "state: completed", "repo-root:", "steps: 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestLiveDogfoodRequiresGitLabRefWhenEnabled(t *testing.T) {
	err := Run([]string{"live-dogfood", "--with-gitlab", "--gitlab-project", "group/project"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--gitlab-project and --gitlab-mr are required") {
		t.Fatalf("err = %v, want gitlab ref error", err)
	}
}

func TestLiveDogfoodHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"devflow live-dogfood", "live-dogfood Run opt-in live provider sandbox dogfood"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] Format and verify.

```powershell
gofmt -w internal/cli/cli.go internal/cli/live_dogfood.go internal/cli/live_dogfood_test.go
go test ./internal/cli -count=1
go test ./internal/dogfood -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] Commit.

```powershell
git add internal/cli
git commit -m @'
Expose live sandbox dogfood from the CLI

The live-dogfood command gives operators an explicit opt-in path for real
provider validation while keeping deterministic dogfood as the default check.

Constraint: GitLab live mode must require an explicit MR reference
Rejected: Adding live behavior to devflow dogfood by default | deterministic dogfood must stay stable and offline
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/cli -count=1; go test ./internal/dogfood -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

---

## Task 6: Add Release Readiness Script With Optional Live Gates

**Files:**

- Create: `scripts/release-readiness.ps1`

### Why

Release checks should be one command. Deterministic checks must always run; live checks should run only when explicitly opted in.

- [ ] Create `scripts/release-readiness.ps1`.

```powershell
[CmdletBinding()]
param(
    [string]$Version = "0.1.0-readiness",
    [string]$Root = "",
    [switch]$RunLiveDogfood,
    [switch]$RunGitLabGate,
    [string]$GitLabProject = "",
    [string]$GitLabMR = "",
    [string]$GitLabBaseURL = ""
)

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$readinessRoot = $Root
if ([string]::IsNullOrWhiteSpace($readinessRoot)) {
    $readinessRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('devflow-release-readiness-' + [guid]::NewGuid().ToString('N'))
}
$readinessRoot = [System.IO.Path]::GetFullPath($readinessRoot)
$tempPath = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
if (-not $readinessRoot.StartsWith($tempPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Release readiness root must be inside the system temp directory unless the script is edited intentionally: $readinessRoot"
}
New-Item -ItemType Directory -Force -Path $readinessRoot | Out-Null

$report = Join-Path $readinessRoot 'release-readiness.md'
Set-Content -LiteralPath $report -Value "# Devflow Release Readiness`n" -Encoding UTF8
Add-Content -LiteralPath $report -Value "Version: ``$Version```n"
Add-Content -LiteralPath $report -Value "Repo: ``$repoRoot```n"

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Command
    )
    Write-Host "==> $Name"
    Add-Content -LiteralPath $report -Value "## $Name`n"
    & $Command 2>&1 | Tee-Object -Variable output
    $exit = $LASTEXITCODE
    if ($null -eq $exit) { $exit = 0 }
    Add-Content -LiteralPath $report -Value '```text'
    Add-Content -LiteralPath $report -Value ($output -join [Environment]::NewLine)
    Add-Content -LiteralPath $report -Value '```'
    Add-Content -LiteralPath $report -Value ""
    if ($exit -ne 0) {
        throw "$Name failed with exit code $exit"
    }
}

Push-Location $repoRoot
try {
    Invoke-Step "go test" { go test ./... -count=1 -timeout 5m }
    Invoke-Step "go vet" { go vet ./... }
    Invoke-Step "go build" { go build ./cmd/devflow }
    Invoke-Step "windows build" { powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\build-windows.ps1') -Version $Version -Output (Join-Path $repoRoot 'dist\devflow-windows-amd64.exe') }
    Invoke-Step "deterministic dogfood" { powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $repoRoot 'scripts\dogfood-local.ps1') -Version $Version }
    Invoke-Step "git diff check" { git diff --check }

    if ($RunLiveDogfood) {
        if ($env:DEVFLOW_LIVE_DOGFOOD -ne "1") {
            throw "RunLiveDogfood requires DEVFLOW_LIVE_DOGFOOD=1"
        }
        Invoke-Step "live dogfood" { .\dist\devflow-windows-amd64.exe live-dogfood --root (Join-Path $readinessRoot 'live-dogfood') }
    } else {
        Add-Content -LiteralPath $report -Value "## live dogfood`nSkipped. Pass -RunLiveDogfood and set DEVFLOW_LIVE_DOGFOOD=1 to run.`n"
    }

    if ($RunGitLabGate) {
        if ([string]::IsNullOrWhiteSpace($GitLabProject) -or [string]::IsNullOrWhiteSpace($GitLabMR)) {
            throw "-RunGitLabGate requires -GitLabProject and -GitLabMR"
        }
        $args = @('review-gate', '--gitlab-project', $GitLabProject, '--gitlab-mr', $GitLabMR)
        if (-not [string]::IsNullOrWhiteSpace($GitLabBaseURL)) {
            $args += @('--gitlab-base-url', $GitLabBaseURL)
        }
        Invoke-Step "gitlab review gate" { .\dist\devflow-windows-amd64.exe @args }
    } else {
        Add-Content -LiteralPath $report -Value "## gitlab review gate`nSkipped. Pass -RunGitLabGate with -GitLabProject and -GitLabMR to run.`n"
    }
}
finally {
    Pop-Location
}

Write-Host "release readiness report: $report"
```

- [ ] Run deterministic readiness script.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave9
```

Expected:

```text
==> go test
==> go vet
==> go build
==> windows build
==> deterministic dogfood
==> git diff check
release readiness report: <temp>\devflow-release-readiness-<id>\release-readiness.md
```

- [ ] Verify.

```powershell
git diff --check
go test ./... -count=1 -timeout 5m
```

- [ ] Commit.

```powershell
git add scripts/release-readiness.ps1
git commit -m @'
Add a release readiness wrapper with optional live gates

Release verification now has one script that always runs deterministic checks
and records skipped or executed live gates in a report.

Constraint: Live provider and GitLab checks must be explicit opt-ins
Rejected: Making live gates mandatory in the release script | credentials and external service state are not CI-stable
Confidence: high
Scope-risk: narrow
Tested: powershell -NoProfile -ExecutionPolicy Bypass -File scripts\\release-readiness.ps1 -Version 0.1.0-wave9; git diff --check; go test ./... -count=1 -timeout 5m
'@
```

---

## Task 7: Document Live Dogfood And GitLab Release Gates

**Files:**

- Create: `docs/user-guide/live-dogfood.md`
- Create: `docs/examples/dogfood/live-sandbox-report.md`
- Modify: `README.md`
- Modify: `docs/release/v0.1.md`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/user-guide/dogfood-smoke.md`

- [ ] Create `docs/user-guide/live-dogfood.md`.

````markdown
# Live Dogfood Guide

Deterministic dogfood is the default release gate. Live dogfood is an optional confidence check for a private developer shell.

## Provider Setup

Initialize an OpenAI-compatible config:

```powershell
devflow init --provider openai-compat --base-url https://ark.cn-beijing.volces.com/api/coding/v3 --model ark-code-latest
```

Set the key only in the shell:

```powershell
$env:OPENAI_API_KEY = "<private key>"
$env:DEVFLOW_LIVE_DOGFOOD = "1"
```

Do not commit `.devflow/config.local.yaml`, token values, terminal logs containing secrets, or screenshots of secrets.

## Run Live Sandbox Dogfood

```powershell
devflow live-dogfood
```

The command creates a temporary sandbox with:

- `repo/` for editable code;
- `artifacts/` for `.devflow/demands/...`;
- `go test ./... -count=1 -timeout 2m` as the quality gate.

The Devflow repository is not edited by live dogfood.

## Optional GitLab Review Gate

```powershell
$env:GITLAB_TOKEN = "<private gitlab token>"
devflow review-gate --gitlab-project "group/project" --gitlab-mr "123"
```

The command exits nonzero if unresolved blocking comments remain.

## One-Command Release Readiness

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-dev
```

Optional live gates:

```powershell
$env:DEVFLOW_LIVE_DOGFOOD = "1"
$env:OPENAI_API_KEY = "<private key>"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-dev -RunLiveDogfood
```

Optional GitLab gate:

```powershell
$env:GITLAB_TOKEN = "<private gitlab token>"
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-dev -RunGitLabGate -GitLabProject "group/project" -GitLabMR "123"
```
````

- [ ] Create `docs/examples/dogfood/live-sandbox-report.md`.

```markdown
# Live Dogfood Report: live-dogfood-coupon-eligibility

Root: `<temp>\devflow-live-dogfood-<id>`

RepoRoot: `<temp>\devflow-live-dogfood-<id>\repo`

DemandRoot: `<temp>\devflow-live-dogfood-<id>\artifacts`

FinalState: `completed`

## Steps

- `requirements` -> `requirements_review`: requirements drafted by demand runner
- `confirm requirements` -> `plan_drafting`: live requirements accepted for sandbox dogfood
- `plan` -> `plan_review`: plan drafted by demand runner
- `confirm plan` -> `implementation`: live plan accepted for sandbox dogfood
- `implementation` -> `mr_review`: implementation completed and quality gate passed
- `mr-review` -> `verification`: mr review cleared, no blocking unresolved comments
- `verification` -> `verification`: verification drafted by demand runner
- `confirm verification` -> `closeout`: live sandbox verification passed
- `closeout` -> `closeout`: closeout and memory candidates drafted by demand runner
- `confirm closeout` -> `completed`: live sandbox closeout accepted
```

- [ ] Update `README.md`.

Add command:

```text
devflow live-dogfood
devflow review-gate
```

Add docs link:

```markdown
- [Live dogfood guide](docs/user-guide/live-dogfood.md)
```

- [ ] Update `docs/release/v0.1.md`.

Add feature bullets:

```markdown
- Optional live sandbox dogfood through `devflow live-dogfood`.
- Direct GitLab MR review checks through `devflow review-gate`.
- Release readiness wrapper through `scripts/release-readiness.ps1`.
```

Replace the verification block with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0
```

Add note:

```markdown
Live provider and GitLab checks are optional private-shell gates. They must not run in default CI unless credentials and target MR state are intentionally provided.
```

- [ ] Update `docs/user-guide/backend-demand-loop.md`.

In the MR Review section, add:

```markdown
Before running the workflow MR stage, you can check a real MR directly:

```powershell
$env:GITLAB_TOKEN = '<your-token>'
devflow review-gate --gitlab-project "group/project" --gitlab-mr "123"
```
```

- [ ] Update `docs/user-guide/dogfood-smoke.md`.

Add:

```markdown
For live provider validation, see [Live dogfood guide](live-dogfood.md). Live dogfood is separate from deterministic dogfood and requires explicit environment opt-in.
```

- [ ] Verify docs.

```powershell
git diff --check
go test ./... -count=1 -timeout 5m
```

- [ ] Commit.

```powershell
git add README.md docs/release/v0.1.md docs/user-guide/backend-demand-loop.md docs/user-guide/dogfood-smoke.md docs/user-guide/live-dogfood.md docs/examples/dogfood/live-sandbox-report.md
git commit -m @'
Document live dogfood and release gates

The docs now separate deterministic release checks from private live provider
and GitLab gates, including exact commands and secret-handling boundaries.

Constraint: Documentation must never imply secrets belong in committed files
Rejected: Folding live checks into the deterministic dogfood page only | users need the risk boundary to be explicit
Confidence: high
Scope-risk: narrow
Tested: git diff --check; go test ./... -count=1 -timeout 5m
'@
```

---

## Task 8: Final Verification, PR, And Handoff

**Files:** none unless final fixes are needed

- [ ] Run full deterministic verification.

```powershell
go test ./internal/demandflow -count=1
go test ./internal/dogfood -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-wave9 -Output dist\devflow-windows-amd64.exe
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave9
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave9
git diff --check
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-9
```

- [ ] Optional live provider verification, only in a private shell with credentials.

```powershell
$env:DEVFLOW_LIVE_DOGFOOD = "1"
$env:OPENAI_API_KEY = "<private key>"
go test ./internal/dogfood -run TestLiveSandboxDogfood -count=1 -v -timeout 15m
.\dist\devflow-windows-amd64.exe live-dogfood --root (Join-Path ([System.IO.Path]::GetTempPath()) ('devflow-live-cli-' + [guid]::NewGuid().ToString('N')))
```

Record the result in the PR body as either:

```text
Live provider dogfood: PASS
```

or:

```text
Live provider dogfood: not run; credentials unavailable
```

- [ ] Optional real GitLab verification, only with a safe target MR.

```powershell
$env:GITLAB_TOKEN = "<private token>"
.\dist\devflow-windows-amd64.exe review-gate --gitlab-project "group/project" --gitlab-mr "123"
```

Record the result in the PR body as either:

```text
GitLab review gate: PASS for group/project!123
```

or:

```text
GitLab review gate: not run; target MR unavailable
```

- [ ] Push branch.

```powershell
git push -u origin feature/devflow-wave-9
```

- [ ] Create PR.

```powershell
gh pr create --base main --head feature/devflow-wave-9 --title "Wave 9: live sandbox and GitLab release gates" --body @'
## Summary
- Separates agent runner root from demand artifact root.
- Adds direct GitLab MR review-gate checks.
- Adds opt-in live provider sandbox dogfood.
- Adds release-readiness script and docs for deterministic vs live gates.

## Test Plan
- [ ] go test ./internal/demandflow -count=1
- [ ] go test ./internal/dogfood -count=1
- [ ] go test ./internal/cli -count=1
- [ ] go test ./... -count=1 -timeout 5m
- [ ] go vet ./...
- [ ] go build ./cmd/devflow
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-wave9 -Output dist\devflow-windows-amd64.exe
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave9
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave9
- [ ] git diff --check

## Optional Live Gates
- Live provider dogfood: not run; credentials unavailable
- GitLab review gate: not run; target MR unavailable
'@
```

Do not merge until deterministic verification and CI pass. If optional live gates are run, include their result in the PR body.

---

## Definition Of Done

Wave 9 is complete when:

- `demandflow.Options.RunnerRoot` exists and defaults to `Root`.
- `devflow run --runner-root <path>` sends RuntimeRunner tools to the runner root while storing artifacts under `--root`.
- `devflow review-gate` checks unresolved GitLab MR comments directly and exits nonzero on blockers.
- `CreateLiveSandbox` creates separate `repo/` and `artifacts/` roots.
- `RunLiveSandbox` requires `DEVFLOW_LIVE_DOGFOOD=1`.
- `devflow live-dogfood` exposes the live sandbox path.
- `scripts/release-readiness.ps1` runs deterministic checks and records skipped or executed live gates.
- Docs explain deterministic dogfood, live dogfood, GitLab review gate, release readiness, and secret handling.
- Full deterministic verification passes:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave9
git diff --check
```

- PR from `feature/devflow-wave-9` to `main` is open.

## Self-Review Notes

Spec coverage:

- Runner root separation is covered by Task 1.
- GitLab direct gate is covered by Task 2.
- Safe editable sandbox is covered by Task 3.
- Live provider orchestration is covered by Task 4.
- CLI exposure is covered by Task 5.
- Release readiness wrapper is covered by Task 6.
- Docs are covered by Task 7.
- Final verification and PR are covered by Task 8.

Placeholder scan:

- No step uses unfinished-marker wording or open-ended implementation instructions.
- Credential values are represented as private shell environment variables and must not be committed.
- Optional live gates have explicit skip/pass wording for PR reporting.

Type consistency:

- `RunnerRoot` is introduced before live dogfood uses it.
- `CreateLiveSandbox` is introduced before `RunLiveSandbox`.
- `LiveOptions` and `LiveResult` are introduced before CLI uses them.
- `review-gate` reuses existing `newReviewAdapter`, `adapters.ReviewRef`, and `adapters.ReviewAdapter`.

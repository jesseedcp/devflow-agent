# Runtime Foundation Wave 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first usable backend-demand delivery loop on top of the migrated runtime: demand input -> `requirements.md` -> `plan.md` -> implementation + quality gate -> MR review coordination -> `verification.md` -> `closeout.md` + stable memory candidates.

**Architecture:** Add a thin product orchestration package, `internal/demandflow`, that coordinates the existing `artifacts`, `workflow`, `quality`, `adapters`, `memory`, and runtime agent packages without moving product state into the TUI. The orchestrator owns stage prompts, state transitions, artifact writes, and event recording; CLI code stays a small command adapter. Runtime LLM execution is behind a `Runner` interface so tests use deterministic fake runners while production uses the Wave 1-4 runtime agent.

**Tech Stack:** Go 1.25.0, PowerShell on Windows, existing Devflow `.devflow` artifact store, `internal/runtime/agent`, `internal/runtime/tools`, OpenAI/Anthropic-compatible runtime config, standard `net/http` for GitLab MR review adapter, and the existing `quality.Gate` command runner.

---

## Current Environment

Execution worktree:

```powershell
$target = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1'
Set-Location $target
git status --short --branch
```

Expected starting status after Wave 4:

```text
## feature/devflow-v0.1...origin/feature/devflow-v0.1
?? .gocache/
?? devflow.exe
```

Known local artifacts:

- `.gocache/`
- `devflow.exe`

Do not commit either file.

Wave 4 has already been pushed to:

```text
origin feature/devflow-v0.1
```

## Product Boundary For Wave 5

Wave 5 is the first product loop. It is not another MewCode package migration.

Use existing packages as authorities:

| Concern | Existing authority |
|---|---|
| demand files and `.devflow/demands/<id>` paths | `internal/artifacts` |
| workflow states and legal transitions | `internal/workflow` |
| quality gate command execution | `internal/quality` |
| MR review abstraction | `internal/adapters` |
| stable knowledge search | `internal/memory` |
| LLM/tool loop | `internal/runtime/agent` and `internal/runtime/*` |
| command-line UX | `internal/cli` |

Add exactly one new product orchestration package:

```text
internal/demandflow
```

Do not put demand workflow state transitions inside `internal/runtime/tui`. The TUI is an interactive runtime surface; `internal/demandflow` is the backend-demand delivery engine.

## User-Facing Flow

The intended MVP flow after Wave 5:

```powershell
devflow start --title "Add coupon check" --description "Only active members can claim coupons"
devflow run --demand add-coupon-check --stage requirements
devflow confirm --demand add-coupon-check --stage requirements --by dd --summary "requirements ok"
devflow run --demand add-coupon-check --stage plan
devflow confirm --demand add-coupon-check --stage plan --by dd --summary "plan ok"
devflow run --demand add-coupon-check --stage implementation --quality-command "go test ./..."
devflow run --demand add-coupon-check --stage mr-review --gitlab-project "group/project" --gitlab-mr "123"
devflow run --demand add-coupon-check --stage verification --quality-command "go test ./..."
devflow confirm --demand add-coupon-check --stage verification --by dd --summary "verification ok"
devflow run --demand add-coupon-check --stage closeout
devflow confirm --demand add-coupon-check --stage closeout --by dd --summary "closeout ok"
```

The command is intentionally `run` rather than many top-level commands. The existing `start`, `confirm`, `verify`, and `closeout` commands remain stable.

## Stage Semantics

| Stage | Required current state | State after successful run | Main artifact effect |
|---|---|---|---|
| `requirements` | `created` or `context_loaded` | `requirements_review` | write `requirements.md` |
| `plan` | `plan_drafting` | `plan_review` | write `plan.md` |
| `implementation` | `implementation` | `mr_review` if quality passes, `failed_quality_gate` if quality fails | append `progress.md`; runtime agent may edit code |
| `mr-review` | `mr_review` | `verification` when no blocking unresolved comments remain | append `progress.md`; optional adapter replies |
| `verification` | `verification` | `verification` | write `verification.md`; human confirmation advances to `closeout` |
| `closeout` | `closeout` | `closeout` | write `closeout.md` and `memory-candidates.md`; human confirmation advances to `completed` |

Why `verification` and `closeout` do not advance themselves:

- Existing `devflow confirm --stage verification` moves `verification -> closeout`.
- Existing `devflow confirm --stage closeout` moves `closeout -> completed`.
- Wave 5 keeps human confirmation gates intact.

## File Map

Create:

```text
internal/demandflow/types.go
internal/demandflow/context.go
internal/demandflow/prompts.go
internal/demandflow/runner.go
internal/demandflow/runtime_runner.go
internal/demandflow/engine.go
internal/demandflow/review.go
internal/demandflow/engine_test.go
internal/demandflow/prompts_test.go
internal/demandflow/runtime_runner_test.go
internal/demandflow/review_test.go
internal/adapters/gitlab.go
internal/adapters/gitlab_test.go
internal/cli/run.go
internal/cli/run_test.go
```

Modify:

```text
internal/cli/cli.go
internal/cli/cli_test.go
internal/adapters/review.go
docs/migration/mewcode-source-manifest.md
```

`docs/migration/mewcode-source-manifest.md` should get a Wave 5 section noting this is new Devflow product orchestration, not copied MewCode source.

## Task 0: Preflight

**Files:** none

- [ ] Run current status.

```powershell
git status --short --branch
```

Expected: clean tracked tree, only optional `.gocache/` and `devflow.exe` untracked.

- [ ] Run baseline verification.

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

No commit for Task 0.

## Task 1: Add Demandflow Types And Context Loading

**Files:**

- Create: `internal/demandflow/types.go`
- Create: `internal/demandflow/context.go`
- Create: `internal/demandflow/context_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

**Step 1: Add core stage types**

Create `internal/demandflow/types.go` with these public types:

```go
package demandflow

import (
    "time"

    "github.com/jesseedcp/devflow-agent/internal/artifacts"
    "github.com/jesseedcp/devflow-agent/internal/quality"
)

type Stage string

const (
    StageRequirements   Stage = "requirements"
    StagePlan           Stage = "plan"
    StageImplementation Stage = "implementation"
    StageMRReview       Stage = "mr-review"
    StageVerification   Stage = "verification"
    StageCloseout       Stage = "closeout"
)

func ParseStage(value string) (Stage, error) {
    switch Stage(value) {
    case StageRequirements, StagePlan, StageImplementation, StageMRReview, StageVerification, StageCloseout:
        return Stage(value), nil
    default:
        return "", fmt.Errorf("unsupported stage %q", value)
    }
}

type ArtifactSnapshot struct {
    Requirements     string
    Plan             string
    Progress         string
    Verification     string
    Closeout         string
    MemoryCandidates string
}

type ContextSnapshot struct {
    Demand    artifacts.Demand
    Artifacts ArtifactSnapshot
    Memories  []MemoryHit
}

type MemoryHit struct {
    DemandID string
    Path     string
    Snippet  string
}

type Options struct {
    Root            string
    DemandID        string
    Stage           Stage
    QualityCommands []quality.Command
    Review          ReviewOptions
    Runner          Runner
    Now             func() time.Time
}
```

Add the missing `fmt` import when implementing `ParseStage`.

**Step 2: Add context loader**

Create `internal/demandflow/context.go`:

```go
package demandflow

import (
    "errors"
    "os"

    "github.com/jesseedcp/devflow-agent/internal/artifacts"
    memorystore "github.com/jesseedcp/devflow-agent/internal/memory"
)

type contextLoader struct {
    store artifacts.Store
    root  string
}

func newContextLoader(root string) contextLoader {
    return contextLoader{
        store: artifacts.NewStore(root),
        root:  root,
    }
}

func (l contextLoader) Load(demandID string) (ContextSnapshot, error) {
    demand, err := l.store.LoadDemand(demandID)
    if err != nil {
        return ContextSnapshot{}, err
    }

    snapshot := ContextSnapshot{Demand: demand}
    snapshot.Artifacts.Requirements = l.readArtifact(demandID, artifacts.RequirementsFile)
    snapshot.Artifacts.Plan = l.readArtifact(demandID, artifacts.PlanFile)
    snapshot.Artifacts.Progress = l.readArtifact(demandID, artifacts.ProgressFile)
    snapshot.Artifacts.Verification = l.readArtifact(demandID, artifacts.VerificationFile)
    snapshot.Artifacts.Closeout = l.readArtifact(demandID, artifacts.CloseoutFile)
    snapshot.Artifacts.MemoryCandidates = l.readArtifact(demandID, artifacts.MemoryCandidatesFile)

    if hits, err := memorystore.NewStore(l.root).Search(demand.Title + " " + demand.Description); err == nil {
        for _, hit := range hits {
            if hit.DemandID == demand.ID {
                continue
            }
            snapshot.Memories = append(snapshot.Memories, MemoryHit{
                DemandID: hit.DemandID,
                Path:     hit.Path,
                Snippet:  hit.Snippet,
            })
        }
    }

    return snapshot, nil
}

func (l contextLoader) readArtifact(demandID, name string) string {
    path := l.store.DemandDir(demandID)
    data, err := os.ReadFile(filepath.Join(path, name))
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return ""
        }
        return ""
    }
    return string(data)
}
```

Add the missing `path/filepath` import.

**Step 3: Test context loading**

Create `internal/demandflow/context_test.go` with tests that:

- create a demand through `artifacts.NewStore(root).CreateDemand`;
- write a prior demand's `memory-candidates.md`;
- load context for the new demand;
- assert demand metadata and artifact text are present;
- assert memory search excludes the current demand ID.

Expected command:

```powershell
go test ./internal/demandflow -run TestContextLoader -count=1
```

**Step 4: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow docs/migration/mewcode-source-manifest.md
git commit -m @'
Ground demand stages in artifact context

Wave 5 starts with a small context layer so product-stage prompts can
read demand metadata, existing artifacts, and reusable memory without
coupling CLI code to filesystem details.

Constraint: Existing .devflow demand workspace remains the source of truth
Rejected: Loading context inside each CLI handler | duplicates path and memory logic
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 2: Add Runner Interface And Static Test Runner

**Files:**

- Create: `internal/demandflow/runner.go`
- Create or modify: `internal/demandflow/engine_test.go`

**Step 1: Add runner contracts**

Create `internal/demandflow/runner.go`:

```go
package demandflow

import "context"

type RunnerRequest struct {
    Stage     Stage
    Root      string
    DemandID  string
    Prompt    string
    Context   ContextSnapshot
    ToolMode  ToolMode
}

type ToolMode string

const (
    ToolModeReadOnly      ToolMode = "read-only"
    ToolModeEdit          ToolMode = "edit"
    ToolModeEditAndShell  ToolMode = "edit-and-shell"
)

type RunnerResponse struct {
    Text        string
    ToolSummary []string
}

type Runner interface {
    Run(ctx context.Context, req RunnerRequest) (RunnerResponse, error)
}

type StaticRunner struct {
    Responses map[Stage]RunnerResponse
    Requests  []RunnerRequest
}

func (r *StaticRunner) Run(ctx context.Context, req RunnerRequest) (RunnerResponse, error) {
    r.Requests = append(r.Requests, req)
    if resp, ok := r.Responses[req.Stage]; ok {
        return resp, nil
    }
    return RunnerResponse{Text: "# " + string(req.Stage) + "\n\nstatic response\n"}, nil
}
```

**Step 2: Test fake runner captures request**

Add a test:

```go
func TestStaticRunnerCapturesRequests(t *testing.T) {
    runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
        StageRequirements: {Text: "requirements body"},
    }}
    resp, err := runner.Run(context.Background(), RunnerRequest{
        Stage:    StageRequirements,
        Root:     t.TempDir(),
        DemandID: "add-coupon-check",
        Prompt:   "prompt",
    })
    if err != nil {
        t.Fatalf("run: %v", err)
    }
    if resp.Text != "requirements body" {
        t.Fatalf("response = %q", resp.Text)
    }
    if len(runner.Requests) != 1 || runner.Requests[0].DemandID != "add-coupon-check" {
        t.Fatalf("request not captured: %#v", runner.Requests)
    }
}
```

**Step 3: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
git diff --check
git add internal/demandflow
git commit -m @'
Separate demandflow orchestration from agent execution

The Runner interface keeps product-stage state transitions testable
without live LLM calls while leaving a production runtime-agent runner
available for later tasks.

Constraint: Product workflow tests must be deterministic
Rejected: Calling the runtime LLM directly from Engine | would make stage tests flaky and slow
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/demandflow -count=1; git diff --check
'@
```

## Task 3: Add Stage Prompt Builders

**Files:**

- Create: `internal/demandflow/prompts.go`
- Create: `internal/demandflow/prompts_test.go`

**Step 1: Implement prompt builder**

Create `internal/demandflow/prompts.go`:

```go
package demandflow

import (
    "fmt"
    "strings"
)

func BuildPrompt(stage Stage, ctx ContextSnapshot) (string, ToolMode, error) {
    switch stage {
    case StageRequirements:
        return requirementsPrompt(ctx), ToolModeReadOnly, nil
    case StagePlan:
        return planPrompt(ctx), ToolModeReadOnly, nil
    case StageImplementation:
        return implementationPrompt(ctx), ToolModeEditAndShell, nil
    case StageVerification:
        return verificationPrompt(ctx), ToolModeReadOnly, nil
    case StageCloseout:
        return closeoutPrompt(ctx), ToolModeReadOnly, nil
    default:
        return "", "", fmt.Errorf("stage %s does not have an agent prompt", stage)
    }
}

func requirementsPrompt(ctx ContextSnapshot) string {
    return strings.TrimSpace(fmt.Sprintf(`# Role
You are the backend business requirements expert for Devflow.

# Demand
Title: %s
Description:
%s

# Reusable memory
%s

# Output contract
Return the complete requirements.md body only.
Use these headings exactly:
- # Requirements: %s
- ## 目标行为
- ## 非目标范围
- ## 业务规则
- ## 用户/调用方影响
- ## 验收标准
- ## 风险与歧义
- ## 待确认问题
- ## 人工确认记录
`, ctx.Demand.Title, ctx.Demand.Description, renderMemoryHits(ctx.Memories), ctx.Demand.Title))
}
```

Add matching `planPrompt`, `implementationPrompt`, `verificationPrompt`, and `closeoutPrompt` functions. Each prompt must include:

- current artifact content needed for that stage;
- exact output contract;
- the target artifact name;
- instruction not to include chat commentary around the artifact body.

Use these target artifacts:

```go
plan.md
progress.md
verification.md
closeout.md
memory-candidates.md
```

For `StageCloseout`, instruct the runner to return two sections separated by this exact marker:

```text
---DEVFLOW-MEMORY-CANDIDATES---
```

**Step 2: Implement memory rendering**

Add:

```go
func renderMemoryHits(hits []MemoryHit) string {
    if len(hits) == 0 {
        return "(none)"
    }
    var b strings.Builder
    for _, hit := range hits {
        b.WriteString("- ")
        b.WriteString(hit.DemandID)
        if strings.TrimSpace(hit.Snippet) != "" {
            b.WriteString(": ")
            b.WriteString(strings.TrimSpace(hit.Snippet))
        }
        b.WriteString("\n")
    }
    return strings.TrimSpace(b.String())
}
```

**Step 3: Test prompt contracts**

Create `internal/demandflow/prompts_test.go` with tests:

- requirements prompt contains demand title, demand description, and `requirements.md` output contract;
- plan prompt includes current `requirements.md`;
- implementation prompt uses `ToolModeEditAndShell`;
- closeout prompt includes `---DEVFLOW-MEMORY-CANDIDATES---`;
- unsupported `StageMRReview` returns an error from `BuildPrompt`.

**Step 4: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
git diff --check
git add internal/demandflow
git commit -m @'
Make backend demand stages promptable

Prompt builders turn artifact context into explicit stage contracts so
requirements, plans, verification, and closeout can be generated by any
Runner without leaking chat prose into product files.

Constraint: Stage outputs are markdown artifacts, not conversational replies
Rejected: One generic prompt for every stage | loses artifact-specific acceptance criteria
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/demandflow -count=1; git diff --check
'@
```

## Task 4: Implement Stage Engine And Artifact Writes

**Files:**

- Create: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/engine_test.go`

**Step 1: Add engine skeleton**

Create `internal/demandflow/engine.go`:

```go
package demandflow

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/jesseedcp/devflow-agent/internal/artifacts"
    "github.com/jesseedcp/devflow-agent/internal/quality"
    "github.com/jesseedcp/devflow-agent/internal/templates"
    "github.com/jesseedcp/devflow-agent/internal/workflow"
)

type Engine struct {
    Store artifacts.Store
    Gate  quality.Gate
}

func NewEngine(root string) Engine {
    return Engine{
        Store: artifacts.NewStore(root),
        Gate:  quality.Gate{},
    }
}

func (e Engine) Run(ctx context.Context, opts Options) error {
    if opts.Runner == nil {
        return fmt.Errorf("runner is required")
    }
    if opts.Now == nil {
        opts.Now = time.Now
    }
    return e.Store.WithDemandLock(opts.DemandID, func() error {
        switch opts.Stage {
        case StageRequirements:
            return e.runRequirements(ctx, opts)
        case StagePlan:
            return e.runPlan(ctx, opts)
        case StageImplementation:
            return e.runImplementation(ctx, opts)
        case StageMRReview:
            return e.runMRReview(ctx, opts)
        case StageVerification:
            return e.runVerification(ctx, opts)
        case StageCloseout:
            return e.runCloseout(ctx, opts)
        default:
            return fmt.Errorf("unsupported stage %q", opts.Stage)
        }
    })
}
```

**Step 2: Add state helper**

Add:

```go
func (e Engine) advance(demand *artifacts.Demand, next workflow.State) error {
    current := workflow.State(demand.State)
    advanced, err := workflow.Advance(current, next)
    if err != nil {
        return err
    }
    demand.State = string(advanced)
    return e.Store.SaveDemand(*demand)
}
```

**Step 3: Implement requirements and plan stages**

`runRequirements` must:

- allow `created` or `context_loaded`;
- if current is `created`, advance to `context_loaded`;
- advance to `requirements_drafting`;
- build prompt and call runner;
- write `requirements.md`;
- append event `requirements.drafted`;
- advance to `requirements_review`.

`runPlan` must:

- require `plan_drafting`;
- call runner;
- write `plan.md`;
- append event `plan.drafted`;
- advance to `plan_review`.

**Step 4: Implement implementation stage**

`runImplementation` must:

- require `implementation`;
- call runner with `ToolModeEditAndShell`;
- append runner text and tool summary to `progress.md`;
- if quality commands are provided, run `quality.Gate`;
- append quality evidence to `progress.md`;
- on quality failure, advance to `failed_quality_gate` and return an error that includes `quality gate failed`;
- on quality success, advance to `mr_review`.

**Step 5: Implement verification stage**

`runVerification` must:

- require `verification`;
- call runner with read-only prompt;
- run quality commands when provided;
- write `verification.md` with generated verification body plus quality output;
- append event `verification.drafted`;
- keep state as `verification`.

**Step 6: Implement closeout stage**

`runCloseout` must:

- require `closeout`;
- call runner with read-only prompt;
- split response on `---DEVFLOW-MEMORY-CANDIDATES---`;
- write first part to `closeout.md`;
- write second part to `memory-candidates.md`;
- if marker is absent, write the whole response into `closeout.md` and keep template-based `memory-candidates.md` with a note saying no stable candidates were generated;
- append event `closeout.drafted`;
- keep state as `closeout`.

**Step 7: Test stage transitions**

`internal/demandflow/engine_test.go` must cover:

- requirements: `created -> requirements_review`, writes generated requirements;
- plan: `plan_drafting -> plan_review`, writes generated plan;
- implementation quality pass: `implementation -> mr_review`, appends progress;
- implementation quality fail: `implementation -> failed_quality_gate`, returns error;
- verification: stays `verification`, writes verification;
- closeout: stays `closeout`, writes closeout and memory candidates.

Use `StaticRunner` and a fake `quality.Runner` so tests do not shell out.

**Step 8: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow
git commit -m @'
Run demand stages through the workflow engine

The engine centralizes state transitions, artifact writes, quality
evidence, and stage events so the CLI and future TUI commands share one
product workflow path.

Constraint: Human confirmation gates must remain explicit
Rejected: Auto-advancing verification and closeout to completed | removes required user checkpoints
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 5: Add Runtime Agent Runner

**Files:**

- Create: `internal/demandflow/runtime_runner.go`
- Create: `internal/demandflow/runtime_runner_test.go`

**Step 1: Add production runner options**

Create `internal/demandflow/runtime_runner.go`:

```go
package demandflow

import (
    "context"
    "fmt"
    "strings"

    "github.com/jesseedcp/devflow-agent/internal/runtime/agent"
    "github.com/jesseedcp/devflow-agent/internal/runtime/config"
    "github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
    "github.com/jesseedcp/devflow-agent/internal/runtime/llm"
    "github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
    "github.com/jesseedcp/devflow-agent/internal/runtime/tools"
)

type RuntimeRunner struct {
    ConfigPath     string
    PermissionMode permissions.PermissionMode
    MaxIterations  int
}
```

**Step 2: Build tool registry**

Add:

```go
func runtimeRegistry() *tools.Registry {
    registry := tools.CreateDefaultRegistry()
    registry.Register(&tools.ToolSearchTool{Registry: registry, Protocol: "openai-compat"})
    return registry
}
```

If `CreateDefaultRegistry` requires no duplicate `ToolSearch`, inspect the function before registering. Register `ToolSearch` only if the default registry does not already include it.

**Step 3: Implement `Run`**

`RuntimeRunner.Run` must:

- load config with `config.LoadConfig(r.ConfigPath)`;
- select the first provider;
- build a system prompt that identifies Devflow as the backend demand delivery agent;
- create an LLM client with `llm.NewClient`;
- create a conversation and add `req.Prompt` as user message;
- create `agent.New(client, registry, provider.Protocol)`;
- set `WorkDir` to `req.Root`;
- set `MaxIterations` to configured value or `20`;
- set permissions:
  - read-only stages use `permissions.ModePlan`;
  - implementation uses `r.PermissionMode`;
  - if no permission mode is supplied for implementation, return an error explaining that implementation needs an explicit permission mode;
- collect `agent.StreamText`, `agent.ToolResultEvent`, `agent.ErrorEvent`, and `agent.LoopComplete`;
- return final text and tool summary.

**Step 4: Test without live LLM**

In `runtime_runner_test.go`, do not call real APIs. Test:

- `runtimeRegistry` contains core file tools;
- implementation stage without explicit permission mode returns a clear error;
- read-only stages map to plan permission mode through a helper function:

```go
func permissionModeFor(req RunnerRequest, explicit permissions.PermissionMode) (permissions.PermissionMode, error)
```

**Step 5: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow
git commit -m @'
Connect demand stages to the runtime agent loop

Production demandflow execution now has a RuntimeRunner that can use
the migrated Devflow agent loop while tests stay deterministic through
the Runner interface.

Constraint: Non-interactive implementation must not silently bypass permissions
Rejected: Always using bypassPermissions | unsafe default for code-changing stages
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 6: Add GitLab Review Adapter

**Files:**

- Modify: `internal/adapters/review.go`
- Create: `internal/adapters/gitlab.go`
- Create: `internal/adapters/gitlab_test.go`

**Step 1: Extend review reference**

Modify `ReviewRef`:

```go
type ReviewRef struct {
    Project      string
    MergeRequest string
    BaseURL      string
    Token        string
}
```

Existing tests should continue to pass.

**Step 2: Implement GitLab adapter**

Create `internal/adapters/gitlab.go` using only `net/http`, `encoding/json`, and standard library URL escaping.

Required behavior:

- default base URL: `https://gitlab.com`;
- token from `ReviewRef.Token`, else `GITLAB_TOKEN`;
- `ListUnresolved` calls:

```text
GET /api/v4/projects/{url.PathEscape(project)}/merge_requests/{iid}/discussions
```

- parse unresolved notes where `resolved == false`;
- encode comment ID as `discussionID + ":" + noteID`;
- mark blocking true when unresolved;
- include `FilePath` and `Line` when GitLab note position has `new_path` and `new_line`.

`Reply` must:

- split comment ID into discussion ID and note ID;
- call:

```text
POST /api/v4/projects/{project}/merge_requests/{iid}/discussions/{discussionID}/notes
```

- send form field `body=<reply text>`.

**Step 3: Test with `httptest.Server`**

Tests must cover:

- request includes `PRIVATE-TOKEN`;
- `ListUnresolved` flattens unresolved notes;
- resolved notes are ignored;
- `Reply` posts to the expected discussion path;
- missing token returns a clear error.

**Step 4: Commit**

```powershell
gofmt -w internal/adapters/*.go
go test ./internal/adapters -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/adapters
git commit -m @'
Let demandflow inspect GitLab MR review state

The GitLab adapter implements the existing review abstraction with
standard-library HTTP so MR review can become a workflow gate without
adding dependencies.

Constraint: No new dependency for GitLab API access
Rejected: Baking GitLab calls into demandflow | would prevent alternate review adapters
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/adapters -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 7: Add MR Review Demandflow Stage

**Files:**

- Create: `internal/demandflow/review.go`
- Create: `internal/demandflow/review_test.go`
- Modify: `internal/demandflow/engine.go`

**Step 1: Add review options**

Add to `internal/demandflow/review.go`:

```go
package demandflow

import (
    "context"
    "fmt"
    "strings"

    "github.com/jesseedcp/devflow-agent/internal/adapters"
    "github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ReviewOptions struct {
    Adapter adapters.ReviewAdapter
    Ref     adapters.ReviewRef
}
```

**Step 2: Implement `runMRReview`**

`runMRReview` must:

- require demand state `mr_review`;
- require a non-nil adapter;
- list unresolved comments;
- append a markdown summary to `progress.md`;
- if any comment has `Blocking == true`, keep state `mr_review` and return error `blocking review comments remain`;
- if no blocking unresolved comments remain, advance to `verification`;
- append event `mr_review.cleared`.

**Step 3: Test MR review behavior**

Use a fake adapter:

```go
type fakeReviewAdapter struct {
    comments []adapters.ReviewComment
}

func (f fakeReviewAdapter) ListUnresolved(ctx context.Context, ref adapters.ReviewRef) ([]adapters.ReviewComment, error) {
    return f.comments, nil
}

func (f fakeReviewAdapter) Reply(ctx context.Context, ref adapters.ReviewRef, commentID string, body string) error {
    return nil
}
```

Tests:

- no comments: `mr_review -> verification`;
- nonblocking comment: `mr_review -> verification`;
- blocking comment: remains `mr_review` and returns error.

**Step 4: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow
git commit -m @'
Make MR review a demandflow gate

MR review now participates in the product state machine: unresolved
blocking comments hold the demand in review, while a clean adapter
result advances toward verification.

Constraint: Review state must be adapter-driven, not hardcoded to GitLab
Rejected: Advancing to verification immediately after implementation | skips required collaboration evidence
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 8: Add `devflow run` CLI

**Files:**

- Create: `internal/cli/run.go`
- Create: `internal/cli/run_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Step 1: Update help text**

Add to `helpText`:

```text
  devflow run --demand <id> --stage <requirements|plan|implementation|mr-review|verification|closeout>
```

Add command description:

```text
  run       Run one backend-demand agent stage
```

**Step 2: Dispatch command**

In `Run`, add:

```go
case "run":
    return runDemandStage(args[1:], stdout, stderr)
```

**Step 3: Implement flags**

`runDemandStage` supports:

```text
--root .
--demand <id>
--stage <requirements|plan|implementation|mr-review|verification|closeout>
--config <path>
--permission-mode <acceptEdits|bypassPermissions>
--quality-command <program and args> repeated
--gitlab-project <project>
--gitlab-mr <iid>
--gitlab-base-url <url>
```

Use existing `parseCommandLine` to parse each `--quality-command`.

**Step 4: Create runner**

For production:

```go
runner := demandflow.RuntimeRunner{
    ConfigPath:     configPath,
    PermissionMode: parsedPermissionMode,
    MaxIterations:  20,
}
```

For tests, add a package variable:

```go
var newDemandRunner = func(configPath string, mode permissions.PermissionMode) demandflow.Runner {
    return demandflow.RuntimeRunner{ConfigPath: configPath, PermissionMode: mode, MaxIterations: 20}
}
```

Tests can stub `newDemandRunner`.

**Step 5: Wire review adapter**

For `--stage mr-review`, require `--gitlab-project` and `--gitlab-mr` unless tests inject an adapter through a package variable:

```go
var newReviewAdapter = func() adapters.ReviewAdapter {
    return adapters.GitLabReviewAdapter{}
}
```

Build `adapters.ReviewRef` from flags.

**Step 6: Print concise success**

On success:

```text
stage <stage> completed for <demand>
```

On quality failure or blocking MR comments, return nonzero error with the engine error.

**Step 7: CLI tests**

`internal/cli/run_test.go` must cover:

- missing `--demand` errors;
- missing `--stage` errors;
- unsupported stage errors;
- requirements stage calls engine and writes artifact using stub runner;
- quality command parsing handles quoted arguments;
- `mr-review` requires GitLab ref flags;
- help output includes `run`.

**Step 8: Commit**

```powershell
gofmt -w internal/cli/*.go
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
git add internal/cli
git commit -m @'
Expose backend demand stages through devflow run

The CLI now has a single stage runner command that drives demandflow
while keeping existing start, confirm, verify, closeout, and chat
commands stable.

Constraint: Existing CLI behavior is already covered and must stay compatible
Rejected: Adding separate top-level commands for every stage | fragments the product flow
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

## Task 9: Add End-To-End Slim Loop Test

**Files:**

- Create: `internal/demandflow/e2e_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

**Step 1: Add slim loop test**

Create a test that performs:

1. `artifacts.Store.CreateDemand`
2. `Engine.Run(StageRequirements)` with fake runner
3. `workflow.Advance` through confirmation-equivalent state changes by saving demand states directly in test helper
4. `Engine.Run(StagePlan)`
5. confirmation-equivalent state change to `implementation`
6. `Engine.Run(StageImplementation)` with passing fake quality runner
7. `Engine.Run(StageMRReview)` with fake adapter and no blocking comments
8. `Engine.Run(StageVerification)`
9. confirmation-equivalent state change to `closeout`
10. `Engine.Run(StageCloseout)`

Assertions:

- final state before last human confirmation is `closeout`;
- all artifacts contain fake runner output;
- `memory-candidates.md` contains the stable knowledge section;
- `events.jsonl` contains `requirements.drafted`, `plan.drafted`, `implementation.completed`, `mr_review.cleared`, `verification.drafted`, and `closeout.drafted`.

**Step 2: Commit**

```powershell
gofmt -w internal/demandflow/*.go
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
git diff --check
git add internal/demandflow docs/migration/mewcode-source-manifest.md
git commit -m @'
Prove the slim backend demand loop end to end

The end-to-end demandflow test locks the MVP product path from
requirements through closeout without live LLM, GitLab, or shell
dependencies.

Constraint: The first product loop must be deterministic in CI
Rejected: Using a live provider for the loop test | would make core workflow validation environment-dependent
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

## Task 10: Final Verification And Push

**Files:** none unless final documentation corrections are needed

Run:

```powershell
go test ./internal/demandflow -count=1
go test ./internal/adapters -count=1
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
git status --short --branch
```

Expected tracked tree: clean.

Expected allowed untracked files:

```text
?? .gocache/
?? devflow.exe
```

Push after final verification:

```powershell
git push origin feature/devflow-v0.1
```

## Manual Smoke Tests

After automated verification, run these manually from the worktree:

```powershell
.\devflow.exe help
.\devflow.exe start --title "Smoke coupon check" --description "Only active members can claim coupons"
```

If a local `.devflow/config.yaml` exists and points to a test provider:

```powershell
.\devflow.exe run --demand smoke-coupon-check --stage requirements
```

Do not print API keys in logs or final reports.

## Definition Of Done

Wave 5 is complete when:

- `internal/demandflow` exists and owns product-stage orchestration.
- `devflow run --stage requirements` writes `requirements.md` and moves to `requirements_review`.
- `devflow run --stage plan` writes `plan.md` and moves to `plan_review`.
- `devflow run --stage implementation` can use the runtime agent runner and quality gate, then moves to `mr_review` or `failed_quality_gate`.
- `devflow run --stage mr-review` uses the review adapter gate and moves to `verification` only when blocking comments are clear.
- `devflow run --stage verification` writes `verification.md` and keeps the human confirmation gate.
- `devflow run --stage closeout` writes `closeout.md` and `memory-candidates.md` and keeps the human confirmation gate.
- Existing commands `start`, `confirm`, `verify`, `closeout`, `help`, `chat`, and `tui` still pass tests.
- Full verification passes:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- The branch is pushed to `origin/feature/devflow-v0.1`.

# Devflow Agent v0.1 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable Devflow Agent foundation: a Go CLI that creates backend demand workspaces, enforces workflow gates, writes stage artifacts, records evidence, and prepares the repository for later MewCode/Eino-powered agent execution.

**Architecture:** Devflow owns the product workflow state machine and artifact model. v0.1 uses deterministic CLI commands and file-backed project memory so the delivery loop is testable before adding LLM execution, GitLab network calls, or Eino graphs. MewCode remains the reference execution kernel; this plan records the integration seam without copying MewCode internals yet.

**Tech Stack:** Go 1.23 standard library, file-backed JSON/Markdown artifacts, table-driven Go tests, no external runtime dependencies in v0.1.

---

## Scope

This plan implements the v0.1 foundation from the product spec:

```text
需求文本/PRD 链接
-> requirements.md
-> plan.md
-> implementation/test quality gate
-> MR collaboration interface
-> verification.md
-> closeout.md
-> stable knowledge candidates
-> next-demand reuse
```

This plan deliberately does not implement LLM calls, Eino graphs, real GitLab API calls, IM callbacks, or production deployment. It creates the interfaces and deterministic local loop those integrations will plug into.

## File Structure

Create these files:

- `go.mod` - module definition for `github.com/jesseedcp/devflow-agent`.
- `cmd/devflow/main.go` - process entry point.
- `internal/cli/cli.go` - command parsing and command dispatch.
- `internal/workflow/state.go` - workflow states, transitions, and gate errors.
- `internal/workflow/state_test.go` - workflow transition tests.
- `internal/artifacts/model.go` - demand metadata, event, and artifact names.
- `internal/artifacts/store.go` - file-backed workspace read/write operations.
- `internal/artifacts/store_test.go` - workspace creation and persistence tests.
- `internal/templates/templates.go` - Markdown templates for requirements, plan, verification, closeout, and memory candidates.
- `internal/quality/gate.go` - command runner abstraction and quality gate execution.
- `internal/quality/gate_test.go` - fake runner tests for pass/fail quality gates.
- `internal/adapters/review.go` - review adapter interfaces and MR comment model.
- `internal/memory/store.go` - local project memory candidate storage and search.
- `internal/memory/store_test.go` - local memory indexing and search tests.
- `docs/architecture/mewcode-reuse.md` - explicit MewCode reuse assessment.

Modify these files:

- `README.md` - add v0.1 build/test/use instructions.

## Task 1: Scaffold Go Module And CLI Entry Point

**Files:**
- Create: `go.mod`
- Create: `cmd/devflow/main.go`
- Create: `internal/cli/cli.go`
- Modify: `README.md`

- [ ] **Step 1: Write the initial CLI tests as command examples in README**

Add this section to `README.md` after the existing spec link:

```markdown
## v0.1 CLI shape

The first implementation exposes a deterministic local CLI:

```bash
go test ./...
go run ./cmd/devflow help
go run ./cmd/devflow start --title "Add coupon eligibility check" --description "Only active members can claim coupons"
```

The CLI writes demand workspaces under `.devflow/demands/<demand-id>/`.
```

- [ ] **Step 2: Add the module file**

Create `go.mod`:

```go
module github.com/jesseedcp/devflow-agent

go 1.23
```

- [ ] **Step 3: Add the process entry point**

Create `cmd/devflow/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/jesseedcp/devflow-agent/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Add minimal command dispatch**

Create `internal/cli/cli.go`:

```go
package cli

import (
	"fmt"
	"io"
)

const helpText = `devflow - backend demand delivery agent

Usage:
  devflow help
  devflow start --title <title> --description <text>

Commands:
  help    Show this help text
  start   Create a new demand workspace
`

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		_, err := fmt.Fprint(stdout, helpText)
		return err
	}
	switch args[0] {
	case "start":
		return fmt.Errorf("start command requires the artifact store")
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}
```

- [ ] **Step 5: Verify the scaffold**

Run: `go test ./...`

Expected: `?    github.com/jesseedcp/devflow-agent/cmd/devflow [no test files]` and `?    github.com/jesseedcp/devflow-agent/internal/cli [no test files]`.

- [ ] **Step 6: Commit**

```bash
git add go.mod cmd/devflow/main.go internal/cli/cli.go README.md
git commit -m "Create the Devflow CLI foundation" -m "The empty product repository needs a runnable Go entry point before workflow behavior can be added. This commit keeps the first CLI deterministic and dependency-free so later workflow tests have a stable surface.

Constraint: Use Go 1.23 because that is the installed local toolchain
Confidence: high
Scope-risk: narrow
Directive: Keep v0.1 CLI behavior deterministic until the workflow core is tested
Tested: go test ./...
Not-tested: No demand workflow commands exist yet"
```

## Task 2: Implement Workflow State Machine

**Files:**
- Create: `internal/workflow/state.go`
- Create: `internal/workflow/state_test.go`

- [ ] **Step 1: Write failing workflow tests**

Create `internal/workflow/state_test.go`:

```go
package workflow

import "testing"

func TestAdvanceHappyPath(t *testing.T) {
	state := Created
	path := []State{
		ContextLoaded,
		RequirementsDrafting,
		RequirementsReview,
		PlanDrafting,
		PlanReview,
		Implementation,
		MRReview,
		Verification,
		Closeout,
		Completed,
	}
	for _, next := range path {
		var err error
		state, err = Advance(state, next)
		if err != nil {
			t.Fatalf("advance to %s: %v", next, err)
		}
	}
	if state != Completed {
		t.Fatalf("expected completed, got %s", state)
	}
}

func TestAdvanceRejectsSkippedGate(t *testing.T) {
	_, err := Advance(RequirementsReview, Implementation)
	if err == nil {
		t.Fatal("expected skipped plan gate to fail")
	}
}

func TestReturnedStates(t *testing.T) {
	state, err := Advance(MRReview, ReturnedToRequirements)
	if err != nil {
		t.Fatalf("return to requirements: %v", err)
	}
	if state != ReturnedToRequirements {
		t.Fatalf("expected returned_to_requirements, got %s", state)
	}
	state, err = Advance(ReturnedToRequirements, RequirementsDrafting)
	if err != nil {
		t.Fatalf("restart requirements drafting: %v", err)
	}
	if state != RequirementsDrafting {
		t.Fatalf("expected requirements_drafting, got %s", state)
	}
}

func TestRequiresHumanConfirmation(t *testing.T) {
	if !RequiresHumanConfirmation(RequirementsReview) {
		t.Fatal("requirements_review must require confirmation")
	}
	if !RequiresHumanConfirmation(PlanReview) {
		t.Fatal("plan_review must require confirmation")
	}
	if RequiresHumanConfirmation(Implementation) {
		t.Fatal("implementation does not directly ask for confirmation")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workflow`

Expected: FAIL with `undefined: Created`.

- [ ] **Step 3: Implement workflow states**

Create `internal/workflow/state.go`:

```go
package workflow

import "fmt"

type State string

const (
	Created              State = "created"
	ContextLoaded        State = "context_loaded"
	RequirementsDrafting State = "requirements_drafting"
	RequirementsReview   State = "requirements_review"
	PlanDrafting         State = "plan_drafting"
	PlanReview           State = "plan_review"
	Implementation       State = "implementation"
	MRReview             State = "mr_review"
	Verification         State = "verification"
	Closeout             State = "closeout"
	Completed            State = "completed"

	BlockedNeedUser       State = "blocked_need_user"
	BlockedNeedPlatform   State = "blocked_need_platform"
	FailedQualityGate     State = "failed_quality_gate"
	ReturnedToRequirements State = "returned_to_requirements"
	ReturnedToPlan         State = "returned_to_plan"
	Cancelled             State = "cancelled"
)

var allowedTransitions = map[State]map[State]bool{
	Created:              {ContextLoaded: true, Cancelled: true},
	ContextLoaded:        {RequirementsDrafting: true, BlockedNeedUser: true, Cancelled: true},
	RequirementsDrafting: {RequirementsReview: true, BlockedNeedUser: true, Cancelled: true},
	RequirementsReview:   {PlanDrafting: true, RequirementsDrafting: true, Cancelled: true},
	PlanDrafting:         {PlanReview: true, BlockedNeedUser: true, Cancelled: true},
	PlanReview:           {Implementation: true, PlanDrafting: true, Cancelled: true},
	Implementation:       {MRReview: true, FailedQualityGate: true, BlockedNeedPlatform: true, Cancelled: true},
	MRReview:             {Verification: true, ReturnedToRequirements: true, ReturnedToPlan: true, BlockedNeedUser: true, Cancelled: true},
	Verification:         {Closeout: true, FailedQualityGate: true, BlockedNeedUser: true, Cancelled: true},
	Closeout:             {Completed: true, BlockedNeedUser: true, Cancelled: true},
	ReturnedToRequirements: {RequirementsDrafting: true, Cancelled: true},
	ReturnedToPlan:         {PlanDrafting: true, Cancelled: true},
	FailedQualityGate:      {Implementation: true, Verification: true, Cancelled: true},
	BlockedNeedUser:        {RequirementsDrafting: true, PlanDrafting: true, Implementation: true, MRReview: true, Verification: true, Closeout: true, Cancelled: true},
	BlockedNeedPlatform:    {Implementation: true, MRReview: true, Verification: true, Cancelled: true},
}

func Advance(current State, next State) (State, error) {
	if current == next {
		return current, nil
	}
	if allowedTransitions[current][next] {
		return next, nil
	}
	return current, fmt.Errorf("invalid workflow transition from %s to %s", current, next)
}

func RequiresHumanConfirmation(state State) bool {
	switch state {
	case RequirementsReview, PlanReview, Verification, Closeout:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: Run workflow tests**

Run: `go test ./internal/workflow`

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/workflow/state.go internal/workflow/state_test.go
git commit -m "Make workflow gates explicit" -m "The product depends on visible stage transitions rather than prompt-only discipline. This commit introduces the workflow states and legal transitions that prevent skipped requirements, plan, review, verification, and closeout gates.

Constraint: v0.1 uses deterministic state transitions before LLM execution is added
Rejected: Encode the workflow only in prompts | agents could skip gates under pressure
Confidence: high
Scope-risk: narrow
Directive: Do not add new transitions without tests that show why the gate remains safe
Tested: go test ./...
Not-tested: No CLI command writes workflow state yet"
```

## Task 3: Add Demand Workspace And Artifact Store

**Files:**
- Create: `internal/artifacts/model.go`
- Create: `internal/artifacts/store.go`
- Create: `internal/artifacts/store_test.go`

- [ ] **Step 1: Write failing artifact store tests**

Create `internal/artifacts/store_test.go`:

```go
package artifacts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDemandWorkspace(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand := Demand{
		ID:          "add-coupon-check",
		Title:       "Add coupon check",
		Description: "Only active members can claim coupons",
		Source:      "manual",
		State:       "created",
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	for _, name := range []string{
		"demand.json",
		"requirements.md",
		"plan.md",
		"progress.md",
		"verification.md",
		"closeout.md",
		"memory-candidates.md",
		"events.jsonl",
	} {
		if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", demand.ID, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
}

func TestLoadDemand(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	original := Demand{ID: "risk-flag", Title: "Risk flag", Description: "Expose risk flag", Source: "manual", State: "created"}
	if err := store.CreateDemand(original); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	loaded, err := store.LoadDemand(original.ID)
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if loaded.Title != original.Title {
		t.Fatalf("expected title %q, got %q", original.Title, loaded.Title)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/artifacts`

Expected: FAIL with `undefined: NewStore`.

- [ ] **Step 3: Define artifact models**

Create `internal/artifacts/model.go`:

```go
package artifacts

import "time"

type Demand struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Event struct {
	Time    time.Time         `json:"time"`
	Type    string            `json:"type"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data,omitempty"`
}

const (
	DemandFile           = "demand.json"
	RequirementsFile     = "requirements.md"
	PlanFile             = "plan.md"
	ProgressFile         = "progress.md"
	VerificationFile     = "verification.md"
	CloseoutFile         = "closeout.md"
	MemoryCandidatesFile = "memory-candidates.md"
	EventsFile           = "events.jsonl"
)
```

- [ ] **Step 4: Implement file-backed store**

Create `internal/artifacts/store.go`:

```go
package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/templates"
)

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) DemandDir(id string) string {
	return filepath.Join(s.root, ".devflow", "demands", id)
}

func (s *Store) CreateDemand(d Demand) error {
	now := time.Now().UTC()
	if d.ID == "" {
		return fmt.Errorf("demand id is required")
	}
	if d.Title == "" {
		return fmt.Errorf("demand title is required")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	d.UpdatedAt = now
	dir := s.DemandDir(d.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create demand dir: %w", err)
	}
	if err := s.writeJSON(filepath.Join(dir, DemandFile), d); err != nil {
		return err
	}
	files := map[string]string{
		RequirementsFile:     templates.Requirements(d.Title, d.Description),
		PlanFile:             templates.Plan(d.Title),
		ProgressFile:         "# Progress\n\n",
		VerificationFile:     templates.Verification(d.Title),
		CloseoutFile:         templates.Closeout(d.Title),
		MemoryCandidatesFile: templates.MemoryCandidates(d.Title),
		EventsFile:           "",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return s.AppendEvent(d.ID, Event{Time: now, Type: "demand.created", Message: "demand workspace created"})
}

func (s *Store) LoadDemand(id string) (Demand, error) {
	var demand Demand
	data, err := os.ReadFile(filepath.Join(s.DemandDir(id), DemandFile))
	if err != nil {
		return demand, fmt.Errorf("read demand: %w", err)
	}
	if err := json.Unmarshal(data, &demand); err != nil {
		return demand, fmt.Errorf("decode demand: %w", err)
	}
	return demand, nil
}

func (s *Store) SaveDemand(d Demand) error {
	d.UpdatedAt = time.Now().UTC()
	return s.writeJSON(filepath.Join(s.DemandDir(d.ID), DemandFile), d)
}

func (s *Store) AppendEvent(id string, event Event) error {
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	path := filepath.Join(s.DemandDir(id), EventsFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return nil
}

func (s *Store) writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Add artifact templates**

Create `internal/templates/templates.go`:

```go
package templates

import "fmt"

func Requirements(title, description string) string {
	return fmt.Sprintf(`# Requirements: %s

## 目标行为
%s

## 非目标范围

## 业务规则

## 用户/调用方影响

## 验收标准

## 风险与歧义

## 待确认问题

## 人工确认记录
`, title, description)
}

func Plan(title string) string {
	return fmt.Sprintf(`# Technical Plan: %s

## 当前实现与代码事实

## 目标设计

## 改动范围

## 数据结构/API/配置变化

## 测试策略

## 验收方式

## 风险与回滚

## 不做事项

## 人工确认记录
`, title)
}

func Verification(title string) string {
	return fmt.Sprintf(`# Verification: %s

## 验收标准映射

## 自动化测试结果

## 手动验证记录

## 接口/日志/监控证据

## 未覆盖风险

## 结论
`, title)
}

func Closeout(title string) string {
	return fmt.Sprintf(`# Closeout: %s

## 需求结果

## 关键产物链接

## MR 评论与处理摘要

## 验收证据摘要

## 稳定知识候选

## 流程改进候选

## 一次性材料归档

## 人工确认记录
`, title)
}

func MemoryCandidates(title string) string {
	return fmt.Sprintf(`# Memory Candidates: %s

## 稳定知识候选

## 流程改进候选

## 不进入长期知识的材料
`, title)
}
```

- [ ] **Step 6: Run artifact tests**

Run: `go test ./internal/artifacts`

Expected: PASS.

- [ ] **Step 7: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/artifacts internal/templates
git commit -m "Persist demand workspaces and stage artifacts" -m "The workflow needs durable files before any agent execution can be trusted. This commit adds the demand workspace shape and the required stage artifacts from the product spec.

Constraint: v0.1 uses local files for project memory and demand state
Confidence: high
Scope-risk: narrow
Directive: Keep artifact filenames stable because later adapters and agents will depend on them
Tested: go test ./...
Not-tested: CLI start command is added in the next task"
```

## Task 4: Wire The `start` CLI Command

**Files:**
- Modify: `internal/cli/cli.go`
- Create: `internal/cli/cli_test.go`

- [ ] **Step 1: Write failing CLI start test**

Create `internal/cli/cli_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartCreatesDemandWorkspace(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	err := Run([]string{
		"start",
		"--root", root,
		"--title", "Add coupon check",
		"--description", "Only active members can claim coupons",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "add-coupon-check") {
		t.Fatalf("expected demand id in output, got %q", output)
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "demands", "add-coupon-check", "requirements.md")); err != nil {
		t.Fatalf("requirements file missing: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli`

Expected: FAIL with `start command requires the artifact store`.

- [ ] **Step 3: Implement start command parsing**

Replace `internal/cli/cli.go` with:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

const helpText = `devflow - backend demand delivery agent

Usage:
  devflow help
  devflow start --title <title> --description <text>

Commands:
  help    Show this help text
  start   Create a new demand workspace
`

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		_, err := fmt.Fprint(stdout, helpText)
		return err
	}
	switch args[0] {
	case "start":
		return runStart(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}

func runStart(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	title := fs.String("title", "", "demand title")
	description := fs.String("description", "", "demand description")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*title) == "" {
		return fmt.Errorf("--title is required")
	}
	id := slugify(*title)
	store := artifacts.NewStore(*root)
	demand := artifacts.Demand{
		ID:          id,
		Title:       strings.TrimSpace(*title),
		Description: strings.TrimSpace(*description),
		Source:      "manual",
		State:       string(workflow.RequirementsReview),
	}
	if err := store.CreateDemand(demand); err != nil {
		return err
	}
	absRoot, _ := os.Getwd()
	if *root != "." {
		absRoot = *root
	}
	_, err := fmt.Fprintf(stdout, "Created demand %s under %s\n", id, absRoot)
	return err
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "demand"
	}
	return s
}
```

- [ ] **Step 4: Run CLI tests**

Run: `go test ./internal/cli`

Expected: PASS.

- [ ] **Step 5: Run the CLI manually**

Run:

```bash
go run ./cmd/devflow start --title "Add coupon check" --description "Only active members can claim coupons"
```

Expected: output includes `Created demand add-coupon-check`.

- [ ] **Step 6: Remove the manual demand workspace**

Run:

```powershell
Remove-Item -Recurse -Force .devflow
```

Expected: `.devflow` is removed from the repository working tree.

- [ ] **Step 7: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "Create demand workspaces from the CLI" -m "The first user-facing loop starts with a demand workspace. This commit wires the start command to create the spec-defined artifacts without introducing LLM or platform dependencies.

Constraint: Start must be deterministic and testable with temp directories
Confidence: high
Scope-risk: narrow
Directive: Do not add network calls to start; demand ingestion adapters belong behind separate interfaces
Tested: go test ./...; go run ./cmd/devflow start --title \"Add coupon check\" --description \"Only active members can claim coupons\"
Not-tested: PRD URL ingestion is outside this foundation plan"
```

## Task 5: Add Human Confirmation Command

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/artifacts/store.go`

- [ ] **Step 1: Write failing confirmation test**

Append to `internal/cli/cli_test.go`:

```go
func TestConfirmRequirementsAdvancesToPlanDrafting(t *testing.T) {
	root := t.TempDir()
	if err := Run([]string{"start", "--root", root, "--title", "Add coupon check", "--description", "Only active members can claim coupons"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	var stdout bytes.Buffer
	err := Run([]string{"confirm", "--root", root, "--demand", "add-coupon-check", "--stage", "requirements", "--by", "alice", "--summary", "requirements are accurate"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !strings.Contains(stdout.String(), "requirements confirmed") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", "requirements.md"))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if !strings.Contains(string(data), "alice") {
		t.Fatalf("confirmation author missing from requirements: %s", data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli`

Expected: FAIL with `unknown command "confirm"`.

- [ ] **Step 3: Add append helper to artifact store**

Add this method to `internal/artifacts/store.go`:

```go
func (s *Store) AppendToArtifact(id string, name string, content string) error {
	path := filepath.Join(s.DemandDir(id), name)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open artifact %s: %w", name, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("append artifact %s: %w", name, err)
	}
	return nil
}
```

- [ ] **Step 4: Add confirm command**

Update `internal/cli/cli.go`:

1. Add import:

```go
	"time"
```

2. Add `confirm` to `helpText`:

```text
  devflow confirm --demand <id> --stage <requirements|plan|verification|closeout> --by <name> --summary <text>
```

3. Add switch case:

```go
	case "confirm":
		return runConfirm(args[1:], stdout)
```

4. Add this function:

```go
func runConfirm(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("confirm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	demandID := fs.String("demand", "", "demand id")
	stage := fs.String("stage", "", "stage name")
	by := fs.String("by", "", "confirming person")
	summary := fs.String("summary", "", "confirmation summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *demandID == "" || *stage == "" || *by == "" || *summary == "" {
		return fmt.Errorf("--demand, --stage, --by, and --summary are required")
	}
	store := artifacts.NewStore(*root)
	demand, err := store.LoadDemand(*demandID)
	if err != nil {
		return err
	}
	artifactName, nextState, label, err := confirmationTarget(*stage)
	if err != nil {
		return err
	}
	current := workflow.State(demand.State)
	advanced, err := workflow.Advance(current, nextState)
	if err != nil {
		return err
	}
	demand.State = string(advanced)
	if err := store.SaveDemand(demand); err != nil {
		return err
	}
	record := fmt.Sprintf("\n- %s confirmed by %s at %s: %s\n", label, *by, time.Now().UTC().Format(time.RFC3339), *summary)
	if err := store.AppendToArtifact(*demandID, artifactName, record); err != nil {
		return err
	}
	if err := store.AppendEvent(*demandID, artifacts.Event{Type: "stage.confirmed", Message: label + " confirmed", Data: map[string]string{"by": *by, "stage": *stage}}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s confirmed for %s\n", *stage, *demandID)
	return err
}

func confirmationTarget(stage string) (artifact string, next workflow.State, label string, err error) {
	switch stage {
	case "requirements":
		return artifacts.RequirementsFile, workflow.PlanDrafting, "requirements", nil
	case "plan":
		return artifacts.PlanFile, workflow.Implementation, "plan", nil
	case "verification":
		return artifacts.VerificationFile, workflow.Closeout, "verification", nil
	case "closeout":
		return artifacts.CloseoutFile, workflow.Completed, "closeout", nil
	default:
		return "", "", "", fmt.Errorf("unsupported confirmation stage %q", stage)
	}
}
```

- [ ] **Step 5: Run confirmation tests**

Run: `go test ./internal/cli`

Expected: PASS.

- [ ] **Step 6: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go internal/artifacts/store.go
git commit -m "Record human confirmations as workflow gates" -m "The product spec treats human confirmation as a quality gate rather than an exception. This commit adds the first CLI confirmation path and records confirmations in both artifact files and event logs.

Constraint: v0.1 uses CLI confirmation before IM confirmation exists
Confidence: medium
Scope-risk: moderate
Directive: Keep stage confirmation explicit; do not silently advance workflow states
Tested: go test ./...
Not-tested: Confirmation from MR comments or IM messages"
```

## Task 6: Add Quality Gate Runner

**Files:**
- Create: `internal/quality/gate.go`
- Create: `internal/quality/gate_test.go`

- [ ] **Step 1: Write failing quality gate tests**

Create `internal/quality/gate_test.go`:

```go
package quality

import (
	"context"
	"testing"
)

type fakeRunner struct {
	code   int
	stdout string
	stderr string
}

func (f fakeRunner) Run(ctx context.Context, dir string, name string, args ...string) Result {
	return Result{Command: name, Args: args, Dir: dir, ExitCode: f.code, Stdout: f.stdout, Stderr: f.stderr}
}

func TestGatePasses(t *testing.T) {
	gate := Gate{Runner: fakeRunner{code: 0, stdout: "ok"}}
	result := gate.Run(context.Background(), ".", Command{Name: "go", Args: []string{"test", "./..."}})
	if !result.Passed {
		t.Fatalf("expected gate to pass: %+v", result)
	}
}

func TestGateFails(t *testing.T) {
	gate := Gate{Runner: fakeRunner{code: 1, stderr: "tests failed"}}
	result := gate.Run(context.Background(), ".", Command{Name: "go", Args: []string{"test", "./..."}})
	if result.Passed {
		t.Fatalf("expected gate to fail: %+v", result)
	}
	if result.Results[0].Stderr != "tests failed" {
		t.Fatalf("stderr not captured: %+v", result)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/quality`

Expected: FAIL with `undefined: Result`.

- [ ] **Step 3: Implement quality gate**

Create `internal/quality/gate.go`:

```go
package quality

import (
	"bytes"
	"context"
	"os/exec"
)

type Command struct {
	Name string
	Args []string
}

type Result struct {
	Command  string
	Args     []string
	Dir      string
	ExitCode int
	Stdout   string
	Stderr   string
}

type GateResult struct {
	Passed  bool
	Results []Result
}

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) Result
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = 1
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
	}
	return Result{Command: name, Args: args, Dir: dir, ExitCode: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

type Gate struct {
	Runner Runner
}

func (g Gate) Run(ctx context.Context, dir string, commands ...Command) GateResult {
	runner := g.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	result := GateResult{Passed: true}
	for _, command := range commands {
		r := runner.Run(ctx, dir, command.Name, command.Args...)
		result.Results = append(result.Results, r)
		if r.ExitCode != 0 {
			result.Passed = false
		}
	}
	return result
}
```

- [ ] **Step 4: Run quality tests**

Run: `go test ./internal/quality`

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/quality/gate.go internal/quality/gate_test.go
git commit -m "Capture quality gate evidence" -m "Verification and implementation stages need command evidence, not just narrative claims. This commit adds a runner abstraction that can execute real gates or fake gates in tests.

Constraint: v0.1 starts with local command evidence
Confidence: high
Scope-risk: narrow
Directive: Preserve stdout, stderr, and exit codes for verification reports
Tested: go test ./...
Not-tested: Long-running commands and timeout policy"
```

## Task 7: Generate Verification And Closeout Reports

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/artifacts/store.go`

- [ ] **Step 1: Write failing verification CLI test**

Append to `internal/cli/cli_test.go`:

```go
func TestVerifyWritesCommandEvidence(t *testing.T) {
	root := t.TempDir()
	if err := Run([]string{"start", "--root", root, "--title", "Add coupon check", "--description", "Only active members can claim coupons"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	var stdout bytes.Buffer
	err := Run([]string{"verify", "--root", root, "--demand", "add-coupon-check", "--command", "go version"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", "verification.md"))
	if err != nil {
		t.Fatalf("read verification: %v", err)
	}
	if !strings.Contains(string(data), "go version") {
		t.Fatalf("verification missing command evidence: %s", data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli`

Expected: FAIL with `unknown command "verify"`.

- [ ] **Step 3: Add artifact overwrite helper**

Add this method to `internal/artifacts/store.go`:

```go
func (s *Store) WriteArtifact(id string, name string, content string) error {
	path := filepath.Join(s.DemandDir(id), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write artifact %s: %w", name, err)
	}
	return nil
}
```

- [ ] **Step 4: Add verify command**

Update `internal/cli/cli.go`:

1. Add imports:

```go
	"context"
	"os/exec"
```

2. Add switch case:

```go
	case "verify":
		return runVerify(args[1:], stdout)
```

3. Add function:

```go
func runVerify(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	demandID := fs.String("demand", "", "demand id")
	commandText := fs.String("command", "", "command to run")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *demandID == "" || *commandText == "" {
		return fmt.Errorf("--demand and --command are required")
	}
	parts := strings.Fields(*commandText)
	if len(parts) == 0 {
		return fmt.Errorf("--command must contain a program")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = *root
	out, err := cmd.CombinedOutput()
	status := "PASS"
	if err != nil {
		status = "FAIL"
	}
	store := artifacts.NewStore(*root)
	content := fmt.Sprintf(`# Verification

## 自动化测试结果

- Command: %s
- Status: %s

`+"```text\n%s\n```\n", *commandText, status, string(out))
	if err := store.WriteArtifact(*demandID, artifacts.VerificationFile, content); err != nil {
		return err
	}
	if err := store.AppendEvent(*demandID, artifacts.Event{Type: "verification.recorded", Message: "verification command recorded", Data: map[string]string{"command": *commandText, "status": status}}); err != nil {
		return err
	}
	_, writeErr := fmt.Fprintf(stdout, "verification recorded for %s: %s\n", *demandID, status)
	if err != nil {
		return fmt.Errorf("verification command failed: %s", string(out))
	}
	return writeErr
}
```

- [ ] **Step 5: Run verification tests**

Run: `go test ./internal/cli`

Expected: PASS.

- [ ] **Step 6: Add closeout test**

Append to `internal/cli/cli_test.go`:

```go
func TestCloseoutWritesKnowledgeCandidate(t *testing.T) {
	root := t.TempDir()
	if err := Run([]string{"start", "--root", root, "--title", "Add coupon check", "--description", "Only active members can claim coupons"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	err := Run([]string{"closeout", "--root", root, "--demand", "add-coupon-check", "--result", "Implemented active member coupon check", "--knowledge", "Coupon claims require active membership"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("closeout: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", "memory-candidates.md"))
	if err != nil {
		t.Fatalf("read memory candidates: %v", err)
	}
	if !strings.Contains(string(data), "Coupon claims require active membership") {
		t.Fatalf("knowledge candidate missing: %s", data)
	}
}
```

- [ ] **Step 7: Run closeout test to verify it fails**

Run: `go test ./internal/cli`

Expected: FAIL with `unknown command "closeout"`.

- [ ] **Step 8: Add closeout command**

Update `internal/cli/cli.go`:

1. Add switch case:

```go
	case "closeout":
		return runCloseout(args[1:], stdout)
```

2. Add function:

```go
func runCloseout(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("closeout", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	demandID := fs.String("demand", "", "demand id")
	result := fs.String("result", "", "demand result")
	knowledge := fs.String("knowledge", "", "stable knowledge candidate")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *demandID == "" || *result == "" || *knowledge == "" {
		return fmt.Errorf("--demand, --result, and --knowledge are required")
	}
	store := artifacts.NewStore(*root)
	closeout := fmt.Sprintf(`# Closeout

## 需求结果
%s

## 稳定知识候选
- %s

## 人工确认记录
`, *result, *knowledge)
	if err := store.WriteArtifact(*demandID, artifacts.CloseoutFile, closeout); err != nil {
		return err
	}
	if err := store.WriteArtifact(*demandID, artifacts.MemoryCandidatesFile, "# Memory Candidates\n\n## 稳定知识候选\n- "+*knowledge+"\n"); err != nil {
		return err
	}
	if err := store.AppendEvent(*demandID, artifacts.Event{Type: "closeout.created", Message: "closeout generated"}); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "closeout written for %s\n", *demandID)
	return err
}
```

- [ ] **Step 9: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go internal/artifacts/store.go
git commit -m "Write verification and closeout evidence" -m "The delivery loop needs proof and memory candidates after implementation. This commit adds deterministic local commands for verification evidence and closeout knowledge candidates.

Constraint: v0.1 records local command evidence and manually supplied knowledge candidates
Confidence: medium
Scope-risk: moderate
Directive: Keep verification evidence mapped to artifacts; do not rely on console output alone
Tested: go test ./...
Not-tested: SLS, monitoring, CI, and GitLab evidence adapters"
```

## Task 8: Add Review Adapter Interfaces

**Files:**
- Create: `internal/adapters/review.go`
- Create: `internal/adapters/review_test.go`

- [ ] **Step 1: Write review adapter contract test**

Create `internal/adapters/review_test.go`:

```go
package adapters

import (
	"context"
	"testing"
)

type fakeReview struct {
	comments []ReviewComment
}

func (f fakeReview) ListUnresolved(ctx context.Context, ref ReviewRef) ([]ReviewComment, error) {
	return f.comments, nil
}

func (f fakeReview) Reply(ctx context.Context, ref ReviewRef, commentID string, body string) error {
	return nil
}

func TestReviewAdapterContract(t *testing.T) {
	adapter := fakeReview{comments: []ReviewComment{{ID: "1", Body: "please add test", Blocking: true, Category: CommentImplementation}}}
	comments, err := adapter.ListUnresolved(context.Background(), ReviewRef{Project: "demo", MergeRequest: "1"})
	if err != nil {
		t.Fatalf("list unresolved: %v", err)
	}
	if len(comments) != 1 || !comments[0].Blocking {
		t.Fatalf("expected one blocking comment: %+v", comments)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapters`

Expected: FAIL with `undefined: ReviewComment`.

- [ ] **Step 3: Implement review adapter interfaces**

Create `internal/adapters/review.go`:

```go
package adapters

import "context"

type CommentCategory string

const (
	CommentRequirements   CommentCategory = "requirements"
	CommentPlan           CommentCategory = "plan"
	CommentImplementation CommentCategory = "implementation"
	CommentTest           CommentCategory = "test"
	CommentStyle          CommentCategory = "style"
)

type ReviewRef struct {
	Project      string
	MergeRequest string
}

type ReviewComment struct {
	ID       string
	Author   string
	Body     string
	FilePath string
	Line     int
	Blocking bool
	Category CommentCategory
}

type ReviewAdapter interface {
	ListUnresolved(ctx context.Context, ref ReviewRef) ([]ReviewComment, error)
	Reply(ctx context.Context, ref ReviewRef, commentID string, body string) error
}
```

- [ ] **Step 4: Run adapter tests**

Run: `go test ./internal/adapters`

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/review.go internal/adapters/review_test.go
git commit -m "Define the review adapter seam" -m "MR collaboration is central to the product, but v0.1 should not bind to GitLab network behavior before the local loop is tested. This commit defines the review adapter seam and comment categories.

Constraint: GitLab is the first planned review backend, but tests use an in-memory adapter
Confidence: high
Scope-risk: narrow
Directive: Keep GitLab-specific fields out of the generic adapter unless a v0.1 use case requires them
Tested: go test ./...
Not-tested: Real GitLab API calls"
```

## Task 9: Add File-Based Project Memory Search

**Files:**
- Create: `internal/memory/store.go`
- Create: `internal/memory/store_test.go`

- [ ] **Step 1: Write memory tests**

Create `internal/memory/store_test.go`:

```go
package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchCandidates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".devflow", "demands", "coupon", "")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "# Memory Candidates\n\n## 稳定知识候选\n- Coupon claims require active membership\n"
	if err := os.WriteFile(filepath.Join(dir, "memory-candidates.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write memory: %v", err)
	}
	store := NewStore(root)
	results, err := store.Search("coupon active")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %+v", results)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/memory`

Expected: FAIL with `undefined: NewStore`.

- [ ] **Step 3: Implement memory store**

Create `internal/memory/store.go`:

```go
package memory

import (
	"os"
	"path/filepath"
	"strings"
)

type Result struct {
	DemandID string
	Path     string
	Snippet  string
}

type Store struct {
	root string
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) Search(query string) ([]Result, error) {
	queryTerms := strings.Fields(strings.ToLower(query))
	base := filepath.Join(s.root, ".devflow", "demands")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var results []Result
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(base, entry.Name(), "memory-candidates.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if matchesAll(strings.ToLower(text), queryTerms) {
			results = append(results, Result{DemandID: entry.Name(), Path: path, Snippet: firstLine(text)})
		}
	}
	return results, nil
}

func matchesAll(text string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}
```

- [ ] **Step 4: Run memory tests**

Run: `go test ./internal/memory`

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/memory/store.go internal/memory/store_test.go
git commit -m "Search confirmed project memory candidates" -m "The closeout loop only creates value if later demands can reuse confirmed knowledge. This commit adds a file-backed memory search over local demand candidates.

Constraint: v0.1 uses local file search before vector retrieval or a memory repository exists
Confidence: medium
Scope-risk: narrow
Directive: Treat memory candidates as reviewable material, not unquestioned facts
Tested: go test ./...
Not-tested: Ranking quality beyond exact term matching"
```

## Task 10: Document MewCode And Eino Integration Decision

**Files:**
- Create: `docs/architecture/mewcode-reuse.md`
- Modify: `README.md`

- [ ] **Step 1: Create architecture decision document**

Create `docs/architecture/mewcode-reuse.md`:

```markdown
# MewCode Reuse And Eino Integration Decision

## Decision

Devflow Agent owns the product workflow state machine and stage artifacts.
MewCode is the preferred source of agent execution concepts and reusable code.
Eino is optional for later stage-level LLM graph orchestration.

## Why Devflow owns workflow

The workflow rules are product semantics:

- requirements must be confirmed before plan
- plan must be confirmed before implementation
- failed quality gates block verification
- blocking MR comments prevent closeout
- memory candidates require confirmation before reuse as stable knowledge

These rules must be visible in Devflow code and tests. They should not live only inside prompts, Eino graphs, or a generic coding-agent loop.

## What to reuse from MewCode first

Evaluate these MewCode packages before building duplicate primitives:

- `internal/tools`: tool interface and registry shape
- `internal/skills`: skill metadata and render behavior
- `internal/memory`: project memory directory conventions
- `internal/worktree`: isolated implementation workspace behavior
- `internal/agent`: streaming loop and tool execution behavior

MewCode uses `internal` packages, so Devflow cannot import them directly from another module. Reuse requires one of these paths:

1. copy a small package with attribution and tests
2. extract stable packages into a shared module
3. keep Devflow independent and use MewCode as implementation reference

## Eino placement

Eino can be introduced after v0.1 when the deterministic local workflow is passing.
Use Eino for stage-level LLM subflows:

- clarify agent
- planning agent
- review comment classifier
- verification summarizer
- closeout summarizer

Do not use Eino as the top-level product state machine.
```

- [ ] **Step 2: Link the decision from README**

Add this under the product spec link in `README.md`:

```markdown
- [MewCode reuse and Eino integration decision](docs/architecture/mewcode-reuse.md)
```

- [ ] **Step 3: Verify docs are reachable**

Run: `rg -n "MewCode|Eino|Devflow owns workflow" README.md docs/architecture/mewcode-reuse.md`

Expected: output includes the README link and the decision heading.

- [ ] **Step 4: Run tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/architecture/mewcode-reuse.md
git commit -m "Record the MewCode and Eino architecture boundary" -m "Devflow needs to benefit from MewCode without hiding product workflow semantics inside a generic agent loop. This decision records the boundary before implementation starts depending on either MewCode or Eino.

Constraint: MewCode packages are internal and cannot be directly imported across modules
Rejected: Use Eino as the top-level product state machine | workflow gates are product semantics and need first-class tests
Rejected: Rebuild all MewCode primitives immediately | duplicates working agent engineering before the workflow is validated
Confidence: high
Scope-risk: narrow
Directive: Revisit shared-module extraction only after the v0.1 local workflow passes
Tested: rg -n \"MewCode|Eino|Devflow owns workflow\" README.md docs/architecture/mewcode-reuse.md; go test ./...
Not-tested: Actual MewCode package extraction"
```

## Self-Review

### Spec Coverage

- Demand input creates a workspace: Task 4.
- Stage artifacts exist with standard names: Task 3.
- Workflow gates exist and are tested: Task 2 and Task 5.
- Local implementation/test evidence can be recorded: Task 6 and Task 7.
- Review adapter seam exists for GitLab MR collaboration: Task 8.
- Verification and closeout artifacts are generated: Task 7.
- Stable knowledge candidates and next-demand reuse begin with file-backed search: Task 9.
- MewCode/Eino boundary is explicit: Task 10.

### Known Gaps After This Plan

- Real LLM execution is outside this plan.
- Real GitLab API calls are outside this plan.
- IM notification and confirmation are outside this plan.
- CI, logs, monitoring, and release systems remain adapters for later plans.
- Requirements and plan generation are templates, not agent-authored content.

These gaps are intentional for the v0.1 foundation. The next plan should add either GitLab adapter implementation or MewCode-backed stage agent execution, not both at the same time.

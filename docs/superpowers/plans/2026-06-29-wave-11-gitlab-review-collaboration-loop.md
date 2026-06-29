# Wave 11 GitLab Review Collaboration Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn unresolved GitLab MR comments into categorized workflow actions, review evidence, and deterministic state routing for the backend demand loop.

**Architecture:** Keep GitLab API access inside `internal/adapters`, and keep workflow decisions inside `internal/demandflow`. Add deterministic review-comment classification first, then use the existing `mr-review` stage to build a review action plan and route the demand back to requirements, plan, or implementation when blocking comments require rework. Do not auto-reply to GitLab comments in this wave; generate evidence and route work safely.

**Tech Stack:** Go 1.25.0, existing `internal/adapters`, existing `internal/demandflow`, existing `internal/workflow`, existing CLI command surfaces, standard library only, no new Go dependencies.

---

## Current Baseline

Repository:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
```

Base branch:

```text
main
```

Wave 11 branch:

```text
feature/devflow-wave-11
```

Recommended worktree:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-wave-11
```

Current main after Wave 10:

```text
a5a8515 Make Wave 10 MR sync reachable and blocking
```

Current review data model:

```go
type CommentCategory string

const (
	CommentRequirements   CommentCategory = "requirements"
	CommentPlan           CommentCategory = "plan"
	CommentImplementation CommentCategory = "implementation"
	CommentTest           CommentCategory = "test"
	CommentStyle          CommentCategory = "style"
)

type ReviewComment struct {
	ID       string
	Author   string
	Body     string
	FilePath string
	Line     int
	Blocking bool
	Category CommentCategory
}
```

Current MR-review behavior:

```text
unresolved blocking comments -> stay in mr_review and return error
no blocking comments -> advance to verification
```

Wave 11 target behavior:

```text
requirements comment -> returned_to_requirements
plan/design comment -> returned_to_plan
implementation/test/style comment -> implementation
no blocking comments -> verification
```

When several blocking categories appear at once, choose the earliest safe workflow state:

```text
requirements > plan > implementation
```

That means a requirements issue wins over implementation comments because implementation fixes may be invalid until requirements are corrected.

## Scope

In scope:

- Deterministic comment classification based on file path and body text.
- GitLab adapter populates `ReviewComment.Category`.
- `devflow review-gate` prints category evidence.
- `mr-review` stage renders a review action plan into `progress.md`.
- Blocking comments route workflow to `returned_to_requirements`, `returned_to_plan`, or `implementation`.
- Events record review action category counts and selected next state.
- `devflow next` gives useful next commands for returned states.
- Docs explain the MR collaboration loop.

Out of scope:

- Auto-replying to GitLab comments.
- Resolving GitLab discussions.
- Assigning reviewers, approvals, labels, or milestones.
- LLM-based classification.
- Non-GitLab adapters.
- Push branch automation.

## File Map

Create:

- `internal/adapters/review_classifier.go` - deterministic category classifier.
- `internal/adapters/review_classifier_test.go` - classifier coverage.
- `internal/demandflow/review_action.go` - review action plan model, routing, rendering.
- `internal/demandflow/review_action_test.go` - action-plan unit tests.

Modify:

- `internal/adapters/gitlab.go` - classify GitLab comments during `ListUnresolved`.
- `internal/adapters/gitlab_test.go` - assert GitLab comments include category.
- `internal/cli/review_gate.go` - print comment category in direct review gate output.
- `internal/cli/review_gate_test.go` - assert category output.
- `internal/demandflow/engine.go` - use review action plan in `runMRReview`.
- `internal/demandflow/review.go` - render category in MR review summary.
- `internal/demandflow/review_test.go` - assert route decisions and evidence.
- `internal/demandflow/status.go` - next actions for returned states and implementation rework.
- `internal/demandflow/status_test.go` - expected commands for returned states.
- `internal/workflow/state.go` - allow `MRReview -> Implementation`.
- `internal/workflow/state_test.go` - transition coverage.
- `internal/dogfood/runner.go` - offline dogfood keeps no-blocker path stable.
- `internal/dogfood/runner_test.go` - assert action plan evidence is present.
- `docs/user-guide/backend-demand-loop.md` - user-facing MR collaboration loop.
- `docs/user-guide/dogfood-smoke.md` - mention categorized review evidence.
- `docs/release/v0.1.md` - update known limits.

## Task 0: Preflight And Worktree

**Files:**

- Read: `git status`
- Read: `docs/superpowers/plans/2026-06-29-wave-11-gitlab-review-collaboration-loop.md`

- [ ] Confirm main is clean and up to date.

```powershell
git status --short --branch
git log --oneline --decorate -3
gh pr list --state open --limit 10
```

Expected:

```text
## main...origin/main
```

Open PRs may exist, but there must be no open Wave 10 PR and no uncommitted files.

- [ ] Create isolated Wave 11 worktree.

```powershell
git fetch origin
git worktree add -b feature/devflow-wave-11 .worktrees/devflow-wave-11 origin/main
cd .worktrees/devflow-wave-11
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-11
```

- [ ] Run baseline verification.

```powershell
go test ./internal/adapters ./internal/demandflow ./internal/cli ./internal/workflow -count=1
git diff --check
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/adapters
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
ok  	github.com/jesseedcp/devflow-agent/internal/cli
ok  	github.com/jesseedcp/devflow-agent/internal/workflow
```

No `git diff --check` output.

## Task 1: Add Deterministic Review Comment Classification

**Files:**

- Create: `internal/adapters/review_classifier.go`
- Create: `internal/adapters/review_classifier_test.go`

- [ ] Write failing classifier tests.

Create `internal/adapters/review_classifier_test.go`:

```go
package adapters

import "testing"

func TestClassifyReviewCommentByFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		path string
		want CommentCategory
	}{
		{name: "requirements file", body: "missing acceptance criteria", path: ".devflow/demands/add/requirements.md", want: CommentRequirements},
		{name: "plan file", body: "design misses rollback", path: ".devflow/demands/add/plan.md", want: CommentPlan},
		{name: "test file", body: "assert the boundary case", path: "internal/service/coupon_test.go", want: CommentTest},
		{name: "implementation file", body: "nil handling is wrong", path: "internal/service/coupon.go", want: CommentImplementation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyReviewComment(tc.body, tc.path); got != tc.want {
				t.Fatalf("category = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestClassifyReviewCommentByBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want CommentCategory
	}{
		{name: "requirements keyword", body: "This changes the business rule and acceptance criteria.", want: CommentRequirements},
		{name: "plan keyword", body: "The architecture should use the existing adapter boundary.", want: CommentPlan},
		{name: "test keyword", body: "Please add regression coverage for the failure path.", want: CommentTest},
		{name: "style keyword", body: "nit: rename this helper for readability.", want: CommentStyle},
		{name: "implementation fallback", body: "This branch can panic when the user is nil.", want: CommentImplementation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyReviewComment(tc.body, ""); got != tc.want {
				t.Fatalf("category = %s, want %s", got, tc.want)
			}
		})
	}
}
```

- [ ] Run tests and verify RED.

```powershell
go test ./internal/adapters -run TestClassifyReviewComment -count=1
```

Expected:

```text
undefined: ClassifyReviewComment
```

- [ ] Implement classifier.

Create `internal/adapters/review_classifier.go`:

```go
package adapters

import "strings"

// ClassifyReviewComment maps a review comment to the earliest workflow surface
// that can resolve it. It is deterministic so release gates do not depend on an LLM.
func ClassifyReviewComment(body string, filePath string) CommentCategory {
	path := strings.ToLower(strings.TrimSpace(filePath))
	text := strings.ToLower(strings.TrimSpace(body))

	switch {
	case containsAny(path, "requirements.md", "requirement", "prd"):
		return CommentRequirements
	case containsAny(path, "plan.md", "design", "architecture"):
		return CommentPlan
	case containsAny(path, "_test.go", ".test.", ".spec.", "/test/", "\\test\\"):
		return CommentTest
	}

	switch {
	case containsAny(text, "requirement", "requirements", "acceptance criteria", "business rule", "scope", "需求", "验收", "业务规则"):
		return CommentRequirements
	case containsAny(text, "plan", "design", "architecture", "adapter boundary", "方案", "架构", "设计"):
		return CommentPlan
	case containsAny(text, "test", "tests", "coverage", "regression", "quality gate", "测试", "覆盖"):
		return CommentTest
	case containsAny(text, "nit", "style", "rename", "format", "readability", "typo", "命名", "格式"):
		return CommentStyle
	default:
		return CommentImplementation
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
```

- [ ] Run tests and verify GREEN.

```powershell
go test ./internal/adapters -run TestClassifyReviewComment -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/adapters
```

- [ ] Commit classifier.

```powershell
git add internal/adapters/review_classifier.go internal/adapters/review_classifier_test.go
git commit -m "Classify review comments for workflow routing" -m "MR review needs deterministic categories before demandflow can route blocking comments back to the right stage." -m "Constraint: Classification must be stable in CI and cannot depend on an LLM" -m "Rejected: Treat every unresolved comment as implementation work | requirements and plan comments need earlier workflow gates" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/adapters -run TestClassifyReviewComment -count=1"
```

## Task 2: Populate And Display Review Categories

**Files:**

- Modify: `internal/adapters/gitlab.go`
- Modify: `internal/adapters/gitlab_test.go`
- Modify: `internal/cli/review_gate.go`
- Modify: `internal/cli/review_gate_test.go`

- [ ] Add failing GitLab adapter test for category population.

In `internal/adapters/gitlab_test.go`, add:

```go
func TestGitLabListUnresolvedClassifiesComments(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]gitlabDiscussion{{
			ID: "discussion-1",
			Notes: []gitlabNote{{
				ID:         7,
				Body:       "Please add regression coverage.",
				Resolvable: true,
				Resolved:   false,
				Author:     gitlabNoteAuthor{Username: "reviewer"},
				Position:   &gitlabNotePosition{NewPath: "internal/service/coupon.go", NewLine: 12},
			}},
		}})
	}))
	defer server.Close()

	adapter := GitLabReviewAdapter{Client: server.Client()}
	comments, err := adapter.ListUnresolved(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "1",
		BaseURL:      server.URL,
		Token:        "glpat-test",
	})
	if err != nil {
		t.Fatalf("ListUnresolved: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(comments))
	}
	if comments[0].Category != CommentTest {
		t.Fatalf("category = %s, want %s", comments[0].Category, CommentTest)
	}
}
```

- [ ] Run adapter test and verify RED.

```powershell
go test ./internal/adapters -run TestGitLabListUnresolvedClassifiesComments -count=1
```

Expected:

```text
category = , want test
```

- [ ] Set category in `ListUnresolved`.

In `internal/adapters/gitlab.go`, inside the `ReviewComment` literal in `ListUnresolved`, keep existing fields and add:

```go
Category: ClassifyReviewComment(note.Body, commentFilePath(note.Position)),
```

Add helper near the GitLab note types:

```go
func commentFilePath(position *gitlabNotePosition) string {
	if position == nil {
		return ""
	}
	return position.NewPath
}
```

The final comment construction should preserve the existing file and line assignment:

```go
comment := ReviewComment{
	ID:       discussion.ID + ":" + strconv.FormatInt(note.ID, 10),
	Author:   note.Author.Username,
	Body:     note.Body,
	Blocking: true,
	Category: ClassifyReviewComment(note.Body, commentFilePath(note.Position)),
}
```

- [ ] Run adapter test and verify GREEN.

```powershell
go test ./internal/adapters -run TestGitLabListUnresolvedClassifiesComments -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/adapters
```

- [ ] Add failing review-gate output test.

In `internal/cli/review_gate_test.go`, update the existing blocked-output assertion to include category. If the current test uses `fakeReviewGateAdapter`, set a comment like:

```go
adapters.ReviewComment{
	ID: "1", Author: "reviewer", Body: "missing test coverage",
	FilePath: "internal/service.go", Line: 42, Blocking: true, Category: adapters.CommentTest,
}
```

Then assert:

```go
for _, want := range []string{"review gate blocked", "[test]", "internal/service.go:42", "missing test coverage"} {
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
	}
}
```

- [ ] Run CLI test and verify RED.

```powershell
go test ./internal/cli -run TestReviewGate -count=1
```

Expected:

```text
stdout missing "[test]"
```

- [ ] Print category in `review-gate`.

In `internal/cli/review_gate.go`, replace the comment print line with:

```go
category := comment.Category
if category == "" {
	category = adapters.ClassifyReviewComment(comment.Body, comment.FilePath)
}
fmt.Fprintf(stdout, "- [%s] %s by %s: %s\n", category, location, comment.Author, strings.TrimSpace(comment.Body))
```

- [ ] Run CLI test and verify GREEN.

```powershell
go test ./internal/cli -run TestReviewGate -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] Commit category propagation.

```powershell
git add internal/adapters/gitlab.go internal/adapters/gitlab_test.go internal/cli/review_gate.go internal/cli/review_gate_test.go
git commit -m "Show review categories from GitLab gates" -m "Direct and workflow review gates need category evidence so users can see why a comment routes to a stage." -m "Constraint: Existing GitLab comment listing behavior must stay compatible" -m "Rejected: Classifying only in demandflow | direct review-gate output would remain blind to category" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/adapters -run TestGitLabListUnresolvedClassifiesComments -count=1" -m "Tested: go test ./internal/cli -run TestReviewGate -count=1"
```

## Task 3: Build Review Action Plans

**Files:**

- Create: `internal/demandflow/review_action.go`
- Create: `internal/demandflow/review_action_test.go`

- [ ] Write failing review action tests.

Create `internal/demandflow/review_action_test.go`:

```go
package demandflow

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestBuildReviewActionPlanRoutesEarliestBlockingCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		comments []adapters.ReviewComment
		want     workflow.State
	}{
		{
			name: "requirements wins over implementation",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "business rule changed", Blocking: true, Category: adapters.CommentRequirements},
				{ID: "2", Body: "nil handling", Blocking: true, Category: adapters.CommentImplementation},
			},
			want: workflow.ReturnedToRequirements,
		},
		{
			name: "plan routes to returned_to_plan",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "adapter boundary is wrong", Blocking: true, Category: adapters.CommentPlan},
			},
			want: workflow.ReturnedToPlan,
		},
		{
			name: "test routes to implementation",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "missing regression coverage", Blocking: true, Category: adapters.CommentTest},
			},
			want: workflow.Implementation,
		},
		{
			name: "no blocking comments routes to verification",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "nit rename", Blocking: false, Category: adapters.CommentStyle},
			},
			want: workflow.Verification,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := BuildReviewActionPlan(tc.comments)
			if plan.NextState != tc.want {
				t.Fatalf("next state = %s, want %s", plan.NextState, tc.want)
			}
		})
	}
}

func TestRenderReviewActionPlanIncludesEvidence(t *testing.T) {
	t.Parallel()

	plan := BuildReviewActionPlan([]adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "missing acceptance criteria", Blocking: true, Category: adapters.CommentRequirements, FilePath: "requirements.md", Line: 3},
	})

	body := RenderReviewActionPlan(plan)
	for _, want := range []string{"## MR Review Action Plan", "returned_to_requirements", "[requirements]", "missing acceptance criteria"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}
```

- [ ] Run tests and verify RED.

```powershell
go test ./internal/demandflow -run TestBuildReviewActionPlan -count=1
```

Expected:

```text
undefined: BuildReviewActionPlan
```

- [ ] Implement review action plan.

Create `internal/demandflow/review_action.go`:

```go
package demandflow

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ReviewActionPlan struct {
	Comments  []adapters.ReviewComment
	Counts    map[adapters.CommentCategory]int
	NextState workflow.State
	Message   string
}

func BuildReviewActionPlan(comments []adapters.ReviewComment) ReviewActionPlan {
	plan := ReviewActionPlan{
		Comments:  comments,
		Counts:    map[adapters.CommentCategory]int{},
		NextState: workflow.Verification,
		Message:   "mr review cleared, no blocking unresolved comments",
	}

	blockingRequirements := false
	blockingPlan := false
	blockingImplementation := false
	for i := range plan.Comments {
		comment := &plan.Comments[i]
		if comment.Category == "" {
			comment.Category = adapters.ClassifyReviewComment(comment.Body, comment.FilePath)
		}
		plan.Counts[comment.Category]++
		if !comment.Blocking {
			continue
		}
		switch comment.Category {
		case adapters.CommentRequirements:
			blockingRequirements = true
		case adapters.CommentPlan:
			blockingPlan = true
		default:
			blockingImplementation = true
		}
	}

	switch {
	case blockingRequirements:
		plan.NextState = workflow.ReturnedToRequirements
		plan.Message = "blocking review comments require requirements updates"
	case blockingPlan:
		plan.NextState = workflow.ReturnedToPlan
		plan.Message = "blocking review comments require plan updates"
	case blockingImplementation:
		plan.NextState = workflow.Implementation
		plan.Message = "blocking review comments require implementation updates"
	}
	return plan
}

func RenderReviewActionPlan(plan ReviewActionPlan) string {
	var b strings.Builder
	b.WriteString("## MR Review Action Plan\n\n")
	fmt.Fprintf(&b, "Next state: `%s`\n\n", plan.NextState)
	fmt.Fprintf(&b, "Decision: %s\n\n", plan.Message)
	b.WriteString("Category counts:\n")
	for _, category := range []adapters.CommentCategory{
		adapters.CommentRequirements,
		adapters.CommentPlan,
		adapters.CommentImplementation,
		adapters.CommentTest,
		adapters.CommentStyle,
	} {
		if plan.Counts[category] == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s: %d\n", category, plan.Counts[category])
	}
	if len(plan.Comments) == 0 {
		b.WriteString("- none: 0\n")
	}
	b.WriteString("\nActions:\n")
	if len(plan.Comments) == 0 {
		b.WriteString("- No unresolved review comments. Continue to verification.\n\n")
		return b.String()
	}
	for _, comment := range plan.Comments {
		location := strings.TrimSpace(comment.FilePath)
		if comment.Line > 0 {
			location = fmt.Sprintf("%s:%d", comment.FilePath, comment.Line)
		}
		if location == "" {
			location = "(no file location)"
		}
		status := "nonblocking"
		if comment.Blocking {
			status = "blocking"
		}
		fmt.Fprintf(&b, "- [%s][%s] %s by %s: %s\n",
			comment.Category,
			status,
			location,
			comment.Author,
			strings.TrimSpace(comment.Body),
		)
	}
	b.WriteString("\n")
	return b.String()
}
```

- [ ] Run tests and verify GREEN.

```powershell
go test ./internal/demandflow -run "TestBuildReviewActionPlan|TestRenderReviewActionPlan" -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] Commit review action plan.

```powershell
git add internal/demandflow/review_action.go internal/demandflow/review_action_test.go
git commit -m "Plan MR review actions by comment category" -m "The MR review stage needs a deterministic bridge from unresolved comments to workflow actions before it can route demands safely." -m "Constraint: Mixed blocking comments must route to the earliest safe workflow state" -m "Rejected: Returning one action per comment without a selected next state | the workflow needs one deterministic next state" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -run \"TestBuildReviewActionPlan|TestRenderReviewActionPlan\" -count=1"
```

## Task 4: Route MR Review Workflow By Action Plan

**Files:**

- Modify: `internal/workflow/state.go`
- Modify: `internal/workflow/state_test.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/review.go`
- Modify: `internal/demandflow/review_test.go`

- [ ] Add failing workflow transition test.

In `internal/workflow/state_test.go`, add:

```go
func TestAdvanceAllowsMRReviewBackToImplementation(t *testing.T) {
	t.Parallel()

	next, err := Advance(MRReview, Implementation)
	if err != nil {
		t.Fatalf("advance %s -> %s returned error: %v", MRReview, Implementation, err)
	}
	if next != Implementation {
		t.Fatalf("state after advance = %s, want %s", next, Implementation)
	}
}
```

- [ ] Run workflow test and verify RED.

```powershell
go test ./internal/workflow -run TestAdvanceAllowsMRReviewBackToImplementation -count=1
```

Expected:

```text
invalid workflow transition from mr_review to implementation
```

- [ ] Allow MR-review implementation rework.

In `internal/workflow/state.go`, add `Implementation` to the `MRReview` transitions:

```go
MRReview: {
	Implementation:         {},
	Verification:           {},
	ReturnedToRequirements: {},
	ReturnedToPlan:         {},
	BlockedNeedUser:        {},
	Cancelled:              {},
},
```

- [ ] Run workflow test and verify GREEN.

```powershell
go test ./internal/workflow -run TestAdvanceAllowsMRReviewBackToImplementation -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/workflow
```

- [ ] Add failing demandflow route tests.

In `internal/demandflow/review_test.go`, add:

```go
func TestEngineMRReviewRequirementsCommentReturnsToRequirements(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "missing acceptance criteria", Blocking: true, Category: adapters.CommentRequirements},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "requirements updates") {
		t.Fatalf("err = %v want requirements updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.ReturnedToRequirements) {
		t.Fatalf("state = %q want returned_to_requirements", demand.State)
	}
	body := readArtifact(t, engine, artifacts.ProgressFile)
	if !strings.Contains(body, "MR Review Action Plan") || !strings.Contains(body, "returned_to_requirements") {
		t.Fatalf("progress.md missing action plan:\n%s", body)
	}
}

func TestEngineMRReviewPlanCommentReturnsToPlan(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "adapter boundary is wrong", Blocking: true, Category: adapters.CommentPlan},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "plan updates") {
		t.Fatalf("err = %v want plan updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.ReturnedToPlan) {
		t.Fatalf("state = %q want returned_to_plan", demand.State)
	}
}

func TestEngineMRReviewImplementationCommentReturnsToImplementation(t *testing.T) {
	t.Parallel()

	engine, root := newTestEngine(t, workflow.MRReview)
	adapter := fakeReviewAdapter{comments: []adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "nil handling is wrong", Blocking: true, Category: adapters.CommentImplementation},
	}}
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageMRReview,
		Runner:   &StaticRunner{},
		Review:   mrReviewOptions(adapter),
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "implementation updates") {
		t.Fatalf("err = %v want implementation updates", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Implementation) {
		t.Fatalf("state = %q want implementation", demand.State)
	}
}
```

- [ ] Run demandflow tests and verify RED.

```powershell
go test ./internal/demandflow -run "TestEngineMRReview.*Returns" -count=1
```

Expected:

```text
state = "mr_review" want returned_to_requirements
```

- [ ] Render category in existing review summary.

In `internal/demandflow/review.go`, update `renderReviewSummary` comment lines to include category:

```go
category := c.Category
if category == "" {
	category = adapters.ClassifyReviewComment(c.Body, c.FilePath)
}
fmt.Fprintf(&b, "- [%s][%s] %s", category, status, c.Author)
```

Keep the existing file/line and body rendering.

- [ ] Use action plan in `runMRReview`.

In `internal/demandflow/engine.go`, replace the current blocking-comment loop in `runMRReview` with:

```go
actionPlan := BuildReviewActionPlan(comments)
if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, RenderReviewActionPlan(actionPlan)); err != nil {
	return err
}

if actionPlan.NextState != workflow.Verification {
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "mr_review.action_required",
		Message: actionPlan.Message,
		Data: map[string]string{
			"next_state": string(actionPlan.NextState),
		},
	}); err != nil {
		return err
	}
	if err := e.advance(&demand, actionPlan.NextState); err != nil {
		return err
	}
	result.Message = actionPlan.Message
	return fmt.Errorf(actionPlan.Message)
}
```

Place this after writing `renderReviewSummary(comments)` and before optional MR sync.

- [ ] Run demandflow tests and verify GREEN.

```powershell
go test ./internal/demandflow -run "TestEngineMRReview" -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] Commit workflow routing.

```powershell
git add internal/workflow/state.go internal/workflow/state_test.go internal/demandflow/engine.go internal/demandflow/review.go internal/demandflow/review_test.go
git commit -m "Route MR review comments to demand stages" -m "Blocking MR comments now carry enough category information to send the demand back to requirements, plan, or implementation instead of leaving every issue in mr_review." -m "Constraint: Requirements comments must win over later-stage comments in mixed review feedback" -m "Rejected: Keeping all blocking comments in mr_review | users get no actionable next workflow stage" -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/workflow -run TestAdvanceAllowsMRReviewBackToImplementation -count=1" -m "Tested: go test ./internal/demandflow -run \"TestEngineMRReview\" -count=1"
```

## Task 5: Improve Next Actions For Returned And Rework States

**Files:**

- Modify: `internal/demandflow/status.go`
- Modify: `internal/demandflow/status_test.go`

- [ ] Add failing next-action tests.

In `internal/demandflow/status_test.go`, update the table in `TestNextActionsMapStatesToCommands` or add a new test:

```go
func TestNextActionsForMRReviewReturns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state workflow.State
		label string
		want  string
	}{
		{workflow.ReturnedToRequirements, "Revise requirements", "devflow run --demand add-coupon-check --stage requirements"},
		{workflow.ReturnedToPlan, "Revise plan", "devflow run --demand add-coupon-check --stage plan"},
		{workflow.Implementation, "Run implementation", "devflow run --demand add-coupon-check --stage implementation"},
	}

	for _, tc := range tests {
		t.Run(string(tc.state), func(t *testing.T) {
			actions := NextActions(tc.state, "add-coupon-check")
			if len(actions) == 0 {
				t.Fatal("expected action")
			}
			if actions[0].Label != tc.label {
				t.Fatalf("label = %q, want %q", actions[0].Label, tc.label)
			}
			if !strings.Contains(actions[0].Command, tc.want) {
				t.Fatalf("command = %q, want contains %q", actions[0].Command, tc.want)
			}
		})
	}
}
```

- [ ] Run status test and verify RED.

```powershell
go test ./internal/demandflow -run TestNextActionsForMRReviewReturns -count=1
```

Expected:

```text
label = "Inspect manually", want "Revise requirements"
```

- [ ] Add returned-state actions.

In `internal/demandflow/status.go`, add cases before `FailedQualityGate`:

```go
case workflow.ReturnedToRequirements:
	return []NextAction{{Label: "Revise requirements", Command: "devflow run --demand " + idArg + " --stage requirements", Reason: "MR review found requirements-level feedback; revise requirements before planning again."}}
case workflow.ReturnedToPlan:
	return []NextAction{{Label: "Revise plan", Command: "devflow run --demand " + idArg + " --stage plan", Reason: "MR review found plan-level feedback; revise the plan before implementation resumes."}}
```

Keep the existing `workflow.Implementation` action unchanged.

- [ ] Run status test and verify GREEN.

```powershell
go test ./internal/demandflow -run TestNextActionsForMRReviewReturns -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] Commit next actions.

```powershell
git add internal/demandflow/status.go internal/demandflow/status_test.go
git commit -m "Guide next actions after MR review routing" -m "Returned requirements and plan states need concrete user commands once MR review comments route the demand backward." -m "Constraint: Next actions must remain copy-pasteable CLI commands" -m "Rejected: Falling back to manual inspection | the workflow already knows the safe restart stage" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -run TestNextActionsForMRReviewReturns -count=1"
```

## Task 6: Update Dogfood And Documentation

**Files:**

- Modify: `internal/dogfood/runner_test.go`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/user-guide/dogfood-smoke.md`
- Modify: `docs/release/v0.1.md`

- [ ] Add dogfood evidence assertion.

In `internal/dogfood/runner_test.go`, extend the existing progress assertions with:

```go
if !strings.Contains(string(progressData), "MR Review Action Plan") {
	t.Fatalf("progress.md missing MR review action plan:\n%s", string(progressData))
}
if !strings.Contains(string(progressData), "Next state: `verification`") {
	t.Fatalf("progress.md missing verification routing:\n%s", string(progressData))
}
```

- [ ] Run dogfood test and verify RED or GREEN.

```powershell
go test ./internal/dogfood -run TestDogfood -count=1
```

Expected before Task 4 is merged:

```text
progress.md missing MR review action plan
```

Expected after Task 4 is merged:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/dogfood
```

- [ ] Update backend demand guide.

In `docs/user-guide/backend-demand-loop.md`, replace the MR review section with:

~~~~markdown
## 7. Run MR Review Collaboration

```powershell
$env:GITLAB_TOKEN = '<your-token>'
devflow run --demand add-coupon-eligibility-check --stage mr-review --gitlab-project "group/project" --gitlab-mr "123"
```

The MR-review stage records two sections in `progress.md`:

- `MR 评审摘要` lists unresolved comments.
- `MR Review Action Plan` classifies comments and selects the next workflow state.

Blocking comments route the demand as follows:

- requirements feedback -> `returned_to_requirements`
- plan or architecture feedback -> `returned_to_plan`
- implementation, test, or style feedback -> `implementation`

When no blocking comments remain, the demand advances to `verification`.

Before running the workflow MR stage, you can inspect a real MR directly:

```powershell
$env:GITLAB_TOKEN = '<your-token>'
devflow review-gate --gitlab-project "group/project" --gitlab-mr "123"
```
~~~~

- [ ] Update dogfood smoke guide.

In `docs/user-guide/dogfood-smoke.md`, add:

```markdown
The deterministic dogfood path also records an `MR Review Action Plan` section. In the default offline path, the selected next state is `verification` because the offline review adapter returns no blocking comments.
```

- [ ] Update release known limits.

In `docs/release/v0.1.md`, keep these limits explicit:

```markdown
- GitLab comment replies, resolving discussions, reviewer assignment, and approvals are still external.
- Live provider smoke tests are opt-in and require local API credentials.
- The first product target is backend demand flow only; frontend, test, and PD Agents remain future extensions.
```

- [ ] Run docs and dogfood verification.

```powershell
go test ./internal/dogfood -count=1
git diff --check
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/dogfood
```

No `git diff --check` output.

- [ ] Commit docs and dogfood.

```powershell
git add internal/dogfood/runner_test.go docs/user-guide/backend-demand-loop.md docs/user-guide/dogfood-smoke.md docs/release/v0.1.md
git commit -m "Document MR review collaboration routing" -m "Wave 11 changes the MR review stage from a blocker check into a categorized collaboration loop, so dogfood and user docs need to show the evidence and routing behavior." -m "Constraint: Auto replies and approvals remain outside v0.1" -m "Rejected: Updating code without user docs | the new state routing changes how operators continue after review feedback" -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/dogfood -count=1" -m "Tested: git diff --check"
```

## Task 7: Final Verification And PR

**Files:**

- Verify: whole repository

- [ ] Run targeted verification.

```powershell
go test ./internal/adapters ./internal/cli ./internal/demandflow ./internal/dogfood ./internal/workflow -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/adapters
ok  	github.com/jesseedcp/devflow-agent/internal/cli
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
ok  	github.com/jesseedcp/devflow-agent/internal/dogfood
ok  	github.com/jesseedcp/devflow-agent/internal/workflow
```

- [ ] Run full verification.

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-11
```

No uncommitted files after the final commit.

- [ ] Run release readiness.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave11
```

Expected:

```text
dogfood completed for dogfood-coupon-eligibility
state: completed
release readiness report:
```

- [ ] Push branch.

```powershell
git push -u origin feature/devflow-wave-11
```

- [ ] Create PR.

```powershell
gh pr create --base main --head feature/devflow-wave-11 --title "Wave 11: GitLab review collaboration loop" --body @'
## Summary
- Classifies unresolved MR comments into requirements, plan, implementation, test, and style categories.
- Records an MR Review Action Plan in demand progress evidence.
- Routes blocking review feedback back to the earliest safe workflow stage.
- Updates direct review-gate output, dogfood evidence, and user docs.

## Test Plan
- [ ] go test ./internal/adapters ./internal/cli ./internal/demandflow ./internal/dogfood ./internal/workflow -count=1
- [ ] go test ./... -count=1 -timeout 5m
- [ ] go vet ./...
- [ ] go build ./cmd/devflow
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave11
- [ ] git diff --check

## Live GitLab
- Not required. Real GitLab review checks remain opt-in through `GITLAB_TOKEN`.
'@
```

Do not merge until deterministic verification and CI pass.

## Definition Of Done

Wave 11 is complete when:

- `adapters.ClassifyReviewComment` exists and is tested.
- GitLab unresolved comments include `ReviewComment.Category`.
- `devflow review-gate` prints categories.
- `BuildReviewActionPlan` selects `returned_to_requirements`, `returned_to_plan`, `implementation`, or `verification`.
- `mr-review` writes `MR Review Action Plan` to `progress.md`.
- Blocking requirements comments route to `returned_to_requirements`.
- Blocking plan comments route to `returned_to_plan`.
- Blocking implementation, test, or style comments route to `implementation`.
- No blocking comments still advance to `verification`.
- `devflow next` gives concrete commands for returned states.
- Dogfood progress includes the action plan.
- Full verification and release readiness pass.
- PR from `feature/devflow-wave-11` to `main` is open and green.

## Self-Review Notes

Spec coverage:

- Comment classification is covered by Task 1.
- GitLab category propagation and direct gate visibility are covered by Task 2.
- Action-plan generation is covered by Task 3.
- Workflow routing is covered by Task 4.
- Operator next actions are covered by Task 5.
- Dogfood and docs are covered by Task 6.
- Full verification and PR are covered by Task 7.

Placeholder scan:

- No step uses unfinished-marker wording.
- All code tasks include concrete snippets.
- All verification steps include expected outcomes.
- Secret values are represented as environment variables and are not written to files.

Type consistency:

- `CommentCategory` values come from `internal/adapters/review.go`.
- `BuildReviewActionPlan` returns `workflow.State` values already known by `internal/workflow/state.go`.
- The only new transition is `MRReview -> Implementation`.
- `ReviewActionPlan` stays inside `internal/demandflow`; adapters do not import demandflow.

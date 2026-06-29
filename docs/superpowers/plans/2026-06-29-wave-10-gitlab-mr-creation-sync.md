# Wave 10 GitLab MR Creation Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the GitLab MR creation gap by letting Devflow create or reuse a GitLab merge request after implementation, record the MR evidence in demand artifacts, and expose the same operation as a direct CLI command.

**Architecture:** Extend the existing `internal/adapters` GitLab boundary with a merge-request sync interface beside the review-comment interface. Wire that interface into `demandflow` implementation-stage completion as an optional operation, so deterministic and live dogfood can exercise MR sync without forcing network calls by default. Keep real GitLab calls explicit through CLI flags and environment-backed tokens, and preserve the existing `mr-review` unresolved-comment gate.

**Tech Stack:** Go 1.25.0, standard-library `net/http`, existing `internal/adapters.GitLabReviewAdapter`, existing `internal/demandflow`, existing CLI patterns, existing deterministic/live dogfood packages, PowerShell verification scripts, no new Go dependencies.

---

## Current Environment

Repository:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
```

Expected base branch:

```text
main
```

Wave 10 branch:

```text
feature/devflow-wave-10
```

Recommended worktree:

```powershell
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-wave-10
```

Current local base observed before writing this plan:

```text
9ddf412 Merge Wave 9 live sandbox and GitLab release gates into main
```

Important coordination note:

```text
At plan-writing time, local main contained the Wave 9 merge, while origin/main still needed to receive it and PR #5 was still open. Before executing Wave 10, make sure the execution base includes Wave 9 files such as internal/cli/live_dogfood.go, internal/cli/review_gate.go, internal/dogfood/live.go, and scripts/release-readiness.ps1.
```

## Product Thesis

The original product loop says:

```text
requirements -> plan -> implementation -> create/update MR -> review gate -> verification -> closeout
```

Waves 5-9 already cover implementation, review-gate checking, live provider sandboxing, and release-readiness verification. The remaining visible gap is in `docs/release/v0.1.md`:

```text
GitLab MR creation is not automated.
```

Wave 10 should make this true:

```text
devflow run --stage implementation --create-mr ...
  -> agent implements code
  -> quality gate passes
  -> Devflow finds an existing open MR for source_branch + target_branch
  -> or creates one if none exists
  -> progress.md records MR iid, state, URL, and whether it was created or reused
  -> workflow advances to mr_review
  -> devflow run --stage mr-review checks unresolved comments as before
```

It should also expose a direct command:

```text
devflow mr ensure --gitlab-project group/project --source-branch feature/x --target-branch main --title "..."
```

## Scope

In scope:

- Add a merge-request sync interface in `internal/adapters`.
- Implement GitLab find-or-create MR behavior using the GitLab REST API.
- Add `devflow mr ensure`.
- Add optional MR sync to the implementation stage.
- Add `--create-mr` flags to `devflow run --stage implementation`.
- Update deterministic dogfood to exercise MR sync through a fake adapter.
- Update docs and release notes to remove the known limit.

Out of scope:

- Pushing Git branches to GitLab.
- Creating commits.
- Resolving GitLab comments.
- Assigning reviewers, labels, milestones, approvals, or pipelines.
- Supporting GitHub, Codeup, Gitee, or other review systems.
- Making MR creation mandatory for deterministic dogfood or CI.

## File Map

Create:

```text
internal/adapters/merge_request.go
internal/adapters/merge_request_test.go
internal/cli/mr.go
internal/cli/mr_test.go
```

Modify:

```text
README.md
docs/release/v0.1.md
docs/user-guide/backend-demand-loop.md
docs/user-guide/live-dogfood.md
internal/adapters/gitlab.go
internal/adapters/gitlab_test.go
internal/cli/cli.go
internal/cli/run.go
internal/cli/run_test.go
internal/demandflow/engine.go
internal/demandflow/engine_test.go
internal/demandflow/types.go
internal/dogfood/runner.go
internal/dogfood/runner_test.go
internal/dogfood/live.go
```

No `go.mod` or `go.sum` changes should be needed.

---

## Task 0: Preflight And Worktree

**Files:** none

- [ ] Confirm Wave 9 is present before branching.

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git status --short --branch
git log --oneline --decorate --max-count=5
Test-Path internal\cli\live_dogfood.go
Test-Path internal\cli\review_gate.go
Test-Path scripts\release-readiness.ps1
```

Expected:

```text
## main...origin/main
True
True
True
```

If `main` is ahead of `origin/main` with the Wave 9 merge commit and PR #5 is already approved for merge, push `main` before starting Wave 10:

```powershell
git push origin main
```

If PR #5 is still open and should stay open for human review, do not push `main`; instead create Wave 10 from the local `main` that contains Wave 9 so the plan can compile locally.

- [ ] Create the Wave 10 worktree.

```powershell
git worktree add .worktrees\devflow-wave-10 -b feature/devflow-wave-10 main
cd .worktrees\devflow-wave-10
```

Expected:

```text
Preparing worktree (new branch 'feature/devflow-wave-10')
```

- [ ] Verify clean baseline.

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
## feature/devflow-wave-10
```

No commit for Task 0.

---

## Task 1: Define Merge Request Sync Interface And GitLab Implementation

**Files:**

- Create: `internal/adapters/merge_request.go`
- Create: `internal/adapters/merge_request_test.go`
- Modify: `internal/adapters/gitlab.go`
- Modify: `internal/adapters/gitlab_test.go`

### Why

`ReviewAdapter` handles unresolved comments. MR creation/reuse is a different adapter responsibility. Keep the interfaces separate so review-gate behavior remains stable.

- [ ] Create `internal/adapters/merge_request.go`.

```go
package adapters

import "context"

type MergeRequestSpec struct {
	Project            string
	SourceBranch       string
	TargetBranch       string
	Title              string
	Description        string
	BaseURL            string
	Token              string
	RemoveSourceBranch bool
}

type MergeRequestResult struct {
	Project      string
	MergeRequest string
	WebURL       string
	State        string
	Created      bool
}

type MergeRequestAdapter interface {
	EnsureMergeRequest(ctx context.Context, spec MergeRequestSpec) (MergeRequestResult, error)
}
```

- [ ] Add validation helper in `internal/adapters/gitlab.go`.

Add near the token/baseURL helpers:

```go
func validateMergeRequestSpec(spec MergeRequestSpec) error {
	var missing []string
	if strings.TrimSpace(spec.Project) == "" {
		missing = append(missing, "project")
	}
	if strings.TrimSpace(spec.SourceBranch) == "" {
		missing = append(missing, "source_branch")
	}
	if strings.TrimSpace(spec.TargetBranch) == "" {
		missing = append(missing, "target_branch")
	}
	if strings.TrimSpace(spec.Title) == "" {
		missing = append(missing, "title")
	}
	if len(missing) > 0 {
		return fmt.Errorf("merge request spec missing: %s", strings.Join(missing, ", "))
	}
	return nil
}
```

- [ ] Add GitLab API response type in `internal/adapters/gitlab.go`.

Add near existing `gitlabDiscussion` types:

```go
type gitlabMergeRequest struct {
	IID    int64  `json:"iid"`
	WebURL string `json:"web_url"`
	State  string `json:"state"`
}
```

- [ ] Implement `EnsureMergeRequest` in `internal/adapters/gitlab.go`.

```go
func (a GitLabReviewAdapter) EnsureMergeRequest(ctx context.Context, spec MergeRequestSpec) (MergeRequestResult, error) {
	if err := validateMergeRequestSpec(spec); err != nil {
		return MergeRequestResult{}, err
	}
	token, err := a.token(ReviewRef{Token: spec.Token})
	if err != nil {
		return MergeRequestResult{}, err
	}

	baseURL := a.baseURL(ReviewRef{BaseURL: spec.BaseURL})
	project := strings.TrimSpace(spec.Project)
	sourceBranch := strings.TrimSpace(spec.SourceBranch)
	targetBranch := strings.TrimSpace(spec.TargetBranch)

	existing, err := a.findOpenMergeRequest(ctx, baseURL, token, project, sourceBranch, targetBranch)
	if err != nil {
		return MergeRequestResult{}, err
	}
	if existing != nil {
		return MergeRequestResult{
			Project:      project,
			MergeRequest: strconv.FormatInt(existing.IID, 10),
			WebURL:       existing.WebURL,
			State:        existing.State,
			Created:      false,
		}, nil
	}

	created, err := a.createMergeRequest(ctx, baseURL, token, spec)
	if err != nil {
		return MergeRequestResult{}, err
	}
	return MergeRequestResult{
		Project:      project,
		MergeRequest: strconv.FormatInt(created.IID, 10),
		WebURL:       created.WebURL,
		State:        created.State,
		Created:      true,
	}, nil
}
```

- [ ] Add helper methods in `internal/adapters/gitlab.go`.

```go
func (a GitLabReviewAdapter) findOpenMergeRequest(ctx context.Context, baseURL, token, project, sourceBranch, targetBranch string) (*gitlabMergeRequest, error) {
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", baseURL, url.PathEscape(project))
	values := url.Values{}
	values.Set("state", "opened")
	values.Set("source_branch", sourceBranch)
	values.Set("target_branch", targetBranch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build gitlab find merge request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab find merge request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab find merge request status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var mergeRequests []gitlabMergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&mergeRequests); err != nil {
		return nil, fmt.Errorf("decode gitlab merge request search: %w", err)
	}
	if len(mergeRequests) == 0 {
		return nil, nil
	}
	return &mergeRequests[0], nil
}

func (a GitLabReviewAdapter) createMergeRequest(ctx context.Context, baseURL, token string, spec MergeRequestSpec) (gitlabMergeRequest, error) {
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", baseURL, url.PathEscape(strings.TrimSpace(spec.Project)))
	form := url.Values{}
	form.Set("source_branch", strings.TrimSpace(spec.SourceBranch))
	form.Set("target_branch", strings.TrimSpace(spec.TargetBranch))
	form.Set("title", strings.TrimSpace(spec.Title))
	if strings.TrimSpace(spec.Description) != "" {
		form.Set("description", spec.Description)
	}
	if spec.RemoveSourceBranch {
		form.Set("remove_source_branch", "true")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return gitlabMergeRequest{}, fmt.Errorf("build gitlab create merge request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return gitlabMergeRequest{}, fmt.Errorf("gitlab create merge request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return gitlabMergeRequest{}, fmt.Errorf("gitlab create merge request status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var created gitlabMergeRequest
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return gitlabMergeRequest{}, fmt.Errorf("decode gitlab created merge request: %w", err)
	}
	return created, nil
}
```

- [ ] Add unit tests in `internal/adapters/merge_request_test.go`.

```go
package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabEnsureMergeRequestReusesExistingOpenMR(t *testing.T) {
	var postCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != "secret-token" {
			t.Fatalf("PRIVATE-TOKEN = %q", got)
		}
		if r.Method == http.MethodPost {
			postCalled = true
			t.Fatalf("unexpected POST")
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "source_branch=feature%2Fcoupon") || !strings.Contains(r.URL.RawQuery, "target_branch=main") {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"iid":123,"web_url":"https://gitlab.example/group/project/-/merge_requests/123","state":"opened"}]`))
	}))
	defer server.Close()

	result, err := (GitLabReviewAdapter{Client: server.Client()}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Project:      "group/project",
		SourceBranch: "feature/coupon",
		TargetBranch: "main",
		Title:        "Add coupon check",
		BaseURL:      server.URL,
		Token:        "secret-token",
	})
	if err != nil {
		t.Fatalf("ensure merge request: %v", err)
	}
	if postCalled {
		t.Fatal("POST should not be called for existing MR")
	}
	if result.Created {
		t.Fatal("result.Created = true, want false")
	}
	if result.MergeRequest != "123" || result.WebURL == "" || result.State != "opened" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGitLabEnsureMergeRequestCreatesWhenMissing(t *testing.T) {
	var sawPost bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("PRIVATE-TOKEN"); got != "secret-token" {
			t.Fatalf("PRIVATE-TOKEN = %q", got)
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case http.MethodPost:
			sawPost = true
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			want := map[string]string{
				"source_branch":        "feature/coupon",
				"target_branch":        "main",
				"title":                "Add coupon check",
				"description":          "Generated by Devflow",
				"remove_source_branch": "true",
			}
			for key, expected := range want {
				if got := r.Form.Get(key); got != expected {
					t.Fatalf("form[%s] = %q, want %q", key, got, expected)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"iid":124,"web_url":"https://gitlab.example/group/project/-/merge_requests/124","state":"opened"}`))
		default:
			t.Fatalf("method = %s", r.Method)
		}
	}))
	defer server.Close()

	result, err := (GitLabReviewAdapter{Client: server.Client()}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Project:            "group/project",
		SourceBranch:       "feature/coupon",
		TargetBranch:       "main",
		Title:              "Add coupon check",
		Description:        "Generated by Devflow",
		BaseURL:            server.URL,
		Token:              "secret-token",
		RemoveSourceBranch: true,
	})
	if err != nil {
		t.Fatalf("ensure merge request: %v", err)
	}
	if !sawPost {
		t.Fatal("expected POST to create MR")
	}
	if !result.Created || result.MergeRequest != "124" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGitLabEnsureMergeRequestValidatesSpec(t *testing.T) {
	_, err := (GitLabReviewAdapter{}).EnsureMergeRequest(context.Background(), MergeRequestSpec{Project: "group/project"})
	if err == nil || !strings.Contains(err.Error(), "source_branch") || !strings.Contains(err.Error(), "target_branch") || !strings.Contains(err.Error(), "title") {
		t.Fatalf("err = %v, want missing-field error", err)
	}
}
```

- [ ] Verify adapters.

```powershell
gofmt -w internal/adapters/merge_request.go internal/adapters/merge_request_test.go internal/adapters/gitlab.go internal/adapters/gitlab_test.go
go test ./internal/adapters -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/adapters
git commit -m @'
Teach GitLab adapter to create or reuse merge requests

MR creation is separate from review-comment inspection, so this adds a
dedicated merge-request sync contract while reusing the existing GitLab
transport, token, and base URL behavior.

Constraint: Tokens must stay in headers or environment variables and never appear in output
Rejected: Folding MR creation into ReviewAdapter | comment review and MR lifecycle are different adapter responsibilities
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/adapters -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 2: Add Direct `devflow mr ensure` CLI

**Files:**

- Create: `internal/cli/mr.go`
- Create: `internal/cli/mr_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/run.go`

### Why

Operators need a direct command to create or reuse an MR outside the demand workflow. This also gives tests and scripts a small surface before integrating MR sync into implementation stage.

- [ ] Add adapter factory in `internal/cli/run.go`.

Place below `newReviewAdapter`:

```go
var newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
	return adapters.GitLabReviewAdapter{}
}
```

- [ ] Update help and dispatch in `internal/cli/cli.go`.

Add usage:

```text
  devflow mr ensure --gitlab-project <project> --source-branch <branch> --target-branch <branch> --title <text>
```

Add command description:

```text
  mr        Create or reuse GitLab merge requests
```

Add switch case:

```go
case "mr":
	return runMR(args[1:], stdout, stderr)
```

- [ ] Create `internal/cli/mr.go`.

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

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func runMR(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("mr subcommand is required: ensure")
	}
	switch args[0] {
	case "ensure":
		return runMREnsure(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unsupported mr subcommand %q", args[0])
	}
}

func runMREnsure(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("mr ensure", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var project, sourceBranch, targetBranch, title, description, descriptionFile, baseURL string
	var removeSourceBranch bool
	fs.StringVar(&project, "gitlab-project", "", "GitLab project path or id")
	fs.StringVar(&sourceBranch, "source-branch", "", "source branch")
	fs.StringVar(&targetBranch, "target-branch", "main", "target branch")
	fs.StringVar(&title, "title", "", "merge request title")
	fs.StringVar(&description, "description", "", "merge request description")
	fs.StringVar(&descriptionFile, "description-file", "", "file containing merge request description")
	fs.StringVar(&baseURL, "gitlab-base-url", "", "GitLab base url override")
	fs.BoolVar(&removeSourceBranch, "remove-source-branch", false, "ask GitLab to remove the source branch after merge")
	if err := fs.Parse(args); err != nil {
		return err
	}

	body, err := mergeRequestDescription(description, descriptionFile)
	if err != nil {
		return err
	}
	spec := adapters.MergeRequestSpec{
		Project:            strings.TrimSpace(project),
		SourceBranch:       strings.TrimSpace(sourceBranch),
		TargetBranch:       strings.TrimSpace(targetBranch),
		Title:              strings.TrimSpace(title),
		Description:        body,
		BaseURL:            strings.TrimSpace(baseURL),
		RemoveSourceBranch: removeSourceBranch,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := newMergeRequestAdapter().EnsureMergeRequest(ctx, spec)
	if err != nil {
		return err
	}
	status := "reused"
	if result.Created {
		status = "created"
	}
	fmt.Fprintf(stdout, "merge request %s for %s!%s\n", status, result.Project, result.MergeRequest)
	if strings.TrimSpace(result.WebURL) != "" {
		fmt.Fprintf(stdout, "url: %s\n", result.WebURL)
	}
	if strings.TrimSpace(result.State) != "" {
		fmt.Fprintf(stdout, "state: %s\n", result.State)
	}
	return nil
}

func mergeRequestDescription(description, descriptionFile string) (string, error) {
	if strings.TrimSpace(descriptionFile) == "" {
		return strings.TrimSpace(description), nil
	}
	if strings.TrimSpace(description) != "" {
		return "", fmt.Errorf("--description and --description-file cannot both be set")
	}
	body, err := os.ReadFile(descriptionFile)
	if err != nil {
		return "", fmt.Errorf("read description file: %w", err)
	}
	return strings.TrimSpace(string(body)), nil
}
```

- [ ] Create `internal/cli/mr_test.go`.

```go
package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

type fakeMergeRequestAdapter struct {
	spec   adapters.MergeRequestSpec
	result adapters.MergeRequestResult
}

func (f *fakeMergeRequestAdapter) EnsureMergeRequest(_ context.Context, spec adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	f.spec = spec
	if f.result.Project == "" {
		f.result = adapters.MergeRequestResult{Project: spec.Project, MergeRequest: "42", WebURL: "https://gitlab.example/mr/42", State: "opened", Created: true}
	}
	return f.result, nil
}

func TestMREnsureCreatesMergeRequest(t *testing.T) {
	adapter := &fakeMergeRequestAdapter{}
	original := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = original }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter { return adapter }

	var stdout bytes.Buffer
	err := Run([]string{
		"mr", "ensure",
		"--gitlab-project", "group/project",
		"--source-branch", "feature/coupon",
		"--target-branch", "main",
		"--title", "Add coupon check",
		"--description", "Generated by Devflow",
		"--gitlab-base-url", "https://gitlab.example",
		"--remove-source-branch",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("mr ensure: %v", err)
	}
	if adapter.spec.Project != "group/project" || adapter.spec.SourceBranch != "feature/coupon" || adapter.spec.TargetBranch != "main" {
		t.Fatalf("spec = %#v", adapter.spec)
	}
	if adapter.spec.Description != "Generated by Devflow" || !adapter.spec.RemoveSourceBranch {
		t.Fatalf("spec = %#v", adapter.spec)
	}
	for _, want := range []string{"merge request created", "group/project!42", "url:"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestMREnsureReadsDescriptionFile(t *testing.T) {
	dir := t.TempDir()
	descriptionPath := filepath.Join(dir, "mr.md")
	if err := os.WriteFile(descriptionPath, []byte("file description\n"), 0o600); err != nil {
		t.Fatalf("write description: %v", err)
	}

	adapter := &fakeMergeRequestAdapter{}
	original := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = original }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter { return adapter }

	err := Run([]string{
		"mr", "ensure",
		"--gitlab-project", "group/project",
		"--source-branch", "feature/coupon",
		"--target-branch", "main",
		"--title", "Add coupon check",
		"--description-file", descriptionPath,
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("mr ensure: %v", err)
	}
	if adapter.spec.Description != "file description" {
		t.Fatalf("description = %q", adapter.spec.Description)
	}
}

func TestMREnsureRejectsConflictingDescriptionSources(t *testing.T) {
	err := Run([]string{
		"mr", "ensure",
		"--gitlab-project", "group/project",
		"--source-branch", "feature/coupon",
		"--target-branch", "main",
		"--title", "Add coupon check",
		"--description", "inline",
		"--description-file", "file.md",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "cannot both be set") {
		t.Fatalf("err = %v, want description conflict", err)
	}
}

func TestMRHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"devflow mr ensure", "mr        Create or reuse GitLab merge requests"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] Format and verify.

```powershell
gofmt -w internal/cli/cli.go internal/cli/run.go internal/cli/mr.go internal/cli/mr_test.go
go test ./internal/cli -run 'TestMREnsure|TestMRHelp' -count=1 -v
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] Commit.

```powershell
git add internal/cli
git commit -m @'
Expose merge request sync as a direct CLI command

Operators can now create or reuse a GitLab MR without manually calling the
GitLab API, which gives the product loop a small and testable MR lifecycle
surface.

Constraint: The command must not read or print GitLab tokens
Rejected: Hiding MR sync only behind implementation stage | direct operation is needed for diagnostics and release checks
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/cli -run 'TestMREnsure|TestMRHelp' -count=1 -v; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

---

## Task 3: Integrate MR Sync Into Implementation Stage

**Files:**

- Modify: `internal/demandflow/types.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/engine_test.go`

### Why

MR sync belongs after implementation and quality gate success. If MR creation fails, the workflow should not advance to `mr_review`; it should block on platform setup so the operator can fix GitLab credentials, branch state, or project settings.

- [ ] Add merge request options in `internal/demandflow/types.go`.

Add import:

```go
"github.com/jesseedcp/devflow-agent/internal/adapters"
```

Add type near `Options`:

```go
type MergeRequestOptions struct {
	Adapter adapters.MergeRequestAdapter
	Spec    adapters.MergeRequestSpec
}
```

Add field to `Options`:

```go
MergeRequest MergeRequestOptions
```

The final shape should include:

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
	MergeRequest    MergeRequestOptions
	Now             func() time.Time
}
```

- [ ] Add MR evidence renderer in `internal/demandflow/engine.go`.

```go
func renderMergeRequestEvidence(result adapters.MergeRequestResult) string {
	status := "reused"
	if result.Created {
		status = "created"
	}
	var b strings.Builder
	b.WriteString("## Merge Request\n\n")
	fmt.Fprintf(&b, "- Status: %s\n", status)
	fmt.Fprintf(&b, "- Project: %s\n", result.Project)
	fmt.Fprintf(&b, "- IID: %s\n", result.MergeRequest)
	if strings.TrimSpace(result.WebURL) != "" {
		fmt.Fprintf(&b, "- URL: %s\n", result.WebURL)
	}
	if strings.TrimSpace(result.State) != "" {
		fmt.Fprintf(&b, "- State: %s\n", result.State)
	}
	b.WriteString("\n")
	return b.String()
}
```

Add `internal/adapters` to imports.

- [ ] Add helper in `internal/demandflow/engine.go`.

```go
func (e Engine) syncMergeRequest(ctx context.Context, opts Options, demand *artifacts.Demand, result *RunResult) error {
	if opts.MergeRequest.Adapter == nil {
		return nil
	}
	mr, err := opts.MergeRequest.Adapter.EnsureMergeRequest(ctx, opts.MergeRequest.Spec)
	if err != nil {
		if advanceErr := e.advance(demand, workflow.BlockedNeedPlatform); advanceErr != nil {
			return advanceErr
		}
		result.Message = "merge request sync failed: " + err.Error()
		return fmt.Errorf("merge request sync failed: %w", err)
	}
	if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, renderMergeRequestEvidence(mr)); err != nil {
		return err
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    "merge_request.synced",
		Message: "merge request synced",
		Data: map[string]string{
			"project":       mr.Project,
			"merge_request": mr.MergeRequest,
			"url":           mr.WebURL,
			"state":         mr.State,
			"created":       fmt.Sprintf("%t", mr.Created),
		},
	}); err != nil {
		return err
	}
	return nil
}
```

- [ ] Call MR sync in `runImplementation`.

Place after the quality gate block and before `implementation.completed` event:

```go
if err := e.syncMergeRequest(ctx, opts, &demand, result); err != nil {
	return err
}
```

- [ ] Add fake MR adapter and tests in `internal/demandflow/engine_test.go`.

```go
type fakeMergeRequestSyncAdapter struct {
	result adapters.MergeRequestResult
	err    error
	spec   adapters.MergeRequestSpec
}

func (f *fakeMergeRequestSyncAdapter) EnsureMergeRequest(_ context.Context, spec adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	f.spec = spec
	if f.err != nil {
		return adapters.MergeRequestResult{}, f.err
	}
	if f.result.Project == "" {
		f.result = adapters.MergeRequestResult{Project: spec.Project, MergeRequest: "7", WebURL: "https://gitlab.example/mr/7", State: "opened", Created: true}
	}
	return f.result, nil
}
```

Add test:

```go
func TestEngineImplementationSyncsMergeRequestAfterQualityPass(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	mr := &fakeMergeRequestSyncAdapter{}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## Implementation\n\nimplemented\n"},
	}}
	err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		MergeRequest: MergeRequestOptions{
			Adapter: mr,
			Spec: adapters.MergeRequestSpec{
				Project:      "group/project",
				SourceBranch: "feature/coupon",
				TargetBranch: "main",
				Title:        "Add coupon check",
			},
		},
		Now: fixedNow,
	})
	if err != nil {
		t.Fatalf("implementation: %v", err)
	}
	if mr.spec.SourceBranch != "feature/coupon" {
		t.Fatalf("mr spec = %#v", mr.spec)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.MRReview) {
		t.Fatalf("state = %q want mr_review", demand.State)
	}
	progress := readArtifact(t, engine, artifacts.ProgressFile)
	for _, want := range []string{"## Merge Request", "Status: created", "group/project", "https://gitlab.example/mr/7"} {
		if !strings.Contains(progress, want) {
			t.Fatalf("progress missing %q:\n%s", want, progress)
		}
	}
}
```

Add failure test:

```go
func TestEngineImplementationBlocksWhenMergeRequestSyncFails(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## Implementation\n\nimplemented\n"},
	}}
	err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		MergeRequest: MergeRequestOptions{
			Adapter: &fakeMergeRequestSyncAdapter{err: fmt.Errorf("gitlab unavailable")},
			Spec:    adapters.MergeRequestSpec{Project: "group/project", SourceBranch: "feature/coupon", TargetBranch: "main", Title: "Add coupon check"},
		},
		Now: fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "merge request sync failed") {
		t.Fatalf("err = %v, want merge request sync failed", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.BlockedNeedPlatform) {
		t.Fatalf("state = %q want blocked_need_platform", demand.State)
	}
}
```

Ensure imports include:

```go
"fmt"
"github.com/jesseedcp/devflow-agent/internal/adapters"
```

- [ ] Format and verify.

```powershell
gofmt -w internal/demandflow/types.go internal/demandflow/engine.go internal/demandflow/engine_test.go
go test ./internal/demandflow -run 'TestEngineImplementationSyncsMergeRequestAfterQualityPass|TestEngineImplementationBlocksWhenMergeRequestSyncFails' -count=1 -v
go test ./internal/demandflow -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/demandflow
git commit -m @'
Sync merge requests after implementation passes quality

Implementation now has an optional MR sync step that records MR evidence
before entering mr_review, and blocks on platform setup when GitLab cannot
create or reuse the MR.

Constraint: MR sync must run only after quality gates pass
Rejected: Advancing to mr_review when MR creation fails | the review gate cannot run without an MR reference
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/demandflow -run 'TestEngineImplementationSyncsMergeRequestAfterQualityPass|TestEngineImplementationBlocksWhenMergeRequestSyncFails' -count=1 -v; go test ./internal/demandflow -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 4: Wire MR Sync Flags Into `devflow run --stage implementation`

**Files:**

- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

### Why

The product workflow should be able to perform implementation plus MR sync in one command. Keep MR sync opt-in through `--create-mr`.

- [ ] Add flags in `internal/cli/run.go`.

Extend variable declaration:

```go
var root, runnerRoot, qualityRoot, demandID, stage, configPath, permissionMode, gitlabProject, gitlabMR, gitlabBaseURL string
var mrSourceBranch, mrTargetBranch, mrTitle, mrDescription, mrDescriptionFile string
var createMR, mrRemoveSourceBranch bool
```

Add flags:

```go
fs.BoolVar(&createMR, "create-mr", false, "create or reuse a GitLab MR after implementation quality passes")
fs.StringVar(&mrSourceBranch, "mr-source-branch", "", "source branch for --create-mr")
fs.StringVar(&mrTargetBranch, "mr-target-branch", "main", "target branch for --create-mr")
fs.StringVar(&mrTitle, "mr-title", "", "merge request title for --create-mr")
fs.StringVar(&mrDescription, "mr-description", "", "merge request description for --create-mr")
fs.StringVar(&mrDescriptionFile, "mr-description-file", "", "file containing merge request description for --create-mr")
fs.BoolVar(&mrRemoveSourceBranch, "mr-remove-source-branch", false, "ask GitLab to remove source branch after merge")
```

- [ ] Add helper in `internal/cli/run.go`.

```go
func configureMergeRequest(opts *demandflow.Options, createMR bool, stage demandflow.Stage, project, sourceBranch, targetBranch, title, description, descriptionFile, baseURL string, removeSourceBranch bool) error {
	if !createMR {
		return nil
	}
	if stage != demandflow.StageImplementation {
		return fmt.Errorf("--create-mr is only supported for --stage implementation")
	}
	if strings.TrimSpace(project) == "" || strings.TrimSpace(sourceBranch) == "" || strings.TrimSpace(title) == "" {
		return fmt.Errorf("--create-mr requires --gitlab-project, --mr-source-branch, and --mr-title")
	}
	body, err := mergeRequestDescription(description, descriptionFile)
	if err != nil {
		return err
	}
	opts.MergeRequest = demandflow.MergeRequestOptions{
		Adapter: newMergeRequestAdapter(),
		Spec: adapters.MergeRequestSpec{
			Project:            strings.TrimSpace(project),
			SourceBranch:       strings.TrimSpace(sourceBranch),
			TargetBranch:       strings.TrimSpace(targetBranch),
			Title:              strings.TrimSpace(title),
			Description:        body,
			BaseURL:            strings.TrimSpace(baseURL),
			RemoveSourceBranch: removeSourceBranch,
		},
	}
	return nil
}
```

- [ ] Call helper in `runDemandStage`.

After MR-review option setup and before creating the engine:

```go
if err := configureMergeRequest(&opts, createMR, parsedStage, gitlabProject, mrSourceBranch, mrTargetBranch, mrTitle, mrDescription, mrDescriptionFile, gitlabBaseURL, mrRemoveSourceBranch); err != nil {
	return err
}
```

- [ ] Add tests in `internal/cli/run_test.go`.

Add fake adapter:

```go
type cliFakeMergeRequestAdapter struct {
	spec adapters.MergeRequestSpec
}

func (a *cliFakeMergeRequestAdapter) EnsureMergeRequest(_ context.Context, spec adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	a.spec = spec
	return adapters.MergeRequestResult{Project: spec.Project, MergeRequest: "42", WebURL: "https://gitlab.example/mr/42", State: "opened", Created: true}, nil
}
```

Add test:

```go
func TestRunImplementationCanCreateMergeRequest(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Implementation)

	originalRunner := newDemandRunner
	defer func() { newDemandRunner = originalRunner }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageImplementation: {Text: "## Implementation\n\nstubbed implementation\n"},
		}}
	}

	adapter := &cliFakeMergeRequestAdapter{}
	originalMR := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = originalMR }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter { return adapter }

	t.Setenv("DEVFLOW_CLI_HELPER", "args")
	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$ -- --quality ok`, executable)

	var stdout bytes.Buffer
	err := Run([]string{
		"run",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "implementation",
		"--permission-mode", "acceptEdits",
		"--quality-command", commandText,
		"--create-mr",
		"--gitlab-project", "group/project",
		"--mr-source-branch", "feature/coupon",
		"--mr-target-branch", "main",
		"--mr-title", "Add coupon check",
		"--mr-description", "Generated by Devflow",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run implementation: %v", err)
	}
	if adapter.spec.Project != "group/project" || adapter.spec.SourceBranch != "feature/coupon" || adapter.spec.Title != "Add coupon check" {
		t.Fatalf("spec = %#v", adapter.spec)
	}
	progress, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if !strings.Contains(string(progress), "## Merge Request") || !strings.Contains(string(progress), "https://gitlab.example/mr/42") {
		t.Fatalf("progress missing MR evidence:\n%s", progress)
	}
}
```

Add validation test:

```go
func TestRunCreateMRRequiresImplementationStageAndSpec(t *testing.T) {
	err := Run([]string{
		"run",
		"--root", t.TempDir(),
		"--demand", "add-coupon-check",
		"--stage", "requirements",
		"--create-mr",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--create-mr is only supported") {
		t.Fatalf("err = %v, want stage error", err)
	}
}
```

Ensure imports include `context` and `github.com/jesseedcp/devflow-agent/internal/adapters`.

- [ ] Format and verify.

```powershell
gofmt -w internal/cli/run.go internal/cli/run_test.go
go test ./internal/cli -run 'TestRunImplementationCanCreateMergeRequest|TestRunCreateMRRequiresImplementationStageAndSpec' -count=1 -v
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/cli
git commit -m @'
Let implementation stage create or reuse GitLab MRs

The run command can now opt into MR sync after implementation quality passes,
so the normal demand workflow can move from implementation to mr_review with
MR evidence already captured.

Constraint: MR sync must be explicit and implementation-only
Rejected: Inferring MR sync from --gitlab-project alone | mr-review already uses that flag for a different operation
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/cli -run 'TestRunImplementationCanCreateMergeRequest|TestRunCreateMRRequiresImplementationStageAndSpec' -count=1 -v; go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 5: Cover MR Sync In Deterministic And Live Dogfood

**Files:**

- Modify: `internal/dogfood/runner.go`
- Modify: `internal/dogfood/runner_test.go`
- Modify: `internal/dogfood/live.go`

### Why

Release dogfood should prove the MR sync boundary without hitting GitLab. Live dogfood should also be able to use offline MR sync by default before optionally checking a real MR review gate.

- [ ] Add offline MR sync adapter in `internal/dogfood/runner.go`.

Add below `offlineReviewAdapter`:

```go
type offlineMergeRequestAdapter struct{}

func (offlineMergeRequestAdapter) EnsureMergeRequest(_ context.Context, spec adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	return adapters.MergeRequestResult{
		Project:      spec.Project,
		MergeRequest: "1",
		WebURL:       "https://gitlab.example/devflow/dogfood/-/merge_requests/1",
		State:        "opened",
		Created:      true,
	}, nil
}
```

- [ ] Configure deterministic implementation stage in `internal/dogfood/runner.go`.

Change implementation stage configuration:

```go
if err := runStage("implementation", demandflow.StageImplementation, func(o *demandflow.Options) {
	o.QualityCommands = opts.QualityCommands
	o.MergeRequest = demandflow.MergeRequestOptions{
		Adapter: offlineMergeRequestAdapter{},
		Spec: adapters.MergeRequestSpec{
			Project:      "dogfood/offline",
			SourceBranch: "feature/dogfood-coupon-eligibility",
			TargetBranch: "main",
			Title:        "Dogfood coupon eligibility",
			Description:  "Deterministic dogfood MR sync evidence.",
		},
	}
}); err != nil {
	return result, err
}
```

- [ ] Configure live sandbox implementation stage in `internal/dogfood/live.go`.

Inside the implementation stage configuration, add offline MR sync:

```go
if err := runStage("implementation", demandflow.StageImplementation, func(o *demandflow.Options) {
	o.QualityCommands = sandbox.QualityCommands
	o.MergeRequest = demandflow.MergeRequestOptions{
		Adapter: offlineMergeRequestAdapter{},
		Spec: adapters.MergeRequestSpec{
			Project:      "live-dogfood/offline",
			SourceBranch: "feature/live-dogfood-coupon-eligibility",
			TargetBranch: "main",
			Title:        "Live dogfood coupon eligibility",
			Description:  "Live sandbox dogfood MR sync evidence.",
		},
	}
}); err != nil {
	return result, err
}
```

- [ ] Update deterministic dogfood test in `internal/dogfood/runner_test.go`.

In `TestRunCompletesFullDeterministicLoop`, after reading `progress.md`, assert MR evidence:

```go
progress, err := os.ReadFile(filepath.Join(demandDir, artifacts.ProgressFile))
if err != nil {
	t.Fatalf("read progress: %v", err)
}
for _, want := range []string{"## Merge Request", "dogfood/offline", "merge_requests/1"} {
	if !strings.Contains(string(progress), want) {
		t.Fatalf("progress missing %q:\n%s", want, progress)
	}
}
```

- [ ] Format and verify.

```powershell
gofmt -w internal/dogfood/runner.go internal/dogfood/runner_test.go internal/dogfood/live.go
go test ./internal/dogfood -run TestRunCompletesFullDeterministicLoop -count=1 -v
go test ./internal/dogfood -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
git diff --check
```

- [ ] Commit.

```powershell
git add internal/dogfood
git commit -m @'
Make dogfood prove merge request sync evidence

Deterministic and live dogfood now exercise the MR sync interface with an
offline adapter, so release verification covers the MR lifecycle boundary
without requiring GitLab credentials.

Constraint: Dogfood must remain deterministic and offline by default
Rejected: Using the real GitLab adapter in default dogfood | external MR state would make CI unstable
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/dogfood -run TestRunCompletesFullDeterministicLoop -count=1 -v; go test ./internal/dogfood -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; git diff --check
'@
```

---

## Task 6: Update Docs And Remove The MR Creation Limit

**Files:**

- Modify: `README.md`
- Modify: `docs/release/v0.1.md`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/user-guide/live-dogfood.md`

- [ ] Update `README.md`.

Add command:

```text
devflow mr ensure
```

Add a short release-readiness line:

```markdown
Wave 10 adds GitLab MR creation/reuse through `devflow mr ensure` and `devflow run --stage implementation --create-mr`.
```

- [ ] Update `docs/release/v0.1.md`.

Add feature bullet:

```markdown
- GitLab MR creation/reuse through `devflow mr ensure` and implementation-stage `--create-mr`.
```

Remove this known limit:

```markdown
- GitLab MR creation is not automated.
```

Add a new known limit:

```markdown
- Git branch push, reviewer assignment, approvals, labels, and pipeline waiting are still manual or external.
```

- [ ] Update `docs/user-guide/backend-demand-loop.md`.

In the implementation section, replace the command with:

```powershell
devflow run --demand add-coupon-eligibility-check --stage implementation --permission-mode acceptEdits --quality-command "go test ./..." --create-mr --gitlab-project "group/project" --mr-source-branch "feature/add-coupon-eligibility-check" --mr-target-branch "main" --mr-title "Add coupon eligibility check" --mr-description-file ".devflow/demands/add-coupon-eligibility-check/plan.md"
```

Add direct MR command:

```markdown
You can also create or reuse the MR directly:

```powershell
devflow mr ensure --gitlab-project "group/project" --source-branch "feature/add-coupon-eligibility-check" --target-branch "main" --title "Add coupon eligibility check" --description-file ".devflow/demands/add-coupon-eligibility-check/plan.md"
```
```

- [ ] Update `docs/user-guide/live-dogfood.md`.

Add section:

```markdown
## Merge Request Sync

Create or reuse an MR directly:

```powershell
$env:GITLAB_TOKEN = "<private gitlab token>"
devflow mr ensure --gitlab-project "group/project" --source-branch "feature/coupon" --target-branch "main" --title "Add coupon check" --description "Generated by Devflow"
```

During implementation, add `--create-mr` to sync the MR after quality gates pass.
```

- [ ] Verify docs.

```powershell
git diff --check
go test ./... -count=1 -timeout 5m
```

- [ ] Commit.

```powershell
git add README.md docs/release/v0.1.md docs/user-guide/backend-demand-loop.md docs/user-guide/live-dogfood.md
git commit -m @'
Document GitLab merge request creation

The user-facing docs now show MR creation and reuse as part of the backend
demand loop and remove the earlier v0.1 limitation.

Constraint: Docs must keep branch push and reviewer workflows outside Wave 10 scope
Rejected: Claiming full GitLab automation | pushing branches, assigning reviewers, and approvals are still external
Confidence: high
Scope-risk: narrow
Tested: git diff --check; go test ./... -count=1 -timeout 5m
'@
```

---

## Task 7: Final Verification And PR

**Files:** none unless final fixes are needed

- [ ] Run targeted verification.

```powershell
go test ./internal/adapters -count=1
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
go test ./internal/dogfood -count=1
```

- [ ] Run full verification.

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-wave10 -Output dist\devflow-windows-amd64.exe
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave10
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave10
git diff --check
git status --short --branch
```

Expected:

```text
## feature/devflow-wave-10
```

- [ ] Optional live GitLab MR sync check, only with a safe project and disposable branch already pushed.

```powershell
$env:GITLAB_TOKEN = "<private gitlab token>"
.\dist\devflow-windows-amd64.exe mr ensure --gitlab-project "group/project" --source-branch "feature/disposable-devflow-wave10-smoke" --target-branch "main" --title "Devflow Wave 10 MR sync smoke" --description "Private smoke test for MR sync."
```

Record one of these in the PR:

```text
Live GitLab MR sync: PASS for group/project!<iid>
```

or:

```text
Live GitLab MR sync: not run; safe target branch unavailable
```

- [ ] Push branch.

```powershell
git push -u origin feature/devflow-wave-10
```

- [ ] Create PR.

```powershell
gh pr create --base main --head feature/devflow-wave-10 --title "Wave 10: GitLab MR creation sync" --body @'
## Summary
- Adds GitLab create-or-reuse MR adapter behavior.
- Adds `devflow mr ensure`.
- Adds implementation-stage `--create-mr` integration.
- Makes dogfood record offline MR sync evidence.
- Updates docs to remove the MR creation limitation.

## Test Plan
- [ ] go test ./internal/adapters -count=1
- [ ] go test ./internal/demandflow -count=1
- [ ] go test ./internal/cli -count=1
- [ ] go test ./internal/dogfood -count=1
- [ ] go test ./... -count=1 -timeout 5m
- [ ] go vet ./...
- [ ] go build ./cmd/devflow
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-wave10 -Output dist\devflow-windows-amd64.exe
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1 -Version 0.1.0-wave10
- [ ] powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave10
- [ ] git diff --check

## Optional Live Gate
- Live GitLab MR sync: not run; safe target branch unavailable
'@
```

Do not merge until deterministic verification and CI pass.

---

## Definition Of Done

Wave 10 is complete when:

- `adapters.MergeRequestAdapter` exists.
- `GitLabReviewAdapter.EnsureMergeRequest` reuses an existing open MR when one exists.
- `GitLabReviewAdapter.EnsureMergeRequest` creates an MR when none exists.
- `devflow mr ensure` exists and prints created/reused MR evidence.
- `devflow run --stage implementation --create-mr ...` records MR evidence in `progress.md`.
- Implementation-stage MR sync failure blocks on `blocked_need_platform`.
- Deterministic dogfood includes offline MR sync evidence.
- Docs no longer list "GitLab MR creation is not automated" as a v0.1 limit.
- Full verification passes:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave10
git diff --check
```

- A PR from `feature/devflow-wave-10` to `main` is open.

## Self-Review Notes

Spec coverage:

- Adapter contract and GitLab implementation are covered by Task 1.
- Direct CLI operation is covered by Task 2.
- Demandflow implementation-stage integration is covered by Task 3.
- `devflow run` flags are covered by Task 4.
- Dogfood evidence is covered by Task 5.
- Docs are covered by Task 6.
- Final verification and PR are covered by Task 7.

Marker scan:

- No step uses unfinished-marker wording or open-ended implementation instructions.
- Secret values stay in environment variables and are never written into docs, reports, tests, or command output.
- Optional live GitLab checks have explicit pass-or-not-run wording.

Type consistency:

- `MergeRequestSpec`, `MergeRequestResult`, and `MergeRequestAdapter` are introduced before CLI, demandflow, or dogfood use them.
- `GitLabReviewAdapter` keeps its existing review-comment behavior and gains MR sync behavior without renaming the type.
- `newMergeRequestAdapter` mirrors existing `newReviewAdapter` so CLI tests can stub network behavior.

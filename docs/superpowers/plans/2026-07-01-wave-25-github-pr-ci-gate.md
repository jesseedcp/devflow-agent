# Wave 25 GitHub PR CI Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a GitHub PR CI gate so the backend-demand loop can block `mr-review -> verification` until the target GitHub PR checks are green.

**Architecture:** Keep the existing GitLab review adapter intact. Add a small provider-neutral CI gate interface in `internal/adapters`, implement a GitHub REST adapter, then wire it into `devflow ci-gate` and optionally into the existing `mr-review` stage through GitHub-specific CLI flags. The workflow state remains `mr_review` for now; a later wave can rename MR/PR terminology globally.

**Tech Stack:** Go standard library only (`net/http`, `encoding/json`, `flag`, `httptest`), existing `artifacts.Store`, existing `demandflow.Engine`, existing console/workbench summary surfaces.

---

## Context

The repository currently has GitLab-oriented review/MR surfaces:

- `internal/adapters/review.go` defines `ReviewRef` and `ReviewAdapter`.
- `internal/adapters/gitlab.go` implements unresolved GitLab MR comments.
- `internal/cli/review_gate.go` provides `devflow review-gate --gitlab-project --gitlab-mr`.
- `internal/cli/run.go` wires `--gitlab-project`, `--gitlab-mr`, and `--gitlab-base-url` into `StageMRReview`.
- `internal/demandflow/engine.go` advances from `workflow.MRReview` to `workflow.Verification` when review comments are clear.
- `internal/demandflow/workspace.go`, `internal/cli/console.go`, and `internal/cli/workbench_snapshot.go` summarize MR/review state.

Wave 25 changes direction from GitLab to GitHub for CI gating. Do not delete the GitLab adapter in this wave. The narrow product outcome is:

```text
GitHub PR checks not green -> mr-review remains blocked
GitHub PR checks green + review comments clear -> advance to verification
```

Official GitHub API references used for this plan:

- Checks API: `GET /repos/{owner}/{repo}/commits/{ref}/check-runs`
- Pull Requests API: `GET /repos/{owner}/{repo}/pulls/{pull_number}`
- GitHub Actions workflow statuses/conclusions: `success`, `failure`, `cancelled`, `timed_out`, `queued`, `in_progress`, `pending`, etc.

## Scope

In scope:

- Add provider-neutral CI gate types.
- Add GitHub CI adapter using REST API.
- Add direct CLI command: `devflow ci-gate --github-repo owner/repo --github-pr 123`.
- Add optional `mr-review` integration through `devflow run --stage mr-review --github-repo owner/repo --github-pr 123`.
- Record CI gate evidence in `progress.md`.
- Record events in `events.jsonl`.
- Surface CI gate state in workspace, console, and workbench snapshot.
- Add deterministic fake-server tests.
- Add release-readiness smoke with no live GitHub dependency.
- Update user docs and release notes.

Out of scope:

- Do not rename all `MR`/`mr_review` workflow state names to `PR`.
- Do not remove GitLab adapter or GitLab docs.
- Do not implement GitHub PR creation.
- Do not implement GitHub PR review comment replies.
- Do not require real `GITHUB_TOKEN` in default tests or CI.

## File Structure

Create:

- `internal/adapters/ci.go`  
  Provider-neutral CI gate model and interface.

- `internal/adapters/github_ci.go`  
  GitHub REST adapter. Fetches PR head SHA, fetches check runs for that SHA, normalizes statuses/conclusions.

- `internal/adapters/github_ci_test.go`  
  Fake GitHub server tests for pass/fail/pending/auth/error cases.

- `internal/demandflow/ci_gate.go`  
  Rendering and event helper logic for CI gate evidence.

- `internal/demandflow/ci_gate_test.go`  
  Unit tests for rendering and engine behavior helpers.

- `internal/cli/ci_gate.go`  
  Direct `devflow ci-gate` command.

- `internal/cli/ci_gate_test.go`  
  CLI tests using fake adapter injection.

Modify:

- `internal/demandflow/types.go`  
  Add `CIGate CIGateOptions` to `Options`.

- `internal/demandflow/engine.go`  
  Check CI gate inside `runMRReview` after review comments are clear and before advancing to verification.

- `internal/cli/cli.go`  
  Add help text and command dispatch for `ci-gate`.

- `internal/cli/run.go`  
  Add `--github-repo`, `--github-pr`, `--github-base-url`; wire `CIGateOptions` for `StageMRReview`.

- `internal/cli/console.go`  
  Pass GitHub flags through `console --run-next`.

- `internal/demandflow/workspace.go`  
  Add `CIGateSummary`, summarize events, and adjust attention/next-actions.

- `internal/cli/workbench_snapshot.go`  
  Print CI gate evidence in snapshot.

- `scripts/release-readiness.ps1`  
  Add deterministic CI gate smoke using a local fake GitHub server helper if needed.

- `docs/user-guide/backend-demand-loop.md`

- `docs/user-guide/dogfood-smoke.md`

- `docs/user-guide/live-dogfood.md`

- `docs/release/v0.1.md`

## Data Model

Add this model in `internal/adapters/ci.go`:

```go
package adapters

import "context"

type CIStatus string

const (
	CIStatusPassed  CIStatus = "passed"
	CIStatusFailed  CIStatus = "failed"
	CIStatusPending CIStatus = "pending"
	CIStatusUnknown CIStatus = "unknown"
)

type CIRef struct {
	Provider string
	Repo     string
	PR       string
	BaseURL  string
	Token    string
}

type CICheck struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
}

type CIResult struct {
	Provider string
	Repo     string
	PR       string
	HeadSHA  string
	Status   CIStatus
	Checks   []CICheck
	Message  string
}

type CIGateAdapter interface {
	Check(ctx context.Context, ref CIRef) (CIResult, error)
}
```

Normalization rules:

- If at least one check has `conclusion` in `failure`, `cancelled`, `timed_out`, or `action_required`, the result is `failed`.
- If no failing check exists and at least one check is not completed, or has empty conclusion, the result is `pending`.
- If all completed check runs have `success`, `neutral`, or `skipped`, the result is `passed`.
- If GitHub returns zero check runs, the result is `pending`, not passed. A PR with no CI must not advance automatically.

## Task 0: Prepare The Worktree

**Files:**

- No source file changes.

- [ ] **Step 1: Ensure main is current and clean**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git status --short
git branch --show-current
```

Expected:

```text
main
```

The status should be clean. If it is not clean, inspect the changes and do not overwrite unrelated user work.

- [ ] **Step 2: Create an isolated Wave 25 worktree**

Run:

```powershell
git fetch origin
git worktree add .worktrees/wave25-github-pr-ci-gate -b wave25-github-pr-ci-gate origin/main
cd .worktrees/wave25-github-pr-ci-gate
```

Expected:

```text
Preparing worktree
HEAD is now at ...
```

- [ ] **Step 3: Record the branch**

Run:

```powershell
git branch --show-current
```

Expected:

```text
wave25-github-pr-ci-gate
```

## Task 1: Add Provider-Neutral CI Gate Types

**Files:**

- Create: `internal/adapters/ci.go`
- Test: `internal/adapters/github_ci_test.go` begins in Task 2

- [ ] **Step 1: Create the CI gate interface**

Add `internal/adapters/ci.go` with the exact model from the Data Model section.

- [ ] **Step 2: Run package tests**

Run:

```powershell
go test ./internal/adapters -count=1
```

Expected:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/adapters
```

- [ ] **Step 3: Commit**

Run:

```powershell
git add internal/adapters/ci.go
git commit -m "Model provider-neutral CI gate results" -m "Introduce a small CI gate contract before adding GitHub-specific HTTP behavior so demandflow can depend on stable local types instead of GitHub response shapes." -m "Constraint: Keep GitLab review support intact while adding GitHub CI support." -m "Rejected: Reuse ReviewAdapter for CI checks | Review comments and CI status have different lifecycles and evidence." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/adapters -count=1"
```

## Task 2: Implement GitHub CI Adapter

**Files:**

- Create: `internal/adapters/github_ci.go`
- Create: `internal/adapters/github_ci_test.go`

- [ ] **Step 1: Write failing tests for successful check runs**

Create `internal/adapters/github_ci_test.go` with this first test:

```go
package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubCIGatePassesWhenAllChecksSucceed(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/42":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 42,
				"head": map[string]any{"sha": "abc123"},
			})
		case "/repos/owner/repo/commits/abc123/check-runs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 2,
				"check_runs": []map[string]any{
					{"name": "Go verification (ubuntu-latest)", "status": "completed", "conclusion": "success", "html_url": "https://github.test/checks/1"},
					{"name": "Go verification (windows-latest)", "status": "completed", "conclusion": "success", "html_url": "https://github.test/checks/2"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := (GitHubCIAdapter{Client: server.Client()}).Check(context.Background(), CIRef{
		Repo:    "owner/repo",
		PR:      "42",
		BaseURL: server.URL,
		Token:   "secret-token",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want Bearer token", gotAuth)
	}
	if result.Status != CIStatusPassed {
		t.Fatalf("Status = %s, want %s", result.Status, CIStatusPassed)
	}
	if result.HeadSHA != "abc123" {
		t.Fatalf("HeadSHA = %q, want abc123", result.HeadSHA)
	}
	if len(result.Checks) != 2 {
		t.Fatalf("Checks = %d, want 2", len(result.Checks))
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```powershell
go test ./internal/adapters -run TestGitHubCIGatePassesWhenAllChecksSucceed -count=1
```

Expected: fail because `GitHubCIAdapter` is undefined.

- [ ] **Step 3: Implement minimal GitHub adapter**

Create `internal/adapters/github_ci.go`:

```go
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const defaultGitHubAPIBaseURL = "https://api.github.com"

type GitHubCIAdapter struct {
	Client *http.Client
}

type githubPullResponse struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type githubCheckRunsResponse struct {
	TotalCount int              `json:"total_count"`
	CheckRuns  []githubCheckRun `json:"check_runs"`
}

type githubCheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

func (a GitHubCIAdapter) Check(ctx context.Context, ref CIRef) (CIResult, error) {
	repo := strings.TrimSpace(ref.Repo)
	pr := strings.TrimSpace(ref.PR)
	if repo == "" || !strings.Contains(repo, "/") {
		return CIResult{}, fmt.Errorf("github repo must be owner/repo")
	}
	if pr == "" {
		return CIResult{}, fmt.Errorf("github pr is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(ref.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGitHubAPIBaseURL
	}
	token := strings.TrimSpace(ref.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}

	var pull githubPullResponse
	if err := a.getJSON(ctx, baseURL+"/repos/"+url.PathEscape(repo)+"/pulls/"+url.PathEscape(pr), token, &pull); err != nil {
		return CIResult{}, fmt.Errorf("fetch github pull request: %w", err)
	}
	if strings.TrimSpace(pull.Head.SHA) == "" {
		return CIResult{}, fmt.Errorf("github pull request response missing head sha")
	}

	var checks githubCheckRunsResponse
	checksURL := baseURL + "/repos/" + url.PathEscape(repo) + "/commits/" + url.PathEscape(pull.Head.SHA) + "/check-runs"
	if err := a.getJSON(ctx, checksURL, token, &checks); err != nil {
		return CIResult{}, fmt.Errorf("fetch github check runs: %w", err)
	}

	result := CIResult{
		Provider: "github",
		Repo:     repo,
		PR:       pr,
		HeadSHA:  pull.Head.SHA,
		Checks:   make([]CICheck, 0, len(checks.CheckRuns)),
	}
	for _, check := range checks.CheckRuns {
		result.Checks = append(result.Checks, CICheck{
			Name:       check.Name,
			Status:     check.Status,
			Conclusion: check.Conclusion,
			URL:        check.HTMLURL,
		})
	}
	result.Status = normalizeGitHubCIStatus(result.Checks)
	result.Message = githubCIMessage(result)
	return result, nil
}

func (a GitHubCIAdapter) getJSON(ctx context.Context, endpoint, token string, out any) error {
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("github api returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func normalizeGitHubCIStatus(checks []CICheck) CIStatus {
	if len(checks) == 0 {
		return CIStatusPending
	}
	pending := false
	for _, check := range checks {
		status := strings.ToLower(strings.TrimSpace(check.Status))
		conclusion := strings.ToLower(strings.TrimSpace(check.Conclusion))
		if status != "completed" || conclusion == "" {
			pending = true
			continue
		}
		switch conclusion {
		case "success", "neutral", "skipped":
		case "failure", "cancelled", "timed_out", "action_required":
			return CIStatusFailed
		default:
			return CIStatusUnknown
		}
	}
	if pending {
		return CIStatusPending
	}
	return CIStatusPassed
}

func githubCIMessage(result CIResult) string {
	switch result.Status {
	case CIStatusPassed:
		return "github ci passed"
	case CIStatusFailed:
		return "github ci failed"
	case CIStatusPending:
		return "github ci pending"
	default:
		return "github ci status unknown"
	}
}
```

- [ ] **Step 4: Fix URL escaping if the test exposes `%2F` path mismatch**

If the fake server sees `/repos/owner%2Frepo/...`, replace the two endpoint builders with this helper:

```go
func githubRepoPath(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	return url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
}
```

Then use:

```go
baseURL + "/repos/" + githubRepoPath(repo) + "/pulls/" + url.PathEscape(pr)
baseURL + "/repos/" + githubRepoPath(repo) + "/commits/" + url.PathEscape(pull.Head.SHA) + "/check-runs"
```

- [ ] **Step 5: Add failing tests for failed, pending, zero-check, and API error cases**

Append tests:

```go
func TestNormalizeGitHubCIStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []CICheck
		want   CIStatus
	}{
		{name: "zero checks pending", checks: nil, want: CIStatusPending},
		{name: "failed conclusion", checks: []CICheck{{Status: "completed", Conclusion: "failure"}}, want: CIStatusFailed},
		{name: "cancelled conclusion", checks: []CICheck{{Status: "completed", Conclusion: "cancelled"}}, want: CIStatusFailed},
		{name: "in progress", checks: []CICheck{{Status: "in_progress"}}, want: CIStatusPending},
		{name: "queued", checks: []CICheck{{Status: "queued"}}, want: CIStatusPending},
		{name: "skipped is allowed", checks: []CICheck{{Status: "completed", Conclusion: "skipped"}}, want: CIStatusPassed},
		{name: "neutral is allowed", checks: []CICheck{{Status: "completed", Conclusion: "neutral"}}, want: CIStatusPassed},
		{name: "unknown conclusion", checks: []CICheck{{Status: "completed", Conclusion: "mystery"}}, want: CIStatusUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeGitHubCIStatus(tc.checks); got != tc.want {
				t.Fatalf("normalizeGitHubCIStatus() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestGitHubCIGateReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := (GitHubCIAdapter{Client: server.Client()}).Check(context.Background(), CIRef{
		Repo:    "owner/repo",
		PR:      "42",
		BaseURL: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "github api returned 500") {
		t.Fatalf("err = %v, want github 500", err)
	}
}
```

- [ ] **Step 6: Run adapter tests**

Run:

```powershell
go test ./internal/adapters -count=1
```

Expected: pass.

- [ ] **Step 7: Commit**

Run:

```powershell
git add internal/adapters/github_ci.go internal/adapters/github_ci_test.go
git commit -m "Read GitHub PR check runs for CI gating" -m "Fetch the pull request head SHA, then normalize check runs on that SHA into the provider-neutral CI gate result used by demandflow." -m "Constraint: Default tests must not call live GitHub or require GITHUB_TOKEN." -m "Rejected: Depend on gh CLI | local installs and auth state would make release readiness flaky." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/adapters -count=1"
```

## Task 3: Render CI Evidence In Demandflow

**Files:**

- Create: `internal/demandflow/ci_gate.go`
- Create: `internal/demandflow/ci_gate_test.go`
- Modify: `internal/demandflow/types.go`

- [ ] **Step 1: Add demandflow options**

Modify `internal/demandflow/types.go`:

```go
type CIGateOptions struct {
	Adapter adapters.CIGateAdapter
	Ref     adapters.CIRef
}
```

Add this field to `Options`:

```go
CIGate CIGateOptions
```

Add the `adapters` import to `types.go`.

- [ ] **Step 2: Add render tests**

Create `internal/demandflow/ci_gate_test.go`:

```go
package demandflow

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func TestRenderCIGateEvidenceIncludesStatusAndChecks(t *testing.T) {
	body := renderCIGateEvidence(adapters.CIResult{
		Provider: "github",
		Repo:     "owner/repo",
		PR:       "42",
		HeadSHA:  "abc123",
		Status:   adapters.CIStatusFailed,
		Checks: []adapters.CICheck{{
			Name:       "Go verification",
			Status:     "completed",
			Conclusion: "failure",
			URL:        "https://github.test/checks/1",
		}},
	})
	for _, want := range []string{"## CI Gate", "github", "owner/repo#42", "failed", "Go verification", "failure"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered evidence missing %q:\n%s", want, body)
		}
	}
}
```

- [ ] **Step 3: Add render implementation**

Create `internal/demandflow/ci_gate.go`:

```go
package demandflow

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func renderCIGateEvidence(result adapters.CIResult) string {
	var b strings.Builder
	b.WriteString("## CI Gate\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", result.Provider)
	fmt.Fprintf(&b, "Target: %s#%s\n", result.Repo, result.PR)
	if strings.TrimSpace(result.HeadSHA) != "" {
		fmt.Fprintf(&b, "Head SHA: %s\n", result.HeadSHA)
	}
	fmt.Fprintf(&b, "Status: %s\n\n", result.Status)
	if len(result.Checks) == 0 {
		b.WriteString("- no check runs found\n\n")
		return b.String()
	}
	for _, check := range result.Checks {
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "(unnamed check)"
		}
		fmt.Fprintf(&b, "- %s: status=%s conclusion=%s", name, check.Status, check.Conclusion)
		if strings.TrimSpace(check.URL) != "" {
			fmt.Fprintf(&b, " url=%s", check.URL)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}
```

- [ ] **Step 4: Run demandflow tests**

Run:

```powershell
go test ./internal/demandflow -run TestRenderCIGateEvidenceIncludesStatusAndChecks -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```powershell
git add internal/demandflow/types.go internal/demandflow/ci_gate.go internal/demandflow/ci_gate_test.go
git commit -m "Render CI gate evidence in demandflow" -m "Add CI gate options and markdown evidence rendering so engine and CLI work can record GitHub check status without leaking GitHub response details across packages." -m "Constraint: Keep workflow states unchanged in Wave 25." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -run TestRenderCIGateEvidenceIncludesStatusAndChecks -count=1"
```

## Task 4: Add Direct `devflow ci-gate`

**Files:**

- Create: `internal/cli/ci_gate.go`
- Create: `internal/cli/ci_gate_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Add CLI adapter injection**

In `internal/cli/run.go`, near `newReviewAdapter`, add:

```go
var newCIGateAdapter = func() adapters.CIGateAdapter {
	return adapters.GitHubCIAdapter{}
}
```

- [ ] **Step 2: Add failing CLI tests**

Create `internal/cli/ci_gate_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

type fakeCIGateAdapter struct {
	result adapters.CIResult
	err    error
	ref    adapters.CIRef
}

func (f *fakeCIGateAdapter) Check(ctx context.Context, ref adapters.CIRef) (adapters.CIResult, error) {
	f.ref = ref
	return f.result, f.err
}

func TestRunCIGatePassesGitHubFlags(t *testing.T) {
	original := newCIGateAdapter
	defer func() { newCIGateAdapter = original }()
	fake := &fakeCIGateAdapter{result: adapters.CIResult{
		Provider: "github",
		Repo:     "owner/repo",
		PR:       "42",
		Status:   adapters.CIStatusPassed,
	}}
	newCIGateAdapter = func() adapters.CIGateAdapter { return fake }

	var stdout bytes.Buffer
	err := runCIGate([]string{"--github-repo", "owner/repo", "--github-pr", "42", "--github-base-url", "https://github.test"}, &stdout, ioDiscard{})
	if err != nil {
		t.Fatalf("runCIGate returned error: %v", err)
	}
	if fake.ref.Repo != "owner/repo" || fake.ref.PR != "42" || fake.ref.BaseURL != "https://github.test" {
		t.Fatalf("ref = %#v", fake.ref)
	}
	if !strings.Contains(stdout.String(), "ci gate passed for owner/repo#42") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCIGateFailsWhenChecksFail(t *testing.T) {
	original := newCIGateAdapter
	defer func() { newCIGateAdapter = original }()
	newCIGateAdapter = func() adapters.CIGateAdapter {
		return &fakeCIGateAdapter{result: adapters.CIResult{
			Provider: "github",
			Repo:     "owner/repo",
			PR:       "42",
			Status:   adapters.CIStatusFailed,
			Checks:   []adapters.CICheck{{Name: "Go verification", Status: "completed", Conclusion: "failure"}},
		}}
	}

	var stdout bytes.Buffer
	err := runCIGate([]string{"--github-repo", "owner/repo", "--github-pr", "42"}, &stdout, ioDiscard{})
	if err == nil || !strings.Contains(err.Error(), "ci gate blocked") {
		t.Fatalf("err = %v, want blocked", err)
	}
	if !strings.Contains(stdout.String(), "Go verification") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
```

If `ioDiscard` conflicts with an existing helper, replace it with `io.Discard` and import `io`.

- [ ] **Step 3: Run failing CLI tests**

Run:

```powershell
go test ./internal/cli -run "TestRunCIGate" -count=1
```

Expected: fail because `runCIGate` is undefined.

- [ ] **Step 4: Implement `runCIGate`**

Create `internal/cli/ci_gate.go`:

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

func runCIGate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("ci-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var repo, pr, baseURL string
	fs.StringVar(&repo, "github-repo", "", "GitHub repository in owner/repo form")
	fs.StringVar(&pr, "github-pr", "", "GitHub pull request number")
	fs.StringVar(&baseURL, "github-base-url", "", "GitHub API base url override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repo = strings.TrimSpace(repo)
	pr = strings.TrimSpace(pr)
	if repo == "" || pr == "" {
		return fmt.Errorf("--github-repo and --github-pr are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := newCIGateAdapter().Check(ctx, adapters.CIRef{
		Provider: "github",
		Repo:     repo,
		PR:       pr,
		BaseURL:  strings.TrimSpace(baseURL),
	})
	if err != nil {
		return err
	}
	for _, check := range result.Checks {
		fmt.Fprintf(stdout, "- %s: status=%s conclusion=%s\n", check.Name, check.Status, check.Conclusion)
	}
	switch result.Status {
	case adapters.CIStatusPassed:
		_, err := fmt.Fprintf(stdout, "ci gate passed for %s#%s\n", repo, pr)
		return err
	case adapters.CIStatusFailed, adapters.CIStatusPending, adapters.CIStatusUnknown:
		fmt.Fprintf(stdout, "ci gate blocked for %s#%s: %s\n", repo, pr, result.Status)
		return fmt.Errorf("ci gate blocked: %s", result.Status)
	default:
		return fmt.Errorf("ci gate blocked: unexpected status %s", result.Status)
	}
}
```

- [ ] **Step 5: Register command and help**

Modify `internal/cli/cli.go`.

Add to usage:

```text
  devflow ci-gate --github-repo <owner/repo> --github-pr <number>
```

Add to command list:

```text
  ci-gate  Check GitHub PR CI status directly
```

Add dispatch:

```go
case "ci-gate":
	return runCIGate(args[1:], stdout, stderr)
```

- [ ] **Step 6: Run CLI tests**

Run:

```powershell
go test ./internal/cli -run "TestRunCIGate|TestHelp" -count=1
```

Expected: pass. If no `TestHelp` exists, run only `TestRunCIGate`.

- [ ] **Step 7: Commit**

Run:

```powershell
git add internal/cli/ci_gate.go internal/cli/ci_gate_test.go internal/cli/cli.go internal/cli/run.go
git commit -m "Expose GitHub CI gate as a direct CLI command" -m "Add devflow ci-gate so operators can check GitHub PR checks without running a whole demand stage." -m "Constraint: The command must be deterministic under tests through adapter injection." -m "Rejected: Hide CI checking only inside mr-review | operators need a direct diagnostic command." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run TestRunCIGate -count=1"
```

## Task 5: Gate `mr-review` On GitHub CI

**Files:**

- Modify: `internal/demandflow/engine.go`
- Modify: `internal/cli/run.go`
- Test: `internal/demandflow/review_test.go`
- Test: `internal/cli/run_test.go`

- [ ] **Step 1: Add demandflow test for pending CI blocking verification**

Append to `internal/demandflow/review_test.go`:

```go
type fakeCIGate struct {
	result adapters.CIResult
	err    error
}

func (f fakeCIGate) Check(context.Context, adapters.CIRef) (adapters.CIResult, error) {
	return f.result, f.err
}

func TestEngineMRReviewClearCommentsButPendingCIRemainsBlocked(t *testing.T) {
	engine, demand := setupDemandAtState(t, workflow.MRReview)
	err := engine.Run(context.Background(), Options{
		DemandID: demand.ID,
		Stage:   StageMRReview,
		Runner:  staticRunner{},
		Review:  mrReviewOptions(fakeReviewAdapter{}),
		CIGate: CIGateOptions{
			Adapter: fakeCIGate{result: adapters.CIResult{
				Provider: "github",
				Repo:     "owner/repo",
				PR:       "42",
				Status:   adapters.CIStatusPending,
				Checks:   []adapters.CICheck{{Name: "Go verification", Status: "queued"}},
			}},
			Ref: adapters.CIRef{Provider: "github", Repo: "owner/repo", PR: "42"},
		},
		Now: fixedTime,
	})
	if err == nil || !strings.Contains(err.Error(), "ci gate blocked") {
		t.Fatalf("err = %v, want ci gate blocked", err)
	}
	updated, loadErr := engine.Store.LoadDemand(demand.ID)
	if loadErr != nil {
		t.Fatalf("LoadDemand returned error: %v", loadErr)
	}
	if workflow.State(updated.State) != workflow.MRReview {
		t.Fatalf("state = %s, want %s", updated.State, workflow.MRReview)
	}
	body := readDemandflowArtifact(t, engine.Store, demand.ID, artifacts.ProgressFile)
	if !strings.Contains(body, "## CI Gate") || !strings.Contains(body, "pending") {
		t.Fatalf("progress.md missing CI gate evidence:\n%s", body)
	}
}
```

Adjust helper names if this file already uses different local test helpers. Do not create a second `fakeCIGate` if the name already exists.

- [ ] **Step 2: Run failing demandflow test**

Run:

```powershell
go test ./internal/demandflow -run TestEngineMRReviewClearCommentsButPendingCIRemainsBlocked -count=1
```

Expected: fail because `runMRReview` ignores `CIGate`.

- [ ] **Step 3: Implement engine CI block**

In `internal/demandflow/engine.go`, inside `runMRReview`, after confirming `actionPlan.NextState == workflow.Verification` and before `opts.MergeRequest.Adapter != nil`, add:

```go
if opts.CIGate.Adapter != nil {
	ciResult, ciErr := opts.CIGate.Adapter.Check(ctx, opts.CIGate.Ref)
	if ciErr != nil {
		return fmt.Errorf("check ci gate: %w", ciErr)
	}
	if err := e.Store.AppendToArtifact(opts.DemandID, artifacts.ProgressFile, renderCIGateEvidence(ciResult)); err != nil {
		return err
	}
	eventType := "ci_gate.passed"
	if ciResult.Status != adapters.CIStatusPassed {
		eventType = "ci_gate.blocked"
	}
	if err := e.Store.AppendEvent(opts.DemandID, artifacts.Event{
		Time:    opts.Now(),
		Type:    eventType,
		Message: ciResult.Message,
		Data: map[string]string{
			"provider": string(ciResult.Provider),
			"repo":     ciResult.Repo,
			"pr":       ciResult.PR,
			"head_sha": ciResult.HeadSHA,
			"status":   string(ciResult.Status),
		},
	}); err != nil {
		return err
	}
	if ciResult.Status != adapters.CIStatusPassed {
		result.Message = "ci gate blocked: " + string(ciResult.Status)
		return fmt.Errorf("ci gate blocked: %s", ciResult.Status)
	}
}
```

If `string(ciResult.Provider)` fails because `Provider` is already a string, use `ciResult.Provider`.

- [ ] **Step 4: Add passing CI test**

Append another test:

```go
func TestEngineMRReviewClearCommentsAndPassingCIAdvancesToVerification(t *testing.T) {
	engine, demand := setupDemandAtState(t, workflow.MRReview)
	err := engine.Run(context.Background(), Options{
		DemandID: demand.ID,
		Stage:   StageMRReview,
		Runner:  staticRunner{},
		Review:  mrReviewOptions(fakeReviewAdapter{}),
		CIGate: CIGateOptions{
			Adapter: fakeCIGate{result: adapters.CIResult{
				Provider: "github",
				Repo:     "owner/repo",
				PR:       "42",
				Status:   adapters.CIStatusPassed,
				Checks:   []adapters.CICheck{{Name: "Go verification", Status: "completed", Conclusion: "success"}},
				Message:  "github ci passed",
			}},
			Ref: adapters.CIRef{Provider: "github", Repo: "owner/repo", PR: "42"},
		},
		Now: fixedTime,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	updated, loadErr := engine.Store.LoadDemand(demand.ID)
	if loadErr != nil {
		t.Fatalf("LoadDemand returned error: %v", loadErr)
	}
	if workflow.State(updated.State) != workflow.Verification {
		t.Fatalf("state = %s, want %s", updated.State, workflow.Verification)
	}
}
```

- [ ] **Step 5: Run demandflow review tests**

Run:

```powershell
go test ./internal/demandflow -run "TestEngineMRReview.*CI|TestEngineMRReview" -count=1
```

Expected: pass.

- [ ] **Step 6: Wire GitHub flags into `runDemandStage`**

Modify `internal/cli/run.go`.

Add local variables:

```go
var githubRepo, githubPR, githubBaseURL string
```

Add flags:

```go
fs.StringVar(&githubRepo, "github-repo", "", "GitHub repository in owner/repo form for mr-review CI gate")
fs.StringVar(&githubPR, "github-pr", "", "GitHub pull request number for mr-review CI gate")
fs.StringVar(&githubBaseURL, "github-base-url", "", "GitHub API base url override")
```

Inside `if parsedStage == demandflow.StageMRReview`, after existing GitLab review setup, add:

```go
githubRepo = strings.TrimSpace(githubRepo)
githubPR = strings.TrimSpace(githubPR)
if githubRepo != "" || githubPR != "" {
	if githubRepo == "" || githubPR == "" {
		return fmt.Errorf("--github-repo and --github-pr must be provided together for mr-review CI gate")
	}
	opts.CIGate = demandflow.CIGateOptions{
		Adapter: newCIGateAdapter(),
		Ref: adapters.CIRef{
			Provider: "github",
			Repo:     githubRepo,
			PR:       githubPR,
			BaseURL:  strings.TrimSpace(githubBaseURL),
		},
	}
}
```

Do not remove `--gitlab-project` or `--gitlab-mr` requirement in this task. This wave adds GitHub CI on top of the existing review adapter. A later wave can add GitHub review comments and remove GitLab from this path.

- [ ] **Step 7: Add CLI test for GitHub CI flags**

In `internal/cli/run_test.go`, add a test that stubs both `newReviewAdapter` and `newCIGateAdapter`, runs:

```powershell
devflow run --demand add-coupon-check --stage mr-review --gitlab-project group/project --gitlab-mr 1 --github-repo owner/repo --github-pr 42
```

Assert:

- output contains `state: mr_review -> verification`
- progress contains `## CI Gate`
- events contain `ci_gate.passed`

Use existing run test helpers in the file rather than creating a new demand setup utility.

- [ ] **Step 8: Run CLI run tests**

Run:

```powershell
go test ./internal/cli -run "TestRun.*MR|TestRun.*CI|TestRunDemandStage" -count=1
```

Expected: pass.

- [ ] **Step 9: Commit**

Run:

```powershell
git add internal/demandflow/engine.go internal/demandflow/review_test.go internal/cli/run.go internal/cli/run_test.go
git commit -m "Block mr-review on GitHub CI status" -m "When GitHub CI options are supplied, mr-review now records CI evidence and only advances to verification if the GitHub check runs are green." -m "Constraint: Existing GitLab unresolved-comment gate remains the review adapter for this wave." -m "Rejected: Rename mr-review to pr-review now | global workflow rename would expand blast radius beyond CI gating." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/demandflow -run TestEngineMRReview -count=1; go test ./internal/cli -run TestRun -count=1"
```

## Task 6: Surface CI Gate In Workspace, Console, And Workbench

**Files:**

- Modify: `internal/demandflow/workspace.go`
- Modify: `internal/demandflow/workspace_test.go`
- Modify: `internal/cli/console.go`
- Modify: `internal/cli/console_test.go`
- Modify: `internal/cli/workbench_snapshot.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add workspace CI summary model**

In `internal/demandflow/workspace.go`, add:

```go
type CIGateSummary struct {
	Status   string
	Provider string
	Repo     string
	PR       string
	HeadSHA  string
	Message  string
}
```

Add field to `WorkspaceSummary`:

```go
CIGate CIGateSummary
```

Inside `InspectWorkspace`, after `summary.MergeRequest = ...`, add:

```go
summary.CIGate = summarizeCIGate(events)
```

Add function:

```go
func summarizeCIGate(events []artifacts.Event) CIGateSummary {
	summary := CIGateSummary{Status: "not_checked"}
	for _, event := range events {
		switch event.Type {
		case "ci_gate.passed", "ci_gate.blocked":
			status := strings.TrimSpace(event.Data["status"])
			if status == "" && event.Type == "ci_gate.passed" {
				status = "passed"
			}
			if status == "" {
				status = "blocked"
			}
			summary = CIGateSummary{
				Status:   status,
				Provider: event.Data["provider"],
				Repo:     event.Data["repo"],
				PR:       event.Data["pr"],
				HeadSHA:  event.Data["head_sha"],
				Message:  event.Message,
			}
		}
	}
	return summary
}
```

- [ ] **Step 2: Adjust attention and next actions**

In `WorkspaceNextActions`, change the MRReview cleared condition so verification is only recommended when CI is passed or not configured:

```go
if summary.State == workflow.MRReview && summary.MergeRequest.Status == "cleared" {
	if summary.CIGate.Status == "failed" || summary.CIGate.Status == "pending" || summary.CIGate.Status == "unknown" {
		return []NextAction{{Label: "Wait for GitHub CI", Command: "", Reason: "GitHub CI gate is not passing yet."}}
	}
	return []NextAction{{Label: "Draft verification", Command: "devflow run --demand " + idArg + " --stage verification --quality-command \"go test ./...\"", Reason: "MR review is clear and CI is passing."}}
}
```

In `workspaceAttention`, inside `workflow.MRReview`, prefer CI message:

```go
if summary.CIGate.Status == "failed" || summary.CIGate.Status == "pending" || summary.CIGate.Status == "unknown" {
	return "needs GitHub CI gate"
}
```

- [ ] **Step 3: Add workspace tests**

In `internal/demandflow/workspace_test.go`, add an event setup that appends:

```go
artifacts.Event{
	Type:    "ci_gate.blocked",
	Message: "github ci pending",
	Data: map[string]string{
		"provider": "github",
		"repo":     "owner/repo",
		"pr":       "42",
		"status":   "pending",
	},
}
```

Assert:

```go
if summary.CIGate.Status != "pending" {
	t.Fatalf("CIGate.Status = %q, want pending", summary.CIGate.Status)
}
if summary.Attention != "needs GitHub CI gate" {
	t.Fatalf("Attention = %q, want needs GitHub CI gate", summary.Attention)
}
```

- [ ] **Step 4: Print CI in console**

In `internal/cli/console.go`, in `printConsoleEvidence`, after MR output add:

```go
ci := humanStatus(workspace.CIGate.Status)
if workspace.CIGate.Repo != "" && workspace.CIGate.PR != "" {
	ci = workspace.CIGate.Repo + "#" + workspace.CIGate.PR + " " + ci
}
fmt.Fprintf(stdout, "  %-14s %s\n", "ci", ci)
```

- [ ] **Step 5: Print CI in workbench snapshot**

In `internal/cli/workbench_snapshot.go`, after manual evidence line add:

```go
ci := humanStatus(detail.Workspace.CIGate.Status)
if detail.Workspace.CIGate.Repo != "" && detail.Workspace.CIGate.PR != "" {
	ci = detail.Workspace.CIGate.Repo + "#" + detail.Workspace.CIGate.PR + " " + ci
}
fmt.Fprintf(&builder, "  %-14s %s\n", "ci", ci)
```

- [ ] **Step 6: Run focused surface tests**

Run:

```powershell
go test ./internal/demandflow -run "Test.*Workspace.*CI|TestInspectWorkspace" -count=1
go test ./internal/cli -run "Test.*Console.*CI|Test.*Workbench.*CI|TestConsole|TestWorkbench" -count=1
```

Expected: pass. If the broad regex catches unrelated tests, run the exact new test names.

- [ ] **Step 7: Commit**

Run:

```powershell
git add internal/demandflow/workspace.go internal/demandflow/workspace_test.go internal/cli/console.go internal/cli/console_test.go internal/cli/workbench_snapshot.go internal/cli/workbench_test.go
git commit -m "Show GitHub CI gate status in operator surfaces" -m "Workspace summaries, console, and workbench snapshots now expose the latest CI gate event so operators can see why a demand is waiting." -m "Constraint: The display remains event-derived and read-only." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/demandflow -run Workspace -count=1; go test ./internal/cli -run Console -count=1"
```

## Task 7: Add Release Readiness And Docs

**Files:**

- Modify: `scripts/release-readiness.ps1`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/user-guide/dogfood-smoke.md`
- Modify: `docs/user-guide/live-dogfood.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Add release-readiness notes without live GitHub dependency**

Modify `scripts/release-readiness.ps1` to add a report section after the existing GitLab optional gate section:

```powershell
Add-Content -LiteralPath $report -Value "## github ci gate`nDefault readiness does not call live GitHub. Run devflow ci-gate --github-repo <owner/repo> --github-pr <number> with GITHUB_TOKEN for a live private check.`n"
```

Do not add a required live GitHub call to release readiness.

- [ ] **Step 2: Document direct CI gate usage**

In `docs/user-guide/backend-demand-loop.md`, add a GitHub CI section near the existing review-gate docs:

```markdown
### GitHub CI Gate

For repositories hosted on GitHub, check PR CI directly:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow ci-gate --github-repo "jesseedcp/devflow-agent" --github-pr "18"
```

To include CI status in the backend-demand `mr-review` stage:

```powershell
devflow run --demand add-coupon-eligibility-check --stage mr-review --gitlab-project "group/project" --gitlab-mr "123" --github-repo "jesseedcp/devflow-agent" --github-pr "18"
```

Wave 25 keeps GitLab review comments and GitHub CI as separate gates. If GitHub CI is pending or failing, the demand remains in `mr_review` and `verification` is not drafted.
```
```

- [ ] **Step 3: Update live dogfood docs**

In `docs/user-guide/live-dogfood.md`, add:

```markdown
Optional GitHub CI gate:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow ci-gate --github-repo "jesseedcp/devflow-agent" --github-pr "18"
```
```

- [ ] **Step 4: Update dogfood smoke docs**

In `docs/user-guide/dogfood-smoke.md`, add a sentence:

```markdown
The default deterministic dogfood path does not call GitHub. Use `devflow ci-gate` separately when you want to validate a real GitHub PR's CI status.
```

- [ ] **Step 5: Update release notes**

In `docs/release/v0.1.md`, add:

```markdown
- Adds `devflow ci-gate --github-repo <owner/repo> --github-pr <number>` for direct GitHub PR CI status checks.
- `devflow run --stage mr-review` can optionally include GitHub CI status through `--github-repo` and `--github-pr`; pending or failing CI blocks the move to verification.
```

- [ ] **Step 6: Run docs/readiness verification**

Run:

```powershell
go test ./internal/adapters ./internal/demandflow ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave25
```

Expected: all pass.

- [ ] **Step 7: Commit**

Run:

```powershell
git add scripts/release-readiness.ps1 docs/user-guide/backend-demand-loop.md docs/user-guide/dogfood-smoke.md docs/user-guide/live-dogfood.md docs/release/v0.1.md
git commit -m "Document GitHub CI gate workflow" -m "Update release readiness and user guidance so Wave 25's GitHub CI gate is discoverable without making live GitHub calls mandatory." -m "Constraint: Default release readiness must remain credential-free." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/adapters ./internal/demandflow ./internal/cli -count=1; go vet ./...; go build ./cmd/devflow; git diff --check; scripts/release-readiness.ps1 -Version 0.1.0-wave25"
```

## Task 8: Final Verification

**Files:**

- No planned source edits unless verification fails.

- [ ] **Step 1: Run full verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave25
```

Expected:

```text
all tests pass
go vet exits 0
go build exits 0
git diff --check exits 0
release readiness exits 0
```

- [ ] **Step 2: Manual local CLI smoke**

Run:

```powershell
.\devflow.exe ci-gate --github-repo owner/repo --github-pr 42 --github-base-url http://127.0.0.1:1
```

Expected: fails with a connection error. This only proves CLI routing; real GitHub is intentionally not required.

If `.\devflow.exe` does not exist, run:

```powershell
go run ./cmd/devflow ci-gate --github-repo owner/repo --github-pr 42 --github-base-url http://127.0.0.1:1
```

- [ ] **Step 3: Inspect final diff**

Run:

```powershell
git status --short
git log --oneline -8
```

Expected:

- Worktree may have tracked committed changes only; no uncommitted changes after final commit.
- Commits are small and match the task sequence.

- [ ] **Step 4: Open PR**

Run:

```powershell
git push -u origin wave25-github-pr-ci-gate
gh pr create --base main --head wave25-github-pr-ci-gate --title "Wave 25: GitHub PR CI gate" --body "Adds a GitHub PR CI gate, direct devflow ci-gate command, optional mr-review integration, operator surface summaries, and docs. Default tests and release readiness remain offline/deterministic."
```

Expected: GitHub returns a PR URL.

Do not merge until CI is green.

## Acceptance Criteria

- `devflow ci-gate --github-repo owner/repo --github-pr 123` exists.
- GitHub CI adapter reads PR head SHA and check runs using only Go standard library.
- Failed, pending, unknown, and zero-check cases block.
- Successful, neutral, and skipped completed checks pass.
- `devflow run --stage mr-review` can optionally include GitHub CI gate using `--github-repo` and `--github-pr`.
- CI gate evidence is appended to `progress.md`.
- `events.jsonl` records `ci_gate.passed` or `ci_gate.blocked`.
- Workspace, console, and workbench snapshot show the latest CI gate status.
- Default release readiness does not require `GITHUB_TOKEN` or live GitHub.
- GitLab review behavior remains working and unchanged except for optional GitHub CI composition.

## Known Risks

- GitHub branch protection can include required status contexts that do not appear as check runs in some legacy integrations. This wave uses check runs first; if the repo relies on legacy commit statuses, add a follow-up wave to query `GET /repos/{owner}/{repo}/commits/{ref}/status`.
- The existing workflow state is still named `mr_review`; this is intentionally left alone to keep Wave 25 small.
- The first integrated path still uses GitLab comments plus GitHub CI. A later wave should add a true GitHub review adapter if we want GitHub-only review gating.

## Recommended Follow-Up

- Wave 26: GitHub PR review adapter for unresolved PR review comments.
- Wave 27: Provider-neutral terminology migration from `MR` to `ChangeRequest`/`ReviewGate`.
- Wave 28: GitHub PR ensure/create adapter to replace GitLab MR creation in GitHub-hosted dogfood.

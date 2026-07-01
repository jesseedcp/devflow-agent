# Wave 25-28 GitHub Review Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish Wave 25, then move Devflow from GitLab-first MR review toward a GitHub-capable review and pull-request workflow without destabilizing the v0.1 backend-demand loop.

**Architecture:** Treat Wave 25 as the release gate, then ship Wave 26, Wave 27, and Wave 28 as separate worktrees and PRs. Wave 26 adds GitHub PR review comments through the existing `ReviewAdapter`; Wave 27 introduces provider-neutral naming while preserving compatibility; Wave 28 adds GitHub PR ensure/create support through the existing merge-request creation seam.

**Tech Stack:** Go standard library, existing `internal/adapters`, `internal/demandflow`, `internal/cli`, PowerShell release readiness, GitHub CLI for PR operations, no new dependencies.

---

## Scope And Sequencing

This is one roadmap plan, but it must execute as four separate delivery gates:

1. **Gate 0 / Wave 25 closeout:** merge PR #19 and verify `main`.
2. **Wave 26:** GitHub PR review adapter for unresolved PR review comments.
3. **Wave 27:** provider-neutral terminology migration from MR/PR-specific names to change-request/review-gate names.
4. **Wave 28:** GitHub PR ensure/create adapter and CLI integration.

Do not combine Wave 26, Wave 27, and Wave 28 into one PR. Each wave should produce working, testable software independently.

## Current Repository Facts

- Main repository: `D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent`
- Wave 25 worktree: `D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\wave25-github-pr-ci-gate`
- Wave 25 PR: <https://github.com/jesseedcp/devflow-agent/pull/19>
- Wave 25 status at plan time:
  - PR is open.
  - PR is mergeable.
  - Ubuntu CI passed.
  - Windows CI passed.
- Main currently tracks Wave 24.
- Main has one untracked plan file:
  - `docs/superpowers/plans/2026-07-01-wave-25-github-pr-ci-gate.md`

## File Structure

Wave 25 closeout:

- Modify only if needed:
  - `docs/superpowers/plans/2026-07-01-wave-25-github-pr-ci-gate.md`
- Verification:
  - `scripts/release-readiness.ps1`

Wave 26 GitHub review adapter:

- Create:
  - `internal/adapters/github_review.go`
  - `internal/adapters/github_review_test.go`
- Modify:
  - `internal/adapters/review.go`
  - `internal/cli/review_gate.go`
  - `internal/cli/review_gate_test.go`
  - `internal/cli/run.go`
  - `internal/cli/run_test.go`
  - `internal/cli/console.go`
  - `internal/cli/console_test.go`
  - `docs/user-guide/backend-demand-loop.md`
  - `docs/user-guide/live-dogfood.md`
  - `docs/release/v0.1.md`
  - `scripts/release-readiness.ps1`

Wave 27 terminology migration:

- Modify:
  - `internal/adapters/review.go`
  - `internal/adapters/merge_request.go`
  - `internal/demandflow/types.go`
  - `internal/demandflow/engine.go`
  - `internal/demandflow/workspace.go`
  - `internal/cli/cli.go`
  - `internal/cli/run.go`
  - `internal/cli/review_gate.go`
  - `internal/cli/mr.go`
  - `docs/user-guide/backend-demand-loop.md`
  - `docs/release/v0.1.md`
- Create only if compatibility aliases are cleaner than editing in place:
  - `internal/adapters/change_request.go`

Wave 28 GitHub PR ensure/create:

- Create:
  - `internal/adapters/github_pr.go`
  - `internal/adapters/github_pr_test.go`
- Modify:
  - `internal/adapters/merge_request.go`
  - `internal/cli/mr.go`
  - `internal/cli/run.go`
  - `internal/cli/cli.go`
  - `internal/cli/mr_test.go`
  - `internal/cli/run_test.go`
  - `internal/demandflow/engine.go`
  - `internal/demandflow/workspace.go`
  - `docs/user-guide/backend-demand-loop.md`
  - `docs/user-guide/live-dogfood.md`
  - `docs/release/v0.1.md`
  - `scripts/release-readiness.ps1`

---

## Gate 0: Merge Wave 25 And Verify Main

**Files:**
- Review: GitHub PR #19
- Modify, if preserving the plan file: `docs/superpowers/plans/2026-07-01-wave-25-github-pr-ci-gate.md`

- [ ] **Step 1: Confirm Wave 25 PR status**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\wave25-github-pr-ci-gate
gh pr view 19 --json number,state,mergeable,statusCheckRollup,url
```

Expected:

```text
state: OPEN
mergeable: MERGEABLE
all statusCheckRollup entries completed with conclusion SUCCESS
```

- [ ] **Step 2: Merge Wave 25**

Run:

```powershell
gh pr merge 19 --merge --delete-branch
```

Expected:

```text
PR #19 merged
remote branch wave25-github-pr-ci-gate deleted
```

If GitHub rejects merge because branch protection requires squash or rebase, run the merge mode required by the repository protection rule and record the exact mode in the final report.

- [ ] **Step 3: Update local main**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git checkout main
git pull --ff-only origin main
git status --short --branch
```

Expected:

```text
## main...origin/main
```

Only known untracked plan files may remain.

- [ ] **Step 4: Decide the Wave 25 plan file**

Run:

```powershell
git status --short
```

If `docs/superpowers/plans/2026-07-01-wave-25-github-pr-ci-gate.md` is still untracked and should be preserved, commit it:

```powershell
git add docs/superpowers/plans/2026-07-01-wave-25-github-pr-ci-gate.md
git commit -m "Record Wave 25 implementation plan" -m "Keep the GitHub CI gate execution plan in the project history now that Wave 25 is merged." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: documentation-only change"
```

If the plan file is only a local scratch copy, remove it:

```powershell
Remove-Item -LiteralPath docs\superpowers\plans\2026-07-01-wave-25-github-pr-ci-gate.md
```

- [ ] **Step 5: Verify main after Wave 25**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave25-main
```

Expected:

```text
all commands exit 0
release-readiness report includes the github ci gate section
```

---

## Wave 26: GitHub PR Review Adapter

**Goal:** Let GitHub-hosted projects use `review-gate` and `run --stage mr-review` without GitLab review comments.

**Branch:** `wave26-github-pr-review-adapter`

**Files:**
- Create: `internal/adapters/github_review.go`
- Create: `internal/adapters/github_review_test.go`
- Modify: `internal/adapters/review.go`
- Modify: `internal/cli/review_gate.go`
- Modify: `internal/cli/review_gate_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Create isolated worktree**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git worktree add .worktrees/wave26-github-pr-review-adapter -b wave26-github-pr-review-adapter origin/main
cd .worktrees/wave26-github-pr-review-adapter
```

Expected:

```text
branch wave26-github-pr-review-adapter checked out from origin/main
```

- [ ] **Step 2: Extend review ref without breaking GitLab**

Modify `internal/adapters/review.go` so `ReviewRef` can carry provider-neutral and GitHub names while preserving old GitLab fields:

```go
type ReviewRef struct {
	Provider     string
	Project      string
	MergeRequest string
	Repo         string
	PullRequest  string
	BaseURL      string
	Token        string
}
```

Keep existing `Project` and `MergeRequest` fields because `GitLabReviewAdapter` already depends on them.

- [ ] **Step 3: Write failing GitHub adapter tests**

Create `internal/adapters/github_review_test.go` with tests that cover:

```text
1. unresolved review thread becomes a blocking ReviewComment
2. resolved thread is ignored
3. file path and line are copied from GraphQL location data
4. missing token returns a clear github token required error
5. non-200 or GraphQL errors return a clear adapter error
```

Use `httptest.NewServer` and a fake GraphQL response. The first test should assert:

```go
if len(comments) != 1 {
	t.Fatalf("len(comments) = %d, want 1", len(comments))
}
if comments[0].Blocking != true {
	t.Fatalf("Blocking = false, want true")
}
if comments[0].Category == "" {
	t.Fatalf("Category is empty")
}
```

Run:

```powershell
go test ./internal/adapters -run TestGitHubReview -count=1
```

Expected:

```text
FAIL: GitHubReviewAdapter undefined
```

- [ ] **Step 4: Implement GitHub review adapter**

Create `internal/adapters/github_review.go`.

Implementation requirements:

```text
Adapter name: GitHubReviewAdapter
Interface: ReviewAdapter
Auth: ReviewRef.Token or GITHUB_TOKEN
Default API base: https://api.github.com/graphql
Primary API: GitHub GraphQL PullRequest.reviewThreads
Only include threads where isResolved == false
Use first comment body/author/location for ReviewComment
ID format: threadID:commentID
Blocking: true
Category: ClassifyReviewComment(body, filePath)
Reply: return a clear unsupported error for Wave 26
```

Recommended public methods:

```go
type GitHubReviewAdapter struct {
	Client *http.Client
}

func (a GitHubReviewAdapter) ListUnresolved(ctx context.Context, ref ReviewRef) ([]ReviewComment, error)
func (a GitHubReviewAdapter) Reply(ctx context.Context, ref ReviewRef, commentID string, body string) error
```

The Wave 26 `Reply` implementation should return:

```go
return fmt.Errorf("github review replies are not implemented in Wave 26")
```

- [ ] **Step 5: Verify adapter package**

Run:

```powershell
gofmt -w internal/adapters/review.go internal/adapters/github_review.go internal/adapters/github_review_test.go
go test ./internal/adapters -count=1
```

Expected:

```text
ok github.com/jesseedcp/devflow-agent/internal/adapters
```

- [ ] **Step 6: Commit adapter**

Run:

```powershell
git add internal/adapters/review.go internal/adapters/github_review.go internal/adapters/github_review_test.go
git commit -m "Read unresolved GitHub PR review threads" -m "GitHub-hosted projects need review comment gating without depending on GitLab discussions. The adapter maps unresolved GitHub review threads into the existing ReviewComment contract." -m "Constraint: Keep GitLab ReviewRef fields and adapter behavior backward compatible." -m "Rejected: Rename mr-review in this wave | terminology migration is Wave 27." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/adapters -count=1"
```

- [ ] **Step 7: Add GitHub flags to `review-gate`**

Modify `internal/cli/review_gate.go`:

```text
Add flags:
--github-repo <owner/repo>
--github-pr <number>
--github-base-url <url>

Validation:
- GitLab mode requires --gitlab-project and --gitlab-mr.
- GitHub mode requires --github-repo and --github-pr.
- Supplying both GitLab and GitHub review targets is an error.

Adapter selection:
- GitLab mode uses newReviewAdapter().
- GitHub mode uses newGitHubReviewAdapter().
```

Add package variable in `internal/cli/run.go` or `internal/cli/review_gate.go`:

```go
var newGitHubReviewAdapter = func() adapters.ReviewAdapter {
	return adapters.GitHubReviewAdapter{}
}
```

- [ ] **Step 8: Add `review-gate` CLI tests**

Modify `internal/cli/review_gate_test.go` to test:

```text
1. --github-repo/--github-pr uses GitHub adapter
2. no unresolved comments prints a passed message
3. unresolved comments print blocked message
4. both GitLab and GitHub flags return an error
5. neither target returns a useful validation error
```

Run:

```powershell
go test ./internal/cli -run TestReviewGate -count=1
```

Expected:

```text
PASS
```

- [ ] **Step 9: Add GitHub-only `mr-review` integration**

Modify `internal/cli/run.go` so `StageMRReview` accepts either:

```text
GitLab review:
  --gitlab-project + --gitlab-mr

GitHub review:
  --github-repo + --github-pr

GitHub CI-only composition with GitLab review:
  --gitlab-project + --gitlab-mr + --github-repo + --github-pr
```

Because Wave 25 already uses `--github-repo/--github-pr` for CI gate, Wave 26 needs one explicit review selector to avoid ambiguity. Add:

```text
--review-provider <gitlab|github>
```

Rules:

```text
default review provider = gitlab when gitlab flags are present
default review provider = github when github flags are present and gitlab flags are absent
if --review-provider github, GitHub flags provide Review.Ref and also CIGate.Ref when CI gate is enabled
```

Keep CI gating optional. Do not require GitHub CI for GitHub review.

- [ ] **Step 10: Add run-stage tests**

Modify `internal/cli/run_test.go` to cover:

```text
1. GitHub review comments clear -> state advances to verification
2. GitHub blocking comments -> state stays/routes like existing review action plan
3. GitHub review + GitHub CI pending -> review clear but CI blocks verification
```

Run:

```powershell
go test ./internal/cli -run "TestRun.*GitHub.*Review|TestRun.*MRReview" -count=1
go test ./internal/demandflow -run TestEngineMRReview -count=1
```

Expected:

```text
PASS
```

- [ ] **Step 11: Update docs and release notes**

Modify `docs/user-guide/backend-demand-loop.md`:

```markdown
For GitHub PR review comments:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow review-gate --github-repo "jesseedcp/devflow-agent" --github-pr "19"
devflow run --demand add-coupon-eligibility-check --stage mr-review --review-provider github --github-repo "jesseedcp/devflow-agent" --github-pr "19"
```
```

Modify `docs/release/v0.1.md`:

```markdown
### Wave 26 - GitHub PR Review Gate

- Adds GitHub PR review-thread support to `review-gate` and `run --stage mr-review`.
- GitHub unresolved review threads now map into the same review action plan as GitLab comments.
```

- [ ] **Step 12: Final Wave 26 verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave26
```

Expected:

```text
all commands exit 0
```

- [ ] **Step 13: Push and open Wave 26 PR**

Run:

```powershell
git push -u origin wave26-github-pr-review-adapter
gh pr create --base main --head wave26-github-pr-review-adapter --title "Wave 26: GitHub PR review adapter" --body "Adds GitHub unresolved PR review-thread support to review-gate and mr-review. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave26."
```

Expected:

```text
GitHub returns a PR URL
```

---

## Wave 27: Provider-Neutral Review Terminology

**Goal:** Reduce GitLab-specific terminology in public-facing code paths while preserving existing `mr-review` and `mr` commands as compatibility aliases.

**Branch:** `wave27-change-request-terminology`

**Files:**
- Modify: `internal/adapters/review.go`
- Modify: `internal/adapters/merge_request.go`
- Modify: `internal/demandflow/types.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `internal/demandflow/workspace.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/review_gate.go`
- Modify: `internal/cli/mr.go`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Create isolated worktree after Wave 26 merges**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git worktree add .worktrees/wave27-change-request-terminology -b wave27-change-request-terminology origin/main
cd .worktrees/wave27-change-request-terminology
```

Expected:

```text
branch wave27-change-request-terminology checked out from origin/main
```

- [ ] **Step 2: Introduce provider-neutral adapter names**

Modify `internal/adapters/merge_request.go` to add aliases without deleting old names:

```go
type ChangeRequestSpec = MergeRequestSpec
type ChangeRequestResult = MergeRequestResult
type ChangeRequestAdapter = MergeRequestAdapter
```

Do not remove `MergeRequestSpec`, `MergeRequestResult`, or `MergeRequestAdapter` in Wave 27.

- [ ] **Step 3: Introduce provider-neutral demandflow option names**

Modify `internal/demandflow/types.go`:

```go
type ChangeRequestOptions = MergeRequestOptions
```

Add a new field to `Options`:

```go
ChangeRequest ChangeRequestOptions
```

Keep:

```go
MergeRequest MergeRequestOptions
```

Modify engine option normalization so `Options.ChangeRequest` wins when set, otherwise falls back to `Options.MergeRequest`.

- [ ] **Step 4: Add compatibility tests**

Modify `internal/demandflow/engine_test.go` or the existing MR review tests:

```text
1. old MergeRequest option still syncs successfully
2. new ChangeRequest option syncs successfully
3. if both are set, ChangeRequest is used
```

Run:

```powershell
go test ./internal/demandflow -run "Test.*ChangeRequest|Test.*MergeRequest" -count=1
```

Expected:

```text
PASS
```

- [ ] **Step 5: Add CLI alias commands without removing old commands**

Modify `internal/cli/cli.go` help:

```text
devflow review-gate ...
devflow change-request ensure ...
devflow mr ensure ...
```

Add dispatch:

```go
case "change-request", "cr":
	return runChangeRequest(args[1:], stdout, stderr)
```

Implement `runChangeRequest` as a wrapper around the existing `runMR` implementation for Wave 27:

```go
func runChangeRequest(args []string, stdout io.Writer, stderr io.Writer) error {
	return runMR(args, stdout, stderr)
}
```

The old `mr` command remains supported.

- [ ] **Step 6: Rename user-facing docs**

Modify docs so new guidance says:

```text
change request review gate
GitLab MR and GitHub PR are providers
mr-review remains the v0.1 workflow stage name for compatibility
```

Do not rename the workflow state in Wave 27 unless every existing test is updated and compatibility is preserved.

- [ ] **Step 7: Final Wave 27 verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave27
```

Expected:

```text
all commands exit 0
```

- [ ] **Step 8: Push and open Wave 27 PR**

Run:

```powershell
git push -u origin wave27-change-request-terminology
gh pr create --base main --head wave27-change-request-terminology --title "Wave 27: provider-neutral review terminology" --body "Adds change-request terminology and CLI aliases while preserving mr-review and mr compatibility. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave27."
```

---

## Wave 28: GitHub PR Ensure/Create Adapter

**Goal:** Let GitHub-hosted projects create or reuse pull requests through the same implementation-stage change-request sync path that GitLab uses.

**Branch:** `wave28-github-pr-ensure-create`

**Files:**
- Create: `internal/adapters/github_pr.go`
- Create: `internal/adapters/github_pr_test.go`
- Modify: `internal/adapters/merge_request.go`
- Modify: `internal/cli/mr.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/mr_test.go`
- Modify: `internal/cli/run_test.go`
- Modify: `internal/demandflow/engine.go`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Create isolated worktree after Wave 27 merges**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git worktree add .worktrees/wave28-github-pr-ensure-create -b wave28-github-pr-ensure-create origin/main
cd .worktrees/wave28-github-pr-ensure-create
```

Expected:

```text
branch wave28-github-pr-ensure-create checked out from origin/main
```

- [ ] **Step 2: Extend create spec for GitHub**

Modify `internal/adapters/merge_request.go`:

```go
type MergeRequestSpec struct {
	Provider     string
	Project      string
	Repo         string
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
	BaseURL      string
	Token        string
}
```

Keep GitLab behavior using `Project`.

For GitHub:

```text
Repo must be owner/repo
SourceBranch maps to head
TargetBranch maps to base
Title maps to title
Description maps to body
```

- [ ] **Step 3: Write failing GitHub PR adapter tests**

Create `internal/adapters/github_pr_test.go` covering:

```text
1. reuses existing open PR with matching head/base
2. creates PR when no matching open PR exists
3. validates owner/repo, source branch, target branch, and title
4. reads token from spec.Token or GITHUB_TOKEN
5. surfaces GitHub API error bodies
```

Run:

```powershell
go test ./internal/adapters -run TestGitHubPREnsure -count=1
```

Expected:

```text
FAIL: GitHubPullRequestAdapter undefined
```

- [ ] **Step 4: Implement GitHub PR adapter**

Create `internal/adapters/github_pr.go`.

Public type:

```go
type GitHubPullRequestAdapter struct {
	Client *http.Client
}
```

Method:

```go
func (a GitHubPullRequestAdapter) EnsureMergeRequest(ctx context.Context, spec MergeRequestSpec) (MergeRequestResult, error)
```

API behavior:

```text
GET  /repos/{owner}/{repo}/pulls?state=open&head={owner}:{source}&base={target}
POST /repos/{owner}/{repo}/pulls
```

Map GitHub PR number into `MergeRequestResult.IID`.

Set:

```text
WebURL = html_url
State = state
WasCreated = false for existing, true for created
```

- [ ] **Step 5: Verify adapter package**

Run:

```powershell
gofmt -w internal/adapters/merge_request.go internal/adapters/github_pr.go internal/adapters/github_pr_test.go
go test ./internal/adapters -count=1
```

Expected:

```text
PASS
```

- [ ] **Step 6: Commit adapter**

Run:

```powershell
git add internal/adapters/merge_request.go internal/adapters/github_pr.go internal/adapters/github_pr_test.go
git commit -m "Create or reuse GitHub pull requests" -m "GitHub-hosted backend demand flows need the same change-request sync support that GitLab merge requests already have." -m "Constraint: Keep GitLab merge request behavior backward compatible." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/adapters -count=1"
```

- [ ] **Step 7: Add CLI provider selection for ensure/create**

Modify `internal/cli/mr.go`:

```text
Add flags:
--provider <gitlab|github>
--github-repo <owner/repo>
--github-base-url <url>

Existing GitLab flags remain:
--gitlab-project
--gitlab-base-url
```

Rules:

```text
provider defaults to gitlab when --gitlab-project is present
provider defaults to github when --github-repo is present and --gitlab-project is absent
github requires --github-repo, --source-branch, --target-branch, --title
gitlab requires --gitlab-project, --source-branch, --target-branch, --title
```

Modify `newMergeRequestAdapter` selection so GitHub mode uses:

```go
adapters.GitHubPullRequestAdapter{}
```

- [ ] **Step 8: Add implementation-stage GitHub PR create flags**

Modify `internal/cli/run.go`:

```text
Add:
--create-change-request
--change-request-provider <gitlab|github>
--github-repo <owner/repo>

Keep old --create-mr and --gitlab-project flags as aliases.
```

Implementation stage rules:

```text
--create-mr continues to mean GitLab unless --change-request-provider github is supplied
--create-change-request is provider-neutral and recommended in help text
```

- [ ] **Step 9: Add CLI tests**

Modify `internal/cli/mr_test.go` and `internal/cli/run_test.go` to cover:

```text
1. devflow mr ensure --provider github creates/reuses PR
2. devflow change-request ensure --provider github creates/reuses PR
3. devflow run --stage implementation --create-change-request --change-request-provider github records change-request evidence
4. old GitLab mr ensure test still passes
```

Run:

```powershell
go test ./internal/cli -run "TestMR|TestChangeRequest|TestRun.*ChangeRequest|TestRun.*MergeRequest" -count=1
```

Expected:

```text
PASS
```

- [ ] **Step 10: Update docs and release notes**

Modify `docs/user-guide/backend-demand-loop.md`:

```markdown
For GitHub pull request creation:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow change-request ensure --provider github --github-repo "jesseedcp/devflow-agent" --source-branch "feature/coupon" --target-branch "main" --title "Implement coupon eligibility"
```
```

Modify `docs/release/v0.1.md`:

```markdown
### Wave 28 - GitHub Pull Request Creation

- Adds GitHub PR ensure/create support through the provider-neutral change-request path.
- Keeps GitLab MR commands as compatibility aliases.
```

- [ ] **Step 11: Final Wave 28 verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave28
```

Expected:

```text
all commands exit 0
```

- [ ] **Step 12: Push and open Wave 28 PR**

Run:

```powershell
git push -u origin wave28-github-pr-ensure-create
gh pr create --base main --head wave28-github-pr-ensure-create --title "Wave 28: GitHub PR ensure and create" --body "Adds GitHub pull request ensure/create support through the provider-neutral change-request flow while preserving GitLab MR compatibility. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave28."
```

---

## Final Acceptance Criteria

- Wave 25 is merged into `main`.
- `main` passes:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0
```

- GitHub PR review comments can block or clear `mr-review`.
- GitHub CI gate and GitHub review gate can be used together.
- GitLab review and MR creation still work.
- Provider-neutral change-request terminology exists in CLI/docs while old `mr`/`mr-review` compatibility remains.
- GitHub PR ensure/create works without requiring GitLab settings.
- Default release readiness remains deterministic and does not require live GitHub credentials.

## Self-Review

- **Spec coverage:** The five requested items are covered: Wave 25 merge, main release readiness, Wave 26 GitHub review adapter, Wave 27 terminology migration, and Wave 28 GitHub PR ensure/create.
- **Placeholder scan:** The plan avoids TBD markers and gives concrete commands, branches, file paths, and acceptance criteria.
- **Type consistency:** Wave 26 extends `ReviewRef`; Wave 27 aliases `MergeRequest*` to `ChangeRequest*`; Wave 28 extends `MergeRequestSpec` rather than inventing a competing result model.
- **Risk control:** Each wave has its own worktree and PR so failures do not contaminate the v0.1 mainline.


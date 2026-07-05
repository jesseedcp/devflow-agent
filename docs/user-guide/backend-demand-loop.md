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

## 2. Create A Demand From A Local PRD

For a PRD or rough需求文档, prefer intake:

```powershell
devflow intake --file docs/examples/demands/coupon-eligibility.md
```

For an accessible HTTP(S) PRD page, use URL intake:

```powershell
devflow intake --url https://example.com/product/coupon-eligibility-prd
```

`intake` creates a demand workspace, stores the original material in `intake.md`, renders review-ready `requirements.md`, and stops at `requirements_review`. It does not confirm the requirements.
`intake` also writes `context.md`. This file is the reusable-memory snapshot for the demand. It lists approved stable memory separately from historical demand candidates, because candidate memory is useful context but not approved truth.

URL intake fetches public or otherwise directly accessible HTTP(S) pages and normalizes static HTML into text. It does not bypass login, enterprise permissions, WeChat anti-scraping, or JavaScript-only rendering. For those sources, export the PRD to a local file and use `--file`.

Rebuild the context snapshot after promoting or rejecting memory:

```powershell
devflow recall --demand coupon-eligibility
```

After intake, inspect deterministic quality signals:

```powershell
devflow evaluate --demand coupon-eligibility --stage requirements --strict
devflow console --demand coupon-eligibility
```

If you do not have a PRD file yet, create a manual demand:

```powershell
devflow start --title "Add coupon eligibility check" --description "Only active members can claim coupons"
devflow run --demand add-coupon-eligibility-check --stage requirements
```

## 3. Check Status

```powershell
devflow status --demand add-coupon-eligibility-check
devflow next --demand add-coupon-eligibility-check
```


### Demand Workspace Status

Use `devflow status` as the operator checkpoint before deciding the next command.

```powershell
devflow status --demand add-coupon-check
devflow next --demand add-coupon-check
devflow status --all
```

`status --demand` reads only local demand materials under `.devflow/demands/<id>` and summarizes:

- workflow state from `demand.json`;
- confirmation evidence from `events.jsonl`;
- artifact state for requirements, plan, progress, verification, closeout, memory candidates, and events;
- local MR review evidence from events and `progress.md`;
- latest verification PASS/FAIL evidence;
- stable memory candidate counts;
- the recommended next command.

`status --all` scans `.devflow/demands` and sorts demands that need attention ahead of completed work. It does not call GitLab and does not mutate any artifact.

### Operator console

Use `devflow console` when you want an operator view rather than a material audit.

```powershell
devflow console
devflow console --demand add-coupon-check
devflow console --demand add-coupon-check --run-next
```

`console` is built on the same local workspace evidence as `status`, but it separates the recommended action from the run-ready action. `--run-next` only executes runner-safe agent stages such as requirements, plan, implementation, verification, and closeout. It does not auto-confirm human gates, promote memory, reject memory, or merge MRs.

### Guided drive

Use `devflow drive` to run safe agent stages until the next manual gate.

```powershell
devflow drive --demand add-coupon-check
devflow drive --demand add-coupon-check --dry-run
```

Drive never confirms stages, promotes memory, rejects memory, or merges MRs. It stops with an explicit reason when the next step needs a human.


### Codemap Context

`devflow codemap index` builds a local Go code-fact index under `.devflow/codemap/`.
`devflow codemap search <query>` searches functions, methods, types, tests, and route-like strings.
`devflow codemap refresh --demand <id> --query <text>` writes demand-level `codemap.md`.

Codemap is intentionally separate from `context.md`: `context.md` is memory recall, while `codemap.md` is code evidence for planning.
If `devflow codemap search` or `devflow codemap refresh` says the index is missing, run `devflow codemap index` first, then retry the search or refresh command.


### Plan Grounding

After refreshing codemap context, run:

```powershell
devflow plan-context refresh --demand <id>
devflow scope declare --demand <id> --source internal/foo/service.go --test internal/foo/service_test.go
```

`plan-context.md` is the packet used for writing a grounded implementation plan.
`change-scope.md` records which source and test files are expected to change.
`evaluate --stage plan` warns if the plan does not reference the grounded context or declared files.

If `plan-context refresh` reports a missing codemap, run:

```powershell
devflow codemap index
devflow codemap refresh --demand <id> --query "<demand keywords>"
devflow plan-context refresh --demand <id>
```

### Deterministic stage evaluation

Use `devflow evaluate` to inspect structural quality signals before confirming stage outputs.

```powershell
devflow evaluate --demand add-coupon-check
devflow evaluate --demand add-coupon-check --stage requirements --strict
```

Evaluation is deterministic local checking, not semantic LLM review. It reports missing sections, verification evidence, and closeout memory-candidate signals without mutating demand state.

For requirements, evaluation also checks intake/context alignment:

- `requirements.intake_coverage` warns when concrete intake bullets are missing from `requirements.md`.
- `requirements.context_presence` warns when `context.md` is missing or not recalled.
- `requirements.stable_memory_reference` warns when approved memory exists but is not reflected in requirements.
- `requirements.candidate_guard` warns when historical candidate memory exists but requirements have no useful confirmation question.

These checks are deterministic signals for human review. They do not approve requirements automatically.

### Workbench TUI

Use `devflow workbench` for an interactive demand list and selected-demand operator view.

Console and workbench snapshot views surface non-passing requirements checks under `Quality`, so the operator can fix intake/context alignment before confirming requirements.

### Operator dogfood

Use operator dogfood before relying on the workflow for real delivery. It runs the deterministic backend-demand loop while collecting console, drive, evaluate, and workbench evidence.

```powershell
devflow dogfood --operator-loop
```

The command writes `operator-dogfood-report.md` under the generated demand directory. The report is the quickest way to inspect whether the operator-facing loop is still coherent after changes.

### Backend demand defaults

Put repeated operator flags in `.devflow/config.yaml`:

```yaml
backend_demand:
  quality_commands:
    - go test ./... -count=1 -timeout 5m
  permission_mode: acceptEdits
  gitlab:
    project: group/project
    default_target_branch: main
```

Explicit CLI flags override these defaults.

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

To automatically create or reuse a GitLab merge request during implementation:

```powershell
$env:GITLAB_TOKEN = '<your-token>'
devflow run --demand add-coupon-eligibility-check --stage implementation --permission-mode acceptEdits --quality-command "go test ./..." --create-mr-source-branch "feature/your-branch" --create-mr-target-branch "main" --create-mr-title "Implement coupon eligibility"
```

If the quality gate fails, fix the reported problem and rerun the same implementation command.

### Change Request Terminology

GitLab merge requests (MRs) and GitHub pull requests (PRs) are both *change requests*. Devflow exposes provider-neutral vocabulary while keeping the legacy commands:

- `devflow change-request ensure` (alias `cr`) is the provider-neutral entry point; `devflow mr ensure` remains a supported alias.
- The workflow stage name `mr-review` is unchanged in v0.1 for compatibility.
- GitLab MR and GitHub PR are providers behind the same change-request path.

For GitHub pull request creation:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow change-request ensure --provider github --github-repo "jesseedcp/devflow-agent" --source-branch "feature/coupon" --target-branch "main" --title "Implement coupon eligibility"
```

During implementation you can create or reuse a GitHub PR with the provider-neutral flags:

```powershell
devflow run --demand add-coupon-eligibility-check --stage implementation --create-change-request --change-request-provider github --github-repo "jesseedcp/devflow-agent" --create-mr-source-branch "feature/coupon" --create-mr-target-branch "main" --create-mr-title "Implement coupon eligibility"
```

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

### GitHub PR Review Gate

For GitHub-hosted pull requests, gate on unresolved PR review threads instead of GitLab discussions:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow review-gate --github-repo "jesseedcp/devflow-agent" --github-pr "19"
devflow run --demand add-coupon-eligibility-check --stage mr-review --review-provider github --github-repo "jesseedcp/devflow-agent" --github-pr "19"
```

Use `--review-provider github` to select GitHub review threads; GitLab remains the default when GitLab flags are present. Add `--github-ci` to also enforce the GitHub CI gate during a GitHub review.
## 8. Run Verification And Closeout

```powershell
devflow run --demand add-coupon-eligibility-check --stage verification --quality-command "go test ./..."
devflow confirm --demand add-coupon-eligibility-check --stage verification --by dd --summary "verification passed"
devflow run --demand add-coupon-eligibility-check --stage closeout
devflow confirm --demand add-coupon-eligibility-check --stage closeout --by dd --summary "closeout accepted"
```

Record acceptance evidence while the demand is in `verification`:

```powershell
devflow evidence fetch --demand add-coupon-eligibility-check `
  --type api `
  --criterion "Inactive users are blocked" `
  --method POST `
  --url "https://api.example.test/coupon/claim" `
  --expect-status 403 `
  --expect-contains COUPON_USER_INACTIVE

devflow evidence fetch --demand add-coupon-eligibility-check `
  --type link `
  --criterion "QA report is reachable" `
  --url "https://example.test/report/123"

devflow evidence add --demand add-coupon-eligibility-check `
  --type manual `
  --criterion "QA accepted inactive-user blocking" `
  --summary "QA signed off the inactive-user scenario." `
  --link "https://example.test/log/123" `
  --by dd

devflow evidence list --demand add-coupon-eligibility-check
```

Acceptance evidence is local and reviewable. v0.3 can fetch HTTP/API and link reachability evidence, redacts common secrets before writing `verification.md` or `events.jsonl`, and keeps `evidence add` for manual records. Evidence does not auto-confirm verification.

### Release Control Loop

After verification is confirmed, Devflow enters `deployment` instead of `closeout`.

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow deploy trigger --demand add-coupon-eligibility-check --provider github --github-repo "owner/repo" --workflow "release.yml" --ref "main"
devflow deploy status --demand add-coupon-eligibility-check --provider github --github-repo "owner/repo" --workflow "release.yml" --ref "main"
devflow observe refresh --demand add-coupon-eligibility-check
```

If deployment or observation fails, record a release decision:

```powershell
devflow rollback plan --demand add-coupon-eligibility-check --trigger "deployment failed" --impact "release blocked" --recommendation "redeploy after fix"
devflow rollback confirm --demand add-coupon-eligibility-check --decision redeploy_required --by "<name>" --summary "<summary>"
```

`v0.9.0` records rollback decisions only. It does not execute rollback.

### Implementation Review

After implementation and verification evidence exist, run:

```powershell
devflow implementation-review refresh --demand add-coupon-eligibility-check
devflow implementation-review status --demand add-coupon-eligibility-check
devflow evaluate --demand add-coupon-eligibility-check --stage implementation
```

`implementation-review.md` summarizes declared scope, actual changed files, verification, acceptance evidence, and review status. It does not edit code or advance workflow state.

### Stable knowledge review

After closeout, Devflow writes reviewable knowledge candidates to `memory-candidates.md`. These are not stable memory until a human promotes them.

List candidates:

```powershell
devflow memory list --demand add-coupon-check
```

Promote an approved candidate into project memory:

```powershell
devflow memory promote --demand add-coupon-check --candidate 1 --name coupon-eligibility-policy --description "membership gates coupon eligibility" --by dd
```

Reject a candidate that is too narrow or stale:

```powershell
devflow memory reject --demand add-coupon-check --candidate 2 --by dd --reason "too specific to one fixture"
```

Promoted memories are stored under `.devflow/memory/` and indexed in `.devflow/memory/MEMORY.md`. Future requirements and plan stages render approved stable memory before unapproved candidate memory.

### Wiki knowledge distillation

After closeout, distill the closeout material into reviewable wiki candidates:

```powershell
devflow wiki distill --demand add-coupon-check
```

`wiki distill` reads `closeout.md`, `memory-candidates.md`, `implementation-review.md`, and `events.jsonl`. It writes `closeout-raw-log.md` (a consolidated archive of the raw inputs) and `wiki-candidates.md` (reviewable candidates grouped into stable business knowledge, process improvement, and archive-only sections). Distillation is deterministic text extraction; it does not call an LLM.

List the distilled candidates:

```powershell
devflow wiki list --demand add-coupon-check
```

Promote an approved candidate into the local project wiki:

```powershell
devflow wiki promote --demand add-coupon-check --candidate 1 --name coupon-membership-rule --by dd
```

Reject a candidate that should not enter the long-term wiki:

```powershell
devflow wiki reject --demand add-coupon-check --candidate 2 --by dd --reason "too specific to one fixture"
```

Search promoted wiki entries:

```powershell
devflow wiki search coupon
```

Promoted wiki entries live under `.devflow/wiki/<slug>.md` and are indexed in `.devflow/wiki/WIKI.md`. Slug names allow only lowercase letters, digits, hyphens, and underscores. Promotion and rejection are human gates; `--run-next` never auto-promotes or auto-rejects wiki candidates. No external wiki provider (Feishu, GitHub Wiki, Notion, Confluence) is written to.

`evaluate --stage closeout` surfaces two warning-only checks: `closeout.wiki_candidates` (candidates present) and `closeout.wiki_decisions` (all candidates promoted or rejected). These warnings do not block closeout completion in v0.7.0.

## 9. Diagnostics

```powershell
devflow doctor
```

The doctor command reports whether config, git, and GitLab token setup are ready without printing secret values.

## 10. Platform Intake

GitHub Issue intake:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow intake --github-issue owner/repo#123
```

Feishu Doc intake:

```powershell
$env:FEISHU_APP_ID = "<app id>"
$env:FEISHU_APP_SECRET = "<app secret>"
devflow intake --feishu-doc <doc-url-or-token>
```

Feishu Bitable pool:

```powershell
devflow pool list --feishu-bitable <app-token> --table <table-id>
devflow intake --feishu-bitable <app-token> --table <table-id> --record <record-id>
```

Platform sync:

```powershell
devflow sync --github-issue owner/repo#123 --demand <id>
devflow sync --feishu-bitable <app-token> --table <table-id> --record <record-id> --demand <id>
```

Platform doctor:

```powershell
devflow doctor --platform all
```

Doctor reports each platform credential as `set` or `not set` without printing secret values.

## Metrics

After a demand has accumulated events, run:

```powershell
devflow metrics report --demand coupon-metrics
```

The report is deterministic and local. It reads `events.jsonl` and writes `metrics.md`.

Tracked signals include:

- human confirmations
- review returns
- verification runs and pass rate
- acceptance evidence pass/fail/blocked
- wiki candidate promote/reject decisions
- blocked events and CI gate outcomes

Metrics are advisory in v0.8. They do not block closeout automatically.

Metrics can include partial historical data caveats. Older demands may have been created before verification evidence, acceptance evidence, wiki decisions, or metrics events existed. In that case `metrics.md` marks the demand as partial instead of pretending the missing signals were zero-effort or zero-risk outcomes.

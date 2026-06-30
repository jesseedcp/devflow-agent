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

## 8. Run Verification And Closeout

```powershell
devflow run --demand add-coupon-eligibility-check --stage verification --quality-command "go test ./..."
devflow confirm --demand add-coupon-eligibility-check --stage verification --by dd --summary "verification passed"
devflow run --demand add-coupon-eligibility-check --stage closeout
devflow confirm --demand add-coupon-eligibility-check --stage closeout --by dd --summary "closeout accepted"
```


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
## 9. Diagnostics

```powershell
devflow doctor
```

The doctor command reports whether config, git, and GitLab token setup are ready without printing secret values.

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

## 9. Diagnostics

```powershell
devflow doctor
```

The doctor command reports whether config, git, and GitLab token setup are ready without printing secret values.

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

## Merge Request Sync

Live dogfood creates or reuses an offline merge request during the implementation stage by default. The merge request evidence (IID, URL, state, action) is recorded in `progress.md` alongside quality results and implementation summaries.

For production GitLab projects, use `devflow mr ensure` or `devflow run --stage implementation --create-mr-*` with a valid `GITLAB_TOKEN`.


## Optional GitHub CI Gate

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow ci-gate --github-repo "jesseedcp/devflow-agent" --github-pr "18"
```

## Optional Live Release Control

Live release control is opt-in. It dispatches a real GitHub Actions workflow and is not part of the default release-readiness gate. Run it only against a demand in the `deployment` state after verification is confirmed:

```powershell
$env:GITHUB_TOKEN = "<github token>"
devflow deploy trigger --demand add-coupon-eligibility-check --provider github --github-repo "owner/repo" --workflow "release.yml" --ref "main"
devflow deploy status --demand add-coupon-eligibility-check --provider github --github-repo "owner/repo" --workflow "release.yml" --ref "main"
devflow observe refresh --demand add-coupon-eligibility-check
```

If deployment or observation fails, record a rollback decision:

```powershell
devflow rollback plan --demand add-coupon-eligibility-check --trigger "deployment failed" --impact "release blocked" --recommendation "redeploy after fix"
devflow rollback confirm --demand add-coupon-eligibility-check --decision risk_accepted --by "<name>" --summary "<summary>"
```

v0.9.0 records rollback decisions only; it does not execute rollback. The default release-readiness gate covers release control through a local fake GitHub API and does not require a token.


## Live GitHub Actions Release Dogfood

This dogfood dispatches a real GitHub Actions workflow. Use a disposable workflow or the safe `release.yml` marker workflow. Required:

- `GITHUB_TOKEN` or `gh auth token` with workflow dispatch access.
- A demand in `deployment` state.
- A workflow with `workflow_dispatch`.

Recommended command:

```powershell
devflow deploy trigger `
  --demand release-workflow-dogfood `
  --provider github `
  --github-repo "jesseedcp/devflow-agent" `
  --workflow "release.yml" `
  --ref "main" `
  --environment "dogfood" `
  --github-input "demand_id=release-workflow-dogfood" `
  --github-input "release_note=v1.2 live release workflow dogfood"
```

Then refresh observation with the recorded GitHub Actions run URL as HTTP evidence.

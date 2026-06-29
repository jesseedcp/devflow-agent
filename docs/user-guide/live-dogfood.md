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

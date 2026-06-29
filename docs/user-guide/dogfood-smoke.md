# Dogfood And Smoke Guide

## Deterministic Local Dogfood

Run this path before opening a release PR. It does not call a live model provider and does not require API keys.

```powershell
powershell -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1
```

Expected checks:

- `devflow version` prints build metadata.
- `devflow init` creates a no-secret temp config.
- `devflow start` creates `dogfood-coupon-eligibility`.
- `devflow status` prints state and artifact paths.
- `devflow next` recommends the requirements stage.

## Optional Live Requirements Smoke

Run this only when you want to validate the configured model provider. Keep secrets in environment variables and do not paste them into config files or docs.

```powershell
$env:OPENAI_API_KEY = Read-Host -AsSecureString "OPENAI_API_KEY" | ConvertFrom-SecureString
```

The command above stores an encrypted string and is not accepted directly by provider SDKs. For an actual one-off shell session, set `OPENAI_API_KEY` to the private provider token in your terminal using your local secret-handling practice.

Initialize Ark/OpenAI-compatible config:

```powershell
devflow init --provider openai-compat --base-url https://ark.cn-beijing.volces.com/api/coding/v3 --model ark-code-latest
```

Validate setup without requiring GitLab:

```powershell
devflow doctor
```

Run a live requirements-stage smoke:

```powershell
devflow smoke --title "Live coupon eligibility smoke" --description "Only active members can claim coupons once"
```

Review the generated `requirements.md`, then delete the temporary demand workspace if it was only a smoke test.

## MR Review Readiness

GitLab is only required for the `mr-review` stage. Validate it explicitly:

```powershell
$env:GITLAB_TOKEN = "set-this-in-your-private-shell"
devflow doctor --require-gitlab
```

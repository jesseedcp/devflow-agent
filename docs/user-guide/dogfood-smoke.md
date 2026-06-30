# Dogfood And Smoke Guide

For release readiness, prefer the full-loop dogfood guide. This page remains for first-step smoke and optional live provider checks.

- [Full-loop dogfood guide](full-loop-dogfood.md)

## Deterministic Local Dogfood

Run this path before opening a release PR. It does not call a live model provider and does not require API keys.

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1
```

The dogfood script rebuilds the CLI by default and uses a unique temp workspace so it validates the current checkout without reusing previous lock files. Use `-UseExistingBinary` only when intentionally testing a prebuilt artifact.

Expected checks:

- `devflow version` prints build metadata.
- `devflow dogfood` creates `dogfood-coupon-eligibility`.
- The deterministic workflow reaches `state: completed`.
- The script prints the generated `dogfood-report.md` path.

## Optional Live Requirements Smoke

Run this only when you want to validate the configured model provider. Keep secrets in environment variables and do not paste them into config files or docs.

```powershell
$env:OPENAI_API_KEY = "<your-private-provider-token>"
```

Set this only in your private shell session. Do not paste the real token into committed config files, docs, screenshots, or terminal logs.

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

For live provider validation, see [Live dogfood guide](live-dogfood.md). Live dogfood is separate from deterministic dogfood and requires explicit environment opt-in.

The deterministic dogfood path also records an `MR Review Action Plan` section. In the default offline path, the selected next state is `verification` because the offline review adapter returns no blocking comments.

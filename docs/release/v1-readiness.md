# v1 Readiness

## Decision

v0.9.0 delivered the release-control loop: GitHub Actions deployment, post-deployment observation, human-gated rollback decisions, operator visibility, and offline release-readiness coverage. With v0.9 merged and tagged, v1.0.0 is ready to be finalized as a completion boundary rather than another feature-expansion wave.

## v0.9 Verification Basis

v0.9.0 is verified offline through `scripts\release-readiness.ps1 -Version 0.9.0`, including the `release control smoke` and `release rollback smoke` steps against a local fake GitHub Actions API. Full local verification passed: `go test ./...`, `go vet ./...`, `go build ./cmd/devflow`, and `git diff --check`.

## v1.0.0 Finalization Gate

Before tagging v1.0.0, run:

```powershell
go test ./... -count=1 -timeout 5m
go test ./... -count=1 -timeout 8m -p 1
go vet ./...
go build ./cmd/devflow
git diff --check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 1.0.0
```

## v1.0.0 Boundary

v1.0.0 finalizes the current local backend-demand Agent platform for general use. It does not add new providers, external wiki write-back, telemetry, dashboards, enterprise access control, or automated rollback execution.

## Deferred Work

- Optional live release-control dogfood against a real GitHub Actions workflow.
- Full live Feishu dogfood with disposable Feishu Doc and Bitable assets.
- External wiki provider adapters.
- Observability adapters for SLS, Grafana, Datadog, Prometheus, or OpenTelemetry.
- Enterprise permission and credential lifecycle model.
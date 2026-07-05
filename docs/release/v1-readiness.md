# v1 Readiness

## Decision

v0.9.0 delivers the release-control loop: GitHub Actions deployment, post-deployment observation, human-gated rollback decisions, operator visibility, and offline release-readiness coverage. With v0.9 complete and verified, the remaining work toward v1.0.0 is release finalization and optional live dogfood. No v1.0.0 hardening patch is required on the basis of v0.9 verification.

## v0.9 Verification Basis

v0.9.0 is verified offline through `scripts\release-readiness.ps1 -Version 0.9.0`, including the `release control smoke` and `release rollback smoke` steps against a local fake GitHub Actions API. Full local verification passed: `go test ./...`, `go vet ./...`, `go build ./cmd/devflow`, and `git diff --check`.

## Remaining v1 Work

- Merge the v0.9 release-control branch and tag `v0.9.0`.
- Optional live dogfood of the release-control loop against a real GitHub Actions workflow.
- Confirm no regressions on `main` after merge.

## v1.0.0 Boundary

v1.0.0 is a completion boundary, not another feature-expansion wave. v0.9 closes the release-control gap; v1.0.0 finalizes the project for general use.

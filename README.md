# Devflow Agent

Devflow Agent is a product and engineering exploration for a backend business requirements expert Agent.

The first design target is a platform-style MVP that runs a complete but lean backend delivery loop:

```text
requirement input
-> requirements.md
-> plan.md
-> implementation and tests
-> GitLab MR collaboration
-> verification.md
-> closeout.md
-> reusable knowledge candidates
```

See the initial product spec:

- [Backend Business Requirements Agent: Platform MVP Design](docs/superpowers/specs/2026-06-23-backend-business-requirements-agent-platform-mvp-design.md)
- [MewCode reuse and Eino integration decision](docs/architecture/mewcode-reuse.md)
- [Devflow and MewCode single-repository fusion design](docs/superpowers/specs/2026-06-25-devflow-mewcode-single-repo-fusion-design.md)

## v0.1 CLI shape

The first implementation exposes a deterministic local CLI:

```bash
go test ./...
go run ./cmd/devflow help
go run ./cmd/devflow start --title "Add coupon eligibility check" --description "Only active members can claim coupons"
```

The CLI writes demand workspaces under `.devflow/demands/<demand-id>/`.
## Wave 6 usability commands

Wave 6 adds first-run setup, status guidance, diagnostics, and an explicit live smoke path:

```text
devflow init
devflow status
devflow next
devflow doctor
devflow smoke
devflow dogfood
```

User-facing references:

- [Backend demand loop user guide](docs/user-guide/backend-demand-loop.md)
- [OpenAI-compatible example config](docs/examples/config.openai-compat.yaml)
- [Anthropic example config](docs/examples/config.anthropic.yaml)

## Release readiness

Wave 7 adds CI, version metadata, Windows build and local dogfood support:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-windows.ps1 -Version 0.1.0-dev
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dogfood-local.ps1
```

Release and dogfood references:

- [Dogfood and smoke guide](docs/user-guide/dogfood-smoke.md)
- [Full-loop dogfood guide](docs/user-guide/full-loop-dogfood.md)
- [v0.1 release notes](docs/release/v0.1.md)
- [Coupon eligibility sample demand](docs/examples/demands/coupon-eligibility.md)

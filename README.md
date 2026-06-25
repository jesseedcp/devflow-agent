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

## v0.1 CLI shape

The first implementation exposes a deterministic local CLI:

```bash
go test ./...
go run ./cmd/devflow help
go run ./cmd/devflow start --title "Add coupon eligibility check" --description "Only active members can claim coupons"
```

The CLI writes demand workspaces under `.devflow/demands/<demand-id>/`.

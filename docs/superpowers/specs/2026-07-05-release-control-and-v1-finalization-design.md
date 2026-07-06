# Release Control Loop And v1 Finalization Design

## 1. Decision

Devflow should finish as a backend delivery Agent control plane, not expand into a general AI engineering platform.

The final product line is:

```text
intake
-> requirements
-> plan
-> implementation
-> change request review and CI
-> verification
-> deploy
-> observe
-> rollback decision or stable decision
-> closeout
-> wiki, memory, and metrics
```

This spec defines the next feature release, `v0.9.0`, and the project completion boundary for `v1.0.0`.

`v0.9.0` adds the missing release control loop. `v1.0.0` is the point where the backend demand delivery loop is complete enough to call the product shape validated.

## 2. Product Goal

The two reference articles point to the same product from different angles:

- The backend business requirements Agent article defines the product target: a business demand should move through context, planning, implementation, review, verification, feedback, and knowledge reuse.
- The SDD + Harness article defines the execution method: a PRD should become an explicit spec, run through controlled execution, pass human gates, and reach deployment through CI/CD and observable evidence.

Devflow already covers demand intake, spec-like requirements and plans, code context, implementation, review gates, verification evidence, wiki distillation, and metrics.

The remaining product gap is the release control loop:

```text
deploy
observe
rollback
```

Without this loop, Devflow can prove that a change was implemented and verified locally, but it cannot prove that the change reached a release path, was observed after release, and had a rollback decision when release evidence failed.

## 3. v0.9 Scope

`v0.9.0` adds GitHub Actions-backed release control.

### 3.1 Must Do

- Add deployment artifacts:
  - `deployment.md`
  - `observation.md`
  - `rollback.md`
- Add workflow support for deployment and observation after verification.
- Add a GitHub Actions deployment adapter that can trigger and inspect workflow runs.
- Add CLI commands:
  - `devflow deploy trigger`
  - `devflow deploy status`
  - `devflow observe refresh`
  - `devflow rollback plan`
  - `devflow rollback confirm`
- Record release events in `events.jsonl`.
- Block closeout when deployment or observation has not produced acceptable evidence.
- Keep rollback human-gated. Do not execute rollback automatically in `v0.9.0`.
- Add deterministic fake-server release-readiness smoke for deploy, observe, and rollback behavior.
- Document the release control loop in the user guide and release notes.

### 3.2 Must Not Do

- Do not add Jenkins, Alibaba Cloud release platform, Argo CD, or custom internal CD providers in `v0.9.0`.
- Do not add SLS, Grafana, Datadog, Prometheus, OpenTelemetry, or tracing adapters in `v0.9.0`.
- Do not auto-run rollback.
- Do not require real GitHub credentials in default release-readiness.
- Do not make external wiki providers part of the release control loop.
- Do not add a dashboard or hosted service.

## 4. Workflow Design

The release control loop sits between verification and closeout.

Current high-level path:

```text
verification
-> closeout
```

New high-level path:

```text
verification
-> deployment
-> observation
-> closeout
```

Failure and decision paths:

```text
deployment_failed
-> rollback_recommended
-> blocked_need_release_decision

observation_failed
-> rollback_recommended
-> blocked_need_release_decision
```

The exact workflow state names may reuse the existing blocked states if adding new states creates too much churn. The product semantics still need to be visible in artifacts, events, status, console, and evaluation.

## 5. Gates

Release control is a workflow gate, not a prompt convention.

Rules:

1. A demand cannot deploy until verification is confirmed.
2. A demand cannot pass observation until deployment succeeded.
3. A demand cannot close out until observation is pass, or a rollback decision has been explicitly recorded.
4. A failed deployment must create or update `rollback.md`.
5. A failed observation must create or update `rollback.md`.
6. Rollback execution is not automatic in `v0.9.0`; only the decision record is supported.
7. `--run-next` may run safe release-control reads, but must not auto-confirm rollback decisions.

## 6. Artifacts

### 6.1 `deployment.md`

Purpose: prove what release action was requested and what happened.

Required content:

```text
# Deployment

## Summary
## Provider
## Target
## Workflow Run
## Commit And Branch
## Environment
## Status
## Evidence Links
## Events
```

For GitHub Actions, it should include:

- repository
- workflow file or workflow id
- ref
- commit SHA when available
- run id
- run URL
- status
- conclusion
- created and updated timestamps when available

### 6.2 `observation.md`

Purpose: prove the release was observed after deployment.

Required content:

```text
# Observation

## Summary
## Deployment Evidence
## Provider Checks
## Result
## Blocking Findings
## Evidence Links
```

For `v0.9.0`, provider checks are GitHub Actions and GitHub release/deployment evidence only. Runtime systems such as SLS and Grafana are future adapters.

### 6.3 `rollback.md`

Purpose: record the rollback decision material when deploy or observe fails.

Required content:

```text
# Rollback

## Trigger
## Impact
## Recommended Action
## Manual Decision
## Decision Evidence
```

Decision states:

- `pending`
- `rollback_confirmed`
- `risk_accepted`
- `redeploy_required`

`v0.9.0` records the decision only. It does not execute rollback.

## 7. GitHub Actions Adapter

Add a release-control adapter layer separate from existing PR review and CI gates.

The first concrete provider is GitHub Actions.

Expected operations:

```go
type DeploymentAdapter interface {
    TriggerDeployment(ctx context.Context, ref DeploymentRef) (DeploymentResult, error)
    GetDeployment(ctx context.Context, ref DeploymentRef) (DeploymentResult, error)
}

type ObservationAdapter interface {
    ObserveDeployment(ctx context.Context, ref DeploymentRef) (ObservationResult, error)
}
```

The exact Go types can differ, but the boundary should remain provider-neutral. GitHub-specific JSON must stay in the GitHub provider package.

GitHub Actions operations:

- `POST /repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches`
- `GET /repos/{owner}/{repo}/actions/runs`
- `GET /repos/{owner}/{repo}/actions/runs/{run_id}`
- optional jobs/artifacts/release lookups if they can be implemented cleanly without expanding scope

Because `workflow_dispatch` returns `204 No Content` without a run id, the adapter needs a deterministic lookup strategy:

1. Trigger dispatch with repo, workflow, ref, and inputs.
2. Query recent workflow runs for that workflow and ref.
3. Select the newest run created after the dispatch attempt.
4. Record the selected run id and URL.
5. If no run can be identified, return a blocked result with clear recovery guidance.

## 8. CLI Design

### 8.1 Deploy

```powershell
devflow deploy trigger `
  --demand <id> `
  --provider github-actions `
  --github-repo owner/repo `
  --workflow release.yml `
  --ref main `
  --environment staging
```

Optional:

```powershell
--input key=value
--github-base-url <url>
```

Behavior:

- validates verification confirmation
- triggers workflow dispatch
- writes `deployment.md`
- appends `deployment.triggered`
- leaves the demand in deployment/observation-ready state

### 8.2 Deploy Status

```powershell
devflow deploy status --demand <id>
```

Behavior:

- reads `deployment.md`
- refreshes run status if GitHub details are present and credentials are available
- writes updated deployment status
- appends `deployment.passed`, `deployment.failed`, or `deployment.blocked`

### 8.3 Observe

```powershell
devflow observe refresh --demand <id>
```

Behavior:

- requires a successful deployment
- reads deployment evidence
- performs provider observation
- writes `observation.md`
- appends `observation.passed`, `observation.failed`, or `observation.blocked`
- creates rollback material when observation fails

### 8.4 Rollback

```powershell
devflow rollback plan --demand <id>
devflow rollback confirm --demand <id> --decision rollback_confirmed --by <name> --summary <text>
```

Allowed decisions:

- `rollback_confirmed`
- `risk_accepted`
- `redeploy_required`

Behavior:

- `rollback plan` writes or refreshes `rollback.md`
- `rollback confirm` records the human decision
- rollback confirmation does not execute platform rollback in `v0.9.0`

## 9. Status, Console, Workbench, And Evaluation

Operator surfaces must show release-control state.

`status` and `console` should summarize:

```text
Deployment: pass|fail|blocked|missing
Observation: pass|fail|blocked|missing
Rollback: none|pending|confirmed|risk_accepted|redeploy_required
```

`next` and `console --run-next` should suggest:

- deploy when verification is confirmed and no deployment exists
- deploy status when deployment is pending
- observe when deployment passed and no observation exists
- rollback plan when deployment or observation failed
- rollback confirm when rollback decision is pending
- closeout only after observation passed or rollback decision was recorded

Evaluation checks:

```text
verification.deployment_ready
release.deployment_evidence
release.observation_evidence
release.rollback_decision
```

These checks should be warning or blocking according to stage:

- before closeout, missing deployment/observation should block closeout progression
- in evaluation output, evidence should be explicit and actionable

## 10. Configuration

Add release defaults under `.devflow/config.yaml`:

```yaml
release:
  provider: github-actions
  github:
    repo: owner/repo
    workflow: release.yml
    ref: main
    environment: staging
```

CLI flags override config.

Secrets:

- use `GITHUB_TOKEN`
- do not write tokens to artifacts
- redact auth headers and token-like values in errors and event data

## 11. Testing And Release Readiness

Default release readiness must remain offline.

Tests:

- adapter unit tests with `httptest`
- deployment artifact rendering tests
- observation artifact rendering tests
- rollback parser/render tests
- CLI tests for deploy, status, observe, rollback
- workflow/evaluation tests for closeout blocking
- status/console/workbench tests for release summaries

Release-readiness smoke:

- start disposable demand
- confirm verification
- fake GitHub Actions dispatch
- fake workflow run lookup
- refresh deploy status to pass
- refresh observation to pass
- verify closeout can proceed
- simulate failed observation
- generate rollback plan
- confirm rollback decision
- verify closeout gating is satisfied after decision

## 12. v1.0 Finalization Boundary

`v1.0.0` is the product completion point for the backend demand delivery loop.

v1.0 must prove one coherent path:

```text
GitHub Issue or PRD or Feishu intake
-> requirements
-> plan with codemap and scope
-> implementation
-> GitHub PR and CI/review gates
-> verification evidence
-> GitHub Actions deployment
-> observation
-> rollback decision when needed
-> closeout
-> wiki and memory distillation
-> metrics report
```

### 12.1 v1.0 Must Include

- v0.9 release control loop.
- One live GitHub Actions dogfood path.
- One live GitHub Issue or PR dogfood path.
- Feishu live dogfood if credentials and disposable assets are available.
- Release-readiness covering the full local loop.
- User guide reorganized around the final flow.
- `docs/release/v1.0.md`.
- Project-level final status document that states what is supported and what is intentionally deferred.

### 12.2 v1.0 Must Not Include

- Jenkins provider.
- Alibaba Cloud release provider.
- SLS, Grafana, Datadog, Prometheus, OpenTelemetry deep adapters.
- Automatic rollback execution.
- External wiki write-back.
- Multi-agent team execution as the default product path.
- Hosted dashboard.
- Enterprise multi-tenant auth and RBAC.

These belong to `v1.1+`.

## 13. Finalization Roadmap

### v0.9.0: Release Control Loop

- GitHub Actions deploy/observe foundation.
- Deployment, observation, rollback artifacts.
- Workflow gates between verification and closeout.
- Offline release-readiness smoke.

### v0.9.1: Live Release Dogfood

- Run against a disposable GitHub Actions workflow.
- Record sanitized dogfood report.
- Confirm no secrets are written.
- Harden ambiguous GitHub run lookup behavior.

### v0.9.2: Final Product Polish

- Reorganize user guide.
- Audit command help.
- Audit error recovery messages.
- Re-run Feishu live dogfood if credentials/assets are available.
- Clean stale branches, plans, worktrees, and release docs.

### v1.0.0: Backend Delivery Agent Control Plane

- Tag and publish the final completion release.
- State supported first-user path.
- State intentionally deferred platform integrations.
- Freeze the v1.0 product definition.

## 14. Risks

### 14.1 GitHub Actions Run Identification

`workflow_dispatch` does not return a run id. The lookup strategy can pick the wrong run if multiple dispatches happen close together.

Mitigation:

- include a unique correlation input when possible
- filter by workflow, ref, creation time, and actor when available
- record blocked evidence when matching is ambiguous

### 14.2 Release Control Overreach

There is a temptation to add Jenkins, SLS, Grafana, and real rollback in the same release.

Mitigation:

- v0.9 only does GitHub Actions release control
- runtime observation adapters are deferred to v1.1+
- rollback execution is deferred

### 14.3 False Confidence

GitHub Actions success does not prove production behavior.

Mitigation:

- name `v0.9` observation clearly as release-provider observation
- document that SLS/Grafana/Datadog are future runtime observation adapters
- keep observation evidence explicit instead of claiming semantic business health

### 14.4 Closeout Friction

Adding deploy/observe gates can slow local-only users.

Mitigation:

- allow manual deployment and observation records later if needed
- keep deterministic local dogfood easy
- make recovery commands clear

## 15. Success Criteria

`v0.9.0` is successful when:

- a verified demand can trigger a fake GitHub Actions deployment in release-readiness
- deployment evidence is written and visible
- observation evidence is written and visible
- closeout is blocked before deploy/observe evidence exists
- rollback material is generated on failure
- rollback decision is human-confirmed before closeout can continue after failure
- no real credentials are required for default release-readiness

`v1.0.0` is successful when:

- the complete backend demand delivery loop is documented, dogfooded, and released
- v1.0 release notes clearly define supported and deferred capabilities
- Devflow is no longer described as an exploration only, but as a validated local-first backend delivery Agent control plane

## 16. Spec Self-Review

- Scope is focused on GitHub Actions release control and v1 finalization.
- Runtime observability platforms are intentionally deferred.
- Rollback is a decision record in v0.9, not an executor.
- The design keeps provider-specific JSON behind adapters.
- Release readiness remains offline by default.
- The v1 boundary prevents endless feature expansion before the project is considered complete.

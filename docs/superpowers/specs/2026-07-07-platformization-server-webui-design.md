# Devflow Platformization Server And Web UI Design

## 1. Decision

Devflow should evolve from a single-user CLI into a platform-shaped backend demand delivery control plane.

The selected direction is:

```text
Go API server
+ React/Vite Web UI
+ local Docker MySQL
+ existing .devflow artifacts
```

MySQL becomes the platform system of record for users, workspaces, roles, audit events, demand indexes, wiki indexes, release decisions, and observation summaries. Existing `.devflow` markdown and JSONL artifacts remain the process record for requirements, plans, verification, deployment, observation, rollback, closeout, wiki candidates, and metrics.

This design intentionally supports two parallel implementation windows:

- Backend window: Go server, MySQL, API, RBAC, audit, artifact bridge, rollback and observation APIs.
- Frontend window: React/Vite UI shell, API client, mock mode, demand detail, wiki review, release decision, and observation pages.

## 2. Product Goal

The product goal is to make Devflow usable as a team-facing platform:

```text
one or more teams
-> shared workspaces
-> controlled demand lifecycle
-> human-gated review and release actions
-> internal wiki review
-> auditable rollback and observation decisions
```

The platform must not discard the CLI-first product that already works. Instead, the platform wraps and indexes the existing delivery loop so operators can use either the CLI or Web UI.

## 3. Selected Choices

| Area | Decision |
| --- | --- |
| Frontend shape | Go API server + React/Vite Web UI |
| Database | Local Docker MySQL for development and demo |
| Artifact strategy | Keep `.devflow` markdown/jsonl as auditable process material |
| Platform storage | MySQL for users, workspaces, roles, audit, indexes, and UI queries |
| RBAC | Viewer / Developer / Reviewer / Admin |
| Wiki | Internal Devflow wiki only; no Notion, Confluence, GitHub Wiki, or Feishu Wiki provider |
| Rollback | GitHub Actions `rollback.yml` trigger/status first; human-gated |
| Observation adapter | Start with Generic JSON Metric Adapter |
| Feishu live dogfood | Last, after platform and release loops are stable |
| Complex backend dogfood | Last, after platform and release loops are stable |

## 4. Non-Goals

Do not implement these in the first platformization pass:

- Do not build a hosted SaaS control plane.
- Do not implement full enterprise SSO.
- Do not implement Kubernetes, Alibaba Cloud, Jenkins, Argo CD, Harness, or custom release systems.
- Do not execute rollback automatically from failed observation evidence.
- Do not connect external wiki providers.
- Do not add SLS, Grafana, Datadog, or OpenTelemetry first.
- Do not migrate existing `.devflow` artifacts into MySQL and delete the files.
- Do not require Feishu credentials for default tests or release-readiness.

## 5. Architecture

### 5.1 Runtime Shape

```text
devflow server
  -> Go HTTP API
  -> MySQL platform store
  -> local artifact bridge over .devflow roots
  -> existing demandflow / artifacts / wiki / metrics / releasecontrol packages

web/
  -> React/Vite app
  -> HTTP API client
  -> mock client for parallel frontend work
  -> role-aware views
```

The API server should call existing internal packages where possible. The first server pass should avoid reimplementing demandflow logic in SQL.

### 5.2 Data Ownership

MySQL owns:

- users
- workspaces
- workspace members
- roles
- audit events
- demand index rows
- artifact presence summaries
- wiki entry indexes
- wiki candidate indexes
- release operation summaries
- observation summaries

`.devflow` owns:

- `requirements.md`
- `plan.md`
- `progress.md`
- `verification.md`
- `deployment.md`
- `observation.md`
- `rollback.md`
- `closeout.md`
- `events.jsonl`
- `wiki-candidates.md`
- `.devflow/wiki/*.md`
- `metrics.md`

The platform store may cache parsed summaries, but the artifact file remains the readable record.

## 6. Backend Design

### 6.1 Server Entry Point

Add:

```text
devflow server
```

First version behavior:

- reads `DEVFLOW_DATABASE_DSN`
- uses local artifact root from `--root` or current directory
- serves JSON API under `/api`
- optionally serves built frontend static files later
- logs startup checks without printing secrets

### 6.2 Docker MySQL

Development default:

```text
docker compose up -d mysql
```

Default DSN:

```text
devflow:devflow@tcp(127.0.0.1:3306)/devflow?parseTime=true
```

The DSN should be configurable:

```text
DEVFLOW_DATABASE_DSN=devflow:devflow@tcp(127.0.0.1:3306)/devflow?parseTime=true
```

### 6.3 Core Tables

Initial schema:

```text
users
  id
  email
  display_name
  created_at

workspaces
  id
  name
  artifact_root
  created_at

workspace_members
  workspace_id
  user_id
  role
  created_at

audit_events
  id
  workspace_id
  actor_user_id
  action
  subject_type
  subject_id
  metadata_json
  created_at

demands
  id
  workspace_id
  demand_key
  title
  state
  attention
  artifact_path
  updated_at

wiki_entries
  id
  workspace_id
  name
  category
  source_demand_key
  artifact_path
  updated_at

release_operations
  id
  workspace_id
  demand_key
  kind
  provider
  status
  run_url
  decision
  updated_at

observation_records
  id
  workspace_id
  demand_key
  adapter
  status
  summary
  evidence_json
  updated_at
```

### 6.4 RBAC

Roles:

```text
Viewer
  read demands, artifacts, wiki, metrics, release status

Developer
  Viewer permissions
  add evidence
  refresh plan context
  refresh implementation review

Reviewer
  Developer permissions
  confirm requirements / plan / verification / closeout
  promote or reject wiki candidates

Admin
  Reviewer permissions
  configure workspace
  trigger deployment
  trigger rollback
  configure platform secrets
```

Every high-risk action must write an audit event:

- confirm gate
- trigger deploy
- trigger rollback
- accept release risk
- promote wiki
- reject wiki
- configure workspace

### 6.5 API Contract

Initial API paths:

```text
GET  /api/health
GET  /api/workspaces
POST /api/workspaces
GET  /api/workspaces/{workspaceID}/demands
GET  /api/workspaces/{workspaceID}/demands/{demandKey}
GET  /api/workspaces/{workspaceID}/demands/{demandKey}/artifacts/{name}
GET  /api/workspaces/{workspaceID}/wiki
GET  /api/workspaces/{workspaceID}/wiki/candidates
POST /api/workspaces/{workspaceID}/wiki/candidates/{candidateID}/promote
POST /api/workspaces/{workspaceID}/wiki/candidates/{candidateID}/reject
GET  /api/workspaces/{workspaceID}/release/{demandKey}
POST /api/workspaces/{workspaceID}/release/{demandKey}/rollback/trigger
POST /api/workspaces/{workspaceID}/release/{demandKey}/observe
GET  /api/workspaces/{workspaceID}/audit
```

Initial auth can be dev-mode identity:

```text
DEVFLOW_DEV_USER_EMAIL=admin@example.com
DEVFLOW_DEV_USER_ROLE=Admin
```

This gives the frontend a stable user and role before full login exists.

## 7. Frontend Design

### 7.1 App Shape

Create a React/Vite app under:

```text
web/
```

Core pages:

```text
/workspaces
/workspaces/:workspaceId/demands
/workspaces/:workspaceId/demands/:demandKey
/workspaces/:workspaceId/wiki
/workspaces/:workspaceId/wiki/candidates
/workspaces/:workspaceId/release/:demandKey
/workspaces/:workspaceId/audit
```

### 7.2 UI Principles

The UI should feel like an operational tool, not a marketing site.

Use:

- dense but readable tables
- tabs for artifacts
- status badges
- audit timeline
- role-aware disabled actions
- clear blocking reasons

Do not use:

- landing page hero
- decorative cards inside cards
- one-note purple/blue gradient theme
- text-heavy explanations inside the app

### 7.3 Mock Mode

Frontend must support mock mode so it can be developed before backend endpoints are complete:

```text
VITE_DEVFLOW_API_MODE=mock
VITE_DEVFLOW_API_BASE=http://localhost:8080
```

API client shape:

```text
src/api/client.ts
src/api/httpClient.ts
src/api/mockClient.ts
src/api/types.ts
```

The mock data must match the backend API contract in this spec.

## 8. Internal Wiki UI

The internal wiki is not an external provider. It is a UI over Devflow's project-local wiki and candidate artifacts.

User flows:

```text
review candidate
-> inspect source demand and source excerpt
-> promote with name/category
-> reject with reason
-> audit event
-> wiki entry appears in library
```

Frontend pages:

- Wiki Library
- Candidate Review
- Wiki Entry Detail

Backend APIs should keep using existing wiki package behavior where possible.

## 9. Rollback Execution

Rollback execution is human-gated.

Flow:

```text
observation failed or deployment failed
-> rollback.md records pending recommendation
-> Reviewer/Admin reviews
-> Admin triggers rollback workflow
-> Devflow records rollback run URL and status
-> observation refresh runs again
```

First provider:

```text
GitHub Actions rollback.yml
```

Do not trigger rollback automatically from failed observation evidence.

## 10. Generic JSON Observation Adapter

The first non-trivial observation adapter should be generic JSON metrics.

Example endpoint:

```text
GET /metrics.json
```

Example response:

```json
{
  "error_rate": 0,
  "p95_latency_ms": 120,
  "active_alerts": 0
}
```

Example rule:

```text
error_rate <= 0.01
p95_latency_ms <= 300
active_alerts == 0
```

Result mapping:

```text
all expectations pass -> pass
endpoint reachable but values violate rules -> fail
network, timeout, auth, malformed JSON -> blocked
```

Evidence writes to:

```text
observation.md
events.jsonl
observation_records
```

## 11. Version Roadmap

### v1.3 Platform Server + MySQL Foundation

- Docker MySQL setup.
- Go server command.
- MySQL migration runner.
- Workspace, user, demand index, and audit tables.
- API health and demand read endpoints.
- Artifact bridge from `.devflow`.

### v1.4 RBAC / Workspace / Audit

- Dev-mode identity.
- Role enforcement middleware.
- Audit event creation.
- Protected action API skeletons.
- Tests for each role.

### v1.5 React/Vite Web UI Shell

- Web app scaffold.
- Mock and HTTP API clients.
- Workspace and demand list/detail pages.
- Artifact tabs.
- Role-aware action states.

### v1.6 Internal Wiki UI

- Wiki entry and candidate APIs.
- Candidate promote/reject APIs.
- Wiki library and candidate review UI.
- Audit events for wiki decisions.

### v1.7 GitHub Actions Rollback Execution

- `rollback trigger` backend service.
- GitHub Actions rollback adapter.
- rollback status polling.
- UI release decision panel.
- Human gate remains mandatory.

### v1.8 Generic JSON Observation Adapter

- Generic metric rule model.
- JSON metric fetcher.
- observation adapter API.
- UI observation evidence table.
- release-readiness fake metric smoke.

### v1.9 Full Platform Dogfood

- Run the platform UI against a local demo workspace.
- Exercise demand detail, wiki review, rollback, observation, audit, and metrics.
- No Feishu live dependency.

### v2.0 Feishu Live + Complex Backend Dogfood

- Run live Feishu doc/bitable dogfood.
- Run a complex Go backend demand with DB, HTTP handler, service, tests, release workflow, observation, closeout, wiki, and metrics.

## 12. Backend Window Prompt

Use this prompt in the backend implementation window:

```text
You are implementing the backend half of Devflow platformization.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md
- AGENTS.md

Scope:
- Implement only backend/platform tasks assigned to the backend lane.
- Do not create the React frontend except for API contract fixtures if the plan explicitly requires them.
- Use a git worktree branch.
- Use TDD.
- Use MySQL through local Docker Compose.
- Keep .devflow artifacts as the process record; do not migrate/delete them.
- Preserve existing CLI behavior and release-readiness.

Primary deliverable:
v1.3 Platform Server + MySQL Foundation, then v1.4 RBAC / Workspace / Audit if v1.3 is complete and verified.

Required verification:
- go test ./... -count=1 -timeout 5m
- go vet ./...
- go build ./cmd/devflow
- git diff --check
- scripts\release-readiness.ps1 -Version <version>

Report:
- changed files
- schema created
- API endpoints implemented
- tests run
- remaining risks
```

## 13. Frontend Window Prompt

Use this prompt in the frontend implementation window:

```text
You are implementing the frontend half of Devflow platformization.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md
- AGENTS.md

Scope:
- Implement only the React/Vite Web UI lane.
- Do not change backend APIs unless the plan explicitly says to update shared API types/fixtures.
- Build against mockClient first.
- Keep API types aligned with the backend contract in the spec.
- Operational UI only: no landing page, no marketing hero.

Primary deliverable:
v1.5 Web UI Shell with mock mode, workspace list, demand list/detail, artifact tabs, wiki candidate shell, release panel shell, and audit shell.

Required verification:
- npm install / npm test / npm run build or the repo-equivalent commands chosen by the plan
- screenshot or browser smoke for main pages
- no overlapping text or broken responsive layout

Report:
- changed files
- pages implemented
- mock fixtures added
- build/test result
- integration assumptions waiting on backend
```

## 14. Integration Window Prompt

Use this later, after backend and frontend lanes both pass:

```text
You are integrating the backend and frontend lanes of Devflow platformization.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md
- backend lane final report
- frontend lane final report

Scope:
- Resolve API type mismatches.
- Run Docker MySQL.
- Start devflow server.
- Start the frontend against the real API.
- Verify workspace, demand detail, artifact, wiki, release, observation, and audit pages.
- Do not add new product scope during integration.

Required verification:
- backend full Go verification
- frontend build/test
- real API browser smoke
- release-readiness remains green

Report:
- integration fixes
- screenshots or browser smoke evidence
- final risks
```

## 15. Approval Boundary

This spec defines a multi-release platformization route. The immediate execution target should be v1.3 backend foundation and v1.5 frontend mock shell, not the entire v1.3-v2.0 roadmap in one branch.


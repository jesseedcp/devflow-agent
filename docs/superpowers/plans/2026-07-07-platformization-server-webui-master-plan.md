# Devflow Platformization Server And Web UI Master Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Devflow toward a team-facing platform with Go API server, React/Vite Web UI, Docker MySQL, RBAC, audit, internal wiki UI, rollback execution, and a simple observation adapter.

**Architecture:** The platform wraps the existing CLI/artifact workflow instead of replacing it. MySQL stores platform indexes, users, workspaces, roles, audit, and UI summaries; `.devflow` keeps durable delivery artifacts. Backend and frontend lanes can run in separate windows because the API contract and prompts are embedded in this plan.

**Tech Stack:** Go standard library plus existing Devflow packages, local Docker MySQL, SQL migrations, React/Vite frontend, TypeScript API types, PowerShell release-readiness, GitHub Actions for deploy/rollback orchestration.

## Global Constraints

- Use local Docker MySQL for the first platform implementation.
- Keep existing `.devflow` markdown/jsonl artifacts as the process record.
- Do not delete or migrate existing `.devflow` artifacts into MySQL.
- Do not require Feishu credentials in default tests.
- Do not implement external wiki providers.
- Do not implement automatic rollback from failed observation evidence.
- Do not add SLS, Grafana, Datadog, Prometheus, or OpenTelemetry before the Generic JSON Metric Adapter.
- Frontend must be an operational tool UI, not a landing page.
- Backend and frontend windows must keep API contracts synchronized through this plan and the spec.
- Every code task must use TDD where practical and must end with a commit using the Lore protocol.

---

## Context

Read these files before executing:

```text
docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
docs/user-guide/backend-demand-loop.md
docs/release/v1.2.1.md
internal/artifacts/store.go
internal/demandflow/workspace.go
internal/wiki/store.go
internal/releasecontrol/model.go
internal/cli/deploy.go
internal/cli/observe.go
internal/cli/rollback.go
```

Current state:

```text
v1.2.1 exists
Devflow can run the CLI delivery loop
Devflow can trigger GitHub Actions release workflow
Devflow can record deployment.md / observation.md / rollback.md
Devflow has local wiki distillation
Devflow does not yet have a platform server, MySQL store, or Web UI
```

## Branching

Backend lane:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git checkout main
git pull --ff-only origin main
git worktree add .worktrees\v13-platform-backend -b v13-platform-backend main
cd .worktrees\v13-platform-backend
```

Frontend lane:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git checkout main
git pull --ff-only origin main
git worktree add .worktrees\v15-platform-webui -b v15-platform-webui main
cd .worktrees\v15-platform-webui
```

If both lanes edit a shared contract file, coordinate through the integration window before merging.

## File Structure

Backend lane creates or modifies:

```text
docker-compose.yml
.env.example
cmd/devflow/main.go
internal/cli/cli.go
internal/cli/server.go
internal/platform/server/server.go
internal/platform/server/router.go
internal/platform/server/middleware.go
internal/platform/server/handlers.go
internal/platform/store/mysql/store.go
internal/platform/store/mysql/migrations.go
internal/platform/store/mysql/schema.sql
internal/platform/store/model.go
internal/platform/store/rbac.go
internal/platform/store/audit.go
internal/platform/artifactbridge/bridge.go
internal/platform/api/types.go
internal/platform/api/errors.go
internal/platform/server/*_test.go
internal/platform/store/mysql/*_test.go
scripts/release-readiness.ps1
docs/release/v1.3.md
docs/release/v1.4.md
```

Frontend lane creates or modifies:

```text
web/package.json
web/vite.config.ts
web/tsconfig.json
web/index.html
web/src/main.tsx
web/src/App.tsx
web/src/api/types.ts
web/src/api/client.ts
web/src/api/httpClient.ts
web/src/api/mockClient.ts
web/src/mocks/workspaces.ts
web/src/mocks/demands.ts
web/src/pages/WorkspacesPage.tsx
web/src/pages/DemandsPage.tsx
web/src/pages/DemandDetailPage.tsx
web/src/pages/WikiPage.tsx
web/src/pages/WikiCandidatesPage.tsx
web/src/pages/ReleasePage.tsx
web/src/pages/AuditPage.tsx
web/src/components/AppShell.tsx
web/src/components/StatusBadge.tsx
web/src/components/ArtifactTabs.tsx
web/src/components/RoleGate.tsx
web/src/styles.css
docs/release/v1.5.md
```

Integration lane may modify:

```text
internal/platform/api/types.go
web/src/api/types.ts
internal/platform/server/static.go
cmd/devflow/main.go
scripts/release-readiness.ps1
docs/release/v1.6.md and later release notes
```

---

## Execution Prompts

### Backend Window Prompt

```text
You are implementing the backend half of Devflow platformization.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- AGENTS.md
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md

Use skills:
- superpowers:using-git-worktrees
- superpowers:test-driven-development
- superpowers:verification-before-completion
- superpowers:finishing-a-development-branch when complete

Scope:
- Backend/platform only.
- Implement v1.3 Platform Server + MySQL Foundation first.
- If v1.3 is merged and verified, continue with v1.4 RBAC / Workspace / Audit.
- Do not create the React UI.
- Do not remove .devflow artifact behavior.
- Use Docker MySQL for local tests and docs.
- Keep default release-readiness credential-free.

Required final verification:
- go test ./... -count=1 -timeout 5m
- go vet ./...
- go build ./cmd/devflow
- git diff --check
- powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version <version>

Final report must include:
- branch and PR
- changed files
- schema/API implemented
- verification results
- remaining risks
```

### Frontend Window Prompt

```text
You are implementing the frontend half of Devflow platformization.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- AGENTS.md
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md

Use skills:
- superpowers:using-git-worktrees
- superpowers:verification-before-completion
- frontend-skill for UI quality

Scope:
- Frontend/Web UI only.
- Implement v1.5 React/Vite Web UI Shell against mockClient first.
- Do not change backend APIs except shared API type fixtures if the plan explicitly calls for it.
- Match API types from the spec.
- No marketing landing page.
- Build an operational dashboard with dense, readable views.

Required final verification:
- package install command chosen by the repo
- frontend tests if configured
- frontend build
- browser smoke or screenshots for main pages
- no text overlap on desktop and mobile viewports

Final report must include:
- branch and PR
- pages implemented
- API mock coverage
- verification results
- backend integration assumptions
```

### Integration Window Prompt

```text
You are integrating Devflow platform backend and frontend lanes.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- AGENTS.md
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md
- backend lane final report
- frontend lane final report

Scope:
- Resolve API mismatches.
- Start Docker MySQL.
- Run devflow server.
- Run frontend against real API.
- Verify platform pages with real data.
- Do not add new product scope.

Required final verification:
- go test ./... -count=1 -timeout 5m
- go vet ./...
- go build ./cmd/devflow
- frontend build/test
- browser smoke against real API
- scripts\release-readiness.ps1 -Version <version>
```

---

## Task 1: v1.3 Backend Platform Store And Docker MySQL

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `internal/platform/store/model.go`
- Create: `internal/platform/store/mysql/schema.sql`
- Create: `internal/platform/store/mysql/migrations.go`
- Create: `internal/platform/store/mysql/store.go`
- Test: `internal/platform/store/mysql/store_test.go`

**Interfaces:**
- Produces: `type User`, `type Workspace`, `type WorkspaceMember`, `type DemandIndex`, `type AuditEvent`
- Produces: `func Open(ctx context.Context, dsn string) (*Store, error)`
- Produces: `func (s *Store) Migrate(ctx context.Context) error`
- Produces: `func (s *Store) UpsertWorkspace(ctx context.Context, workspace Workspace) error`
- Produces: `func (s *Store) ListWorkspaces(ctx context.Context) ([]Workspace, error)`

- [ ] **Step 1: Write store model tests**

Add tests proving migration creates tables and workspace insert/list round-trips against a MySQL test DSN.

Expected local DSN:

```powershell
$env:DEVFLOW_DATABASE_DSN="devflow:devflow@tcp(127.0.0.1:3306)/devflow?parseTime=true"
```

- [ ] **Step 2: Add Docker MySQL**

Create `docker-compose.yml` with service `mysql`, database `devflow`, user `devflow`, password `devflow`, and port `3306:3306`.

- [ ] **Step 3: Implement migration runner**

Use `database/sql` and `go-sql-driver/mysql` only if the repo accepts the dependency in this task. If adding the dependency is not acceptable, stop and report the blocker because MySQL cannot be used from Go without a driver.

- [ ] **Step 4: Verify**

Run:

```powershell
docker compose up -d mysql
go test ./internal/platform/store/mysql -count=1
```

Expected:

```text
ok github.com/jesseedcp/devflow-agent/internal/platform/store/mysql
```

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 2: v1.3 Server Command And Health API

**Files:**
- Modify: `cmd/devflow/main.go`
- Modify: `internal/cli/cli.go`
- Create: `internal/cli/server.go`
- Create: `internal/platform/server/server.go`
- Create: `internal/platform/server/router.go`
- Create: `internal/platform/api/types.go`
- Test: `internal/cli/server_test.go`
- Test: `internal/platform/server/server_test.go`

**Interfaces:**
- Consumes: `mysql.Open(ctx, dsn)`
- Produces: CLI command `devflow server`
- Produces: `GET /api/health`
- Produces response: `{ "status": "ok", "database": "ok" }`

- [ ] **Step 1: Write failing API test**

Test that `GET /api/health` returns HTTP 200 with JSON status and database status.

- [ ] **Step 2: Add `devflow server` CLI route**

Flags:

```text
--addr string default "127.0.0.1:8080"
--root string existing project root behavior
--database-dsn string optional, falls back to DEVFLOW_DATABASE_DSN
```

- [ ] **Step 3: Implement server startup**

Startup must:

- validate DSN is present
- open MySQL
- run migrations
- start HTTP server
- not print password or full DSN

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/cli ./internal/platform/server -count=1
go build ./cmd/devflow
```

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 3: v1.3 Artifact Bridge And Demand APIs

**Files:**
- Create: `internal/platform/artifactbridge/bridge.go`
- Modify: `internal/platform/api/types.go`
- Modify: `internal/platform/server/router.go`
- Create: `internal/platform/server/demands.go`
- Test: `internal/platform/artifactbridge/bridge_test.go`
- Test: `internal/platform/server/demands_test.go`

**Interfaces:**
- Produces: `func ScanDemands(root string) ([]api.DemandSummary, error)`
- Produces: `GET /api/workspaces/{workspaceID}/demands`
- Produces: `GET /api/workspaces/{workspaceID}/demands/{demandKey}`
- Produces: `GET /api/workspaces/{workspaceID}/demands/{demandKey}/artifacts/{name}`

- [ ] **Step 1: Test artifact scanning**

Create a temp `.devflow/demands/<id>` with `demand.json`, `requirements.md`, and `events.jsonl`. Assert scan returns id, title, state, artifact presence, and updated time.

- [ ] **Step 2: Implement bridge using existing artifacts package**

Prefer existing artifact/demand loading helpers. Do not parse markdown with ad hoc string logic except for presence and raw artifact read.

- [ ] **Step 3: Add API handlers**

Handlers must return JSON for summaries and raw text for artifact content.

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/platform/artifactbridge ./internal/platform/server -count=1
```

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 4: v1.4 RBAC And Audit

**Files:**
- Create: `internal/platform/store/rbac.go`
- Create: `internal/platform/store/audit.go`
- Modify: `internal/platform/server/middleware.go`
- Modify: `internal/platform/server/router.go`
- Test: `internal/platform/store/rbac_test.go`
- Test: `internal/platform/server/rbac_test.go`

**Interfaces:**
- Produces: roles `Viewer`, `Developer`, `Reviewer`, `Admin`
- Produces: `func Can(role Role, action Action) bool`
- Produces: middleware reading `DEVFLOW_DEV_USER_EMAIL` and `DEVFLOW_DEV_USER_ROLE`
- Produces: `GET /api/workspaces/{workspaceID}/audit`

- [ ] **Step 1: Write RBAC table tests**

Assert:

```text
Viewer cannot trigger rollback
Developer can add evidence but cannot promote wiki
Reviewer can promote wiki but cannot configure workspace
Admin can trigger rollback
```

- [ ] **Step 2: Implement RBAC helper**

Keep it deterministic and dependency-free.

- [ ] **Step 3: Add audit write/read path**

Every protected action handler must call audit append before returning success.

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/platform/store ./internal/platform/server -count=1
```

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 5: v1.5 Frontend Scaffold And API Types

**Files:**
- Create all `web/` files listed in File Structure.

**Interfaces:**
- Consumes backend API JSON contract from the spec.
- Produces `ApiClient` with methods:
  - `listWorkspaces()`
  - `listDemands(workspaceID)`
  - `getDemand(workspaceID, demandKey)`
  - `getArtifact(workspaceID, demandKey, artifactName)`
  - `listWikiEntries(workspaceID)`
  - `listWikiCandidates(workspaceID)`
  - `getAuditEvents(workspaceID)`

- [ ] **Step 1: Scaffold Vite React app under `web/`**

Use TypeScript. Keep dependencies minimal.

- [ ] **Step 2: Create exact API types**

Types must mirror backend:

```text
Workspace
DemandSummary
DemandDetail
ArtifactSummary
WikiEntry
WikiCandidate
ReleaseSummary
AuditEvent
CurrentUser
```

- [ ] **Step 3: Add mock client**

Mock data must include:

```text
one workspace
three demands in different states
one promoted wiki entry
two pending wiki candidates
one rollback-needed release summary
five audit events
```

- [ ] **Step 4: Verify**

Run frontend install/build commands chosen by the frontend lane and record them in the final report.

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 6: v1.5 Frontend Operational Pages

**Files:**
- Modify `web/src/pages/*`
- Modify `web/src/components/*`
- Modify `web/src/styles.css`

**Interfaces:**
- Consumes `ApiClient` from Task 5.
- Produces navigable Web UI pages from the spec.

- [ ] **Step 1: Build AppShell**

Layout:

```text
left nav
top workspace/role strip
main content
```

- [ ] **Step 2: Build demand list/detail**

Demand detail must show:

```text
state
attention
release line
quality summary
artifact tabs
acceptance evidence counts
metrics summary
```

- [ ] **Step 3: Build wiki and release shells**

Wiki candidates page must show promote/reject buttons disabled unless role is Reviewer/Admin.

Release page must show rollback trigger disabled unless role is Admin.

- [ ] **Step 4: Verify visually**

Use browser smoke or screenshots for:

```text
desktop demand detail
mobile demand detail
wiki candidates
release decision page
```

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 7: v1.6 Internal Wiki API And UI Integration

**Files:**
- Modify: `internal/platform/server/router.go`
- Create: `internal/platform/server/wiki.go`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/httpClient.ts`
- Modify: `web/src/pages/WikiPage.tsx`
- Modify: `web/src/pages/WikiCandidatesPage.tsx`
- Test: `internal/platform/server/wiki_test.go`

**Interfaces:**
- Produces:
  - `GET /api/workspaces/{workspaceID}/wiki`
  - `GET /api/workspaces/{workspaceID}/wiki/candidates`
  - `POST /api/workspaces/{workspaceID}/wiki/candidates/{candidateID}/promote`
  - `POST /api/workspaces/{workspaceID}/wiki/candidates/{candidateID}/reject`

- [ ] **Step 1: Backend tests for wiki list/promote/reject**

Use temp `.devflow` artifacts. Assert promote writes through existing wiki behavior and audit event is recorded.

- [ ] **Step 2: Implement backend handlers**

Use existing `internal/wiki` package. Do not invent a second wiki storage model.

- [ ] **Step 3: Wire frontend HTTP client**

Add real HTTP methods matching the backend routes.

- [ ] **Step 4: Verify**

Run backend tests and frontend build.

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 8: v1.7 GitHub Actions Rollback Execution

**Files:**
- Modify: `internal/adapters/github_actions.go`
- Modify: `internal/cli/rollback.go`
- Create: `internal/platform/server/rollback.go`
- Test: `internal/adapters/github_actions_test.go`
- Test: `internal/cli/rollback_test.go`
- Test: `internal/platform/server/rollback_test.go`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Create: `docs/release/v1.7.md`

**Interfaces:**
- Produces CLI:
  - `devflow rollback trigger --provider github --github-repo owner/repo --workflow rollback.yml --ref main`
  - `devflow rollback status --provider github --github-repo owner/repo --workflow rollback.yml --ref main`
- Produces API:
  - `POST /api/workspaces/{workspaceID}/release/{demandKey}/rollback/trigger`

- [ ] **Step 1: Write fake GitHub API tests**

Test workflow dispatch, run polling, rollback.md update, and rollback event append.

- [ ] **Step 2: Implement rollback trigger/status**

Reuse existing GitHub Actions adapter patterns from deploy trigger/status.

- [ ] **Step 3: Enforce human gate**

Trigger must fail unless rollback.md decision is `rollback_required` or actor is Admin and explicit confirmation is present.

- [ ] **Step 4: Verify**

Run:

```powershell
go test ./internal/adapters ./internal/cli ./internal/platform/server -count=1
```

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 9: v1.8 Generic JSON Observation Adapter

**Files:**
- Create: `internal/releasecontrol/json_metrics.go`
- Create: `internal/releasecontrol/json_metrics_test.go`
- Modify: `internal/cli/observe.go`
- Create: `internal/platform/server/observe.go`
- Modify: `scripts/release-readiness.ps1`
- Create: `docs/release/v1.8.md`

**Interfaces:**
- Produces CLI:
  - `devflow observe refresh --json-metrics-url <url> --expect "error_rate<=0.01" --expect "active_alerts==0"`
- Produces result mapping:
  - pass when all expectations pass
  - fail when reachable values violate expectations
  - blocked on timeout, auth, malformed JSON, missing field

- [ ] **Step 1: Test metric expression parser**

Support only:

```text
field<=number
field<number
field>=number
field>number
field==number
```

Reject anything else with a clear error.

- [ ] **Step 2: Test fetcher with fake server**

Use response:

```json
{"error_rate":0,"p95_latency_ms":120,"active_alerts":0}
```

Assert observation pass.

- [ ] **Step 3: Implement CLI and observation write**

Write evidence into `observation.md`, `events.jsonl`, and platform observation summary when server context is present.

- [ ] **Step 4: Add release-readiness smoke**

Use local fake HTTP server. Do not require real external observability credentials.

- [ ] **Step 5: Commit**

Commit with Lore protocol.

## Task 10: v1.9 Full Platform Dogfood

**Files:**
- Create: `docs/dogfood/v1.9-platform-dogfood.md`
- Modify: `scripts/release-readiness.ps1`

**Interfaces:**
- Consumes backend server, frontend app, MySQL, wiki UI, rollback API, JSON observation adapter.
- Produces dogfood report with pass/fail verdict.

- [ ] **Step 1: Start Docker MySQL**

Run:

```powershell
docker compose up -d mysql
```

- [ ] **Step 2: Start server**

Run:

```powershell
$env:DEVFLOW_DATABASE_DSN="devflow:devflow@tcp(127.0.0.1:3306)/devflow?parseTime=true"
devflow server --addr 127.0.0.1:8080 --root <dogfood-root>
```

- [ ] **Step 3: Run frontend against real API**

Use:

```text
VITE_DEVFLOW_API_MODE=http
VITE_DEVFLOW_API_BASE=http://127.0.0.1:8080
```

- [ ] **Step 4: Exercise flows**

Verify:

```text
workspace list loads
demand list loads
demand detail artifact tabs load
wiki candidates can be promoted/rejected by Reviewer/Admin
Viewer sees disabled action states
rollback trigger is gated
JSON observation pass/fail/blocked states render
audit events appear
```

- [ ] **Step 5: Write dogfood report**

Report must include:

```text
environment
commands
screenshots or browser-smoke notes
API checks
audit checks
known limits
final verdict
```

- [ ] **Step 6: Commit**

Commit with Lore protocol.

## Task 11: v2.0 Feishu Live And Complex Backend Dogfood

**Files:**
- Create: `docs/dogfood/v2.0-feishu-live-dogfood.md`
- Create: `docs/dogfood/v2.0-complex-backend-dogfood.md`

**Interfaces:**
- Consumes completed platform, rollback, and observation features.

- [ ] **Step 1: Feishu live dogfood**

Requires:

```text
FEISHU_APP_ID
FEISHU_APP_SECRET
DEVFLOW_DOGFOOD_FEISHU_DOC
DEVFLOW_DOGFOOD_BITABLE_APP
DEVFLOW_DOGFOOD_BITABLE_TABLE
DEVFLOW_DOGFOOD_BITABLE_RECORD
```

Stop and record blocked if any are missing.

- [ ] **Step 2: Complex backend dogfood**

Use a Go service with:

```text
HTTP handler
service layer
repository or database
tests
GitHub Actions
JSON metrics endpoint
```

- [ ] **Step 3: Verify full lifecycle**

Demand must reach completed and include:

```text
requirements.md
plan.md
progress.md
verification.md
deployment.md
observation.md
rollback.md when applicable
closeout.md
wiki decisions
metrics.md
audit events
```

- [ ] **Step 4: Commit reports**

Commit dogfood reports only. Do not commit secrets or external platform tokens.

---

## Release Strategy

Recommended releases:

```text
v1.3.0 Platform Server + MySQL Foundation
v1.4.0 RBAC / Workspace / Audit
v1.5.0 React/Vite Web UI Shell
v1.6.0 Internal Wiki UI
v1.7.0 GitHub Actions Rollback Execution
v1.8.0 Generic JSON Observation Adapter
v1.9.0 Full Platform Dogfood
v2.0.0 Feishu Live + Complex Backend Dogfood
```

Each release must:

```text
open PR
pass CI
merge to main
run full release-readiness on main
tag annotated release
publish GitHub Release
```

## Self-Review

Spec coverage:

- Platform server and MySQL: Tasks 1-3.
- RBAC and audit: Task 4.
- Frontend Web UI: Tasks 5-6.
- Internal wiki UI: Task 7.
- Rollback execution: Task 8.
- Generic observation adapter: Task 9.
- Full platform dogfood: Task 10.
- Feishu live and complex backend dogfood: Task 11.

Placeholder scan:

- This plan has concrete tasks, files, prompts, and verification commands.
- Later releases are scoped as separate tasks because they are independent release increments.

Type consistency:

- API type names are shared through `internal/platform/api/types.go` and `web/src/api/types.ts`.
- Role names are fixed as Viewer, Developer, Reviewer, Admin.

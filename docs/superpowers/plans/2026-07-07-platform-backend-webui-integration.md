# Platform Backend And Web UI Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Verify, merge, and integrate the completed backend platform lane (`v13-platform-backend`) and frontend Web UI lane (`v15-platform-webui`) into a working Devflow platform build backed by local Docker MySQL and a real API-connected Web UI.

**Architecture:** Backend remains the source of truth for platform APIs, MySQL schema, RBAC, audit, and `.devflow` artifact reads. Frontend remains a React/Vite operational console with mock and HTTP client modes. The integration branch reconciles API contracts, adds any missing server/frontend glue, verifies real API browser flows, and prepares release-ready platform milestones.

**Tech Stack:** Go, existing Devflow packages, Docker MySQL on host port `3316`, React/Vite/TypeScript, PowerShell release-readiness, GitHub CLI, Playwright or browser smoke if available.

## Global Constraints

- Do not expand product scope beyond platform backend + Web UI integration.
- Do not delete or migrate `.devflow` markdown/jsonl artifacts into MySQL.
- Do not edit the old unrelated untracked plan files under `docs/superpowers/plans`.
- Do not require Feishu credentials in default verification.
- Do not implement external wiki providers in this integration.
- Do not implement automatic rollback execution in this integration.
- Do not add SLS, Grafana, Datadog, Prometheus, or OpenTelemetry in this integration.
- Backend API contract wins when backend and frontend disagree.
- Frontend must keep mock mode working after real API integration.
- Every code change must be verified before being described as complete.

---

## Starting State

Main worktree:

```text
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
main at 9419d5f, ahead of origin/main by the platformization spec/plan commit
```

Backend worktree:

```text
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v13-platform-backend
branch: v13-platform-backend
head: 5ea49e2
```

Backend lane commits:

```text
0fdf44c Add MySQL platform store and Docker MySQL foundation
6678614 Add devflow server command and platform health API
5641d19 Add artifact bridge and demand read APIs
4823978 Enforce RBAC and record audit events for protected actions
a2426e9 Publish Docker MySQL on host port 3316 to avoid native mysqld conflict
5ea49e2 Document v1.3/v1.4 platform releases and add platform server smoke
```

Frontend worktree:

```text
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v15-platform-webui
branch: v15-platform-webui
head: b35e3d0
```

Frontend lane commit:

```text
b35e3d0 Give Devflow an operational web console so teams can drive the demand loop without the CLI
```

Known backend lane gap:

```text
Full scripts\release-readiness.ps1 -Version 1.4.0 has not been run yet.
```

Known frontend lane gap:

```text
Frontend has been verified in mock mode but has not been run against the real backend API.
```

## Integration Window Prompt

Use this prompt in the third execution window:

```text
You are implementing the Devflow platform backend/Web UI integration lane.

Repository:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent

Read first:
- AGENTS.md
- docs/superpowers/specs/2026-07-07-platformization-server-webui-design.md
- docs/superpowers/plans/2026-07-07-platformization-server-webui-master-plan.md
- docs/superpowers/plans/2026-07-07-platform-backend-webui-integration.md
- docs/release/v1.3.md and docs/release/v1.4.md from the backend branch
- docs/release/v1.5.md from the frontend branch

Current lanes:
- Backend branch v13-platform-backend is complete but needs full release-readiness.
- Frontend branch v15-platform-webui is complete in mock mode.

Mission:
1. Verify backend branch.
2. Merge backend branch into main through PR or local merge following repository practice.
3. Verify frontend branch.
4. Rebase or merge frontend onto backend-updated main.
5. Create integration branch v16-platform-integration.
6. Run Docker MySQL on host port 3316.
7. Start devflow server.
8. Run Web UI with VITE_DEVFLOW_API_MODE=http against the real API.
9. Fix API mismatch, CORS, routing, artifact content, and build issues only.
10. Verify backend, frontend, browser smoke, and release-readiness.

Do not:
- add new feature scope
- remove .devflow artifacts
- edit old unrelated untracked plan files
- require Feishu credentials
- implement external wiki, rollback execution, or observability adapters

Final report:
- merged branches / PRs
- changed files
- real API smoke evidence
- verification results
- remaining risks
```

---

## Task 1: Backend Lane Final Verification

**Files:**
- Read: `D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v13-platform-backend`
- Read: `docker-compose.yml`
- Read: `.env.example`
- Read: `scripts/release-readiness.ps1`
- No source edits expected.

**Interfaces:**
- Consumes: backend branch `v13-platform-backend`.
- Produces: verified backend lane ready for PR/merge.

- [ ] **Step 1: Inspect backend worktree status**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v13-platform-backend
git status --short --branch
git log --oneline --decorate --max-count=8
```

Expected:

```text
branch v13-platform-backend
working tree clean
head at 5ea49e2 or a later backend-lane commit
```

- [ ] **Step 2: Start Docker MySQL**

Run:

```powershell
docker compose up -d mysql
docker compose ps
```

Expected:

```text
mysql service running
host port 3316 mapped to container 3306
```

- [ ] **Step 3: Set DSN**

Run:

```powershell
$env:DEVFLOW_DATABASE_DSN="devflow:devflow@tcp(127.0.0.1:3316)/devflow?parseTime=true"
```

Expected:

```text
environment variable set in this shell
```

- [ ] **Step 4: Run focused backend verification**

Run:

```powershell
go test ./internal/platform/... -count=1
go test ./internal/cli -run "TestServer|TestCLI" -count=1
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected:

```text
all commands exit 0
MySQL tests pass against 127.0.0.1:3316
```

- [ ] **Step 5: Run full backend release-readiness**

Run:

```powershell
$log = Join-Path $env:TEMP "devflow-v14-backend-readiness.log"
Remove-Item -LiteralPath $log -ErrorAction SilentlyContinue
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 1.4.0 *> $log
$code = $LASTEXITCODE
Get-Content -LiteralPath $log -Tail 120
"EXIT=$code"
exit $code
```

Expected:

```text
EXIT=0
release-readiness includes the v1.3 platform server smoke
```

- [ ] **Step 6: Record backend result**

Add a short note to the execution report:

```text
Backend lane verified:
- go test ./internal/platform/... -count=1
- go vet ./...
- go build ./cmd/devflow
- git diff --check
- scripts\release-readiness.ps1 -Version 1.4.0
```

Do not commit unless a fix was required.

## Task 2: Backend Lane PR Or Local Merge

**Files:**
- Branch: `v13-platform-backend`
- Target: `main`

**Interfaces:**
- Consumes: verified backend branch.
- Produces: main containing backend platform foundation.

- [ ] **Step 1: Push backend branch**

Run:

```powershell
git push -u origin v13-platform-backend
```

Expected:

```text
branch pushed
```

- [ ] **Step 2: Create backend PR**

Run:

```powershell
$body = @'
## Summary

- adds Docker MySQL platform store on host port 3316
- adds devflow server command and /api platform health
- adds workspace/demand/artifact read APIs
- adds dev-mode RBAC and audit events
- documents v1.3/v1.4 backend platform releases

## Verification

- go test ./internal/platform/... -count=1
- go vet ./...
- go build ./cmd/devflow
- git diff --check
- scripts\release-readiness.ps1 -Version 1.4.0
'@
$bodyPath = Join-Path $env:TEMP "devflow-v13-backend-pr.md"
Set-Content -LiteralPath $bodyPath -Value $body -Encoding UTF8
gh pr create --base main --head v13-platform-backend --title "Add platform server and MySQL foundation" --body-file $bodyPath
```

Expected:

```text
GitHub PR URL
```

- [ ] **Step 3: Watch CI**

Run:

```powershell
gh pr checks --watch --interval 10
```

Expected:

```text
ubuntu-latest pass
windows-latest pass
```

- [ ] **Step 4: Merge backend PR**

Run:

```powershell
gh pr merge --squash --delete-branch
```

If `gh pr merge` reports a local worktree checkout error but `gh pr view` shows merged, treat the server-side merge as source of truth and sync main manually.

- [ ] **Step 5: Sync main**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git checkout main
git pull --ff-only origin main
git status --short --branch
```

Expected:

```text
main includes backend platform commit
old unrelated untracked plan files may remain
```

## Task 3: Frontend Lane Verification

**Files:**
- Read: `D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v15-platform-webui\web`
- Read: `web/package.json`
- No source edits expected.

**Interfaces:**
- Consumes: frontend branch `v15-platform-webui`.
- Produces: verified mock-first Web UI lane ready to rebase onto backend-updated main.

- [ ] **Step 1: Inspect frontend worktree status**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v15-platform-webui
git status --short --branch
git log --oneline --decorate --max-count=5
```

Expected:

```text
branch v15-platform-webui
working tree clean
head at b35e3d0 or a later frontend-lane commit
```

- [ ] **Step 2: Install frontend dependencies**

Run:

```powershell
cd web
npm install
```

Expected:

```text
install exits 0
package-lock.json remains consistent
```

- [ ] **Step 3: Run frontend verification**

Run:

```powershell
npm test
npx tsc -b
npm run build
```

Expected:

```text
all tests pass
TypeScript build exits 0
Vite build exits 0
```

- [ ] **Step 4: Optional browser smoke in mock mode**

Run:

```powershell
npm run dev -- --host 127.0.0.1
```

Open the shown local URL and verify these routes:

```text
Workspaces
Demands
Demand Detail
Wiki Candidates
Release
Audit
```

Expected:

```text
pages render without blank screen
role switcher changes disabled action states
no obvious text overlap at desktop width
```

- [ ] **Step 5: Record frontend result**

Add a short note to the execution report:

```text
Frontend lane verified:
- npm test
- npx tsc -b
- npm run build
- mock browser smoke, if run
```

Do not commit unless a fix was required.

## Task 4: Rebase Frontend Onto Backend-Updated Main

**Files:**
- Branch: `v15-platform-webui`
- Target base: updated `main` after backend merge.

**Interfaces:**
- Consumes: backend-updated main and verified frontend branch.
- Produces: frontend branch ready for integration.

- [ ] **Step 1: Fetch and rebase**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v15-platform-webui
git fetch origin
git rebase origin/main
```

Expected:

```text
rebase succeeds
```

If conflicts occur, resolve only conflicts caused by backend files or release notes. Do not alter frontend behavior unless conflict resolution requires it.

- [ ] **Step 2: Re-run frontend verification**

Run:

```powershell
cd web
npm test
npx tsc -b
npm run build
```

Expected:

```text
all exit 0 after rebase
```

- [ ] **Step 3: Commit conflict resolution if needed**

If conflict resolution created a commit, use Lore protocol:

```text
Keep the Web UI aligned with the platform API base

The frontend lane was rebased onto the backend platform server branch,
so this resolves contract or release-note overlap without adding new
product behavior.

Constraint: Backend API contract is the integration source of truth.
Confidence: high
Scope-risk: narrow
Tested: npm test; npx tsc -b; npm run build
```

## Task 5: Create Integration Branch

**Files:**
- Branch: `v16-platform-integration`
- Base: backend-updated `main`

**Interfaces:**
- Consumes: backend-updated main and rebased frontend work.
- Produces: integration branch containing backend + frontend.

- [ ] **Step 1: Create integration worktree**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git fetch origin
git checkout main
git pull --ff-only origin main
git worktree add .worktrees\v16-platform-integration -b v16-platform-integration main
cd .worktrees\v16-platform-integration
```

Expected:

```text
new worktree on v16-platform-integration
```

- [ ] **Step 2: Merge frontend branch**

Run:

```powershell
git merge --no-ff v15-platform-webui
```

Expected:

```text
frontend files under web/ appear
docs/release/v1.5.md appears
```

If merge conflicts occur, resolve them by preserving backend API contract and frontend mock mode.

- [ ] **Step 3: Verify integration branch status**

Run:

```powershell
git status --short --branch
git diff --name-status main...HEAD
```

Expected:

```text
clean working tree
diff includes web/ plus frontend release notes
```

## Task 6: Real API Contract Check

**Files:**
- Compare: `internal/platform/api/types.go`
- Compare: `web/src/api/types.ts`
- Modify if needed: `web/src/api/httpClient.ts`
- Modify if needed: `internal/platform/server/router.go`
- Test if needed: `internal/platform/server/*_test.go`
- Test if needed: `web/src/api/*.test.ts`

**Interfaces:**
- Consumes: backend API and frontend HTTP client.
- Produces: matching JSON shapes and routes.

- [ ] **Step 1: List backend routes**

Run:

```powershell
rg -n "Handle|/api/" internal\platform\server internal\platform\api internal\cli\server.go
```

Record exact routes in the execution report.

- [ ] **Step 2: List frontend HTTP routes**

Run:

```powershell
rg -n "fetch\\(|/api/|VITE_DEVFLOW_API" web\src\api web\src
```

Record exact routes in the execution report.

- [ ] **Step 3: Compare route matrix**

Expected route matrix:

```text
GET  /api/health
GET  /api/me
GET  /api/workspaces
POST /api/workspaces
GET  /api/workspaces/{workspaceID}/demands
GET  /api/workspaces/{workspaceID}/demands/{demandKey}
GET  /api/workspaces/{workspaceID}/demands/{demandKey}/artifacts/{name}
GET  /api/workspaces/{workspaceID}/audit
```

Frontend may also contain future wiki/release routes from the mock shell. If backend does not implement them yet, the integration must either:

```text
keep those pages mock-backed with a clear client fallback
or add read-only backend stubs that return empty arrays and correct auth behavior
```

Do not implement full wiki promote/reject or rollback trigger in this integration unless it already exists in backend.

- [ ] **Step 4: Fix type mismatches only**

Acceptable fixes:

```text
rename frontend fields to backend JSON names
add defensive optional fields in frontend types
add backend JSON tags if missing
add empty-array backend handlers for planned-but-not-implemented read-only endpoints
add CORS headers for local Vite dev server
```

Not acceptable in this task:

```text
new wiki workflow behavior
rollback execution
new observation adapter
new auth system
```

- [ ] **Step 5: Commit contract fixes**

If any fixes were made, commit:

```text
Align the Web UI with the platform API contract

The frontend mock shell now talks to the real platform server without
changing the product scope. The backend contract remains the source of
truth, with mock mode preserved for independent UI development.

Constraint: Integration must not introduce wiki provider, rollback execution, or observability adapter scope.
Confidence: high
Scope-risk: moderate
Tested: go test ./internal/platform/... -count=1; npm test; npx tsc -b; npm run build
```

## Task 7: Local MySQL And Server Smoke

**Files:**
- Use: `docker-compose.yml`
- Use: `.env.example`
- Use: `cmd/devflow`
- Use: `.devflow` test root under `%TEMP%`

**Interfaces:**
- Consumes: integrated backend server.
- Produces: running local API at `http://127.0.0.1:8080`.

- [ ] **Step 1: Start Docker MySQL**

Run:

```powershell
docker compose up -d mysql
docker compose ps
```

Expected:

```text
mysql healthy or running
127.0.0.1:3316 available
```

- [ ] **Step 2: Build devflow binary**

Run:

```powershell
go build -o .\dist\devflow-platform.exe .\cmd\devflow
```

Expected:

```text
dist\devflow-platform.exe exists
```

- [ ] **Step 3: Create smoke root with one demand**

Run:

```powershell
$smokeRoot = Join-Path $env:TEMP ("devflow-platform-smoke-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $smokeRoot | Out-Null
.\dist\devflow-platform.exe start --root $smokeRoot --title "Platform smoke demand"
```

Expected:

```text
Created demand platform-smoke-demand or similar demand id
.devflow exists under smoke root
```

- [ ] **Step 4: Start server in background**

Run:

```powershell
$env:DEVFLOW_DATABASE_DSN="devflow:devflow@tcp(127.0.0.1:3316)/devflow?parseTime=true"
$env:DEVFLOW_DEV_USER_EMAIL="admin@example.com"
$env:DEVFLOW_DEV_USER_ROLE="Admin"
$server = Start-Process -FilePath ".\dist\devflow-platform.exe" -ArgumentList @("server","--addr","127.0.0.1:8080","--root",$smokeRoot) -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2
```

Expected:

```text
server process running
```

- [ ] **Step 5: Query API**

Run:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/api/health
Invoke-RestMethod http://127.0.0.1:8080/api/me
Invoke-RestMethod http://127.0.0.1:8080/api/workspaces
```

Expected:

```text
health status ok
current user admin@example.com with Admin role
workspaces returned without server error
```

- [ ] **Step 6: Stop server**

Run:

```powershell
if ($server -and -not $server.HasExited) { Stop-Process -Id $server.Id -Force }
```

Expected:

```text
server stopped
```

## Task 8: Frontend Against Real API

**Files:**
- Use: `web/`
- Modify if needed: `web/src/api/httpClient.ts`
- Modify if needed: `web/src/App.tsx`
- Modify if needed: `web/src/styles.css`

**Interfaces:**
- Consumes: running API server at `http://127.0.0.1:8080`.
- Produces: Web UI rendering real API data in HTTP mode.

- [ ] **Step 1: Start backend server**

Use the server startup commands from Task 7 and keep the server running.

- [ ] **Step 2: Start frontend in HTTP mode**

Run in another shell:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\v16-platform-integration\web
$env:VITE_DEVFLOW_API_MODE="http"
$env:VITE_DEVFLOW_API_BASE="http://127.0.0.1:8080"
npm run dev -- --host 127.0.0.1
```

Expected:

```text
Vite dev server URL printed
```

- [ ] **Step 3: Browser smoke**

Open the Vite URL and verify:

```text
Workspaces page loads from real API
Demands page loads from real API or shows empty state without crashing
Demand detail loads for a real demand when available
Artifact tab loads raw artifact text when available
Audit page loads from real API or shows empty state without crashing
Mock-only pages clearly remain usable only in mock mode if backend endpoints are absent
```

- [ ] **Step 4: Responsive smoke**

Check:

```text
desktop width around 1440px
mobile width around 390px
```

Expected:

```text
no overlapping header/nav/content text
tables scroll or collapse predictably
buttons do not overflow their containers
```

- [ ] **Step 5: Fix only integration defects**

Allowed fixes:

```text
CORS headers
relative route bugs
empty-state rendering
artifact content type handling
HTTP client JSON parsing
responsive overflow
```

Not allowed:

```text
new pages
new feature scope
new backend behavior unrelated to UI connection
```

- [ ] **Step 6: Commit real API smoke fixes**

If fixes were made, commit:

```text
Make the platform console work against the real API

The Web UI already worked in mock mode; this integration pass fixes only
the client/server details needed for local devflow server data to render
without breaking mock mode.

Constraint: Keep integration limited to API, CORS, route, and layout fixes.
Confidence: medium
Scope-risk: moderate
Tested: npm test; npx tsc -b; npm run build; local server browser smoke
```

## Task 9: Full Verification On Integration Branch

**Files:**
- Entire repo.

**Interfaces:**
- Consumes: integrated branch.
- Produces: release-ready evidence.

- [ ] **Step 1: Backend verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected:

```text
all exit 0
```

- [ ] **Step 2: Frontend verification**

Run:

```powershell
cd web
npm test
npx tsc -b
npm run build
```

Expected:

```text
all exit 0
```

- [ ] **Step 3: Release-readiness**

Run from repo root:

```powershell
$log = Join-Path $env:TEMP "devflow-v16-integration-readiness.log"
Remove-Item -LiteralPath $log -ErrorAction SilentlyContinue
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 1.6.0 *> $log
$code = $LASTEXITCODE
Get-Content -LiteralPath $log -Tail 120
"EXIT=$code"
exit $code
```

Expected:

```text
EXIT=0
existing deterministic dogfoods still pass
platform server smoke still passes
```

- [ ] **Step 4: Final status**

Run:

```powershell
git status --short --branch
```

Expected:

```text
clean integration worktree
```

## Task 10: Integration PR And Release Decision

**Files:**
- Branch: `v16-platform-integration`
- Target: `main`

**Interfaces:**
- Consumes: verified integration branch.
- Produces: PR ready for review and optional release tag.

- [ ] **Step 1: Push integration branch**

Run:

```powershell
git push -u origin v16-platform-integration
```

- [ ] **Step 2: Create integration PR**

Run:

```powershell
$body = @'
## Summary

- integrates the platform backend and React/Vite Web UI lanes
- verifies the Web UI against real devflow server APIs
- preserves mock mode for independent frontend development
- keeps MySQL local development on host port 3316

## Verification

- go test ./... -count=1 -timeout 5m
- go vet ./...
- go build ./cmd/devflow
- git diff --check
- npm test
- npx tsc -b
- npm run build
- scripts\release-readiness.ps1 -Version 1.6.0
- local Docker MySQL + devflow server + Web UI HTTP-mode smoke
'@
$bodyPath = Join-Path $env:TEMP "devflow-v16-integration-pr.md"
Set-Content -LiteralPath $bodyPath -Value $body -Encoding UTF8
gh pr create --base main --head v16-platform-integration --title "Integrate platform backend and Web UI" --body-file $bodyPath
```

- [ ] **Step 3: Watch CI**

Run:

```powershell
gh pr checks --watch --interval 10
```

Expected:

```text
ubuntu-latest pass
windows-latest pass
```

- [ ] **Step 4: Merge after CI**

Run:

```powershell
gh pr merge --squash --delete-branch
```

If local worktree checkout error occurs after server-side merge, check:

```powershell
gh pr view --json state,mergedAt,mergeCommit,url
```

Then sync main manually.

- [ ] **Step 5: Sync main and run final gate**

Run:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git checkout main
git pull --ff-only origin main
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
cd web
npm test
npx tsc -b
npm run build
cd ..
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 1.6.0
```

Expected:

```text
all exit 0
```

- [ ] **Step 6: Tag only after final gate**

Choose one release strategy:

```text
Strategy A:
  v1.3.0 backend platform foundation
  v1.4.0 RBAC/audit
  v1.5.0 Web UI mock shell
  v1.6.0 integrated platform preview

Strategy B:
  v1.3.0 platform backend + Web UI preview
```

Recommended for this repo:

```text
Tag v1.6.0 after integration lands, because v1.3-v1.5 were implemented as lanes and v1.6 is the first usable platform preview.
```

Run only after final gate:

```powershell
git tag -a v1.6.0 -m "Devflow v1.6.0 platform backend and Web UI integration"
git push origin v1.6.0
gh release create v1.6.0 --title "Devflow v1.6.0" --notes "Platform backend and Web UI integration preview. See docs/release/v1.3.md, docs/release/v1.4.md, and docs/release/v1.5.md for lane details."
```

## Task 11: Cleanup

**Files:**
- Worktrees:
  - `.worktrees\v13-platform-backend`
  - `.worktrees\v15-platform-webui`
  - `.worktrees\v16-platform-integration`

**Interfaces:**
- Consumes: merged branches.
- Produces: clean local workspace.

- [ ] **Step 1: Remove merged worktrees**

Run after merges:

```powershell
git worktree remove .worktrees\v13-platform-backend
git worktree remove .worktrees\v15-platform-webui
git worktree remove .worktrees\v16-platform-integration
```

If a worktree has uncommitted changes, stop and inspect before removing it.

- [ ] **Step 2: Delete merged local branches**

Run:

```powershell
git branch -D v13-platform-backend
git branch -D v15-platform-webui
git branch -D v16-platform-integration
```

- [ ] **Step 3: Final status**

Run:

```powershell
git status --short --branch
git worktree list
```

Expected:

```text
main in sync with origin/main
old unrelated untracked plan files may remain
no v13/v15/v16 worktrees remain
```

## Self-Review

Spec coverage:

- Backend lane verification and merge: Tasks 1-2.
- Frontend lane verification and rebase: Tasks 3-4.
- Integration branch: Task 5.
- API contract reconciliation: Task 6.
- Docker MySQL and server smoke: Task 7.
- Web UI real API smoke: Task 8.
- Full verification: Task 9.
- PR, merge, and tag decision: Task 10.
- Cleanup: Task 11.

Concrete execution:

- Every task has exact commands, expected results, and branch paths.
- The plan preserves old untracked plan files and does not instruct deletion.
- Product scope excludes external wiki, rollback execution, and observability adapter work.

Type consistency:

- Backend API types live in `internal/platform/api/types.go`.
- Frontend API types live in `web/src/api/types.ts`.
- Backend API contract is the source of truth during Task 6.


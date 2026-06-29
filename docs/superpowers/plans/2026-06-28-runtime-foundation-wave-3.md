# Runtime Foundation Wave 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the next dependency-ordered MewCode runtime packages - `compact`, `mcp`, `skills`, `memory`, `agent`, `teams`, `agents`, and `memory/extractor` - into Devflow under `internal/runtime/`, so the fused runtime gains context compaction, MCP tool bridging, skill loading/execution, project memory, the agent loop, team execution, subagent tooling, and memory extraction.

**Architecture:** This wave keeps Devflow Workflow as the product authority and migrates only reusable runtime packages. Packages are migrated in dependency order with one Lore commit per Go package or Go subpackage, retargeting `mewcode/internal/*` imports to the already-migrated `github.com/jesseedcp/devflow-agent/internal/runtime/*` packages. Windows fixes are made only where the package exposes them during migration.

**Tech Stack:** Go 1.23+, PowerShell on Windows, existing Devflow workflow packages, migrated Wave 1-2 runtime packages, Anthropic Go SDK, OpenAI Go SDK, and standard-library filesystem/process APIs.

---

## Scope And Follow-Up Waves

1. **Wave 1 (done):** `conversation`, `hooks`, `config`, `llm`.
2. **Wave 2 (done):** `worktree`, `tools`, `permissions`, `todo`, `planfile`, `prompt`, `toolresult`.
3. **Wave 3, this plan:** `compact`, `mcp`, `skills`, `memory`, `agent`, `teams`, `agents`, `memory/extractor`.
4. **Wave 4:** `commands`, `history`, `session`, dual-mode `tui`, and the single `cmd/devflow` interactive entry.
5. **Wave 5:** Requirements, Plan, execution, unassociated-change, verification, and closeout runtime integrations.

## Source And Repository Locations

```text
Source snapshot:
D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang

Target worktree:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1
```

Use these variables during execution:

```powershell
$source = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang'
$target = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1'
```

## Dependency Order

Resolved from source imports in the local MewCode snapshot:

| Order | Package | Internal dependencies |
|---|---|---|
| 1 | `compact` | `conversation`, `llm` |
| 2 | `mcp` | `tools` |
| 3 | `skills` | `conversation`, `tools` |
| 4 | `memory` | none |
| 5 | `agent` | `compact`, `conversation`, `hooks`, `llm`, `permissions`, `planfile`, `prompt`, `skills`, `toolresult`, `tools` |
| 6 | `teams` | `agent`, `conversation`, `llm`, `tools` |
| 7 | `agents` | `agent`, `conversation`, `llm`, `permissions`, `teams`, `toolresult`, `tools`, `worktree` |
| 8 | `memory/extractor` | `agent`, `agents`, `conversation`, `llm`, `memory`, `permissions`, `tools` |

`teams` and `agents` are intentionally after `agent` because both packages import the agent loop. `memory/extractor` is last because it imports both `agent` and `agents`.

## Commit And Manifest Rule

Keep the Wave 1-2 rule: one Go package or Go subpackage per commit. Each task updates `docs/migration/mewcode-source-manifest.md` with source hashes, fusion notes, verification evidence, and the unique Lore intent line for that task. Ignore only known untracked build artifacts such as `.gocache/` and `devflow.exe` when checking status.

## Import Replacement Reference

| Target package | Old import | New import |
|---|---|---|
| all Wave 3 packages | `mewcode/internal/agent` | `github.com/jesseedcp/devflow-agent/internal/runtime/agent` |
| all Wave 3 packages | `mewcode/internal/agents` | `github.com/jesseedcp/devflow-agent/internal/runtime/agents` |
| all Wave 3 packages | `mewcode/internal/compact` | `github.com/jesseedcp/devflow-agent/internal/runtime/compact` |
| all Wave 3 packages | `mewcode/internal/config` | `github.com/jesseedcp/devflow-agent/internal/runtime/config` |
| all Wave 3 packages | `mewcode/internal/conversation` | `github.com/jesseedcp/devflow-agent/internal/runtime/conversation` |
| all Wave 3 packages | `mewcode/internal/hooks` | `github.com/jesseedcp/devflow-agent/internal/runtime/hooks` |
| all Wave 3 packages | `mewcode/internal/llm` | `github.com/jesseedcp/devflow-agent/internal/runtime/llm` |
| all Wave 3 packages | `mewcode/internal/memory` | `github.com/jesseedcp/devflow-agent/internal/runtime/memory` |
| all Wave 3 packages | `mewcode/internal/mcp` | `github.com/jesseedcp/devflow-agent/internal/runtime/mcp` |
| all Wave 3 packages | `mewcode/internal/permissions` | `github.com/jesseedcp/devflow-agent/internal/runtime/permissions` |
| all Wave 3 packages | `mewcode/internal/planfile` | `github.com/jesseedcp/devflow-agent/internal/runtime/planfile` |
| all Wave 3 packages | `mewcode/internal/prompt` | `github.com/jesseedcp/devflow-agent/internal/runtime/prompt` |
| all Wave 3 packages | `mewcode/internal/skills` | `github.com/jesseedcp/devflow-agent/internal/runtime/skills` |
| all Wave 3 packages | `mewcode/internal/teams` | `github.com/jesseedcp/devflow-agent/internal/runtime/teams` |
| all Wave 3 packages | `mewcode/internal/toolresult` | `github.com/jesseedcp/devflow-agent/internal/runtime/toolresult` |
| all Wave 3 packages | `mewcode/internal/tools` | `github.com/jesseedcp/devflow-agent/internal/runtime/tools` |
| all Wave 3 packages | `mewcode/internal/worktree` | `github.com/jesseedcp/devflow-agent/internal/runtime/worktree` |

---

## File Map

#### File Map 1: compact
- Create: `internal/runtime/compact/{compact,recovery}.go`
- Create: `internal/runtime/compact/{compact_test,recovery_test}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 2: mcp
- Create: `internal/runtime/mcp/{mcp,mcp_test}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 3: skills
- Create: `internal/runtime/skills/*.go`
- Create: `internal/runtime/skills/*_test.go`
- Create: `internal/runtime/skills/builtins/**`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 4: memory
- Create: `internal/runtime/memory/*.go`
- Create: `internal/runtime/memory/*_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 5: agent
- Create: `internal/runtime/agent/*.go`
- Create: `internal/runtime/agent/*_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 6: teams
- Create: `internal/runtime/teams/*.go`
- Create: `internal/runtime/teams/*_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 7: agents
- Create: `internal/runtime/agents/*.go`
- Create: `internal/runtime/agents/*_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 8: memory/extractor
- Create: `internal/runtime/memory/extractor/*.go`
- Create: `internal/runtime/memory/extractor/*_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

#### File Map 9: Verify Wave 3
- Verify only; no file changes.

---

### Task 1: Migrate `compact`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\compact" | Out-Null
Copy-Item "$source\internal\compact\*.go" "$target\internal\runtime\compact\"
```

- [ ] **Step 2: Retarget runtime imports**

Replace imports in `internal/runtime/compact/*.go`:

```text
mewcode/internal/conversation -> github.com/jesseedcp/devflow-agent/internal/runtime/conversation
mewcode/internal/llm -> github.com/jesseedcp/devflow-agent/internal/runtime/llm
```

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/compact
go test ./internal/runtime/compact -count=1 -timeout 2m
```

Expected: PASS. If tests expose provider-tool serialization regressions, fix only `internal/runtime/compact` or the directly implicated migrated runtime package and record the fusion change in the manifest.

- [ ] **Step 4: Update the migration source manifest**

Append:

```markdown
### compact

- Source: `internal/compact`
- Target: `internal/runtime/compact`
- Source files:
  - `compact.go`: `9867CE00E1DAF0D36BDC8D146427E4C3A2B0237116C37FC7EB4A2CA64DC93EE9`
  - `compact_test.go`: `A5C95E382A511D35199D2EFC59A1B714318899484AD396EDDFD1546153C1116F`
  - `recovery.go`: `DD686F65C29DEE5595AB9F2F1491A1A7FDDFCF9ABE137D789F3441C5C38C1A45`
  - `recovery_test.go`: `63460ED0B4F51E2585932E9C77C075F12D935A6A9477E6552C186AE93196E142`
- Fusion changes:
  - Move context compaction under the Devflow runtime boundary.
  - Retarget conversation and llm imports to the migrated runtime packages.
- Windows changes: none required unless package tests expose path or shell behavior.
- Verification: `gofmt -w internal/runtime/compact`; `go test ./internal/runtime/compact -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring context compaction into the Devflow runtime`
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit only compact and manifest**

```powershell
git add -- internal/runtime/compact docs/migration/mewcode-source-manifest.md
git commit -m "Bring context compaction into the Devflow runtime" -m "Migrate MewCode's compact package so the fused runtime can shrink conversation history while preserving provider-facing message semantics." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Compaction must preserve tool-use, tool-result, and thinking metadata used by provider serializers" -m "Tested: go test ./internal/runtime/compact -count=1 -timeout 2m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Long-running production conversation compaction under live model load"
git status --short
```

---

### Task 2: Migrate `mcp`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\mcp" | Out-Null
Copy-Item "$source\internal\mcp\*.go" "$target\internal\runtime\mcp\"
```

- [ ] **Step 2: Retarget tool imports**

Replace in `internal/runtime/mcp/*.go`:

```text
mewcode/internal/tools -> github.com/jesseedcp/devflow-agent/internal/runtime/tools
```

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/mcp
go test ./internal/runtime/mcp -count=1 -timeout 2m
```

Expected: PASS.

- [ ] **Step 4: Update the migration source manifest**

Append:

```markdown
### mcp

- Source: `internal/mcp`
- Target: `internal/runtime/mcp`
- Source files:
  - `mcp.go`: `87B6063D5F347ECFAAC79AB544BD93FD100E621BEE6A91635CCB1D4197FD1E08`
  - `mcp_test.go`: `B82B35AC1BAE5E798AB638658E1818E7A9E31AEA19D3CBE9877C396FA0CCB21C`
- Fusion changes:
  - Move MCP runtime support under the Devflow runtime boundary.
  - Retarget tool interfaces to `internal/runtime/tools`.
- Windows changes: verify command/process handling through package tests; fix only package-local Windows issues.
- Verification: `gofmt -w internal/runtime/mcp`; `go test ./internal/runtime/mcp -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring MCP tool bridging into the Devflow runtime`
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 6: Commit only mcp and manifest**

```powershell
git add -- internal/runtime/mcp docs/migration/mewcode-source-manifest.md
git commit -m "Bring MCP tool bridging into the Devflow runtime" -m "Migrate MewCode's MCP package so Devflow can expose external MCP tools through the same runtime tool interface used by local tools." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: moderate" -m "Directive: MCP tools must stay behind the runtime tool abstraction and must not bypass Workflow state authority" -m "Tested: go test ./internal/runtime/mcp -count=1 -timeout 2m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Live third-party MCP server sessions"
git status --short
```

---

### Task 3: Migrate `skills`

- [ ] **Step 1: Copy source files and bundled skills**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\skills" | Out-Null
Copy-Item "$source\internal\skills\*.go" "$target\internal\runtime\skills\"
Copy-Item "$source\internal\skills\builtins" "$target\internal\runtime\skills\" -Recurse
```

- [ ] **Step 2: Retarget imports**

Replace in `internal/runtime/skills/*.go`:

```text
mewcode/internal/conversation -> github.com/jesseedcp/devflow-agent/internal/runtime/conversation
mewcode/internal/tools -> github.com/jesseedcp/devflow-agent/internal/runtime/tools
```

- [ ] **Step 3: Retarget product identity in bundled skills**

Search:

```powershell
rg -n "mewcode|MewCode|MEWCODE|\.mewcode" internal/runtime/skills
```

For user-facing product identity, replace with Devflow wording. Keep legacy `.mewcode` mentions only when a test or migration note explicitly verifies fallback behavior.

- [ ] **Step 4: Format and run package tests**

```powershell
gofmt -w internal/runtime/skills
go test ./internal/runtime/skills -count=1 -timeout 2m
```

- [ ] **Step 5: Update the migration source manifest**

Append:

```markdown
### skills

- Source: `internal/skills`
- Target: `internal/runtime/skills`
- Source files:
  - `builtins.go`: `5D2795491E892BFF226B0D32B62C283B8873AAEE0866C81DFFC545FC18DAFF8D`
  - `catalog.go`: `D885C94BC0C6269DC055C3841D2CA238554D672409114A2FBFE82AC416BAE0D3`
  - `catalog_test.go`: `A637432FBF45BEBEAA8D612D24E8403C6D3D7809D2AE5334BD4A297D1944A1B0`
  - `directory.go`: `264E9CF8F086F198A031987CC834D521E5C65A7B2BBE5E2AB6ECDE07B7BE8965`
  - `executor.go`: `9F60C2EC418E11389CB9A2408295E9F5A5CF7465A1A26A53AB29C39D2D02E583`
  - `executor_test.go`: `B615B52E0CBD16A75CC0F6FCEB8BBA01F286C2D18AAA1C265BE3F9EB7BCF5E86`
  - `install.go`: `8A25837BE08B94A56364594E953420159A19F4315410D4C2AFCE02684ACA38B1`
  - `install_test.go`: `78272D5C0AAC1C897DD487C3E81FC814B217AFAB0167716E744F6DDC469BEF59`
  - `install_tool.go`: `AE6C7B8B60AF30236DA0EC5ADF93D43672BC8155A4B9546CFAAB66CB335FBAAB`
  - `load_skill_tool.go`: `DBC44ADCFA374FAEE864DC11E6813810EE45C919A184CA67CE7FC2F008B471CB`
  - `parse_resume.go`: `43E47C661832E4CE98EE262B4A1FFEBD024507FB342E1FE87AB7FAE5F2A02EEF`
  - `parser.go`: `55D8D15A63FE92E74B7813EBCA308D1134755374AAF46F11CC0C3FC9A4918D33`
  - `skills.go`: `965FE1BAAEE5D00F1F931AA9EA490335D71BEA9A90057B5A09C3708B10053ADB`
  - `skills_test.go`: `E67661627E671375B5ADEA2435BACF3A00D27E53BEC757B3084AB02F39A6C8F4`
- Fusion changes:
  - Move skill catalog, parser, executor, installation tools, and bundled skills under the Devflow runtime boundary.
  - Retarget conversation and tools imports to runtime packages.
  - Replace user-facing MewCode identity in bundled skills with Devflow where applicable.
- Windows changes: verify path handling and skill installation tests on Windows.
- Verification: `gofmt -w internal/runtime/skills`; `go test ./internal/runtime/skills -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring skill loading and execution into the Devflow runtime`
```

- [ ] **Step 6: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 7: Commit only skills and manifest**

```powershell
git add -- internal/runtime/skills docs/migration/mewcode-source-manifest.md
git commit -m "Bring skill loading and execution into the Devflow runtime" -m "Migrate MewCode's skill runtime so Devflow can load, install, parse, and execute reusable skills through the same tool and conversation boundaries as the rest of the fused runtime." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: moderate" -m "Directive: Skills may guide runtime behavior but must not advance Workflow stages without Workflow confirmation" -m "Tested: go test ./internal/runtime/skills -count=1 -timeout 2m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Remote skill installation against private repositories"
git status --short
```

---

### Task 4: Migrate `memory`

- [ ] **Step 1: Copy the base memory package only**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\memory" | Out-Null
Copy-Item "$source\internal\memory\*.go" "$target\internal\runtime\memory\"
```

Do not copy `internal/memory/extractor` in this task; it is Task 8 because it depends on agent/subagent packages.

- [ ] **Step 2: Format and run package tests**

```powershell
gofmt -w internal/runtime/memory
go test ./internal/runtime/memory -count=1 -timeout 2m
```

Expected: PASS.

- [ ] **Step 3: Scan for legacy state directory names**

```powershell
rg -n "mewcode|MewCode|MEWCODE|\.mewcode" internal/runtime/memory
```

If paths or prompts write new memory state to `.mewcode`, retarget new writes to `.devflow` and keep `.mewcode` only as explicit legacy fallback.

- [ ] **Step 4: Update the migration source manifest**

Append:

```markdown
### memory

- Source: `internal/memory`
- Target: `internal/runtime/memory`
- Source files:
  - `find_relevant_memories.go`: `5D67908640CE24FC78B55152BCD80469BF27DC50DEA780344D944F58646CF389`
  - `find_relevant_memories_test.go`: `09EA599B83878151C48E67AF3E35AB05FCD02B43A06CA37CE57DC6EE04B252FE`
  - `instructions.go`: `58320DEAB80889340CDEC2664CC935DBE4B03805AB24BF23AF7F636FB4F3B9A4`
  - `instructions_test.go`: `A09A71C2B49CEB65E42B654EB54C4E94AC9FD16D2B760DC4CA5B68FC94C11687`
  - `memdir.go`: `8D71BA3798E9A0BB88436BA71CB25496775708D07AB176176CB6C9FA9D42F23C`
  - `memory.go`: `DCDEFFB77D8F68D6F0FE423325B5258085C7FF6AC577F65AA55A4A3CE6E25CD8`
  - `memory_age.go`: `AE2B527D545062F0CDCDA78F903AE0316C1165A124F2C4B83DC9E5EA07269FEF`
  - `memory_age_test.go`: `5E62CE6D833384501E30EAAE788A60CD256B4F449351F2A94129F833CDBBED5A`
  - `memory_scan.go`: `310CCDFFDF4A18B20EADEE00B563A9A57BF05C7105926F87E87C8A8ACF98C346`
  - `memory_scan_test.go`: `F60673292CB55B83BFCB870CC30E7AC3F142F5B6DBAA569538E69FE5EF831841`
  - `memory_test.go`: `36EEF1591E66050564099F343DC8CFF9807E3D6562E0E531331F39ADCAD29FDD`
  - `memory_types.go`: `DCEB7405259E43749117CB978491A240132E03E0E5AEAE896C81747EB0FE5D82`
  - `paths.go`: `5D2E10EE5A1497465BE09CECC077929CBEED3AAE7AE5B0CF78B976EA6717520A`
- Fusion changes:
  - Move base project memory under the Devflow runtime boundary.
  - Keep memory/extractor for a later task because it depends on the agent and subagent packages.
- Windows changes: verify path handling through package tests.
- Verification: `gofmt -w internal/runtime/memory`; `go test ./internal/runtime/memory -count=1 -timeout 2m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring project memory into the Devflow runtime`
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 6: Commit only memory and manifest**

```powershell
git add -- internal/runtime/memory docs/migration/mewcode-source-manifest.md
git commit -m "Bring project memory into the Devflow runtime" -m "Migrate MewCode's base memory package so Devflow can scan, store, age, and inject project memories through a runtime-owned memory boundary before adding extractor-driven candidates." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: moderate" -m "Directive: Stable knowledge candidates from Workflow closeout must remain reviewable before becoming runtime memory" -m "Tested: go test ./internal/runtime/memory -count=1 -timeout 2m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Cross-session memory reuse through the future TUI integration"
git status --short
```

---

### Task 5: Migrate `agent`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\agent" | Out-Null
Copy-Item "$source\internal\agent\*.go" "$target\internal\runtime\agent\"
```

- [ ] **Step 2: Retarget imports**

Apply the import replacement reference to `internal/runtime/agent/*.go`, especially:

```text
mewcode/internal/compact -> github.com/jesseedcp/devflow-agent/internal/runtime/compact
mewcode/internal/config -> github.com/jesseedcp/devflow-agent/internal/runtime/config
mewcode/internal/conversation -> github.com/jesseedcp/devflow-agent/internal/runtime/conversation
mewcode/internal/hooks -> github.com/jesseedcp/devflow-agent/internal/runtime/hooks
mewcode/internal/llm -> github.com/jesseedcp/devflow-agent/internal/runtime/llm
mewcode/internal/permissions -> github.com/jesseedcp/devflow-agent/internal/runtime/permissions
mewcode/internal/planfile -> github.com/jesseedcp/devflow-agent/internal/runtime/planfile
mewcode/internal/prompt -> github.com/jesseedcp/devflow-agent/internal/runtime/prompt
mewcode/internal/skills -> github.com/jesseedcp/devflow-agent/internal/runtime/skills
mewcode/internal/toolresult -> github.com/jesseedcp/devflow-agent/internal/runtime/toolresult
mewcode/internal/tools -> github.com/jesseedcp/devflow-agent/internal/runtime/tools
```

- [ ] **Step 3: Keep live tests opt-in**

Run:

```powershell
rg -n "LIVE|DEVFLOW_LIVE|MEWCODE|mewcode|\.mewcode" internal/runtime/agent
```

If a live test uses a legacy env gate, rename the new gate to `DEVFLOW_LIVE_AGENT=1` and keep API keys out of output. If a user-facing message says MewCode, retarget it to Devflow unless it documents a legacy fallback.

- [ ] **Step 4: Format and run package tests**

```powershell
gofmt -w internal/runtime/agent
go test ./internal/runtime/agent -count=1 -timeout 3m
```

Expected: PASS with live tests skipped unless `DEVFLOW_LIVE_AGENT=1` is set.

- [ ] **Step 5: Run optional live agent smoke only when configured**

```powershell
$env:OPENAI_API_KEY = [Environment]::GetEnvironmentVariable('OPENAI_API_KEY','User')
if (-not $env:OPENAI_API_KEY) { $env:OPENAI_API_KEY = [Environment]::GetEnvironmentVariable('OPENAI_API_KEY','Machine') }
if ($env:OPENAI_API_KEY) {
  $env:DEVFLOW_LIVE_AGENT='1'
  go test ./internal/runtime/agent -run Live -count=1 -v -timeout 120s
  $code=$LASTEXITCODE
  Remove-Item Env:DEVFLOW_LIVE_AGENT -ErrorAction SilentlyContinue
  Remove-Item Env:OPENAI_API_KEY -ErrorAction SilentlyContinue
  exit $code
} else {
  Write-Output 'SKIP: OPENAI_API_KEY not configured'
}
```

- [ ] **Step 6: Update the migration source manifest**

Append:

```markdown
### agent

- Source: `internal/agent`
- Target: `internal/runtime/agent`
- Source files:
  - `agent.go`: `213F8A091B5BBE4F17725522D912732659B26B36E4AF93BB1A280AAC1956D69F`
  - `agent_live_test.go`: `E3A456E25A7028E78CBE80D71864C47070E01D9719CF815EF7BCAF4EF96C0676`
  - `agent_test.go`: `8C00E83DEA66E0AD2DC624A8D84078CA7C0EF75115DC5ED1E099F1A308CAFF57`
  - `events.go`: `1140F44FBAA608BB6FA4F34AE8FA9AD3BF3D94845F06D81B6A3FBA4BDABFDAAE`
  - `skills_test.go`: `D5223562CA3A86FA19D4B8EA061F3AEDC7E9C60F9965D81AC4F2BE4A51A70E22`
  - `streaming_executor.go`: `1E707421E819E52E9530D4632B14A6B62017169EE4F05CB71C8E92A2E64CB320`
- Fusion changes:
  - Move the agent loop under the Devflow runtime boundary.
  - Retarget all Wave 1-3 runtime imports.
  - Keep live agent tests opt-in and key-safe.
- Windows changes: verify package tests and command execution behavior on Windows.
- Verification: `gofmt -w internal/runtime/agent`; `go test ./internal/runtime/agent -count=1 -timeout 3m`; optional live smoke when `OPENAI_API_KEY` is configured; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring the agent loop into the Devflow runtime`
```

- [ ] **Step 7: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 8: Commit only agent and manifest**

```powershell
git add -- internal/runtime/agent docs/migration/mewcode-source-manifest.md
git commit -m "Bring the agent loop into the Devflow runtime" -m "Migrate MewCode's agent loop so Devflow can run model streams, tool calls, permissions, hooks, compaction, skills, plan files, and tool-result accounting through one runtime-owned execution surface." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: broad" -m "Directive: Runtime agent execution returns evidence to Workflow; it must not directly advance Devflow demand stages" -m "Tested: go test ./internal/runtime/agent -count=1 -timeout 3m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Full Requirements-to-closeout product loop until Wave 5 integration"
git status --short
```

---

### Task 6: Migrate `teams`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\teams" | Out-Null
Copy-Item "$source\internal\teams\*.go" "$target\internal\runtime\teams\"
```

- [ ] **Step 2: Retarget imports**

Replace in `internal/runtime/teams/*.go`:

```text
mewcode/internal/agent -> github.com/jesseedcp/devflow-agent/internal/runtime/agent
mewcode/internal/conversation -> github.com/jesseedcp/devflow-agent/internal/runtime/conversation
mewcode/internal/llm -> github.com/jesseedcp/devflow-agent/internal/runtime/llm
mewcode/internal/tools -> github.com/jesseedcp/devflow-agent/internal/runtime/tools
```

- [ ] **Step 3: Scan platform-specific launcher assumptions**

```powershell
rg -n "tmux|iTerm|osascript|bash|powershell|cmd.exe|windows|darwin|linux" internal/runtime/teams
```

Keep Unix/macOS team backends as migrated capabilities, but ensure Windows package tests either use in-process/file-mailbox behavior or skip unsupported terminal backends explicitly.

- [ ] **Step 4: Format and run package tests**

```powershell
gofmt -w internal/runtime/teams
go test ./internal/runtime/teams -count=1 -timeout 3m
```

- [ ] **Step 5: Update the migration source manifest**

Append:

```markdown
### teams

- Source: `internal/teams`
- Target: `internal/runtime/teams`
- Source files:
  - `backend.go`: `FA2347779E2BC51C6EA53CE1A723804CC725D5378E3DE64E2FCE2C34655B49DE`
  - `coordinator.go`: `8678AB495D60B5CC6B1600CDE76FB93CEA938005FE08A5E9128A7952FEC7C8C0`
  - `filemailbox.go`: `19BED3AD553147D8EDA6A35D787DA03458D8D9C7C24B776036A90F7CB8EFBB68`
  - `filemailbox_test.go`: `FAFD93FFDEAF6EFDAC28AEB2A890D523BDB79A7E0CDE1D41CF3CC09B0DFC851A`
  - `inprocess.go`: `0DD930B7AB2F6659D132A3F9105080BC803A11046DA0982D316994642A8C60E9`
  - `iterm.go`: `D0BEEE3E87F7DDE924FE479BE9F1937E16749AAEC9979D3B51F5FD992F483755`
  - `runner.go`: `E694E9B06D1743F9A5EF05171F4CAE0F59D2902E4D6BEA8A9F056112C86A4F2A`
  - `runner_test.go`: `07ED1135291BB926CF0FE6BF12CC54A1D7CBC4B6CDB0F8E6D9EAF10DA45B80ED`
  - `spawn.go`: `D5237870E28FD066BB7E2D17530D01664D9D7671538000E6007BD70A6951EF6E`
  - `teams.go`: `1BF0DC04F42697975357C99215B4C1CD3946575F0A55DF56A5169C42F26A3350`
  - `teams_test.go`: `C6A4B7E593340CBCD9BBD83374D3557B8F32BA40FC88B8B54DE4DFD5CA9FDA13`
  - `tmux.go`: `DF6183B3114E96D506DEA1BF8B444DDF6391DA6A8F46759047A7410FC1AF339A`
  - `tools.go`: `E8F670EBDAB6D6325558CE900F1ABC428940B7F7656FE6C45BBB1CEB1696C145`
- Fusion changes:
  - Move team orchestration under the Devflow runtime boundary.
  - Retarget agent, conversation, llm, and tools imports to runtime packages.
- Windows changes: package tests must pass on Windows; unsupported terminal backends should be skipped or gated instead of failing by default.
- Verification: `gofmt -w internal/runtime/teams`; `go test ./internal/runtime/teams -count=1 -timeout 3m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring team orchestration into the Devflow runtime`
```

- [ ] **Step 6: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 7: Commit only teams and manifest**

```powershell
git add -- internal/runtime/teams docs/migration/mewcode-source-manifest.md
git commit -m "Bring team orchestration into the Devflow runtime" -m "Migrate MewCode's team orchestration package so Devflow can coordinate multiple runtime agents through file mailbox, in-process, and terminal-backed execution surfaces." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: broad" -m "Directive: Team workers execute bounded runtime tasks and report evidence upward; Workflow remains the stage authority" -m "Tested: go test ./internal/runtime/teams -count=1 -timeout 3m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Real tmux/iTerm multi-pane orchestration on this Windows host"
git status --short
```

---

### Task 7: Migrate `agents`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\agents" | Out-Null
Copy-Item "$source\internal\agents\*.go" "$target\internal\runtime\agents\"
```

- [ ] **Step 2: Retarget imports**

Apply the import replacement reference to `internal/runtime/agents/*.go`, especially `agent`, `conversation`, `llm`, `permissions`, `teams`, `toolresult`, `tools`, and `worktree`.

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/agents
go test ./internal/runtime/agents -count=1 -timeout 3m
```

- [ ] **Step 4: Update the migration source manifest**

Append:

```markdown
### agents

- Source: `internal/agents`
- Target: `internal/runtime/agents`
- Source files:
  - `agent_tool.go`: `1F18247A0B1AB4F187BBE64CEBE7672E7C63E34076541B4F5F34587C3A575CE3`
  - `agent_tool_test.go`: `68BC194991C5F110BDDA84BF22A3AC26EA0283F8D99D8284EE46AC47FD5C29DF`
  - `definition.go`: `3AF7F9A5998B1ECB45ED1520ECAF3567A1D187CDE73836A0E9E77F740F6B1FBC`
  - `loader.go`: `1E2B2E454307223206F5BF11760BA5BDAEA8C56BA2AFA4FD0A757B4D41F153A5`
  - `loader_test.go`: `0413680440C89ABF0286D185904F85A8922A62D15BABAEBE7CFB77E47D235EB7`
  - `subagent.go`: `7BA50304381B99095C29A5DF6B068B5907CA07F412ECC6E80F475178011F728C`
  - `tool_filter.go`: `03BD0A5C1ACF50707D6E300D70FFD2EFB28D69AF46958016BBC568AD4833B144`
  - `tool_filter_test.go`: `229B615C7AFD05A0841AF51D9A54220C15BDA994BD53152E2C0D98B5C155711D`
  - `verification_prompt.go`: `FD1655CBC922F7BFD6310D866A290067B21DEEFF93E173E02C7DB91044B5D5C5`
- Fusion changes:
  - Move subagent definitions, loaders, tool filtering, and agent tools under the Devflow runtime boundary.
  - Retarget all runtime imports.
- Windows changes: verify worktree and team-tool interactions on Windows package tests.
- Verification: `gofmt -w internal/runtime/agents`; `go test ./internal/runtime/agents -count=1 -timeout 3m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring subagent tooling into the Devflow runtime`
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 6: Commit only agents and manifest**

```powershell
git add -- internal/runtime/agents docs/migration/mewcode-source-manifest.md
git commit -m "Bring subagent tooling into the Devflow runtime" -m "Migrate MewCode's subagent package so Devflow can load agent definitions, filter tool access, and expose subagent execution through the runtime agent and team boundaries." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: broad" -m "Directive: Subagents must be bounded runtime executors and must not create or approve Workflow stage transitions on their own" -m "Tested: go test ./internal/runtime/agents -count=1 -timeout 3m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: Real nested subagent execution against long-running provider sessions"
git status --short
```

---

### Task 8: Migrate `memory/extractor`

- [ ] **Step 1: Copy the source subpackage**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\memory\extractor" | Out-Null
Copy-Item "$source\internal\memory\extractor\*.go" "$target\internal\runtime\memory\extractor\"
```

- [ ] **Step 2: Retarget imports**

Apply the import replacement reference to `internal/runtime/memory/extractor/*.go`, especially `agent`, `agents`, `conversation`, `llm`, `memory`, `permissions`, and `tools`.

- [ ] **Step 3: Retarget extraction prompt identity**

Search:

```powershell
rg -n "mewcode|MewCode|MEWCODE|\.mewcode" internal/runtime/memory/extractor
```

Replace user-facing identity with Devflow and keep legacy names only where the text explicitly describes migration fallback.

- [ ] **Step 4: Format and run package tests**

```powershell
gofmt -w internal/runtime/memory/extractor
go test ./internal/runtime/memory/extractor -count=1 -timeout 3m
```

- [ ] **Step 5: Update the migration source manifest**

Append:

```markdown
### memory/extractor

- Source: `internal/memory/extractor`
- Target: `internal/runtime/memory/extractor`
- Source files:
  - `extractor.go`: `D9C561481A19E336643D6EBC5C9024300A99BDD00E2AF7CB688F993119CDDFA6`
  - `extractor_test.go`: `1EFE4E03AFED2E6CFADD35B35D6017DFF31DBA8BA4C7F39E87A24F37507BA893`
  - `prompts.go`: `D3D280C2053C65A97906F63F5241450279BEA98B768E25E957B4180B0B1CBAF4`
  - `prompts_test.go`: `E7A989D62D70B7A772FBA04F78D4C27EB2F05A0C7EB0925FBD4AE31B7D8775EF`
- Fusion changes:
  - Move memory extraction under the Devflow runtime memory boundary.
  - Retarget agent, agents, conversation, llm, memory, permissions, and tools imports to runtime packages.
  - Retarget prompt identity from MewCode to Devflow where user-facing.
- Windows changes: verify package tests on Windows and keep generated memory paths portable.
- Verification: `gofmt -w internal/runtime/memory/extractor`; `go test ./internal/runtime/memory/extractor -count=1 -timeout 3m`; `go test ./internal/runtime/... -count=1 -timeout 5m`; `go vet ./...`; `go build ./cmd/devflow`; `git diff --check`
- Lore intent: `Bring memory extraction into the Devflow runtime`
```

- [ ] **Step 6: Run repository verification**

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- [ ] **Step 7: Commit only memory/extractor and manifest**

```powershell
git add -- internal/runtime/memory/extractor docs/migration/mewcode-source-manifest.md
git commit -m "Bring memory extraction into the Devflow runtime" -m "Migrate MewCode's memory extractor so Devflow can propose reusable project knowledge through runtime agents while keeping Workflow closeout responsible for stable-knowledge approval." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: medium" -m "Scope-risk: broad" -m "Directive: Extracted memory candidates must stay reviewable and must not silently become Workflow-approved knowledge" -m "Tested: go test ./internal/runtime/memory/extractor -count=1 -timeout 3m; go test ./internal/runtime/... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" -m "Not-tested: End-to-end closeout-to-memory approval loop until Wave 5"
git status --short
```

---

### Task 9: Verify Wave 3 As A Clean Eight-Commit Foundation

- [ ] **Step 1: Confirm the eight package commits**

```powershell
git log -8 --format='%H %s'
```

Expected intent lines, newest first:

```text
Bring memory extraction into the Devflow runtime
Bring subagent tooling into the Devflow runtime
Bring team orchestration into the Devflow runtime
Bring the agent loop into the Devflow runtime
Bring project memory into the Devflow runtime
Bring skill loading and execution into the Devflow runtime
Bring MCP tool bridging into the Devflow runtime
Bring context compaction into the Devflow runtime
```

- [ ] **Step 2: Confirm each migrated package is represented in the manifest**

```powershell
Select-String -Path docs/migration/mewcode-source-manifest.md -Pattern '^### (compact|mcp|skills|memory|agent|teams|agents|memory/extractor)$'
Select-String -Path docs/migration/mewcode-source-manifest.md -Pattern 'Lore intent:'
```

Expected: eight Wave 3 package sections and a matching Lore intent for each migrated package, plus Wave 1-2 sections.

- [ ] **Step 3: Run final verification**

```powershell
git status --short --branch
go test ./internal/runtime/... -count=1 -timeout 5m
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: status shows only expected untracked build/cache artifacts; all Go commands exit 0.

- [ ] **Step 4: Record known remaining scope**

Do not create another code commit. Report that Wave 3 provides context compaction, MCP bridging, skills, project memory, the agent loop, teams, subagents, and memory extraction. It does not yet expose commands, history, session, the dual-mode TUI, the single interactive `cmd/devflow` entry, or the Requirements/Plan/implementation/verification/closeout product integrations. Those remain Wave 4-5.

---

## Plan Self-Review

- **Spec coverage:** This plan covers the Wave 3 runtime packages named in the fusion design: `compact`, `agent`, `skills`, `mcp`, `agents`, `teams`, `memory`, and `memory/extractor`.
- **Dependency order verified:** The order follows actual source imports. Base memory is before `memory/extractor`; `agent` is before `teams` and `agents`; `memory/extractor` is last because it imports `agent` and `agents`.
- **Placeholder scan:** No step contains unresolved placeholder wording or unspecified commands. Each task has concrete paths, commands, manifest text, and commit text.
- **Type consistency:** Import targets consistently use `github.com/jesseedcp/devflow-agent/internal/runtime/<pkg>`.
- **Commit consistency:** Each task modifies only one Go package or subpackage plus `docs/migration/mewcode-source-manifest.md`.
- **Deliberate gaps:** Wave 4-5 product integration remains outside this plan; this wave migrates runtime foundations only.

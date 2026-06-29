# Runtime Foundation Wave 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the seven dependency-ordered MewCode runtime packages - `worktree`, `tools`, `permissions`, `todo`, `planfile`, `prompt`, `toolresult` - into Devflow under `internal/runtime/`, so the fused runtime gains git worktree management, the tool registry, permission gates, todo tracking, plan-file management, prompt building, and tool-result accounting.

**Architecture:** Each Go package migrates independently in dependency order, one Lore commit per package, retargeting internal imports to the Devflow runtime boundary and verifying on the supported Windows host. Devflow remains the workflow authority; this wave only migrates reusable runtime foundations without adding stage transitions or TUI behavior.

**Tech Stack:** Go 1.23+, PowerShell on Windows, existing Devflow file-backed workflow, `gopkg.in/yaml.v3`, Anthropic Go SDK and OpenAI Go SDK (already in `go.mod` from Wave 1).

---

## Scope And Follow-Up Waves

1. **Wave 1 (done):** `conversation`, `hooks`, `config`, `llm`.
2. **Wave 2, this plan:** `worktree`, `tools`, `permissions`, `todo`, `planfile`, `prompt`, `toolresult`.
3. **Wave 3:** `compact`, `agent`, `skills`, `mcp`, `agents`, `teams`, `memory`, `memory/extractor`.
4. **Wave 4:** `commands`, `history`, `session`, dual-mode `tui`, and the single `cmd/devflow` interactive entry.
5. **Wave 5:** Requirements, Plan, execution, unassociated-change, verification, and closeout runtime integrations.

## Source And Repository Locations

```text
Source snapshot:
D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang

Target worktree:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1
```

```powershell
$source = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang'
$target = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1'
```

## Dependency Order

Resolved from actual imports in the source snapshot:

| Order | Package | Internal dependencies |
|---|---|---|
| 1 | `worktree` | none |
| 2 | `tools` | `worktree` |
| 3 | `permissions` | `tools` |
| 4 | `todo` | `tools` |
| 5 | `planfile` | none |
| 6 | `prompt` | none |
| 7 | `toolresult` | `conversation` (migrated in Wave 1) |

None of the seven packages import any Wave 3+ package, so the wave is self-contained.

## Commit And Manifest Rule

Same as Wave 1: one package per commit; each task ends with a commit whose unique Lore intent line is recorded in `docs/migration/mewcode-source-manifest.md`. Do not start the next task before the commit exists and `git status --short` is empty (ignoring untracked build artifacts like `devflow.exe` and `.gocache/`).

## Import Replacement Reference

| Package | File(s) | Old import | New import |
|---|---|---|---|
| tools | enter_worktree.go, exit_worktree.go | `mewcode/internal/worktree` | `github.com/jesseedcp/devflow-agent/internal/runtime/worktree` |
| permissions | permissions.go, permissions_test.go | `mewcode/internal/tools` | `github.com/jesseedcp/devflow-agent/internal/runtime/tools` |
| todo | tools.go | `mewcode/internal/tools` | `github.com/jesseedcp/devflow-agent/internal/runtime/tools` |
| toolresult | budget.go, budget_test.go, reconstruct.go | `mewcode/internal/conversation` | `github.com/jesseedcp/devflow-agent/internal/runtime/conversation` |

---
## File Map

### Task 1: worktree
- Create: `internal/runtime/worktree/{agent,changes,cleanup,create,env,filesystem,notice,session,setup,validate}.go`
- Create: `internal/runtime/worktree/*_test.go` (10 test files)
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 2: tools
- Create: `internal/runtime/tools/{ask_user,bash,descriptions,edit_file,enter_worktree,exit_worktree,glob,grep,read_file,tool_search,tool,write_file}.go`
- Create: `internal/runtime/tools/{glob_test,tool_search_test}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 3: permissions
- Create: `internal/runtime/permissions/{permissions,permissions_test}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 4: todo
- Create: `internal/runtime/todo/{store,todo,tools}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 5: planfile
- Create: `internal/runtime/planfile/planfile.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 6: prompt
- Create: `internal/runtime/prompt/{builder,plan_mode,sections}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 7: toolresult
- Create: `internal/runtime/toolresult/{budget,reconstruct,record,state}.go`
- Create: `internal/runtime/toolresult/{budget_test,record_test,state_test}.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 8: Verify
- Verify only; no file changes.

---
### Task 1: Migrate `worktree`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\worktree" | Out-Null
Copy-Item "$source\internal\worktree\*.go" "$target\internal\runtime\worktree\"
```

No internal imports to retarget. Keep the package name `worktree` and all public type names.

- [ ] **Step 2: Format and run package tests**

```powershell
gofmt -w internal/runtime/worktree
go test ./internal/runtime/worktree -count=1 -timeout 2m
```

Expected: PASS. The `env.go` git-subprocess helper already sets `GIT_TERMINAL_PROMPT=0` and `GIT_ASKPASS=""` and leaves `Stdin` nil, so git cannot open a credential prompt on Windows. Tests that need the `git` binary already call `exec.LookPath("git")` and `t.Skip` when git is not on PATH.

- [ ] **Step 3: Verify git subprocess behavior on Windows**

```powershell
go test ./internal/runtime/worktree -run 'TestGitNoPromptEnv|TestRunGit' -v -count=1
```

Expected: PASS. If a test fails because git opens a prompt or blocks on stdin, the fix belongs to `env.go` only.

- [ ] **Step 4: Update the migration source manifest**

Append this section to `docs/migration/mewcode-source-manifest.md`:

```markdown
### worktree

- Source: `internal/worktree`
- Target: `internal/runtime/worktree`
- Source files:
  - `agent.go`: BB80B93FD88D89916558BF44761E311C8A92C9906C8830764FC6A3CF2C9E5D18
  - `changes.go`: 9155E50CD89D6292D269665AADB9E3E9AAF845F1FB58564BF2AC37E25C50C59E
  - `cleanup.go`: F1989D4E0832AF0A2A9DC517878B8B5E1020D37D2F8C598F86D45C3734E59D01
  - `create.go`: 5D2AF4C3619D7C894E8A0F38913160EF95551D097A59F292765F02343D8BA93F
  - `env.go`: DB24FEB836DDD5EAE985D794701F544F2886516361FBE54C3B0788E8D9F3DF8C
  - `filesystem.go`: A1AEE3D08515B44D1EC5BAAFE6A808E9917D57CFF4617A6C05B43BA9D1BE7918
  - `notice.go`: D09D943C50192E3F80D83165BF969F0A310FA9B3AFF0490BD30A56092F5F6FBF
  - `session.go`: 5850BCE0B5E0AF3F23DC38F72B2F338A8AF2EA1243B9DE04ADAAA62347A73328
  - `setup.go`: 4CBE0219E2E7CCA9BFC3B8053F86923042AD88F66F66B73037040B14FD7140E8
  - `validate.go`: 1D61B6C61D67CD5F021672DE9E7737ACE10D565057CD917E1663C873CB66F54F
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - No import retargeting required; the package only uses the Go standard library.
  - Keep the filesystem-first git reader and the `env.go` no-prompt subprocess environment unchanged.
- Windows changes: none required; the package already neutralizes git credential prompts via `GIT_TERMINAL_PROMPT=0`, `GIT_ASKPASS=""`, and a nil `Stdin`, and skips git-dependent tests when `git` is not on PATH.
- Verification: gofmt -w internal/runtime/worktree; go test ./internal/runtime/worktree -count=1 -timeout 2m; go test ./internal/runtime/worktree -run 'TestGitNoPromptEnv|TestRunGit' -v -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
- Lore intent: Bring git worktree management into the Devflow runtime
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0. (`internal/artifacts`, `internal/cli`, `internal/memory` may report filesystem sandbox `Access is denied` errors when run inside a restricted sandbox; these are pre-existing environment limitations, not regressions. Verify they are the same failures as before this task.)

- [ ] **Step 6: Commit only the worktree package and manifest**

```powershell
git add -- internal/runtime/worktree docs/migration/mewcode-source-manifest.md
git commit -m "Bring git worktree management into the Devflow runtime" -m "Migrate MewCode's worktree package so later tools and agents can create, inspect, and clean up git worktrees through one Devflow-owned runtime. The filesystem-first git reader and the no-prompt subprocess environment are preserved unchanged." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Worktree tools must depend on this runtime package rather than spawning git directly" -m "Tested: gofmt; go test ./internal/runtime/worktree; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Worktree creation against a remote fetch when origin is unreachable"
git status --short
```

Expected: commit succeeds and status shows only untracked build artifacts.

---
### Task 2: Migrate `tools`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\tools" | Out-Null
Copy-Item "$source\internal\tools\*.go" "$target\internal\runtime\tools\"
```

- [ ] **Step 2: Retarget the worktree import**

In `internal/runtime/tools/enter_worktree.go` and `internal/runtime/tools/exit_worktree.go`, replace `mewcode/internal/worktree` with `github.com/jesseedcp/devflow-agent/internal/runtime/worktree`.

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/tools
go test ./internal/runtime/tools -count=1 -timeout 2m
```

Expected: PASS.

- [ ] **Step 4: Update the migration source manifest**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown
### tools
- Source: `internal/tools` | Target: `internal/runtime/tools`
- Source files:
  - ask_user.go: 9E97806CAE0576E08A15F122AFB738016E7DA9B3C4C63F650EA654B79692B1FA
  - bash.go: D46A20B2C458A688235202CA590BF57FB4489E1946969AEDBBD0C8E072621A2D
  - descriptions.go: F9EF0FFC05047F0D70B3F4276A5B180CDFF9FFA064F8DAD4E8FE66F5B93DAD89
  - edit_file.go: B4E4F5FEC22A87515D9B9A23CC4639D3855F8863EC18FCCFD2598EF389975E65
  - enter_worktree.go: E002A80B2EB85718C7479B2F11A1B7ADCBF2E3D32926DE498CC10251A510C244
  - exit_worktree.go: C9A5EF7B035015D815D9977489C5FF3E0289B44C9B44ED88C51F4ED433322D10
  - glob.go: 3AFEB16688344135F5F66BF5009E1041B840026D32F7872113714FB520E6621A
  - grep.go: F35F7A5A812B594FFBF46AA125226D8F69C3E3C296B6A228C1693B67B0170999
  - read_file.go: 4A61658B1D56987DF17996ECCCA563698DB2ED1B19E1D4F47D6B243CD225C6A8
  - tool_search.go: DEA7E7756F744B8CFDDDCA065E9DFA3410D41BABE987787C60538D196D8108A0
  - tool.go: B5AC0C9BAAF4C9BE9571E30F3D751E89DFD57A3CCC14DE503B4BCB4C3B39FEBA
  - write_file.go: F5C1DB756B2899351A75ED520EE7DB07404B5BAB6DA63D773BDAA1EDE881A8FB
  - glob_test.go: 24DB4468DC28B81B357A27FC15537767728157AD44E040B90077DF646889AC5F
  - tool_search_test.go: 4FEC3449320CB6B27069E05A98CDB37671B6514770E2E3360BE537EA01360B16
- Fusion changes: Move under Devflow runtime boundary; retarget worktree import in enter_worktree.go and exit_worktree.go to internal/runtime/worktree.
- Windows changes: none required; tool commands use os/exec and filepath without POSIX-only assumptions.
- Verification: gofmt -w internal/runtime/tools; go test ./internal/runtime/tools -count=1 -timeout 2m; go test ./...; go vet; go build; git diff --check
- Lore intent: Bring the tool registry and file tools into the Devflow runtime
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit only the tools package and manifest**

```powershell
git add -- internal/runtime/tools docs/migration/mewcode-source-manifest.md
git commit -m "Bring the tool registry and file tools into the Devflow runtime" -m "Migrate MewCode's tools package so the fused runtime can read, write, edit, search, and search-for tools through one Devflow-owned registry, retargeting the worktree import to the migrated runtime package." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: New tools must register through this package's Tool interface rather than ad-hoc handlers" -m "Tested: gofmt; go test ./internal/runtime/tools; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Live bash tool execution against a real shell session"
git status --short
```

---

### Task 3: Migrate `permissions`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\permissions" | Out-Null
Copy-Item "$source\internal\permissions\*.go" "$target\internal\runtime\permissions\"
```

- [ ] **Step 2: Retarget the tools import**

In `internal/runtime/permissions/permissions.go` and `internal/runtime/permissions/permissions_test.go`, replace `mewcode/internal/tools` with `github.com/jesseedcp/devflow-agent/internal/runtime/tools`.

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/permissions
go test ./internal/runtime/permissions -count=1
```

Expected: PASS.

- [ ] **Step 4: Update the migration source manifest**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown
### permissions
- Source: `internal/permissions` | Target: `internal/runtime/permissions`
- Source files:
  - permissions.go: FC9D84E43B5B002EE1BABACAD5B57553E496233DB1693C1CFA192CCF7478BB1F
  - permissions_test.go: 8A61B6DF702025F86226AC346234131219859907AE8DC631799C33665E89E5E6
- Fusion changes: Move under Devflow runtime boundary; retarget tools import in permissions.go and permissions_test.go to internal/runtime/tools.
- Windows changes: none required; permission checks use filepath-based path resolution.
- Verification: gofmt -w internal/runtime/permissions; go test ./internal/runtime/permissions -count=1; go test ./...; go vet; go build; git diff --check
- Lore intent: Bring permission checks into the Devflow runtime
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit only the permissions package and manifest**

```powershell
git add -- internal/runtime/permissions docs/migration/mewcode-source-manifest.md
git commit -m "Bring permission checks into the Devflow runtime" -m "Migrate MewCode's permissions package so the fused runtime can gate file writes, command execution, and tool calls through one Devflow-owned permission layer, retargeting the tools import to the migrated runtime package." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Runtime tools must route writes through this permission layer, not bypass it" -m "Tested: gofmt; go test ./internal/runtime/permissions; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Interactive permission prompts against a live TUI"
git status --short
```

---
### Task 4: Migrate `todo`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\todo" | Out-Null
Copy-Item "$source\internal\todo\*.go" "$target\internal\runtime\todo\"
```

- [ ] **Step 2: Retarget the tools import**

In `internal/runtime/todo/tools.go`, replace `mewcode/internal/tools` with `github.com/jesseedcp/devflow-agent/internal/runtime/tools`.

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/todo
go test ./internal/runtime/todo -count=1
```

Expected: PASS.

- [ ] **Step 4: Update the migration source manifest**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown
### todo
- Source: `internal/todo` | Target: `internal/runtime/todo`
- Source files:
  - store.go: D9F5556C32A7A0608A703D9FE6AFC303D03C16E7D7B1C1BD1A5DFA17D06AF490
  - todo.go: 4B618FC97F32F608BBE7E0A54343F0E0EB8B65A46CF658DF4765A8E11B93BB1A
  - tools.go: 13E57FD565402D55DC366508C67E3BCB2DBE8D3E8E87C1E719FBA3E9372CB9C5
- Fusion changes: Move under Devflow runtime boundary; retarget tools import in tools.go to internal/runtime/tools.
- Windows changes: none required.
- Verification: gofmt -w internal/runtime/todo; go test ./internal/runtime/todo -count=1; go test ./...; go vet; go build; git diff --check
- Lore intent: Bring todo tracking into the Devflow runtime
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit only the todo package and manifest**

```powershell
git add -- internal/runtime/todo docs/migration/mewcode-source-manifest.md
git commit -m "Bring todo tracking into the Devflow runtime" -m "Migrate MewCode's todo package so the fused runtime can maintain task lists as runtime state, retargeting the tools import to the migrated runtime package." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Workflow stage progress is authoritative; this todo layer is execution-scoped only" -m "Tested: gofmt; go test ./internal/runtime/todo; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Concurrent todo updates under the future agent loop"
git status --short
```

---

### Task 5: Migrate `planfile`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\planfile" | Out-Null
Copy-Item "$source\internal\planfile\*.go" "$target\internal\runtime\planfile\"
```

No internal imports to retarget. Keep the package name `planfile`.

- [ ] **Step 2: Format and run package tests**

```powershell
gofmt -w internal/runtime/planfile
go test ./internal/runtime/planfile -count=1
```

Expected: PASS. If the package has no `_test.go` files, run `go build ./internal/runtime/planfile` instead and confirm it exits 0.

- [ ] **Step 3: Update the migration source manifest**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown
### planfile
- Source: `internal/planfile` | Target: `internal/runtime/planfile`
- Source files:
  - planfile.go: 3388E47A96F2334196127511A1134DD8BC16955FDBB30211B6EE561A40AB9878
- Fusion changes: Move under Devflow runtime boundary; no import retargeting required (stdlib only).
- Windows changes: none required.
- Verification: gofmt -w internal/runtime/planfile; go test ./internal/runtime/planfile -count=1 (or go build); go test ./...; go vet; go build; git diff --check
- Lore intent: Bring plan file management into the Devflow runtime
```

- [ ] **Step 4: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit only the planfile package and manifest**

```powershell
git add -- internal/runtime/planfile docs/migration/mewcode-source-manifest.md
git commit -m "Bring plan file management into the Devflow runtime" -m "Migrate MewCode's planfile package so the fused runtime can read and write plan files through one Devflow-owned helper, ahead of the later Plan-stage integration that retargets plan output to demand artifacts." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Demand plan artifacts belong under .devflow/demands/<id>/plan.md, not the legacy .mewcode plan path" -m "Tested: gofmt; go test ./internal/runtime/planfile; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Plan output retargeting to demand artifacts (Wave 5)"
git status --short
```

---
### Task 6: Migrate `prompt`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\prompt" | Out-Null
Copy-Item "$source\internal\prompt\*.go" "$target\internal\runtime\prompt\"
```

No internal or third-party imports; builds prompts from custom `Section` and `EnvironmentContext` types using only the Go standard library. Keep the package name `prompt`.

- [ ] **Step 2: Format and run package tests**

```powershell
gofmt -w internal/runtime/prompt
go test ./internal/runtime/prompt -count=1
```

Expected: PASS. If the package has no `_test.go` files, run `go build ./internal/runtime/prompt` instead and confirm it exits 0.

- [ ] **Step 3: Update the migration source manifest**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown
### prompt
- Source: `internal/prompt` | Target: `internal/runtime/prompt`
- Source files:
  - builder.go: 07A619603C62F3878FDBF662FF62B229341FD3C8E5AB58FD9A04175DD596A42E
  - plan_mode.go: 9B821EA76BFE1F5AB6EFCB123D78C4DF54EB86194AF289BA9030AB3BD5899099
  - sections.go: B34683A865E8786CF31BC274C9792D8574EAF19AFDE943B3BA320A445FE88EAF
- Fusion changes: Move under Devflow runtime boundary; no import retargeting required (stdlib only).
- Windows changes: none required.
- Verification: gofmt -w internal/runtime/prompt; go test ./internal/runtime/prompt -count=1 (or go build); go test ./...; go vet; go build; git diff --check
- Lore intent: Bring prompt building into the Devflow runtime
```

- [ ] **Step 4: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit only the prompt package and manifest**

```powershell
git add -- internal/runtime/prompt docs/migration/mewcode-source-manifest.md
git commit -m "Bring prompt building into the Devflow runtime" -m "Migrate MewCode's prompt package so the fused runtime can assemble system prompts from environment context and named sections through one Devflow-owned builder, ahead of the later Plan and agent integration." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Product identity in assembled prompts must reflect Devflow, not MewCode, when the TUI and agent layers land in Wave 4" -m "Tested: gofmt; go test ./internal/runtime/prompt; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Prompt identity retargeting to Devflow branding (Wave 4 TUI integration)"
git status --short
```

---

### Task 7: Migrate `toolresult`

- [ ] **Step 1: Copy the source package**

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\toolresult" | Out-Null
Copy-Item "$source\internal\toolresult\*.go" "$target\internal\runtime\toolresult\"
```

- [ ] **Step 2: Retarget the conversation import**

In `internal/runtime/toolresult/budget.go`, `internal/runtime/toolresult/budget_test.go`, and `internal/runtime/toolresult/reconstruct.go`, replace `mewcode/internal/conversation` with `github.com/jesseedcp/devflow-agent/internal/runtime/conversation`.

- [ ] **Step 3: Format and run package tests**

```powershell
gofmt -w internal/runtime/toolresult
go test ./internal/runtime/toolresult -count=1
```

Expected: PASS.

- [ ] **Step 4: Update the migration source manifest**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown
### toolresult
- Source: `internal/toolresult` | Target: `internal/runtime/toolresult`
- Source files:
  - budget.go: 3617162B959CA309F60416B2D36C150C3BE3ACAC61E0916EF1D20DFBF7E8EC31
  - reconstruct.go: 8EC245A3FE0B60693F799FE836DC40FDF29DE786E8B86CC67AD26B525FB84CB6
  - record.go: 5305093961E3F8D8135C86A6E530F00A673D1A5F341869323F93ECF4BC0C0607
  - state.go: 09A2513DA1A9C4C42CD1A7867DDCEC74C698896D460C265121DECA8598658D92
  - budget_test.go: 8EF54AEE1B8E57095B4E138D8127495B3F366B536A30196B676762467525C2F8
  - record_test.go: 76EA67BE9D818CB4E340D1B6BB8DF9F5131948A0F43E484A34EE5D0075459C0B
  - state_test.go: 85492F1D226EF29ADF6FCB2BEB4E4DE8D5DB186C225366888145C259C407AE14
- Fusion changes: Move under Devflow runtime boundary; retarget conversation import in budget.go, budget_test.go, reconstruct.go to internal/runtime/conversation (Wave 1).
- Windows changes: none required.
- Verification: gofmt -w internal/runtime/toolresult; go test ./internal/runtime/toolresult -count=1; go test ./...; go vet; go build; git diff --check
- Lore intent: Bring tool result tracking into the Devflow runtime
```

- [ ] **Step 5: Run repository verification**

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit only the toolresult package and manifest**

```powershell
git add -- internal/runtime/toolresult docs/migration/mewcode-source-manifest.md
git commit -m "Bring tool result tracking into the Devflow runtime" -m "Migrate MewCode's toolresult package so the fused runtime can budget, record, and reconstruct tool-call results against the conversation history, retargeting the conversation import to the migrated Wave 1 runtime package." -m "Constraint: One migrated Go package per commit" -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" -m "Confidence: high" -m "Scope-risk: narrow" -m "Directive: Tool result budget accounting must stay inside the runtime, not be reimplemented per stage handler" -m "Tested: gofmt; go test ./internal/runtime/toolresult; go test ./...; go vet; go build; git diff --check" -m "Not-tested: Token budget exhaustion against a real provider stream (covered by Wave 1 llm tests)"
git status --short
```

---
### Task 8: Verify Wave 2 As A Clean Seven-Commit Foundation

- [ ] **Step 1: Confirm the seven package commits**

```powershell
git log -7 --format='%H %s'
```

Expected intent lines, newest first: Bring tool result tracking / Bring prompt building / Bring plan file management / Bring todo tracking / Bring permission checks / Bring the tool registry / Bring git worktree management (each "into the Devflow runtime").

- [ ] **Step 2: Confirm each migrated package is represented in the manifest**

```powershell
Select-String -Path docs/migration/mewcode-source-manifest.md -Pattern '^### (worktree|tools|permissions|todo|planfile|prompt|toolresult)$'
Select-String -Path docs/migration/mewcode-source-manifest.md -Pattern 'Lore intent:'
```

Expected: seven package sections and a matching Lore intent for each migrated package (plus the four Wave 1 packages).

- [ ] **Step 3: Run final verification from a clean worktree**

```powershell
git status --short
go test ./internal/runtime/... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
```

Expected: empty Git status (ignoring untracked build artifacts) and all Go commands exit 0. The four Wave 1 runtime packages plus the seven Wave 2 packages must all pass.

- [ ] **Step 4: Record known remaining scope**

Do not create a commit. Report that Wave 2 provides worktree management, the tool registry, permission checks, todo tracking, plan-file management, prompt building, and tool-result accounting. It does not yet expose the agent loop, compact, MCP, skills, memory, subagents, teams, session, history, commands, a TUI, a runtime CLI command, or the Requirements/Plan/execution/verification/closeout product integrations. Those belong to Waves 3-5.

---

## Plan Self-Review

- **Spec coverage:** This plan covers the seven Wave 2 packages named in the fusion spec's runtime target directory (`internal/runtime/`), in dependency order, with one-package-per-commit discipline, per-package source hashing, Windows verification, `.devflow` boundary adherence, and manifest updates.
- **Dependency order verified:** Resolved from actual `mewcode/internal/*` imports in the source snapshot; no Wave 3+ dependency exists in any of the seven packages, so the wave is self-contained and can be committed package-by-package without stubs.
- **Placeholder scan:** The plan contains no unresolved marker or unspecified implementation step. Every step has a concrete PowerShell command, exact import replacement strings, or an exact expected output. Manifest entries include pre-computed SHA-256 hashes for every source file.
- **Type consistency:** Internal imports consistently retarget to `github.com/jesseedcp/devflow-agent/internal/runtime/<pkg>`; the `toolresult` package depends on the already-migrated `internal/runtime/conversation` from Wave 1.
- **Commit consistency:** Each package task modifies only that package and the shared source manifest, matching the Wave 1 contract.
- **Deliberate gaps:** Wave 3-5 subsystems are not implemented by this plan and are not claimed as complete.
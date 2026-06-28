# Runtime Foundation Wave 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate MewCode's interactive runtime surface into Devflow: `commands`, `history`, `session`, Bubble Tea `tui`, and the single `cmd/devflow` entry that can run both the existing backend-demand workflow CLI and the fused interactive agent runtime.

**Architecture:** Keep Devflow Workflow as the product authority and put reusable chat/runtime pieces under `internal/runtime/`. The existing `internal/cli` demand workflow commands remain stable. Wave 4 adds a runtime launch path that loads `.devflow` configuration, starts the migrated TUI, and uses the Wave 1-3 runtime packages already migrated under `internal/runtime/*`. `.devflow` remains authoritative for all new writes; `.mewcode` is read-only legacy fallback where needed.

**Tech Stack:** Go 1.25.0, PowerShell on Windows, current branch `feature/devflow-v0.1`, MewCode local snapshot, Devflow runtime packages from Waves 1-3, Bubble Tea/Bubbles/Glamour/Lipgloss terminal UI dependencies copied at MewCode's pinned versions, Anthropic/OpenAI-compatible runtime providers, and standard Go tests/build/vet.

---

## Current Environment

Run execution from this worktree:

```powershell
$source = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang'
$target = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1'
Set-Location $target
git status --short --branch
```

Expected starting status:

```text
## feature/devflow-v0.1...origin/feature/devflow-v0.1 [ahead 25]
?? .gocache/
?? devflow.exe
```

Do not delete or commit `.gocache/` or `devflow.exe`. They are known local artifacts.

The source snapshot is local and user-authorized:

```text
D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang
```

The target module is:

```text
github.com/jesseedcp/devflow-agent
```

The target `go.mod` currently has Go `1.25.0`. If Wave 4 imports TUI packages, add only the MewCode-pinned direct dependencies:

```text
github.com/charmbracelet/bubbles v1.0.0
github.com/charmbracelet/bubbletea v1.3.10
github.com/charmbracelet/glamour v1.0.0
github.com/charmbracelet/lipgloss v1.1.1-0.20250404203927-76690c660834
github.com/muesli/termenv v0.16.0
github.com/rivo/uniseg v0.4.7
```

Let `go mod tidy` manage indirect dependencies. Do not introduce unrelated dependencies.

API keys may already exist in local user/project config. Do not print or commit secrets. Live API smoke tests are optional and must not echo env/config values.

## Starting Point From Wave 3

Wave 3 has already migrated and verified:

- `internal/runtime/compact`
- `internal/runtime/mcp`
- `internal/runtime/skills`
- `internal/runtime/memory`
- `internal/runtime/agent`
- `internal/runtime/teams`
- `internal/runtime/agents`
- `internal/runtime/memory/extractor`
- compatibility fixes in `internal/runtime/tools`

Wave 3 verification already passed:

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Wave 4 should preserve that baseline.

## Scope Boundaries

In scope:

- Migrate `internal/commands` to `internal/runtime/commands`.
- Migrate `internal/history` to `internal/runtime/history`.
- Migrate `internal/session` to `internal/runtime/session`.
- Migrate `internal/tui` to `internal/runtime/tui`.
- Wire an interactive launch path into `internal/cli` and `cmd/devflow`.
- Update source migration manifest for every migrated package.
- Add focused tests for storage paths, command registry behavior, session resume helpers, CLI dispatch, and any pure TUI helper logic touched during the migration.

Out of scope for Wave 4:

- Full backend demand product workflow automation from PRD link to MR closeout.
- GitLab MR adapter work.
- Requirements/plan/verification/closeout product-stage generation.
- Replacing the existing `internal/workflow`, `internal/artifacts`, `internal/quality`, or `internal/adapters` product packages.
- Live end-to-end LLM validation as a required CI gate.

Those are Wave 5+.

## Source File Map

Migrate these source files:

```text
internal/commands/commands.go
internal/commands/loader.go
internal/commands/loader_test.go

internal/history/history.go
internal/history/history_test.go

internal/session/session.go
internal/session/session_test.go

internal/tui/styles.go
internal/tui/tui.go
internal/tui/verbs.go
```

Target files:

```text
internal/runtime/commands/commands.go
internal/runtime/commands/loader.go
internal/runtime/commands/loader_test.go

internal/runtime/history/history.go
internal/runtime/history/history_test.go

internal/runtime/session/session.go
internal/runtime/session/session_test.go

internal/runtime/tui/styles.go
internal/runtime/tui/tui.go
internal/runtime/tui/verbs.go
internal/runtime/tui/*_test.go

internal/cli/cli.go
internal/cli/cli_test.go
cmd/devflow/main.go
go.mod
go.sum
docs/migration/mewcode-source-manifest.md
```

`cmd/devflow/main.go` should remain tiny. Prefer keeping dispatch logic in `internal/cli`.

## State Directory Policy

New writes must use `.devflow`.

| Concern | New write path | Legacy read fallback |
|---|---|---|
| prompt history | `$workDir/.devflow/prompt_history.jsonl` | `$workDir/.mewcode/prompt_history.jsonl` |
| sessions | `$workDir/.devflow/sessions/*.jsonl` | `$workDir/.mewcode/sessions/*.jsonl` |
| user commands | `%USERPROFILE%\.devflow\commands`, `$workDir\.devflow\commands` | matching `.mewcode\commands` directories |
| permissions local file referenced by TUI | `$workDir/.devflow/permissions.local.yaml` | read legacy only if implementation already supports it cleanly |

Rules:

- If both `.devflow` and `.mewcode` files exist for the same concern, read `.devflow`.
- If only `.mewcode` exists, read it for compatibility.
- Any append/create/write must go to `.devflow`.
- Do not migrate files on disk automatically in Wave 4.

## Import Replacement Reference

Apply these replacements in migrated packages:

| Old import | New import |
|---|---|
| `mewcode/internal/agent` | `github.com/jesseedcp/devflow-agent/internal/runtime/agent` |
| `mewcode/internal/agents` | `github.com/jesseedcp/devflow-agent/internal/runtime/agents` |
| `mewcode/internal/commands` | `github.com/jesseedcp/devflow-agent/internal/runtime/commands` |
| `mewcode/internal/compact` | `github.com/jesseedcp/devflow-agent/internal/runtime/compact` |
| `mewcode/internal/config` | `github.com/jesseedcp/devflow-agent/internal/runtime/config` |
| `mewcode/internal/conversation` | `github.com/jesseedcp/devflow-agent/internal/runtime/conversation` |
| `mewcode/internal/history` | `github.com/jesseedcp/devflow-agent/internal/runtime/history` |
| `mewcode/internal/hooks` | `github.com/jesseedcp/devflow-agent/internal/runtime/hooks` |
| `mewcode/internal/llm` | `github.com/jesseedcp/devflow-agent/internal/runtime/llm` |
| `mewcode/internal/mcp` | `github.com/jesseedcp/devflow-agent/internal/runtime/mcp` |
| `mewcode/internal/memory` | `github.com/jesseedcp/devflow-agent/internal/runtime/memory` |
| `mewcode/internal/memory/extractor` | `github.com/jesseedcp/devflow-agent/internal/runtime/memory/extractor` |
| `mewcode/internal/permissions` | `github.com/jesseedcp/devflow-agent/internal/runtime/permissions` |
| `mewcode/internal/planfile` | `github.com/jesseedcp/devflow-agent/internal/runtime/planfile` |
| `mewcode/internal/prompt` | `github.com/jesseedcp/devflow-agent/internal/runtime/prompt` |
| `mewcode/internal/session` | `github.com/jesseedcp/devflow-agent/internal/runtime/session` |
| `mewcode/internal/skills` | `github.com/jesseedcp/devflow-agent/internal/runtime/skills` |
| `mewcode/internal/teams` | `github.com/jesseedcp/devflow-agent/internal/runtime/teams` |
| `mewcode/internal/todo` | `github.com/jesseedcp/devflow-agent/internal/runtime/todo` |
| `mewcode/internal/tools` | `github.com/jesseedcp/devflow-agent/internal/runtime/tools` |
| `mewcode/internal/worktree` | `github.com/jesseedcp/devflow-agent/internal/runtime/worktree` |

## Execution Tasks

### Task 0: Preflight And Baseline

- [ ] Run `git status --short --branch` and confirm only expected untracked `.gocache/` and `devflow.exe` are present.
- [ ] Run `go test ./... -count=1 -timeout 5m` before edits. If it fails, stop and fix only if the failure is caused by the current worktree; otherwise record the pre-existing failure in the final handoff.
- [ ] Run `go build ./cmd/devflow` before edits to confirm the CLI baseline.

No commit is required for Task 0.

### Task 1: Migrate `history`

Files:

- Create `internal/runtime/history/history.go`
- Create `internal/runtime/history/history_test.go`
- Modify `docs/migration/mewcode-source-manifest.md`

Implementation requirements:

- Copy source behavior from `internal/history`.
- Change write path from `.mewcode/prompt_history.jsonl` to `.devflow/prompt_history.jsonl`.
- Add a small path-selection helper so `Load(workDir)` reads `.devflow` first and falls back to `.mewcode` only when the `.devflow` file does not exist.
- Preserve `maxEntries = 200`.
- Preserve duplicate-last-entry suppression.
- Keep API simple: `Load(dir string) []string` and `Append(dir string, text string)` may remain as-is unless tests show a stronger need.

Test requirements:

- [ ] New append creates `$temp/.devflow/prompt_history.jsonl`.
- [ ] Load prefers `.devflow` over `.mewcode`.
- [ ] Load falls back to `.mewcode` when `.devflow` is absent.
- [ ] Append trims to 200 entries.
- [ ] Append skips exact duplicate of the last entry.

Verification:

```powershell
gofmt -w internal/runtime/history/*.go
go test ./internal/runtime/history -count=1
go test ./internal/runtime/... -count=1 -timeout 5m
git diff --check
```

Commit:

```powershell
git add internal/runtime/history docs/migration/mewcode-source-manifest.md
git commit -m @'
Preserve prompt history in the Devflow runtime

History is the lowest-risk interactive state package, so this
migration moves it first and switches new writes to .devflow while
keeping .mewcode readable for existing local users.

Constraint: .devflow is authoritative for new runtime state
Rejected: Auto-migrate legacy files | hidden filesystem mutation during startup
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/runtime/history -count=1; go test ./internal/runtime/... -count=1 -timeout 5m; git diff --check
'@
```

### Task 2: Migrate `session`

Files:

- Create `internal/runtime/session/session.go`
- Create `internal/runtime/session/session_test.go`
- Modify `docs/migration/mewcode-source-manifest.md`

Implementation requirements:

- Copy source behavior from `internal/session`.
- Write new session JSONL files to `$workDir/.devflow/sessions`.
- List sessions from `.devflow/sessions` first.
- Include legacy `.mewcode/sessions` in listing only when there is no same-ID `.devflow` session.
- `LoadSession(workDir, id)` must prefer `.devflow`, then fallback to `.mewcode`.
- Preserve `NewID`, `Message`, `SessionInfo`, `FormatRelativeTime`, `FormatFileSize`, and `MatchesSearch`.
- Keep `currentGitBranch` behavior, but tests should not require the temp directory to be a git repo.
- Consider extracting path helpers:

```go
func sessionsDir(workDir string) string
func legacySessionsDir(workDir string) string
func sessionFilePath(workDir, id string) string
func legacySessionFilePath(workDir, id string) string
```

Test requirements:

- [ ] `SaveMessage` writes JSONL to `.devflow/sessions`.
- [ ] `LoadSession` reads `.devflow` before legacy `.mewcode`.
- [ ] `LoadSession` falls back to `.mewcode`.
- [ ] `ListSessions` returns newest first.
- [ ] `ListSessions` deduplicates same ID in favor of `.devflow`.
- [ ] `FormatRelativeTime`, `FormatFileSize`, and `MatchesSearch` preserve source behavior.

Verification:

```powershell
gofmt -w internal/runtime/session/*.go
go test ./internal/runtime/session -count=1
go test ./internal/runtime/... -count=1 -timeout 5m
git diff --check
```

Commit:

```powershell
git add internal/runtime/session docs/migration/mewcode-source-manifest.md
git commit -m @'
Keep chat sessions resumable under Devflow state

Session persistence is moved before the TUI so the interactive
surface can resume existing MewCode chats while storing new messages
in Devflow-owned state.

Constraint: Existing .mewcode sessions must remain readable
Rejected: Single-directory listing | would hide legacy sessions during migration
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/runtime/session -count=1; go test ./internal/runtime/... -count=1 -timeout 5m; git diff --check
'@
```

### Task 3: Migrate `commands`

Files:

- Create `internal/runtime/commands/commands.go`
- Create `internal/runtime/commands/loader.go`
- Create `internal/runtime/commands/loader_test.go`
- Modify `docs/migration/mewcode-source-manifest.md`

Implementation requirements:

- Copy the command registry from source.
- Rename user-facing strings from `MewCode` to `Devflow`.
- Update command loader search paths:

```text
%USERPROFILE%\.devflow\commands
$workDir\.devflow\commands
%USERPROFILE%\.mewcode\commands
$workDir\.mewcode\commands
```

- Precedence rule: Devflow paths override legacy paths at the same scope. Project commands override user commands.
- Preserve file-command namespacing rule: `sub/dir/foo.md` becomes `/sub:dir:foo`.
- Preserve frontmatter parsing and `$ARGUMENTS` substitution.
- Preserve collision handling in `Registry.Register` and `Registry.HasConflict`.
- Keep the `TypeSkillFork` concept because Wave 3 migrated subagent and skill support.

Test requirements:

- [ ] Registry duplicate name panics.
- [ ] Registry alias collision panics.
- [ ] `Parse("/review abc")` returns `review`, `abc`.
- [ ] `LoadDir` parses frontmatter and body.
- [ ] File command without `$ARGUMENTS` appends `## User Request`.
- [ ] Devflow project command overrides legacy project command.
- [ ] Project command overrides user command.
- [ ] Help/status/skills text says Devflow and `.devflow`, not MewCode or `.mewcode`, except when explicitly documenting fallback.

Verification:

```powershell
gofmt -w internal/runtime/commands/*.go
go test ./internal/runtime/commands -count=1
go test ./internal/runtime/... -count=1 -timeout 5m
git diff --check
```

Commit:

```powershell
git add internal/runtime/commands docs/migration/mewcode-source-manifest.md
git commit -m @'
Expose slash commands through the Devflow runtime

Slash commands are migrated after session state so interactive users
can keep prompt-command workflows while Devflow-owned command folders
take precedence over legacy MewCode folders.

Constraint: User and project command overrides must remain deterministic
Rejected: Only reading .devflow commands | would strand existing MewCode prompt commands
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/runtime/commands -count=1; go test ./internal/runtime/... -count=1 -timeout 5m; git diff --check
'@
```

### Task 4: Migrate `tui` As A Runtime Package

Files:

- Create `internal/runtime/tui/styles.go`
- Create `internal/runtime/tui/tui.go`
- Create `internal/runtime/tui/verbs.go`
- Create focused tests under `internal/runtime/tui`
- Modify `go.mod`
- Modify `go.sum`
- Modify `docs/migration/mewcode-source-manifest.md`

Implementation requirements:

- Copy the source TUI and retarget all imports using the import replacement table.
- Rename visible product identity from MewCode to Devflow where it appears in banners/status/errors.
- Replace `.mewcode` hard-coded paths:
  - permissions local file: prefer `.devflow/permissions.local.yaml`;
  - skills and memory code should already use runtime packages; do not reintroduce `.mewcode` writes from TUI;
  - any direct text instructing users to use `.mewcode` should become `.devflow`, with legacy fallback described only if useful.
- Preserve `commands.TypeLocalUI` behavior for `/clear`, `/compact`, `/plan`, `/do`, `/resume`.
- Preserve the ask-user dialog and subagent progress UI introduced by Wave 3.
- Preserve MCP startup and hook wiring from source, using migrated packages.
- Preserve worktree tool registration and stale worktree cleanup.
- Keep the TUI package separate from `internal/cli` so CLI dispatch can be tested without terminal control.
- If direct TUI tests are hard because of Bubble Tea terminal state, test pure helpers and lightweight model setup:
  - `New` initializes one-provider state as chat;
  - `permissionModeInfo` labels remain stable;
  - `nextPermissionMode` cycles correctly;
  - `coordinatorToolFilter(nil)` returns nil;
  - `renderToolGroupSummary`, `isCollapsibleTool`, or path/reference helper behavior as needed.

Dependency handling:

- Add only MewCode's pinned direct terminal UI dependencies listed in the environment section.
- Run `go mod tidy` after migration.
- If `go mod tidy` changes unrelated indirect versions, inspect the diff before committing and keep only changes explained by the new TUI imports.

Verification:

```powershell
gofmt -w internal/runtime/tui/*.go
go test ./internal/runtime/tui -count=1
go test ./internal/runtime/... -count=1 -timeout 5m
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Commit:

```powershell
git add internal/runtime/tui go.mod go.sum docs/migration/mewcode-source-manifest.md
git commit -m @'
Bring the interactive TUI onto the Devflow runtime

The terminal surface is migrated only after its storage and command
dependencies are Devflow-owned, allowing the UI to reuse Wave 1-3
runtime packages without reopening product workflow boundaries.

Constraint: TUI migration requires the same terminal UI dependencies used by MewCode
Rejected: Rewriting the interface from scratch | higher risk and would delay runtime fusion
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/runtime/tui -count=1; go test ./internal/runtime/... -count=1 -timeout 5m; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

### Task 5: Wire The Single `cmd/devflow` Interactive Entry

Files:

- Modify `internal/cli/cli.go`
- Modify `internal/cli/cli_test.go`
- Modify `cmd/devflow/main.go` only if absolutely necessary
- Modify `docs/migration/mewcode-source-manifest.md` only if adding an entry for entrypoint fusion

Implementation requirements:

- Preserve existing commands:

```text
devflow help
devflow start
devflow confirm
devflow verify
devflow closeout
```

- Add interactive command surface:

```text
devflow chat
devflow tui
devflow
```

- Recommended dispatch:
  - `devflow help` prints help.
  - `devflow start|confirm|verify|closeout` keep current behavior.
  - `devflow chat` starts runtime TUI.
  - `devflow tui` aliases `chat`.
  - `devflow` with no args starts runtime TUI if config exists; if config is missing, return a short actionable error that mentions `.devflow/config.yaml` and `devflow help`.

Do not break existing no-arg help tests without explicitly updating them. The product decision for Wave 4 is that `devflow` becomes the interactive entry; `devflow help` remains the non-interactive help command.

- Put launch code in an internal helper, for example:

```go
func runChat(stdout io.Writer, stderr io.Writer) error
```

- Load config via `internal/runtime/config.LoadConfig("")`.
- Start Bubble Tea from `internal/runtime/tui.New(cfg.Providers, cfg.MCPServers, cfg.Hooks)`.
- Keep actual `tea.NewProgram(...)` construction inside a tiny function that can be replaced in tests, for example:

```go
var runTeaProgram = func(model tea.Model) error {
    _, err := tea.NewProgram(model).Run()
    return err
}
```

- Tests should stub `runTeaProgram` so `go test` does not take over the terminal.
- If config loading needs a current working directory, tests should use `t.TempDir()` and temporarily change directory with restore.

Test requirements:

- [ ] Existing `start`, `confirm`, `verify`, and `closeout` tests still pass.
- [ ] `Run([]string{"help"}, ...)` prints help and exits nil.
- [ ] `Run([]string{"chat"}, ...)` loads config and calls the TUI runner once with stubbed runner.
- [ ] `Run([]string{"tui"}, ...)` aliases `chat`.
- [ ] `Run(nil, ...)` follows the same interactive path.
- [ ] Missing config returns an error that mentions `.devflow/config.yaml`.
- [ ] Unknown command still returns existing unknown-command help error.

Verification:

```powershell
gofmt -w internal/cli/cli.go internal/cli/cli_test.go cmd/devflow/main.go
go test ./internal/cli -count=1
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Commit:

```powershell
git add internal/cli cmd/devflow docs/migration/mewcode-source-manifest.md
git commit -m @'
Make devflow launch the fused interactive runtime

The entrypoint keeps backend-demand workflow commands intact while
turning the bare devflow command into the interactive runtime surface
backed by .devflow configuration.

Constraint: Existing workflow CLI commands must remain stable
Rejected: Separate binary for chat runtime | would split the fused product surface
Confidence: medium
Scope-risk: moderate
Tested: go test ./internal/cli -count=1; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check
'@
```

### Task 6: Final Wave Verification

Run the full verification sequence:

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
git status --short --branch
```

Expected final status should include only known untracked local artifacts:

```text
?? .gocache/
?? devflow.exe
```

If a built `devflow.exe` is regenerated, leave it untracked.

Optional local interactive smoke test:

```powershell
.\devflow.exe help
.\devflow.exe chat
```

For `chat`, do not enter real API keys into the transcript or final report. If it launches and renders the provider/chat screen, quit with `ctrl+c` and record that manual smoke succeeded. If the terminal cannot be controlled in the current execution environment, record that only build/unit verification was performed.

No final commit is required if Task 5 already committed all changes. If final-only docs are added, commit them separately with Lore trailers.

## Manifest Update Requirements

For every migrated package, update `docs/migration/mewcode-source-manifest.md` with:

- Source path.
- Target path.
- Source file SHA256 hashes.
- Fusion changes.
- Windows changes or "none required".
- Verification commands actually run.
- Lore intent line.

Use this PowerShell helper to compute source hashes:

```powershell
Get-FileHash "$source\internal\history\history.go" -Algorithm SHA256
Get-FileHash "$source\internal\session\session.go" -Algorithm SHA256
Get-FileHash "$source\internal\commands\commands.go" -Algorithm SHA256
Get-FileHash "$source\internal\commands\loader.go" -Algorithm SHA256
Get-FileHash "$source\internal\tui\styles.go" -Algorithm SHA256
Get-FileHash "$source\internal\tui\tui.go" -Algorithm SHA256
Get-FileHash "$source\internal\tui\verbs.go" -Algorithm SHA256
```

## Recommended Subagent Split

Use at most four independent workers if the execution window supports native subagents:

- Worker A: `history` and `session`.
- Worker B: `commands`.
- Worker C: `tui` import retargeting and TUI tests.
- Worker D: `internal/cli` entrypoint wiring and CLI tests.

Leader responsibilities:

- Keep commits sequential and review each worker patch before committing.
- Resolve shared `go.mod`, `go.sum`, and `docs/migration/mewcode-source-manifest.md` conflicts.
- Run full verification after integration.

Do not let workers commit independently unless the execution window can guarantee commit ordering. Safer flow: workers prepare patches, leader integrates and commits one module at a time.

## Definition Of Done

Wave 4 is complete when:

- `internal/runtime/history`, `session`, `commands`, and `tui` exist and compile.
- New runtime writes use `.devflow`.
- Legacy `.mewcode` state remains readable where specified.
- `devflow help` still works.
- `devflow start|confirm|verify|closeout` still work through existing tests.
- `devflow`, `devflow chat`, and `devflow tui` launch the interactive runtime path under test stubs.
- Full verification passes:

```powershell
go test ./internal/runtime/... -count=1 -timeout 5m
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

- Each module-sized change is committed with Lore trailers.
- Final worktree status is explained, including known untracked `.gocache/` and `devflow.exe`.

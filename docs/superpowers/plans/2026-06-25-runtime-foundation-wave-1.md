# Runtime Foundation Wave 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the first four dependency-ordered MewCode runtime packages into Devflow so `devflow-agent` can load `.devflow` provider configuration and complete a real streaming LLM request.

**Architecture:** This wave migrates `conversation`, `hooks`, `config`, and `llm` under `internal/runtime` in that order. Each Go package is independently tested, documented in the source manifest, and committed before the next package starts. Devflow remains the workflow authority; this wave only establishes reusable runtime foundations and does not add stage transitions or TUI behavior.

**Tech Stack:** Go 1.23+, PowerShell on Windows, `gopkg.in/yaml.v3`, Anthropic Go SDK, OpenAI Go SDK, existing Devflow file-backed workflow.

---

## Scope And Follow-Up Waves

The approved fusion spec covers several independently testable subsystems, so implementation is split into separate plans:

1. **Wave 1, this plan:** `conversation`, `hooks`, `config`, `llm`.
2. **Wave 2:** `worktree`, `tools`, `permissions`, `planfile`, `prompt`, `todo`, `toolresult`.
3. **Wave 3:** `compact`, `agent`, `skills`, `mcp`, `agents`, `teams`, `memory`, `memory/extractor`.
4. **Wave 4:** `commands`, `history`, `session`, dual-mode `tui`, and the single `cmd/devflow` interactive entry.
5. **Wave 5:** Requirements, Plan, execution, unassociated-change, verification, and closeout runtime integrations.

Every later wave receives its own implementation plan after the previous wave passes its package and repository verification gates.

## Source And Repository Locations

```text
Source snapshot:
D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang

Target worktree:
D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1
```

Use these PowerShell variables during execution:

```powershell
$source = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang'
$target = 'D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent\.worktrees\devflow-v0.1'
```

## Commit And Manifest Rule

Each task below ends with one package migration commit. Do not start the next task before the commit exists and `git status --short` is empty.

A Git commit cannot contain its own final SHA because adding the SHA changes the commit. The source manifest therefore records the package commit's unique Lore intent line in the same commit. After committing, verify its SHA with:

```powershell
git log -1 --format='%H %s'
```

The unique intent line is the stable manifest-to-commit link.

## File Map

### Task 1: Conversation

- Create: `internal/runtime/conversation/conversation.go`
- Create: `internal/runtime/conversation/conversation_test.go`
- Create: `docs/migration/mewcode-source-manifest.md`

### Task 2: Hooks

- Create: `internal/runtime/hooks/hooks.go`
- Create: `internal/runtime/hooks/hooks_test.go`
- Create: `internal/runtime/hooks/command_windows.go`
- Create: `internal/runtime/hooks/command_unix.go`
- Create: `internal/runtime/hooks/command_windows_test.go`
- Create: `internal/runtime/hooks/command_unix_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 3: Configuration

- Create: `internal/runtime/config/config.go`
- Create: `internal/runtime/config/config_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `docs/migration/mewcode-source-manifest.md`

### Task 4: LLM

- Create: `internal/runtime/llm/anthropic.go`
- Create: `internal/runtime/llm/client.go`
- Create: `internal/runtime/llm/errors.go`
- Create: `internal/runtime/llm/events.go`
- Create: `internal/runtime/llm/model_resolver.go`
- Create: `internal/runtime/llm/openai.go`
- Create: `internal/runtime/llm/openai_compat.go`
- Create: `internal/runtime/llm/thinking_test.go`
- Create: `internal/runtime/llm/live_openai_compat_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `docs/migration/mewcode-source-manifest.md`

---

### Task 1: Migrate `conversation`

**Files:**
- Create: `internal/runtime/conversation/conversation.go`
- Create: `internal/runtime/conversation/conversation_test.go`
- Create: `docs/migration/mewcode-source-manifest.md`

- [ ] **Step 1: Write the failing conversation tests**

Create `internal/runtime/conversation/conversation_test.go`:

```go
package conversation

import (
	"strings"
	"testing"
)

func TestManagerPreservesToolRoundTrip(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("check the repository")
	manager.AddToolUseMessage("I will inspect it", "tool-1", "ReadFile", map[string]any{
		"file_path": "README.md",
	})
	manager.AddToolResultMessage("tool-1", "# Devflow", false)

	messages := manager.GetMessages()
	if len(messages) != 3 {
		t.Fatalf("message count = %d, want 3", len(messages))
	}
	if messages[1].ToolUses[0].ToolName != "ReadFile" {
		t.Fatalf("tool name = %q, want ReadFile", messages[1].ToolUses[0].ToolName)
	}
	if messages[2].ToolResults[0].Content != "# Devflow" {
		t.Fatalf("tool result = %q, want # Devflow", messages[2].ToolResults[0].Content)
	}
}

func TestInjectLongTermMemoryUsesDevflowIdentityOnce(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("implement the demand")
	manager.InjectLongTermMemory("Follow AGENTS.md", "Coupon rules require active members")
	manager.InjectLongTermMemory("duplicate", "duplicate")

	messages := manager.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(messages))
	}
	injected := messages[0].Content
	if !strings.Contains(injected, "# devflowMd") {
		t.Fatalf("injected memory does not use Devflow identity: %q", injected)
	}
	if strings.Contains(injected, "# mewcodeMd") {
		t.Fatalf("injected memory still exposes MewCode identity: %q", injected)
	}
	if strings.Count(injected, "Coupon rules require active members") != 1 {
		t.Fatalf("memory should be injected once: %q", injected)
	}
}

func TestGetMessagesReturnsIndependentSlice(t *testing.T) {
	manager := NewManager()
	manager.AddUserMessage("original")

	copyOfMessages := manager.GetMessages()
	copyOfMessages[0].Content = "changed"

	if got := manager.GetMessages()[0].Content; got != "original" {
		t.Fatalf("manager content = %q, want original", got)
	}
}
```

- [ ] **Step 2: Run the test and verify the package is missing**

Run:

```powershell
go test ./internal/runtime/conversation
```

Expected: FAIL because `NewManager` and the conversation types do not exist.

- [ ] **Step 3: Copy the source implementation**

Run:

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\conversation" | Out-Null
Copy-Item "$source\internal\conversation\conversation.go" "$target\internal\runtime\conversation\conversation.go"
```

In `internal/runtime/conversation/conversation.go`, replace:

```go
"# mewcodeMd\nCodebase and user instructions are shown below.
```

with:

```go
"# devflowMd\nCodebase and user instructions are shown below.
```

Keep the package name `conversation`. Do not rename public types in this migration.

- [ ] **Step 4: Format and run package tests**

Run:

```powershell
gofmt -w internal/runtime/conversation
go test ./internal/runtime/conversation -count=1
```

Expected: PASS.

- [ ] **Step 5: Create the migration source manifest**

Create `docs/migration/mewcode-source-manifest.md`:

```markdown
# MewCode Source Migration Manifest

The user confirmed authority to copy and modify the local MewCode snapshot.
Snapshot root: `D:\Users\dd\Desktop\agent学习\mewcode-golang\mewcode-golang`
Snapshot date: `2026-06-25`

Git commits cannot embed their own final SHA. Each package entry records the
unique Lore intent line of its migration commit; resolve the SHA with
`git log --format="%H %s" --grep="<intent line>"`.

## Packages

### conversation

- Source: `internal/conversation`
- Target: `internal/runtime/conversation`
- Source files:
  - `conversation.go`: `9FC69554F3B713D3B57EB987D768AC229FF1BCFE3C999A04B71A35838BB36958`
- Fusion changes:
  - Move the package under the Devflow runtime boundary.
  - Replace the injected `mewcodeMd` identity heading with `devflowMd`.
  - Add focused regression tests for message flow, memory injection, and slice isolation.
- Windows changes: none required; the package uses platform-neutral Go APIs.
- Verification: `go test ./internal/runtime/conversation -count=1`; `go test ./... -count=1`
- Lore intent: `Preserve conversation semantics inside the Devflow runtime`
```

- [ ] **Step 6: Run repository verification**

Run:

```powershell
go test ./... -count=1
go vet ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 7: Commit only the conversation package and manifest**

Run:

```powershell
git add -- internal/runtime/conversation docs/migration/mewcode-source-manifest.md
git commit -m "Preserve conversation semantics inside the Devflow runtime" `
  -m "Migrate MewCode's conversation manager as the first runtime package so later provider and agent packages share one tested message model. The injected project identity is changed to Devflow while the public message and serialization behavior remains stable." `
  -m "Constraint: One migrated Go package per commit" `
  -m "Constraint: User confirmed authority to copy and modify the local MewCode snapshot" `
  -m "Rejected: Redesign the conversation schema during migration | downstream runtime packages depend on the existing message contract" `
  -m "Confidence: high" `
  -m "Scope-risk: narrow" `
  -m "Directive: Keep workflow state and confirmation records out of the generic conversation manager" `
  -m "Tested: go test ./internal/runtime/conversation -count=1; go test ./... -count=1; go vet ./...; git diff --check" `
  -m "Not-tested: Provider-specific serialization until the llm package is migrated"
git status --short
```

Expected: commit succeeds and status is empty.

---

### Task 2: Migrate `hooks` With Windows Shell Support

**Files:**
- Create: `internal/runtime/hooks/hooks.go`
- Create: `internal/runtime/hooks/hooks_test.go`
- Create: `internal/runtime/hooks/command_windows.go`
- Create: `internal/runtime/hooks/command_unix.go`
- Create: `internal/runtime/hooks/command_windows_test.go`
- Create: `internal/runtime/hooks/command_unix_test.go`
- Modify: `docs/migration/mewcode-source-manifest.md`

- [ ] **Step 1: Copy the upstream tests before implementation**

Run:

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\hooks" | Out-Null
Copy-Item "$source\internal\hooks\hooks_test.go" "$target\internal\runtime\hooks\hooks_test.go"
```

- [ ] **Step 2: Add platform-specific shell contract tests**

Create `internal/runtime/hooks/command_windows_test.go`:

```go
//go:build windows

package hooks

import (
	"context"
	"testing"
)

func TestShellCommandUsesPowerShellOnWindows(t *testing.T) {
	cmd := shellCommandContext(context.Background(), `Write-Output $env:DEVFLOW_EVENT`)
	if got := cmd.Path; got == "" {
		t.Fatal("shell command path is empty")
	}
}

func devflowEventEchoCommand() string {
	return `Write-Output "$env:DEVFLOW_EVENT|$env:MEWCODE_EVENT"`
}
```

Create `internal/runtime/hooks/command_unix_test.go`:

```go
//go:build !windows

package hooks

import (
	"context"
	"testing"
)

func TestShellCommandUsesPOSIXShell(t *testing.T) {
	cmd := shellCommandContext(context.Background(), `printf '%s' "$DEVFLOW_EVENT"`)
	if got := cmd.Path; got == "" {
		t.Fatal("shell command path is empty")
	}
}

func devflowEventEchoCommand() string {
	return `printf '%s|%s' "$DEVFLOW_EVENT" "$MEWCODE_EVENT"`
}
```

Append this test to `internal/runtime/hooks/hooks_test.go`:

```go
func TestRunCommandExportsDevflowAndLegacyEnvironment(t *testing.T) {
	hook := Hook{
		ID: "environment",
		Action: Action{
			Type:    ActionCommand,
			Command: devflowEventEchoCommand(),
		},
	}
	result := runCommand(hook, HookContext{EventName: EventPostToolUse})
	if !result.Success {
		t.Fatalf("command failed: %s", result.Output)
	}
	want := string(EventPostToolUse) + "|" + string(EventPostToolUse)
	if !strings.Contains(result.Output, want) {
		t.Fatalf("output = %q, want %q", result.Output, want)
	}
}
```

- [ ] **Step 3: Run tests and verify the implementation is absent**

Run:

```powershell
go test ./internal/runtime/hooks
```

Expected: FAIL because `Hook`, `runCommand`, and `shellCommandContext` do not exist.

- [ ] **Step 4: Copy the hooks implementation**

Run:

```powershell
Copy-Item "$source\internal\hooks\hooks.go" "$target\internal\runtime\hooks\hooks.go"
```

- [ ] **Step 5: Add platform-specific command creation**

Create `internal/runtime/hooks/command_windows.go`:

```go
//go:build windows

package hooks

import (
	"context"
	"os/exec"
)

func shellCommandContext(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command)
}
```

Create `internal/runtime/hooks/command_unix.go`:

```go
//go:build !windows

package hooks

import (
	"context"
	"os/exec"
)

func shellCommandContext(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", command)
}
```

In `internal/runtime/hooks/hooks.go`, remove the `os/exec` import and replace:

```go
cmd := exec.CommandContext(execCtx, "bash", "-c", h.Action.Command)
cmd.Env = append(cmd.Environ(),
	"MEWCODE_EVENT="+string(ctx.EventName),
	"MEWCODE_TOOL="+ctx.ToolName,
	"MEWCODE_FILE_PATH="+ctx.FilePath,
)
```

with:

```go
cmd := shellCommandContext(execCtx, h.Action.Command)
cmd.Env = append(cmd.Environ(),
	"DEVFLOW_EVENT="+string(ctx.EventName),
	"DEVFLOW_TOOL="+ctx.ToolName,
	"DEVFLOW_FILE_PATH="+ctx.FilePath,
	"MEWCODE_EVENT="+string(ctx.EventName),
	"MEWCODE_TOOL="+ctx.ToolName,
	"MEWCODE_FILE_PATH="+ctx.FilePath,
)
```

The `MEWCODE_*` variables remain as migration compatibility aliases. New integrations must use `DEVFLOW_*`.

- [ ] **Step 6: Run package tests on Windows**

Run:

```powershell
gofmt -w internal/runtime/hooks
go test ./internal/runtime/hooks -count=1 -timeout 2m
```

Expected: PASS on Windows without requiring `bash`.

- [ ] **Step 7: Append the hooks source record**

Append to `docs/migration/mewcode-source-manifest.md`:

```markdown

### hooks

- Source: `internal/hooks`
- Target: `internal/runtime/hooks`
- Source files:
  - `hooks.go`: `C7127D660853A1C0DC0223FFEB9460FCACDAF684B312AE3A23B8237FF5FA097B`
  - `hooks_test.go`: `CC0436CC9226E175C0A6EEEDD6C72C32788555D8906088884BDB2BFF3B8D461B`
- Fusion changes:
  - Move hooks under the Devflow runtime boundary.
  - Export new `DEVFLOW_*` hook environment variables while retaining `MEWCODE_*` aliases.
  - Replace the hard-coded Bash launcher with platform-specific shell creation.
- Windows changes:
  - Use non-interactive PowerShell on Windows.
  - Preserve POSIX `sh -c` behavior on non-Windows systems.
- Verification: `go test ./internal/runtime/hooks -count=1 -timeout 2m`; `go test ./... -count=1`
- Lore intent: `Make runtime hooks executable on the supported Windows host`
```

- [ ] **Step 8: Run repository verification**

Run:

```powershell
go test ./... -count=1
go vet ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 9: Commit only the hooks package and manifest update**

Run:

```powershell
git add -- internal/runtime/hooks docs/migration/mewcode-source-manifest.md
git commit -m "Make runtime hooks executable on the supported Windows host" `
  -m "Migrate MewCode's hook engine while replacing its unconditional Bash dependency with platform-specific shell execution. Devflow environment names become authoritative and legacy MewCode names remain available during configuration migration." `
  -m "Constraint: One migrated Go package per commit" `
  -m "Constraint: Windows is a required development and verification platform" `
  -m "Rejected: Require Git Bash on Windows | adds an undocumented runtime dependency" `
  -m "Confidence: high" `
  -m "Scope-risk: moderate" `
  -m "Directive: New hook integrations must consume DEVFLOW_* variables" `
  -m "Tested: go test ./internal/runtime/hooks -count=1 -timeout 2m; go test ./... -count=1; go vet ./...; git diff --check" `
  -m "Not-tested: cmd.exe-specific scripts and Unix CI"
git status --short
```

Expected: commit succeeds and status is empty.

---

### Task 3: Migrate `config` With `.devflow` Authority

**Files:**
- Create: `internal/runtime/config/config.go`
- Create: `internal/runtime/config/config_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `docs/migration/mewcode-source-manifest.md`

- [ ] **Step 1: Write failing discovery and precedence tests**

Create `internal/runtime/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigForTest(t *testing.T, path, name, model string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "providers:\n" +
		"  - name: " + name + "\n" +
		"    protocol: openai-compat\n" +
		"    base_url: https://example.invalid/v1\n" +
		"    model: " + model + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDiscoveredConfigPrefersDevflowAtEachScope(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeConfigForTest(t, filepath.Join(home, ".mewcode", "config.yaml"), "legacy-user", "legacy-user-model")
	writeConfigForTest(t, filepath.Join(home, ".devflow", "config.yaml"), "devflow-user", "devflow-user-model")
	writeConfigForTest(t, filepath.Join(work, ".mewcode", "config.yaml"), "legacy-project", "legacy-project-model")
	writeConfigForTest(t, filepath.Join(work, ".devflow", "config.yaml"), "devflow-project", "devflow-project-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "devflow-project" {
		t.Fatalf("provider name = %q, want devflow-project", got)
	}
}

func TestLoadDiscoveredConfigFallsBackToLegacyMewCode(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	writeConfigForTest(t, filepath.Join(work, ".mewcode", "config.yaml"), "legacy-project", "ark-code-latest")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "legacy-project" {
		t.Fatalf("provider name = %q, want legacy-project", got)
	}
}

func TestProviderAPIKeyPrefersExplicitValueThenEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "environment-key")
	cfg := ProviderConfig{Protocol: "openai-compat", APIKey: "explicit-key"}
	if got := cfg.ResolveAPIKey(); got != "explicit-key" {
		t.Fatalf("explicit key = %q, want explicit-key", got)
	}

	cfg.APIKey = ""
	if got := cfg.ResolveAPIKey(); got != "environment-key" {
		t.Fatalf("environment key = %q, want environment-key", got)
	}
}

func TestLoadDiscoveredConfigRejectsInvalidHooks(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	path := filepath.Join(work, ".devflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `providers:
  - name: ark
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: ark-code-latest
hooks:
  - id: invalid
    event: no_such_event
    action:
      type: prompt
      message: hello
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadDiscoveredConfig(home, work)
	if err == nil {
		t.Fatal("expected invalid hook error")
	}
}
```

- [ ] **Step 2: Run the test and verify config is absent**

Run:

```powershell
go test ./internal/runtime/config
```

Expected: FAIL because the config package does not exist.

- [ ] **Step 3: Add the YAML dependency**

Run:

```powershell
go get gopkg.in/yaml.v3@v3.0.1
```

Expected: `go.mod` and `go.sum` include `gopkg.in/yaml.v3`.

- [ ] **Step 4: Copy and retarget the config implementation**

Run:

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\config" | Out-Null
Copy-Item "$source\internal\config\config.go" "$target\internal\runtime\config\config.go"
```

Replace:

```go
"mewcode/internal/hooks"
```

with:

```go
"github.com/jesseedcp/devflow-agent/internal/runtime/hooks"
```

- [ ] **Step 5: Implement explicit Devflow-first discovery**

Add these helpers to `internal/runtime/config/config.go`:

```go
func preferredConfig(primary, legacy string) string {
	if _, err := os.Stat(primary); err == nil {
		return primary
	}
	return legacy
}

func loadDiscoveredConfig(home, wd string) (*AppConfig, error) {
	candidates := []string{
		preferredConfig(
			filepath.Join(home, ".devflow", "config.yaml"),
			filepath.Join(home, ".mewcode", "config.yaml"),
		),
		preferredConfig(
			filepath.Join(wd, ".devflow", "config.yaml"),
			filepath.Join(wd, ".mewcode", "config.yaml"),
		),
		preferredConfig(
			filepath.Join(wd, ".devflow", "config.local.yaml"),
			filepath.Join(wd, ".mewcode", "config.local.yaml"),
		),
	}

	var merged *AppConfig
	for _, path := range candidates {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		layer, err := loadSingleFile(path)
		if err != nil {
			return nil, err
		}
		if merged == nil {
			merged = layer
		} else {
			merged = mergeConfig(merged, layer)
		}
	}

	if merged == nil {
		return nil, &ConfigError{
			Message: "No config file found. Expected .devflow/config.yaml or legacy .mewcode/config.yaml in the project or user home",
		}
	}
	if err := validateProviders(merged); err != nil {
		return nil, err
	}
	if err := hooks.Validate(merged.Hooks); err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("Invalid hooks configuration: %s", err)}
	}
	return merged, nil
}
```

Replace the discovery body at the end of `LoadConfig` with:

```go
	wd, err := os.Getwd()
	if err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("Failed to get working directory: %s", err)}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("Failed to get user home directory: %s", err)}
	}
	return loadDiscoveredConfig(home, wd)
```

For explicit `LoadConfig(path)`, validate hooks after providers:

```go
if err := hooks.Validate(cfg.Hooks); err != nil {
	return nil, &ConfigError{Message: fmt.Sprintf("Invalid hooks configuration: %s", err)}
}
```

- [ ] **Step 6: Run package tests**

Run:

```powershell
gofmt -w internal/runtime/config
go test ./internal/runtime/config -count=1
```

Expected: PASS.

- [ ] **Step 7: Append the config source record**

Append:

```markdown

### config

- Source: `internal/config`
- Target: `internal/runtime/config`
- Source files:
  - `config.go`: `DCC2376B6382AE0972B7E04B991D0378056361B45368E4816367F0D5DDA12B09`
- Fusion changes:
  - Move configuration under the Devflow runtime boundary.
  - Prefer `.devflow` at user, project, and local-override scopes.
  - Fall back to matching `.mewcode` files only when the Devflow file is absent.
  - Validate hook configuration during load.
- Windows changes: use `filepath`-based discovery and temp-directory tests.
- Verification: `go test ./internal/runtime/config -count=1`; `go test ./... -count=1`
- Lore intent: `Make Devflow configuration authoritative without breaking MewCode users`
```

- [ ] **Step 8: Run repository verification**

Run:

```powershell
go mod tidy
go test ./... -count=1
go vet ./...
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 9: Commit only config, dependency metadata, and manifest**

Run:

```powershell
git add -- internal/runtime/config go.mod go.sum docs/migration/mewcode-source-manifest.md
git commit -m "Make Devflow configuration authoritative without breaking MewCode users" `
  -m "Migrate the provider and integration configuration package with Devflow-first discovery at user and project scopes. Legacy MewCode files remain fallback inputs, environment-based API keys remain supported, and invalid hook definitions now fail during configuration loading." `
  -m "Constraint: One migrated Go package per commit" `
  -m "Constraint: New configuration writes belong under .devflow" `
  -m "Rejected: Merge Devflow and MewCode files at the same scope | two authorities would make overrides ambiguous" `
  -m "Confidence: high" `
  -m "Scope-risk: moderate" `
  -m "Directive: Treat .mewcode configuration as a migration fallback, not a permanent write target" `
  -m "Tested: go test ./internal/runtime/config -count=1; go test ./... -count=1; go vet ./...; git diff --check" `
  -m "Not-tested: Real provider requests until the llm package is migrated"
git status --short
```

Expected: commit succeeds and status is empty.

---

### Task 4: Migrate `llm` And Prove The Ark Connection

**Files:**
- Create: `internal/runtime/llm/anthropic.go`
- Create: `internal/runtime/llm/client.go`
- Create: `internal/runtime/llm/errors.go`
- Create: `internal/runtime/llm/events.go`
- Create: `internal/runtime/llm/model_resolver.go`
- Create: `internal/runtime/llm/openai.go`
- Create: `internal/runtime/llm/openai_compat.go`
- Create: `internal/runtime/llm/thinking_test.go`
- Create: `internal/runtime/llm/live_openai_compat_test.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `docs/migration/mewcode-source-manifest.md`

- [ ] **Step 1: Add a failing live-test contract**

Create `internal/runtime/llm/live_openai_compat_test.go`:

```go
package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
)

func TestLiveOpenAICompat(t *testing.T) {
	if os.Getenv("DEVFLOW_LIVE_LLM") != "1" {
		t.Skip("set DEVFLOW_LIVE_LLM=1 to run the real provider smoke test")
	}

	cfg, err := config.LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	var provider *config.ProviderConfig
	for index := range cfg.Providers {
		if cfg.Providers[index].Protocol == "openai-compat" {
			provider = &cfg.Providers[index]
			break
		}
	}
	if provider == nil {
		t.Fatal("no openai-compat provider configured")
	}

	client, err := NewClient(provider, "Reply concisely and follow the requested exact output.")
	if err != nil {
		t.Fatal(err)
	}
	conversationManager := conversation.NewManager()
	conversationManager.AddUserMessage("Reply with exactly: DEVFLOW_RUNTIME_OK")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	events, errs := client.Stream(ctx, conversationManager, nil)

	var response strings.Builder
	for event := range events {
		if delta, ok := event.(TextDelta); ok {
			response.WriteString(delta.Text)
		}
	}
	for streamErr := range errs {
		if streamErr != nil {
			t.Fatal(streamErr)
		}
	}
	if got := strings.TrimSpace(response.String()); got != "DEVFLOW_RUNTIME_OK" {
		t.Fatalf("response = %q, want DEVFLOW_RUNTIME_OK", got)
	}
}
```

- [ ] **Step 2: Copy upstream LLM tests before implementation**

Run:

```powershell
New-Item -ItemType Directory -Force "$target\internal\runtime\llm" | Out-Null
Copy-Item "$source\internal\llm\thinking_test.go" "$target\internal\runtime\llm\thinking_test.go"
```

Replace the test imports:

```go
"mewcode/internal/config"
"mewcode/internal/conversation"
```

with:

```go
"github.com/jesseedcp/devflow-agent/internal/runtime/config"
"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
```

- [ ] **Step 3: Run tests and verify the implementation is absent**

Run:

```powershell
go test ./internal/runtime/llm
```

Expected: FAIL because client and stream event types do not exist.

- [ ] **Step 4: Add SDK dependencies**

Run:

```powershell
go get github.com/anthropics/anthropic-sdk-go@v1.42.0
go get github.com/openai/openai-go@v1.12.0
```

If the configured Go proxy returns a checksum timeout, retry the same command with the official proxy for that command only:

```powershell
$env:GOPROXY='https://proxy.golang.org,direct'
go get github.com/anthropics/anthropic-sdk-go@v1.42.0
Remove-Item Env:GOPROXY
```

Do not disable `GOSUMDB`.

- [ ] **Step 5: Copy all LLM implementation files**

Run:

```powershell
$llmFiles = @(
  'anthropic.go',
  'client.go',
  'errors.go',
  'events.go',
  'model_resolver.go',
  'openai.go',
  'openai_compat.go'
)
foreach ($file in $llmFiles) {
  Copy-Item "$source\internal\llm\$file" "$target\internal\runtime\llm\$file"
}
```

- [ ] **Step 6: Retarget runtime imports and user-facing configuration paths**

In all copied LLM files, replace:

```go
"mewcode/internal/config"
"mewcode/internal/conversation"
```

with:

```go
"github.com/jesseedcp/devflow-agent/internal/runtime/config"
"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
```

Replace all authentication guidance of the form:

```text
Set it in .mewcode/config.yaml
```

with:

```text
Set it in .devflow/config.yaml, legacy .mewcode/config.yaml, or the matching environment variable
```

Do not rename `Client`, `StreamEvent`, or provider protocol strings.

- [ ] **Step 7: Run unit tests**

Run:

```powershell
gofmt -w internal/runtime/llm
go test ./internal/runtime/llm -count=1 -timeout 2m
```

Expected: PASS with the live test skipped.

- [ ] **Step 8: Run the real Ark smoke test**

The current project has a legacy MewCode provider file and a user-level `OPENAI_API_KEY`; config fallback should discover them.

Run:

```powershell
$env:DEVFLOW_LIVE_LLM='1'
go test ./internal/runtime/llm -run TestLiveOpenAICompat -count=1 -v -timeout 90s
Remove-Item Env:DEVFLOW_LIVE_LLM
```

Expected: PASS and the model response is exactly `DEVFLOW_RUNTIME_OK`. Test output and failures must never print the API key.

- [ ] **Step 9: Append the LLM source record**

Append:

```markdown

### llm

- Source: `internal/llm`
- Target: `internal/runtime/llm`
- Source files:
  - `anthropic.go`: `680AD6F8A93FFFD6C80458986DA43701DBC5677998EA397A5E813B20C84FA85F`
  - `client.go`: `2053006E3852C78881F776A059F39B8E96569513FF2F4A1382AE6E411F918813`
  - `errors.go`: `C5207CBF5DA03E4ADD20BE89B85311943D0E351970BE56D0C786040D26D9E58D`
  - `events.go`: `361C8431A9327EF9B72EEAC6DBC2A9B14320B30577D705E7EFB522B8AB3A6C5E`
  - `model_resolver.go`: `E485E5F476C3F1016C8C101F4BAAA0F3BC7E04C51141C7A173FF60BA8063B5FC`
  - `openai_compat.go`: `FCE2001AB8C7A78B31417D78CA629B82A47E2386CB2940853E37C061A22CA330`
  - `openai.go`: `2B0611EC8A76B4C50A113AC89AD819DB450769FDF112664F1D401BB0C0DA2620`
  - `thinking_test.go`: `2FBF296F2ACE17216F5254369D67C67559B771D80EF60E48CD69D4585CF5D0E3`
- Fusion changes:
  - Move provider clients under the Devflow runtime boundary.
  - Retarget configuration and conversation imports.
  - Update authentication guidance to `.devflow` with legacy fallback.
  - Add an opt-in real OpenAI-compatible provider smoke test.
- Windows changes: verify streaming clients and the Ark endpoint from the supported Windows host.
- Verification: `go test ./internal/runtime/llm -count=1`; live `TestLiveOpenAICompat`; `go test ./... -count=1`
- Lore intent: `Give Devflow a verified streaming model runtime`
```

- [ ] **Step 10: Run repository verification**

Run:

```powershell
go mod tidy
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 11: Commit only the LLM package, dependencies, and manifest**

Run:

```powershell
git add -- internal/runtime/llm go.mod go.sum docs/migration/mewcode-source-manifest.md
git commit -m "Give Devflow a verified streaming model runtime" `
  -m "Migrate MewCode's Anthropic, OpenAI Responses, and OpenAI-compatible streaming clients so later requirement and plan stages can use one Devflow-owned runtime. The package keeps the established event contract, reads Devflow-first configuration, and includes an opt-in real Ark smoke test." `
  -m "Constraint: One migrated Go package per commit" `
  -m "Constraint: API keys remain outside Git and must never appear in test output" `
  -m "Rejected: Implement a second minimal HTTP client for Ark | would discard tested provider and tool-call behavior" `
  -m "Confidence: high" `
  -m "Scope-risk: moderate" `
  -m "Directive: Stage agents must depend on the runtime Client interface rather than provider SDK types" `
  -m "Tested: go test ./internal/runtime/llm -count=1 -timeout 2m; DEVFLOW_LIVE_LLM=1 TestLiveOpenAICompat; go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check" `
  -m "Not-tested: Anthropic and OpenAI Responses against paid live endpoints"
git status --short
```

Expected: commit succeeds and status is empty.

---

### Task 5: Verify Wave 1 As A Clean Four-Commit Foundation

**Files:**
- Verify only; no file changes expected.

- [ ] **Step 1: Confirm the four package commits**

Run:

```powershell
git log -4 --format='%H %s'
```

Expected intent lines, newest first:

```text
Give Devflow a verified streaming model runtime
Make Devflow configuration authoritative without breaking MewCode users
Make runtime hooks executable on the supported Windows host
Preserve conversation semantics inside the Devflow runtime
```

- [ ] **Step 2: Confirm each migrated package is represented in the manifest**

Run:

```powershell
rg -n '^### (conversation|hooks|config|llm)$|^- Lore intent:' docs/migration/mewcode-source-manifest.md
```

Expected: one package section and one unique Lore intent for each migrated package.

- [ ] **Step 3: Run final verification from a clean worktree**

Run:

```powershell
git status --short
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
```

Expected: empty Git status and all Go commands exit 0.

- [ ] **Step 4: Record known remaining scope**

Do not create a commit. Report that Wave 1 provides configuration and model execution only. It does not yet expose a TUI, runtime CLI command, Requirements Agent, Plan integration, tools, permissions, MCP, or code execution. Those belong to later approved waves.

## Plan Self-Review

- Spec coverage: this plan covers the first dependency-complete runtime wave, source tracking, Windows-per-package verification, `.devflow` authority, legacy configuration fallback, API-key safety, and one-package-per-commit discipline.
- Deliberate gaps: the full fusion Spec is decomposed into Waves 2-5 listed above; none are silently treated as complete by Wave 1.
- Placeholder scan: the plan contains no unresolved marker or unspecified implementation step.
- Type consistency: runtime imports consistently use `internal/runtime/config` and `internal/runtime/conversation`; the live test uses the public `Client` and `StreamEvent` contracts migrated in Task 4.
- Commit consistency: each package task modifies only that package, dependency metadata when required, and the shared source manifest.

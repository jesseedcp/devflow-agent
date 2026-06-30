# Wave 19 Backend Demand Defaults Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add backend demand defaults to `.devflow/config.yaml` so run, console, drive, and workbench no longer require repeated quality, permission, and GitLab flags.

**Architecture:** Extend the existing runtime config schema with an optional `backend_demand` section, then add a small CLI resolver that applies config defaults only when explicit CLI flags are absent. Keep provider config behavior unchanged and keep demand commands compatible when no config file exists.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, existing `internal/runtime/config`, existing `internal/cli` command helpers, PowerShell on Windows, Go unit tests.

---

## Precondition

Execute this plan after Wave 18 is merged into `main`. Wave 19 touches `workbench`, `dogfood` documentation, and release-readiness surfaces that Wave 18 just added.

## File Structure

- Modify `internal/runtime/config/config.go` to add backend demand config structs, merge, clone, and validation.
- Modify `internal/runtime/config/config_test.go` for config loading and merge tests.
- Create `internal/cli/demand_defaults.go` for command-facing default resolution.
- Create `internal/cli/demand_defaults_test.go` for resolver behavior.
- Modify `internal/cli/run.go`, `console.go`, `drive.go`, `workbench.go`, and `workbench_model.go` to apply defaults.
- Modify `internal/cli/*_test.go` around run, console, drive, workbench.
- Modify `internal/cli/doctor.go` and `doctor_test.go`.
- Modify `internal/cli/init.go` and `init_test.go`.
- Modify `docs/examples/config.openai-compat.yaml`, `docs/examples/config.anthropic.yaml`, `docs/user-guide/backend-demand-loop.md`, and `docs/release/v0.1.md`.

## Task 1: Extend Runtime Config Schema

**Files:**
- Modify: `internal/runtime/config/config.go`
- Modify: `internal/runtime/config/config_test.go`

- [ ] **Step 1: Write failing config load test**

Append to `internal/runtime/config/config_test.go`:

```go
func TestLoadConfigBackendDemandDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`providers:
  - name: test
    protocol: openai-compat
    base_url: https://example.com/v1
    model: test-model
backend_demand:
  runner_root: .
  quality_root: .
  quality_commands:
    - go test ./... -count=1 -timeout 5m
  permission_mode: acceptEdits
  gitlab:
    project: group/project
    base_url: https://gitlab.example
    default_target_branch: main
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.BackendDemand.RunnerRoot != "." {
		t.Fatalf("RunnerRoot = %q, want .", cfg.BackendDemand.RunnerRoot)
	}
	if got := cfg.BackendDemand.QualityCommands; len(got) != 1 || got[0] != "go test ./... -count=1 -timeout 5m" {
		t.Fatalf("QualityCommands = %#v", got)
	}
	if cfg.BackendDemand.PermissionMode != "acceptEdits" {
		t.Fatalf("PermissionMode = %q", cfg.BackendDemand.PermissionMode)
	}
	if cfg.BackendDemand.GitLab.Project != "group/project" {
		t.Fatalf("GitLab.Project = %q", cfg.BackendDemand.GitLab.Project)
	}
	if cfg.BackendDemand.GitLab.DefaultTargetBranch != "main" {
		t.Fatalf("DefaultTargetBranch = %q", cfg.BackendDemand.GitLab.DefaultTargetBranch)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```powershell
go test ./internal/runtime/config -run TestLoadConfigBackendDemandDefaults -count=1
```

Expected: FAIL because `AppConfig.BackendDemand` does not exist.

- [ ] **Step 3: Add config structs**

Modify `internal/runtime/config/config.go`:

```go
type BackendDemandConfig struct {
	RunnerRoot           string       `yaml:"runner_root"`
	QualityRoot          string       `yaml:"quality_root"`
	QualityCommands      []string     `yaml:"quality_commands"`
	PermissionMode       string       `yaml:"permission_mode"`
	GitLab               GitLabConfig `yaml:"gitlab"`
	CreateMRTargetBranch string       `yaml:"create_mr_target_branch"`
}

type GitLabConfig struct {
	Project             string `yaml:"project"`
	BaseURL             string `yaml:"base_url"`
	DefaultTargetBranch string `yaml:"default_target_branch"`
}
```

Update `AppConfig`:

```go
type AppConfig struct {
	Providers      []ProviderConfig     `yaml:"providers"`
	PermissionMode string               `yaml:"permission_mode"`
	BackendDemand  BackendDemandConfig  `yaml:"backend_demand"`
	MCPServers     []MCPServerConfig    `yaml:"mcp_servers"`
	Hooks          []hooks.Hook         `yaml:"hooks"`
}
```

- [ ] **Step 4: Clone backend demand config**

Add helper:

```go
func cloneBackendDemandConfig(in BackendDemandConfig) BackendDemandConfig {
	out := in
	if in.QualityCommands != nil {
		out.QualityCommands = append([]string(nil), in.QualityCommands...)
	}
	return out
}
```

Update `cloneAppConfig`:

```go
return &AppConfig{
	Providers:      cloneProviderConfigs(cfg.Providers),
	PermissionMode: cfg.PermissionMode,
	BackendDemand:  cloneBackendDemandConfig(cfg.BackendDemand),
	MCPServers:     cloneMCPServers(cfg.MCPServers),
	Hooks:          cloneHooks(cfg.Hooks),
}
```

- [ ] **Step 5: Merge backend demand config**

Add helper:

```go
func mergeBackendDemand(base, override BackendDemandConfig) BackendDemandConfig {
	merged := cloneBackendDemandConfig(base)
	if override.RunnerRoot != "" {
		merged.RunnerRoot = override.RunnerRoot
	}
	if override.QualityRoot != "" {
		merged.QualityRoot = override.QualityRoot
	}
	if override.QualityCommands != nil {
		merged.QualityCommands = append([]string(nil), override.QualityCommands...)
	}
	if override.PermissionMode != "" {
		merged.PermissionMode = override.PermissionMode
	}
	if override.CreateMRTargetBranch != "" {
		merged.CreateMRTargetBranch = override.CreateMRTargetBranch
	}
	if override.GitLab.Project != "" {
		merged.GitLab.Project = override.GitLab.Project
	}
	if override.GitLab.BaseURL != "" {
		merged.GitLab.BaseURL = override.GitLab.BaseURL
	}
	if override.GitLab.DefaultTargetBranch != "" {
		merged.GitLab.DefaultTargetBranch = override.GitLab.DefaultTargetBranch
	}
	return merged
}
```

In `mergeConfig`, after permission mode:

```go
merged.BackendDemand = mergeBackendDemand(merged.BackendDemand, override.BackendDemand)
```

- [ ] **Step 6: Validate backend demand defaults**

Add validation helper:

```go
func validateBackendDemand(cfg BackendDemandConfig) error {
	for i, raw := range cfg.QualityCommands {
		if strings.TrimSpace(raw) == "" {
			return &ConfigError{Message: fmt.Sprintf("backend_demand.quality_commands[%d] must not be empty", i)}
		}
	}
	switch cfg.PermissionMode {
	case "", "default", "acceptEdits", "bypassPermissions":
		return nil
	default:
		return &ConfigError{Message: fmt.Sprintf("backend_demand.permission_mode %q is invalid", cfg.PermissionMode)}
	}
}
```

Call it in `validateLayer` after provider/name validation:

```go
if err := validateBackendDemand(cfg.BackendDemand); err != nil {
	return err
}
```

- [ ] **Step 7: Run config tests**

Run:

```powershell
gofmt -w internal/runtime/config/config.go internal/runtime/config/config_test.go
go test ./internal/runtime/config -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit config schema**

Run:

```powershell
git add internal/runtime/config/config.go internal/runtime/config/config_test.go
git commit -m "Add backend demand defaults to config" -m "Demand commands need stable project defaults for quality gates, permissions, and GitLab settings, so the runtime config now accepts an optional backend_demand section." -m "Constraint: Existing provider config files without backend_demand must remain valid." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/runtime/config -count=1"
```

## Task 2: Add CLI Defaults Resolver

**Files:**
- Create: `internal/cli/demand_defaults.go`
- Create: `internal/cli/demand_defaults_test.go`

- [ ] **Step 1: Write failing resolver tests**

Create `internal/cli/demand_defaults_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDemandDefaultsReturnsEmptyWhenConfigMissing(t *testing.T) {
	defaults, err := resolveDemandDefaults(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("resolveDemandDefaults returned error: %v", err)
	}
	if len(defaults.QualityCommands) != 0 {
		t.Fatalf("QualityCommands = %#v, want empty", defaults.QualityCommands)
	}
}

func TestResolveDemandDefaultsLoadsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`providers:
  - name: test
    protocol: openai-compat
    base_url: https://example.com/v1
    model: test-model
backend_demand:
  runner_root: repo
  quality_root: repo
  quality_commands:
    - go test ./...
  permission_mode: acceptEdits
  gitlab:
    project: group/project
    base_url: https://gitlab.example
    default_target_branch: main
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	defaults, err := resolveDemandDefaults(path)
	if err != nil {
		t.Fatalf("resolveDemandDefaults returned error: %v", err)
	}
	if defaults.RunnerRoot != "repo" || defaults.QualityRoot != "repo" {
		t.Fatalf("roots = %q/%q, want repo/repo", defaults.RunnerRoot, defaults.QualityRoot)
	}
	if len(defaults.QualityCommands) != 1 || defaults.QualityCommands[0] != "go test ./..." {
		t.Fatalf("QualityCommands = %#v", defaults.QualityCommands)
	}
	if defaults.PermissionMode != "acceptEdits" {
		t.Fatalf("PermissionMode = %q", defaults.PermissionMode)
	}
	if defaults.GitLabProject != "group/project" || defaults.GitLabBaseURL != "https://gitlab.example" {
		t.Fatalf("gitlab defaults = %q/%q", defaults.GitLabProject, defaults.GitLabBaseURL)
	}
	if defaults.CreateMRTargetBranch != "main" {
		t.Fatalf("CreateMRTargetBranch = %q", defaults.CreateMRTargetBranch)
	}
}

func writeBackendDemandDefaultsConfig(t *testing.T, root string) string {
	t.Helper()
	configPath := filepath.Join(root, ".devflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`providers:
  - name: test
    protocol: openai-compat
    base_url: https://example.com/v1
    model: test-model
backend_demand:
  runner_root: .
  quality_root: .
  quality_commands:
    - go test ./...
  permission_mode: acceptEdits
  gitlab:
    project: group/project
    base_url: https://gitlab.example
    default_target_branch: main
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```powershell
go test ./internal/cli -run TestResolveDemandDefaults -count=1
```

Expected: FAIL because resolver types/functions do not exist.

- [ ] **Step 3: Implement resolver**

Create `internal/cli/demand_defaults.go`:

```go
package cli

import (
	"os"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
)

type demandCommandDefaults struct {
	RunnerRoot           string
	QualityRoot          string
	QualityCommands      []string
	PermissionMode       string
	GitLabProject        string
	GitLabBaseURL        string
	CreateMRTargetBranch string
}

func resolveDemandDefaults(configPath string) (demandCommandDefaults, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if configPath != "" && os.IsNotExist(err) {
			return demandCommandDefaults{}, nil
		}
		if configPath == "" {
			return demandCommandDefaults{}, nil
		}
		return demandCommandDefaults{}, err
	}
	backend := cfg.BackendDemand
	return demandCommandDefaults{
		RunnerRoot:           strings.TrimSpace(backend.RunnerRoot),
		QualityRoot:          strings.TrimSpace(backend.QualityRoot),
		QualityCommands:      trimStringSlice(backend.QualityCommands),
		PermissionMode:       strings.TrimSpace(backend.PermissionMode),
		GitLabProject:        strings.TrimSpace(backend.GitLab.Project),
		GitLabBaseURL:        strings.TrimSpace(backend.GitLab.BaseURL),
		CreateMRTargetBranch: firstNonEmpty(strings.TrimSpace(backend.CreateMRTargetBranch), strings.TrimSpace(backend.GitLab.DefaultTargetBranch)),
	}, nil
}

func trimStringSlice(values []string) []string {
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
```

- [ ] **Step 4: Run resolver tests**

Run:

```powershell
gofmt -w internal/cli/demand_defaults.go internal/cli/demand_defaults_test.go
go test ./internal/cli -run TestResolveDemandDefaults -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit resolver**

Run:

```powershell
git add internal/cli/demand_defaults.go internal/cli/demand_defaults_test.go
git commit -m "Resolve backend demand defaults for CLI commands" -m "Demand commands now have a small resolver that reads backend_demand config without forcing config files on existing no-config workflows." -m "Constraint: Missing config remains compatible for demand commands." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run TestResolveDemandDefaults -count=1"
```

## Task 3: Apply Defaults to `devflow run`

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`

- [ ] **Step 1: Write failing implementation defaults test**

Append to `internal/cli/run_test.go`:

```go
func TestRunImplementationUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	repoRoot := t.TempDir()
	createDemandAtState(t, root, workflow.Implementation)
	configPath := filepath.Join(root, ".devflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`providers:
  - name: test
    protocol: openai-compat
    base_url: https://example.com/v1
    model: test-model
backend_demand:
  runner_root: `+repoRoot+`
  quality_root: `+repoRoot+`
  quality_commands:
    - go test ./...
  permission_mode: acceptEdits
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldRunner := newDemandRunner
	defer func() { newDemandRunner = oldRunner }()
	var gotMode permissions.PermissionMode
	newDemandRunner = func(configPath string, mode permissions.PermissionMode) demandflow.Runner {
		gotMode = mode
		return demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageImplementation: {Text: "implemented"},
		}}
	}

	var stdout bytes.Buffer
	if err := Run([]string{"run", "--root", root, "--config", configPath, "--demand", "add-coupon-check", "--stage", "implementation"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("run returned error: %v\n%s", err, stdout.String())
	}
	if gotMode != permissions.ModeAcceptEdits {
		t.Fatalf("permission mode = %s, want acceptEdits", gotMode)
	}
	progress, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if !strings.Contains(string(progress), "go test ./...") {
		t.Fatalf("progress missing default quality command:\n%s", string(progress))
	}
}
```

If `run_test.go` already has helpers/imports for `filepath`, `os`, `permissions`, `demandflow`, or `artifacts`, reuse them instead of duplicating imports.

- [ ] **Step 2: Run test and verify failure**

Run:

```powershell
go test ./internal/cli -run TestRunImplementationUsesBackendDemandDefaults -count=1
```

Expected: FAIL because `run` ignores backend demand defaults.

- [ ] **Step 3: Apply defaults in run command**

In `runDemandStage`, after parsing and trimming root/demand/stage, load defaults:

```go
defaults, err := resolveDemandDefaults(configPath)
if err != nil {
	return err
}
runnerRoot = firstNonEmpty(strings.TrimSpace(runnerRoot), defaults.RunnerRoot)
qualityRoot = firstNonEmpty(strings.TrimSpace(qualityRoot), defaults.QualityRoot)
permissionMode = firstNonEmpty(strings.TrimSpace(permissionMode), defaults.PermissionMode)
gitlabProject = firstNonEmpty(strings.TrimSpace(gitlabProject), defaults.GitLabProject)
gitlabBaseURL = firstNonEmpty(strings.TrimSpace(gitlabBaseURL), defaults.GitLabBaseURL)
createMRTargetBranch = firstNonEmpty(strings.TrimSpace(createMRTargetBranch), defaults.CreateMRTargetBranch)
if len(qualityCommands) == 0 {
	for _, command := range defaults.QualityCommands {
		qualityCommands = append(qualityCommands, command)
	}
}
```

Keep the later quality command parsing code unchanged.

- [ ] **Step 4: Run test**

Run:

```powershell
gofmt -w internal/cli/run.go internal/cli/run_test.go
go test ./internal/cli -run TestRunImplementationUsesBackendDemandDefaults -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit run defaults**

Run:

```powershell
git add internal/cli/run.go internal/cli/run_test.go
git commit -m "Apply backend demand defaults to run" -m "The run command now uses configured backend demand defaults for runner root, quality root, quality commands, permission mode, and GitLab fields when flags are omitted." -m "Constraint: Explicit CLI flags remain higher precedence than config." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run TestRunImplementationUsesBackendDemandDefaults -count=1"
```

## Task 4: Apply Defaults to Console, Drive, and Workbench

**Files:**
- Modify: `internal/cli/console.go`
- Modify: `internal/cli/console_test.go`
- Modify: `internal/cli/drive.go`
- Modify: `internal/cli/drive_test.go`
- Modify: `internal/cli/workbench_model.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add console default test**

Append to `internal/cli/console_test.go`:

```go
func TestConsoleRunNextUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "console-defaults", Title: "Console defaults", Source: "test", State: string(workflow.Implementation)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	configPath := writeBackendDemandDefaultsConfig(t, root)
	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var got []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		got = append([]string(nil), args...)
		return nil
	}

	if err := Run([]string{"console", "--root", root, "--config", configPath, "--demand", "console-defaults", "--run-next"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("console returned error: %v", err)
	}
	for _, want := range []string{"--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(got, want) {
			t.Fatalf("console args missing %q: %#v", want, got)
		}
	}
}
```

- [ ] **Step 2: Add drive default test**

Append to `internal/cli/drive_test.go`:

```go
func TestDriveUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-defaults", Title: "Drive defaults", Source: "test", State: string(workflow.Implementation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	configPath := writeBackendDemandDefaultsConfig(t, root)
	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var got []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		got = append([]string(nil), args...)
		loaded, err := store.LoadDemand(demand.ID)
		if err != nil {
			return err
		}
		loaded.State = string(workflow.MRReview)
		return store.SaveDemand(loaded)
	}

	if err := Run([]string{"drive", "--root", root, "--config", configPath, "--demand", demand.ID, "--max-steps", "1"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive returned error: %v", err)
	}
	for _, want := range []string{"--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(got, want) {
			t.Fatalf("drive args missing %q: %#v", want, got)
		}
	}
}
```

- [ ] **Step 3: Add workbench default shortcut test**

Append to `internal/cli/workbench_test.go`:

```go
func TestWorkbenchDriveUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := writeBackendDemandDefaultsConfig(t, root)
	var got workbenchOptions
	old := workbenchDrive
	defer func() { workbenchDrive = old }()
	workbenchDrive = func(opts workbenchOptions, demandID string) string {
		got = opts
		return "drive called"
	}

	model := workbenchModel{opts: workbenchOptions{root: root, configPath: configPath}, demands: []workbenchDemand{{ID: "wb-defaults"}}}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := next.(workbenchModel)
	if updated.message != "drive called" {
		t.Fatalf("message = %q", updated.message)
	}
	if len(got.qualityCommand) != 1 || got.qualityCommand[0] != "go test ./..." {
		t.Fatalf("quality defaults = %#v", got.qualityCommand)
	}
}
```

- [ ] **Step 4: Run tests and verify failure**

Run:

```powershell
go test ./internal/cli -run "TestConsoleRunNextUsesBackendDemandDefaults|TestDriveUsesBackendDemandDefaults|TestWorkbenchDriveUsesBackendDemandDefaults" -count=1
```

Expected: FAIL because console/drive/workbench do not apply config defaults yet.

- [ ] **Step 5: Implement defaults application helper**

In `internal/cli/demand_defaults.go`, add:

```go
func applyDefaultsToConsoleArgs(opts *consoleArgs) error {
	defaults, err := resolveDemandDefaults(opts.configPath)
	if err != nil {
		return err
	}
	opts.runnerRoot = firstNonEmpty(strings.TrimSpace(opts.runnerRoot), defaults.RunnerRoot)
	opts.qualityRoot = firstNonEmpty(strings.TrimSpace(opts.qualityRoot), defaults.QualityRoot)
	opts.permissionMode = firstNonEmpty(strings.TrimSpace(opts.permissionMode), defaults.PermissionMode)
	opts.gitlabProject = firstNonEmpty(strings.TrimSpace(opts.gitlabProject), defaults.GitLabProject)
	opts.gitlabBaseURL = firstNonEmpty(strings.TrimSpace(opts.gitlabBaseURL), defaults.GitLabBaseURL)
	if len(opts.qualityCommand) == 0 {
		for _, command := range defaults.QualityCommands {
			opts.qualityCommand = append(opts.qualityCommand, command)
		}
	}
	return nil
}

func applyDefaultsToWorkbenchOptions(opts *workbenchOptions) error {
	defaults, err := resolveDemandDefaults(opts.configPath)
	if err != nil {
		return err
	}
	if len(opts.qualityCommand) == 0 {
		for _, command := range defaults.QualityCommands {
			opts.qualityCommand = append(opts.qualityCommand, command)
		}
	}
	return nil
}
```

- [ ] **Step 6: Call defaults from console and drive**

In `runConsole`, after parsing:

```go
if err := applyDefaultsToConsoleArgs(&opts); err != nil {
	return err
}
```

In `runDrive`, after parsing:

```go
consoleOpts := consoleArgs{
	root:           opts.root,
	demandID:       opts.demandID,
	runnerRoot:     opts.runnerRoot,
	qualityRoot:    opts.qualityRoot,
	configPath:     opts.configPath,
	permissionMode: opts.permissionMode,
	gitlabProject:  opts.gitlabProject,
	gitlabMR:       opts.gitlabMR,
	gitlabBaseURL:  opts.gitlabBaseURL,
	qualityCommand: opts.qualityCommand,
}
if err := applyDefaultsToConsoleArgs(&consoleOpts); err != nil {
	return err
}
opts.runnerRoot = consoleOpts.runnerRoot
opts.qualityRoot = consoleOpts.qualityRoot
opts.permissionMode = consoleOpts.permissionMode
opts.gitlabProject = consoleOpts.gitlabProject
opts.gitlabBaseURL = consoleOpts.gitlabBaseURL
opts.qualityCommand = consoleOpts.qualityCommand
```

Then keep existing dry-run / loop branch.

- [ ] **Step 7: Call defaults from workbench**

In `runWorkbench`, after parse:

```go
if err := applyDefaultsToWorkbenchOptions(&opts); err != nil {
	return err
}
```

- [ ] **Step 8: Run tests**

Run:

```powershell
gofmt -w internal/cli/demand_defaults.go internal/cli/console.go internal/cli/console_test.go internal/cli/drive.go internal/cli/drive_test.go internal/cli/workbench.go internal/cli/workbench_test.go
go test ./internal/cli -run "TestConsoleRunNextUsesBackendDemandDefaults|TestDriveUsesBackendDemandDefaults|TestWorkbenchDriveUsesBackendDemandDefaults" -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit command defaults**

Run:

```powershell
git add internal/cli/demand_defaults.go internal/cli/console.go internal/cli/console_test.go internal/cli/drive.go internal/cli/drive_test.go internal/cli/workbench.go internal/cli/workbench_test.go
git commit -m "Apply backend demand defaults to operator commands" -m "Console, drive, and workbench now inherit configured backend demand defaults so operator shortcuts behave like explicit CLI invocations without repeated flags." -m "Constraint: Explicit command flags still override config defaults." -m "Confidence: high" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run backend-demand-defaults-focused-tests -count=1"
```

## Task 5: Doctor, Init, Docs, and Examples

**Files:**
- Modify: `internal/cli/doctor.go`
- Modify: `internal/cli/doctor_test.go`
- Modify: `internal/cli/init.go`
- Modify: `internal/cli/init_test.go`
- Modify: `docs/examples/config.openai-compat.yaml`
- Modify: `docs/examples/config.anthropic.yaml`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Add doctor test**

Append to `internal/cli/doctor_test.go`:

```go
func TestDoctorReportsBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := writeBackendDemandDefaultsConfig(t, root)
	t.Setenv("OPENAI_API_KEY", "test-key")
	var stdout bytes.Buffer
	err := Run([]string{"doctor", "--config", configPath}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("doctor returned error: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "[OK] backend-demand: quality command defaults configured") {
		t.Fatalf("doctor output missing backend-demand defaults:\n%s", stdout.String())
	}
}
```

- [ ] **Step 2: Add backend-demand doctor check**

In `runDoctorChecks`, append:

```go
checks = append(checks, checkBackendDemandDefaults(configPath))
```

Add:

```go
func checkBackendDemandDefaults(configPath string) doctorCheck {
	defaults, err := resolveDemandDefaults(configPath)
	if err != nil {
		return doctorCheck{Name: "backend-demand", OK: false, Message: err.Error()}
	}
	if len(defaults.QualityCommands) == 0 && defaults.PermissionMode == "" && defaults.GitLabProject == "" {
		return doctorCheck{Name: "backend-demand", OK: true, Message: "no defaults configured; CLI flags remain required"}
	}
	if len(defaults.QualityCommands) > 0 {
		return doctorCheck{Name: "backend-demand", OK: true, Message: "quality command defaults configured"}
	}
	return doctorCheck{Name: "backend-demand", OK: true, Message: "defaults configured"}
}
```

- [ ] **Step 3: Update init config output**

In each `renderInitialConfig` template, append:

```yaml
backend_demand:
  quality_commands:
    - go test ./... -count=1 -timeout 5m
  permission_mode: acceptEdits
  gitlab:
    default_target_branch: main
```

Keep project-specific GitLab project empty by default.

- [ ] **Step 4: Update docs and examples**

Add backend demand section to example configs:

```yaml
backend_demand:
  quality_root: .
  quality_commands:
    - go test ./... -count=1 -timeout 5m
  permission_mode: acceptEdits
  gitlab:
    default_target_branch: main
```

Add user guide section:

````markdown
### Backend demand defaults

Put repeated operator flags in `.devflow/config.yaml`:

```yaml
backend_demand:
  quality_commands:
    - go test ./... -count=1 -timeout 5m
  permission_mode: acceptEdits
  gitlab:
    project: group/project
    default_target_branch: main
```

Explicit CLI flags override these defaults.
````

Use four backticks around the Markdown snippet in the actual doc edit to avoid nested fence breakage.

- [ ] **Step 5: Run doctor/init/doc tests**

Run:

```powershell
gofmt -w internal/cli/doctor.go internal/cli/doctor_test.go internal/cli/init.go internal/cli/init_test.go
go test ./internal/cli -run "TestDoctor|TestInit" -count=1
rg -n "backend_demand|Backend demand defaults|Wave 19" docs\examples docs\user-guide\backend-demand-loop.md docs\release\v0.1.md
```

Expected: tests pass and docs contain backend demand defaults.

- [ ] **Step 6: Commit docs and discoverability**

Run:

```powershell
git add internal/cli/doctor.go internal/cli/doctor_test.go internal/cli/init.go internal/cli/init_test.go docs/examples/config.openai-compat.yaml docs/examples/config.anthropic.yaml docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document and diagnose backend demand defaults" -m "Init, doctor, examples, and user docs now surface backend_demand defaults so users can discover the shorter operator workflow." -m "Constraint: Generated configs must not include secrets or project-specific GitLab paths." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run \"TestDoctor|TestInit\" -count=1; rg backend_demand docs"
```

## Task 6: Full Verification and PR

**Files:**
- All modified files from Tasks 1-5.

- [ ] **Step 1: Run focused tests**

Run:

```powershell
go test ./internal/runtime/config ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full verification**

Run:

```powershell
go test ./... -count=1 -timeout 5m
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 3: Manual smoke**

Create a temporary config and demand, then run:

```powershell
go run ./cmd/devflow init --root $env:TEMP\devflow-wave19-smoke --provider openai-compat --force
go run ./cmd/devflow doctor --config $env:TEMP\devflow-wave19-smoke\.devflow\config.yaml
```

Expected:

```text
[OK] backend-demand:
```

- [ ] **Step 4: Inspect branch**

Run:

```powershell
git status --short --branch
git log --oneline -8
```

Expected: clean branch with Wave 19 commits.

- [ ] **Step 5: Push and open PR**

Run:

```powershell
git push -u origin <wave-19-branch>
gh pr create --base main --head <wave-19-branch> --title "Wave 19 backend demand defaults" --body "## Summary

- Add backend_demand config defaults.
- Apply defaults to run, console, drive, and workbench.
- Surface defaults in doctor, init, examples, and docs.

## Verification

- go test ./internal/runtime/config ./internal/cli -count=1
- go test ./... -count=1 -timeout 5m
- go vet ./...
- go build ./cmd/devflow
- git diff --check"
```

- [ ] **Step 6: Wait for CI**

Run:

```powershell
gh pr checks --watch --fail-fast
```

Expected: Ubuntu and Windows Go verification pass.

## Acceptance Checklist

- `backend_demand` config loads and merges.
- `run` uses config defaults when flags are omitted.
- `console --run-next` uses config defaults.
- `drive` uses config defaults.
- `workbench` action shortcuts use config defaults.
- Explicit CLI flags override config.
- No-config demand commands remain compatible.
- Doctor reports backend-demand defaults.
- Init/examples/docs show backend_demand.
- Full local verification and CI pass.

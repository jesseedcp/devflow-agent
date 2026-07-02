package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/runtime/hooks"
)

func TestLoadDiscoveredConfigPrefersDevflowInUserScope(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeProviderConfig(t, filepath.Join(home, ".mewcode", "config.yaml"), "legacy-user", "legacy-user-model")
	writeProviderConfig(t, filepath.Join(home, ".devflow", "config.yaml"), "devflow-user", "devflow-user-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "devflow-user" {
		t.Fatalf("provider name = %q, want devflow-user", got)
	}
}

func TestLoadDiscoveredConfigPrefersDevflowInProjectScope(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeProviderConfig(t, filepath.Join(home, ".devflow", "config.yaml"), "devflow-user", "user-model")
	writeProviderConfig(t, filepath.Join(work, ".mewcode", "config.yaml"), "legacy-project", "legacy-project-model")
	writeProviderConfig(t, filepath.Join(work, ".devflow", "config.yaml"), "devflow-project", "devflow-project-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "devflow-project" {
		t.Fatalf("provider name = %q, want devflow-project", got)
	}
}

func TestLoadDiscoveredConfigPrefersDevflowInLocalScope(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeProviderConfig(t, filepath.Join(home, ".devflow", "config.yaml"), "devflow-user", "user-model")
	writeProviderConfig(t, filepath.Join(work, ".devflow", "config.yaml"), "devflow-project", "project-model")
	writeProviderConfig(t, filepath.Join(work, ".mewcode", "config.local.yaml"), "legacy-local", "legacy-local-model")
	writeProviderConfig(t, filepath.Join(work, ".devflow", "config.local.yaml"), "devflow-local", "devflow-local-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "devflow-local" {
		t.Fatalf("provider name = %q, want devflow-local", got)
	}
}

func TestLoadDiscoveredConfigFallsBackToLegacyMewCode(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeProviderConfig(t, filepath.Join(home, ".mewcode", "config.yaml"), "legacy-user", "legacy-user-model")
	writeProviderConfig(t, filepath.Join(work, ".mewcode", "config.yaml"), "legacy-project", "legacy-project-model")
	writeProviderConfig(t, filepath.Join(work, ".mewcode", "config.local.yaml"), "legacy-local", "legacy-local-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "legacy-local" {
		t.Fatalf("provider name = %q, want legacy-local", got)
	}
}

func TestLoadDiscoveredConfigProjectOverridesUser(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeProviderConfig(t, filepath.Join(home, ".devflow", "config.yaml"), "devflow-user", "user-model")
	writeProviderConfig(t, filepath.Join(work, ".devflow", "config.yaml"), "devflow-project", "project-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "devflow-project" {
		t.Fatalf("provider name = %q, want devflow-project", got)
	}
}

func TestLoadDiscoveredConfigLocalOverridesProject(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeProviderConfig(t, filepath.Join(home, ".devflow", "config.yaml"), "devflow-user", "user-model")
	writeProviderConfig(t, filepath.Join(work, ".devflow", "config.yaml"), "devflow-project", "project-model")
	writeProviderConfig(t, filepath.Join(work, ".devflow", "config.local.yaml"), "devflow-local", "local-model")

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "devflow-local" {
		t.Fatalf("provider name = %q, want devflow-local", got)
	}
}

func TestLoadDiscoveredConfigNoConfigMentionsDevflowAndLegacyMewCode(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	_, err := loadDiscoveredConfig(home, work)
	if err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(err.Error(), ".devflow") {
		t.Fatalf("expected missing config error to mention .devflow, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), ".mewcode") {
		t.Fatalf("expected missing config error to mention .mewcode, got %q", err.Error())
	}
}

func TestLoadConfigLoadsExplicitPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom-config.yaml")
	writeProviderConfig(t, path, "explicit-provider", "explicit-model")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Providers[0].Name; got != "explicit-provider" {
		t.Fatalf("provider name = %q, want explicit-provider", got)
	}
}

func TestProviderAPIKeyPrefersExplicitValueThenEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "environment-key")

	provider := ProviderConfig{Protocol: "openai-compat", APIKey: "explicit-key"}
	if got := provider.ResolveAPIKey(); got != "explicit-key" {
		t.Fatalf("explicit key = %q, want explicit-key", got)
	}

	provider.APIKey = ""
	if got := provider.ResolveAPIKey(); got != "environment-key" {
		t.Fatalf("environment key = %q, want environment-key", got)
	}
}

func TestLoadConfigRejectsInvalidProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-provider.yaml")
	writeRawConfig(t, path, `providers:
  - name: bad
    protocol: invalid
    base_url: https://example.invalid/v1
    model: nope
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected invalid provider error")
	}
	if !strings.Contains(err.Error(), "invalid protocol") {
		t.Fatalf("expected invalid protocol error, got %q", err.Error())
	}
}

func TestLoadDiscoveredConfigRejectsInvalidHooks(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	path := filepath.Join(work, ".devflow", "config.yaml")
	writeRawConfig(t, path, `providers:
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
`)

	_, err := loadDiscoveredConfig(home, work)
	if err == nil {
		t.Fatal("expected invalid hooks error")
	}
	if !strings.Contains(err.Error(), "Invalid hooks configuration") {
		t.Fatalf("expected invalid hooks error, got %q", err.Error())
	}
}

func TestLoadConfigExplicitPathRejectsInvalidHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid-hooks.yaml")
	writeRawConfig(t, path, `providers:
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
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected invalid hooks error")
	}
	if !strings.Contains(err.Error(), "Invalid hooks configuration") {
		t.Fatalf("expected invalid hooks error, got %q", err.Error())
	}
}

func TestLoadConfigRejectsDuplicateProviderNamesInFinalSlice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duplicate-providers.yaml")
	writeRawConfig(t, path, `providers:
  - name: duplicate
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: model-one
  - name: duplicate
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: model-two
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected duplicate provider error")
	}
	if !strings.Contains(err.Error(), `duplicate provider name "duplicate"`) {
		t.Fatalf("expected duplicate provider error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "providers[1]") || !strings.Contains(err.Error(), "providers[0]") {
		t.Fatalf("expected provider indexes in error, got %q", err.Error())
	}
}

func TestLoadConfigRejectsDuplicateMCPServerNamesInSameLayer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "duplicate-servers.yaml")
	writeRawConfig(t, path, `providers:
  - name: ark
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: ark-code-latest
mcp_servers:
  - name: shared
    command: first
  - name: shared
    command: second
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected duplicate mcp server error")
	}
	if !strings.Contains(err.Error(), `duplicate mcp server name "shared"`) {
		t.Fatalf("expected duplicate mcp server error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "mcp_servers[1]") || !strings.Contains(err.Error(), "mcp_servers[0]") {
		t.Fatalf("expected mcp server indexes in error, got %q", err.Error())
	}
}

func TestLoadConfigRejectsEmptyMCPServerName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty-server-name.yaml")
	writeRawConfig(t, path, `providers:
  - name: ark
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: ark-code-latest
mcp_servers:
  - command: devflow-mcp
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected empty mcp server name error")
	}
	if !strings.Contains(err.Error(), "mcp_servers[0]") || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected empty name error, got %q", err.Error())
	}
}

func TestLoadDiscoveredConfigAllowsCrossLayerMCPServerOverride(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()

	writeRawConfig(t, filepath.Join(home, ".devflow", "config.yaml"), `providers:
  - name: home-provider
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: home-model
mcp_servers:
  - name: shared
    command: home-command
    args: ["--home"]
  - name: home-only
    command: home-only-command
`)
	writeRawConfig(t, filepath.Join(work, ".devflow", "config.yaml"), `providers:
  - name: project-provider
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: project-model
mcp_servers:
  - name: shared
    command: project-command
    args: ["--project"]
`)

	cfg, err := loadDiscoveredConfig(home, work)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MCPServers) != 2 {
		t.Fatalf("mcp server count = %d, want 2", len(cfg.MCPServers))
	}
	if got := cfg.MCPServers[0].Name; got != "shared" {
		t.Fatalf("first mcp server name = %q, want shared", got)
	}
	if got := cfg.MCPServers[0].Command; got != "project-command" {
		t.Fatalf("shared mcp server command = %q, want project-command", got)
	}
	if got := cfg.MCPServers[1].Name; got != "home-only" {
		t.Fatalf("second mcp server name = %q, want home-only", got)
	}
}

func TestLoadDiscoveredConfigRejectsDuplicateMCPServerNamesWithinLaterLayer(t *testing.T) {
	for _, tc := range []struct {
		name       string
		relPath    string
		layerLabel string
	}{
		{name: "project layer", relPath: filepath.Join(".devflow", "config.yaml"), layerLabel: "project"},
		{name: "local layer", relPath: filepath.Join(".devflow", "config.local.yaml"), layerLabel: "local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			work := t.TempDir()

			writeRawConfig(t, filepath.Join(home, ".devflow", "config.yaml"), `providers:
  - name: home-provider
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: home-model
mcp_servers:
  - name: shared
    command: home-command
`)
			writeRawConfig(t, filepath.Join(work, tc.relPath), `mcp_servers:
  - name: duplicate
    command: first
  - name: duplicate
    command: second
`)

			_, err := loadDiscoveredConfig(home, work)
			if err == nil {
				t.Fatalf("expected duplicate mcp server error for %s layer", tc.layerLabel)
			}
			if !strings.Contains(err.Error(), `duplicate mcp server name "duplicate"`) {
				t.Fatalf("expected duplicate mcp server error, got %q", err.Error())
			}
			if !strings.Contains(err.Error(), "mcp_servers[1]") || !strings.Contains(err.Error(), "mcp_servers[0]") {
				t.Fatalf("expected mcp server indexes in error, got %q", err.Error())
			}
		})
	}
}

func TestPreferredConfigReturnsPrimaryPathWhenItExists(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, ".devflow", "config.yaml")
	legacy := filepath.Join(root, ".mewcode", "config.yaml")
	writeProviderConfig(t, primary, "devflow-user", "user-model")
	writeProviderConfig(t, legacy, "legacy-user", "legacy-model")

	got, err := preferredConfig(primary, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if got != primary {
		t.Fatalf("preferred path = %q, want %q", got, primary)
	}
}

func TestPreferredConfigPropagatesPrimaryStatErrors(t *testing.T) {
	primary := filepath.Join("scope", ".devflow", "config.yaml")
	legacy := filepath.Join("scope", ".mewcode", "config.yaml")

	originalStat := statPath
	statPath = func(path string) (os.FileInfo, error) {
		if path == primary {
			return nil, os.ErrPermission
		}
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		statPath = originalStat
	})

	got, err := preferredConfig(primary, legacy)
	if err == nil {
		t.Fatal("expected stat error")
	}
	if got != "" {
		t.Fatalf("preferred path = %q, want empty on error", got)
	}
	if !strings.Contains(err.Error(), ".devflow") || !strings.Contains(err.Error(), ".mewcode") {
		t.Fatalf("expected error to mention both devflow and legacy mewcode paths, got %q", err.Error())
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestLoadSingleFileReturnsIndependentConfigsAcrossCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeRawConfig(t, path, `providers:
  - name: original
    protocol: openai-compat
    base_url: https://example.invalid/v1
    model: original-model
mcp_servers:
  - name: registry
    command: devflow-mcp
    args: ["--stdio"]
    headers:
      Authorization: Bearer one
hooks:
  - id: first
    event: session_start
    action:
      type: prompt
      message: hi
`)

	first, err := loadSingleFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadSingleFile(path)
	if err != nil {
		t.Fatal(err)
	}

	first.Providers[0].Name = "mutated"
	first.MCPServers[0].Headers["Authorization"] = "Bearer changed"
	first.Hooks[0].ID = "mutated"

	if got := second.Providers[0].Name; got != "original" {
		t.Fatalf("second provider name = %q, want original", got)
	}
	if got := second.MCPServers[0].Headers["Authorization"]; got != "Bearer one" {
		t.Fatalf("second header = %q, want Bearer one", got)
	}
	if got := second.Hooks[0].ID; got != "first" {
		t.Fatalf("second hook id = %q, want first", got)
	}
}

func TestMergeConfigPreservesHookSemanticsWithoutSharingInputs(t *testing.T) {
	base := &AppConfig{
		Providers:      []ProviderConfig{{Name: "base-provider", Protocol: "openai-compat", BaseURL: "https://base.invalid/v1", Model: "base-model"}},
		PermissionMode: "ask",
		MCPServers: []MCPServerConfig{
			{Name: "shared", Command: "base", Args: []string{"--base"}, Headers: map[string]string{"Base": "1"}},
		},
		Hooks: []hooks.Hook{
			{ID: "base-hook", Event: hooks.EventSessionStart, Action: hooks.Action{Type: hooks.ActionPrompt, Message: "base"}},
		},
	}
	override := &AppConfig{
		Providers: []ProviderConfig{{Name: "override-provider", Protocol: "openai-compat", BaseURL: "https://override.invalid/v1", Model: "override-model"}},
		MCPServers: []MCPServerConfig{
			{Name: "shared", Command: "override", Args: []string{"--override"}, Headers: map[string]string{"Override": "2"}},
			{Name: "new", Command: "new", Env: map[string]string{"TOKEN": "abc"}},
		},
		Hooks: []hooks.Hook{
			{ID: "override-hook", Event: hooks.EventTurnEnd, Action: hooks.Action{Type: hooks.ActionPrompt, Message: "override"}},
		},
	}

	merged := mergeConfig(base, override)

	if got := merged.Providers[0].Name; got != "override-provider" {
		t.Fatalf("provider name = %q, want override-provider", got)
	}
	if got := merged.PermissionMode; got != "ask" {
		t.Fatalf("permission mode = %q, want ask", got)
	}
	if len(merged.MCPServers) != 2 {
		t.Fatalf("mcp server count = %d, want 2", len(merged.MCPServers))
	}
	if got := merged.MCPServers[0].Command; got != "override" {
		t.Fatalf("shared server command = %q, want override", got)
	}
	if len(merged.Hooks) != 2 {
		t.Fatalf("hook count = %d, want 2", len(merged.Hooks))
	}
	if got := merged.Hooks[0].ID; got != "base-hook" {
		t.Fatalf("first hook id = %q, want base-hook", got)
	}
	if got := merged.Hooks[1].ID; got != "override-hook" {
		t.Fatalf("second hook id = %q, want override-hook", got)
	}

	base.Providers[0].Name = "mutated-base"
	base.MCPServers[0].Args[0] = "--mutated-base"
	base.MCPServers[0].Headers["Base"] = "mutated"
	base.Hooks[0].ID = "mutated-base-hook"

	override.Providers[0].Name = "mutated-override"
	override.MCPServers[0].Args[0] = "--mutated-override"
	override.MCPServers[0].Headers["Override"] = "mutated"
	override.MCPServers[1].Env["TOKEN"] = "changed"
	override.Hooks[0].ID = "mutated-override-hook"

	if got := merged.Providers[0].Name; got != "override-provider" {
		t.Fatalf("merged provider name changed to %q", got)
	}
	if got := merged.MCPServers[0].Args[0]; got != "--override" {
		t.Fatalf("merged shared args changed to %q", got)
	}
	if got := merged.MCPServers[0].Headers["Override"]; got != "2" {
		t.Fatalf("merged shared header changed to %q", got)
	}
	if got := merged.MCPServers[1].Env["TOKEN"]; got != "abc" {
		t.Fatalf("merged new env changed to %q", got)
	}
	if got := merged.Hooks[0].ID; got != "base-hook" {
		t.Fatalf("merged first hook changed to %q", got)
	}
	if got := merged.Hooks[1].ID; got != "override-hook" {
		t.Fatalf("merged second hook changed to %q", got)
	}
}

func writeProviderConfig(t *testing.T, path, providerName, model string) {
	t.Helper()
	writeRawConfig(t, path, "providers:\n  - name: "+providerName+"\n    protocol: openai-compat\n    base_url: https://example.invalid/v1\n    model: "+model+"\n")
}

func writeRawConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

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
		t.Fatalf("PermissionMode = %q, want acceptEdits", cfg.BackendDemand.PermissionMode)
	}
	if cfg.BackendDemand.GitLab.Project != "group/project" {
		t.Fatalf("GitLab.Project = %q", cfg.BackendDemand.GitLab.Project)
	}
	if cfg.BackendDemand.GitLab.DefaultTargetBranch != "main" {
		t.Fatalf("DefaultTargetBranch = %q", cfg.BackendDemand.GitLab.DefaultTargetBranch)
	}
}

func TestLoadConfigPlatformDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeRawConfig(t, path, `
providers:
  - name: test
    protocol: openai
    base_url: https://api.example.test
    model: test-model
platforms:
  github:
    default_repo: jesseedcp/devflow-agent
    base_url: https://api.github.test
  feishu:
    app_id: cli_test
    base_url: https://open.feishu.test
    bitable:
      default_app_token: app_token
      default_table_id: tbl_table
      fields:
        title: 需求标题
        description: 需求描述
        status: 状态
        priority: 优先级
        owner: 负责人
        devflow_demand_id: Devflow Demand ID
        devflow_state: Devflow 状态
        verification: 验收摘要
        closeout: 交付总结
      status_map:
        completed: 已完成
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.Platforms.GitHub.DefaultRepo != "jesseedcp/devflow-agent" {
		t.Fatalf("GitHub.DefaultRepo = %q", cfg.Platforms.GitHub.DefaultRepo)
	}
	if cfg.Platforms.GitHub.BaseURL != "https://api.github.test" {
		t.Fatalf("GitHub.BaseURL = %q", cfg.Platforms.GitHub.BaseURL)
	}
	if cfg.Platforms.Feishu.AppID != "cli_test" {
		t.Fatalf("Feishu.AppID = %q", cfg.Platforms.Feishu.AppID)
	}
	if cfg.Platforms.Feishu.Bitable.Fields.Title != "需求标题" {
		t.Fatalf("Bitable.Fields.Title = %q", cfg.Platforms.Feishu.Bitable.Fields.Title)
	}
	if cfg.Platforms.Feishu.Bitable.StatusMap["completed"] != "已完成" {
		t.Fatalf("completed status map = %q", cfg.Platforms.Feishu.Bitable.StatusMap["completed"])
	}
}

func TestMergeConfigMergesPlatforms(t *testing.T) {
	base := &AppConfig{
		Platforms: PlatformConfig{
			GitHub: GitHubPlatformConfig{DefaultRepo: "owner/base", BaseURL: "https://api.github.com"},
			Feishu: FeishuPlatformConfig{
				AppID: "cli_base",
				Bitable: FeishuBitableConfig{
					Fields:    FeishuBitableFields{Title: "Title"},
					StatusMap: map[string]string{"created": "待澄清"},
				},
			},
		},
	}
	override := &AppConfig{
		Platforms: PlatformConfig{
			GitHub: GitHubPlatformConfig{DefaultRepo: "owner/override"},
			Feishu: FeishuPlatformConfig{
				BaseURL: "https://open.feishu.test",
				Bitable: FeishuBitableConfig{
					DefaultTableID: "tbl_override",
					Fields:         FeishuBitableFields{Status: "状态"},
					StatusMap:      map[string]string{"completed": "已完成"},
				},
			},
		},
	}

	merged := mergeConfig(base, override)
	if merged.Platforms.GitHub.DefaultRepo != "owner/override" {
		t.Fatalf("GitHub.DefaultRepo = %q", merged.Platforms.GitHub.DefaultRepo)
	}
	if merged.Platforms.GitHub.BaseURL != "https://api.github.com" {
		t.Fatalf("GitHub.BaseURL = %q", merged.Platforms.GitHub.BaseURL)
	}
	if merged.Platforms.Feishu.AppID != "cli_base" {
		t.Fatalf("Feishu.AppID = %q", merged.Platforms.Feishu.AppID)
	}
	if merged.Platforms.Feishu.BaseURL != "https://open.feishu.test" {
		t.Fatalf("Feishu.BaseURL = %q", merged.Platforms.Feishu.BaseURL)
	}
	if merged.Platforms.Feishu.Bitable.DefaultTableID != "tbl_override" {
		t.Fatalf("DefaultTableID = %q", merged.Platforms.Feishu.Bitable.DefaultTableID)
	}
	if merged.Platforms.Feishu.Bitable.Fields.Title != "Title" {
		t.Fatalf("Fields.Title = %q", merged.Platforms.Feishu.Bitable.Fields.Title)
	}
	if merged.Platforms.Feishu.Bitable.Fields.Status != "状态" {
		t.Fatalf("Fields.Status = %q", merged.Platforms.Feishu.Bitable.Fields.Status)
	}
	if merged.Platforms.Feishu.Bitable.StatusMap["created"] != "待澄清" {
		t.Fatalf("created status map = %q", merged.Platforms.Feishu.Bitable.StatusMap["created"])
	}
	if merged.Platforms.Feishu.Bitable.StatusMap["completed"] != "已完成" {
		t.Fatalf("completed status map = %q", merged.Platforms.Feishu.Bitable.StatusMap["completed"])
	}
}

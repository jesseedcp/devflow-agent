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
		t.Fatalf("PermissionMode = %q, want acceptEdits", defaults.PermissionMode)
	}
	if defaults.GitLabProject != "group/project" || defaults.GitLabBaseURL != "https://gitlab.example" {
		t.Fatalf("gitlab defaults = %q/%q", defaults.GitLabProject, defaults.GitLabBaseURL)
	}
	if defaults.CreateMRTargetBranch != "main" {
		t.Fatalf("CreateMRTargetBranch = %q, want main", defaults.CreateMRTargetBranch)
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

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepOutputUsesSlashSeparatedPaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "agent.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package agent\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{}
	res := tool.Execute(context.Background(), map[string]any{
		"pattern": "Run",
		"path":    root,
	})
	if res.IsError {
		t.Fatalf("grep errored: %s", res.Output)
	}
	if !strings.Contains(res.Output, "internal/agent.go:2:func Run() {}") {
		t.Fatalf("expected slash-separated path in output, got:\n%s", res.Output)
	}
}

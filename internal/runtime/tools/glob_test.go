// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package tools

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func setupGlobTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"main.go",
		"cmd/cli/main.go",
		"internal/agents/agent.go",
		"internal/agents/agent_test.go",
		"docs/readme.md",
		"src/foo.ts",
		"src/foo/bar.ts",
	}
	for _, rel := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestGlobDoubleStarPattern(t *testing.T) {
	// Before the fix, `**/*.go` returned "No files matched the pattern."
	// because filepath.Match doesn't understand `**`. Verify the fix
	// recursively matches .go files at every depth.
	root := setupGlobTree(t)
	tool := &GlobTool{}
	res := tool.Execute(context.Background(), map[string]any{
		"pattern": "**/*.go",
		"path":    root,
	})
	if res.IsError {
		t.Fatalf("glob errored: %s", res.Output)
	}
	assertGlobOutput(t, res.Output, []string{
		"cmd/cli/main.go",
		"internal/agents/agent.go",
		"internal/agents/agent_test.go",
		"main.go",
	})
}

func TestGlobPlainPatternStillWorks(t *testing.T) {
	root := setupGlobTree(t)
	tool := &GlobTool{}
	res := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.go",
		"path":    root,
	})
	if res.IsError {
		t.Fatalf("glob errored: %s", res.Output)
	}
	// Plain `*.go` matches only top-level + same base name match at each dir.
	assertGlobOutput(t, res.Output, []string{
		"cmd/cli/main.go",
		"internal/agents/agent.go",
		"internal/agents/agent_test.go",
		"main.go",
	})
}

func TestGlobSlashPatternDoesNotOvermatchNestedPaths(t *testing.T) {
	root := setupGlobTree(t)
	tool := &GlobTool{}
	res := tool.Execute(context.Background(), map[string]any{
		"pattern": "src/*.ts",
		"path":    root,
	})
	if res.IsError {
		t.Fatalf("glob errored: %s", res.Output)
	}
	assertGlobOutput(t, res.Output, []string{"src/foo.ts"})
}

func assertGlobOutput(t *testing.T, output string, want []string) {
	t.Helper()
	got := strings.Split(strings.TrimSpace(output), "\n")
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected glob output\nwant:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

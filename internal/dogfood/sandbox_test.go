package dogfood

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateLiveSandboxWritesEditableRepoAndArtifactRoot(t *testing.T) {
	sandbox, err := CreateLiveSandbox(t.TempDir())
	if err != nil {
		t.Fatalf("create sandbox: %v", err)
	}
	for _, path := range []string{
		filepath.Join(sandbox.RepoRoot, "go.mod"),
		filepath.Join(sandbox.RepoRoot, "coupon", "eligibility.go"),
		filepath.Join(sandbox.RepoRoot, "coupon", "eligibility_test.go"),
		sandbox.DemandRoot,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	if sandbox.RepoRoot == sandbox.DemandRoot {
		t.Fatal("repo root and demand root must be separate")
	}
	if len(sandbox.QualityCommands) != 1 || sandbox.QualityCommands[0].Name != "go" {
		t.Fatalf("quality commands = %#v", sandbox.QualityCommands)
	}
}

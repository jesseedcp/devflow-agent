package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSearchRequiresStoreRoot(t *testing.T) {
	t.Parallel()

	store := NewStore("")

	_, err := store.Search("coupon")
	if err == nil {
		t.Fatal("Search returned nil error")
	}
	if !strings.Contains(err.Error(), "store root is required") {
		t.Fatalf("Search error = %q, want store root is required", err)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())

	_, err := store.Search("   \t\n")
	if err == nil {
		t.Fatal("Search returned nil error")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("Search error = %q, want query is required", err)
	}
}

func TestSearchReturnsNilWhenDemandsDirectoryMissing(t *testing.T) {
	t.Parallel()

	store := NewStore(t.TempDir())

	results, err := store.Search("coupon")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if results != nil {
		t.Fatalf("Search results = %#v, want nil", results)
	}
}

func TestSearchMatchesAllTermsCaseInsensitiveAndReturnsSnippet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMemoryCandidate(t, root, "add-coupon-check", "# Memory Candidates\n\n## Stable\n\n- Coupon validation catches duplicate CHECK failures.\n")
	writeMemoryCandidate(t, root, "single-term-only", "# Memory Candidates\n\n- Coupon behavior only.\n")

	store := NewStore(root)
	results, err := store.Search("coupon check")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search result count = %d, want 1", len(results))
	}
	if results[0].DemandID != "add-coupon-check" {
		t.Fatalf("Search DemandID = %q, want %q", results[0].DemandID, "add-coupon-check")
	}
	if results[0].Snippet != "- Coupon validation catches duplicate CHECK failures." {
		t.Fatalf("Search Snippet = %q, want first non-heading line", results[0].Snippet)
	}
}

func TestSearchSortsResultsByDemandID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMemoryCandidate(t, root, "z-last", "# Memory Candidates\n\n- Shared result.\n")
	writeMemoryCandidate(t, root, "a-first", "# Memory Candidates\n\n- Shared result.\n")

	store := NewStore(root)
	results, err := store.Search("shared")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Search result count = %d, want 2", len(results))
	}
	if results[0].DemandID != "a-first" || results[1].DemandID != "z-last" {
		t.Fatalf("Search order = [%q %q], want [a-first z-last]", results[0].DemandID, results[1].DemandID)
	}
}

func TestSearchSkipsDemandsWithoutMemoryCandidates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	base := demandsDir(root)
	if err := os.MkdirAll(filepath.Join(base, "missing-memory"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(base, "missing-memory", "notes.md"), []byte("coupon"), 0o644); err != nil {
		t.Fatalf("WriteFile notes.md returned error: %v", err)
	}

	store := NewStore(root)
	results, err := store.Search("coupon")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search results = %#v, want empty", results)
	}
}

func TestSearchSkipsUnsafeDemandDirectoryNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeMemoryCandidate(t, root, "valid-memory", "# Memory Candidates\n\n- coupon check inside\n")
	writeMemoryCandidate(t, root, "Upper-Case", "# Memory Candidates\n\n- coupon check outside validation\n")

	store := NewStore(root)
	results, err := store.Search("coupon check")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search result count = %d, want 1", len(results))
	}
	if results[0].DemandID != "valid-memory" {
		t.Fatalf("Search DemandID = %q, want %q", results[0].DemandID, "valid-memory")
	}
}

func TestSearchIgnoresUnsafeLinkedDemandDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, memoryCandidatesFile), []byte("# Memory Candidates\n\n- coupon check outside\n"), 0o644); err != nil {
		t.Fatalf("WriteFile outside memory returned error: %v", err)
	}

	base := demandsDir(root)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	linkPath := filepath.Join(base, "escaped-memory")
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, outside)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Skipf("mklink /J unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
		}
		t.Cleanup(func() {
			if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
				t.Fatalf("Remove(%s) returned error: %v", linkPath, err)
			}
		})
	default:
		if err := os.Symlink(outside, linkPath); err != nil {
			t.Skipf("symlink setup unavailable: %v", err)
		}
	}

	store := NewStore(root)
	results, err := store.Search("coupon check")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search results = %#v, want empty because unsafe link should be skipped", results)
	}
}

func TestSearchRejectsLinkedAncestors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-Windows symlink behavior")
	}

	tests := []struct {
		name       string
		linkPath   func(root string) string
		linkTarget func(outsideRoot string) string
	}{
		{
			name: "devflow",
			linkPath: func(root string) string {
				return filepath.Join(root, ".devflow")
			},
			linkTarget: func(outsideRoot string) string {
				return filepath.Join(outsideRoot, ".devflow")
			},
		},
		{
			name: "demands",
			linkPath: func(root string) string {
				return filepath.Join(root, ".devflow", "demands")
			},
			linkTarget: func(outsideRoot string) string {
				return filepath.Join(outsideRoot, ".devflow", "demands")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			outsideRoot := t.TempDir()
			writeMemoryCandidate(t, outsideRoot, "escaped-memory", "# Memory Candidates\n\n- coupon check outside\n")

			linkPath := tt.linkPath(root)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatalf("MkdirAll link parent returned error: %v", err)
			}
			if err := os.Symlink(tt.linkTarget(outsideRoot), linkPath); err != nil {
				t.Skipf("symlink setup unavailable: %v", err)
			}

			store := NewStore(root)
			results, err := store.Search("coupon check")
			if err == nil {
				t.Fatalf("Search results = %#v, want unsafe ancestor error", results)
			}
			if !strings.Contains(err.Error(), "unsafe") {
				t.Fatalf("Search error = %q, want unsafe ancestor context", err)
			}
		})
	}
}

func TestSearchRejectsWindowsJunctionAncestors(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction behavior")
	}

	tests := []struct {
		name       string
		linkPath   func(root string) string
		linkTarget func(outsideRoot string) string
	}{
		{
			name: "devflow",
			linkPath: func(root string) string {
				return filepath.Join(root, ".devflow")
			},
			linkTarget: func(outsideRoot string) string {
				return filepath.Join(outsideRoot, ".devflow")
			},
		},
		{
			name: "demands",
			linkPath: func(root string) string {
				return filepath.Join(root, ".devflow", "demands")
			},
			linkTarget: func(outsideRoot string) string {
				return filepath.Join(outsideRoot, ".devflow", "demands")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			outsideRoot := t.TempDir()
			writeMemoryCandidate(t, outsideRoot, "escaped-memory", "# Memory Candidates\n\n- coupon check outside\n")

			linkPath := tt.linkPath(root)
			if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
				t.Fatalf("MkdirAll link parent returned error: %v", err)
			}
			createWindowsJunction(t, linkPath, tt.linkTarget(outsideRoot))

			store := NewStore(root)
			results, err := store.Search("coupon check")
			if err == nil {
				t.Fatalf("Search results = %#v, want unsafe ancestor error", results)
			}
			if !strings.Contains(err.Error(), "unsafe") {
				t.Fatalf("Search error = %q, want unsafe ancestor context", err)
			}
		})
	}
}

func TestSearchAllowsLinkedStoreRoot(t *testing.T) {
	targetRoot := t.TempDir()
	writeMemoryCandidate(t, targetRoot, "linked-root-memory", "# Memory Candidates\n\n- coupon check inside linked root\n")

	parent := t.TempDir()
	linkPath := filepath.Join(parent, "linked-root")
	if err := os.Symlink(targetRoot, linkPath); err != nil {
		t.Skipf("symlink setup unavailable: %v", err)
	}

	store := NewStore(linkPath)
	results, err := store.Search("coupon check")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].DemandID != "linked-root-memory" {
		t.Fatalf("Search results = %#v, want linked-root-memory", results)
	}
}

func TestSearchRejectsLinkedMemoryCandidate(t *testing.T) {
	root := t.TempDir()
	demandDir := filepath.Join(demandsDir(root), "escaped-memory")
	if err := os.MkdirAll(demandDir, 0o755); err != nil {
		t.Fatalf("MkdirAll demand directory returned error: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside-memory.md")
	if err := os.WriteFile(outside, []byte("# Memory Candidates\n\n- coupon check outside\n"), 0o644); err != nil {
		t.Fatalf("WriteFile outside memory returned error: %v", err)
	}
	candidatePath := filepath.Join(demandDir, memoryCandidatesFile)
	if err := os.Symlink(outside, candidatePath); err != nil {
		t.Skipf("symlink setup unavailable: %v", err)
	}

	store := NewStore(root)
	results, err := store.Search("coupon check")
	if err == nil {
		t.Fatalf("Search results = %#v, want unsafe candidate error", results)
	}
	if !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("Search error = %q, want unsafe candidate context", err)
	}
}

func TestReadCandidateFileReadsNormalFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, memoryCandidatesFile)
	body := []byte("# Memory Candidates\n\n- coupon check inside\n")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("WriteFile candidate returned error: %v", err)
	}

	data, err := readCandidateFile(path, path)
	if err != nil {
		t.Fatalf("readCandidateFile returned error: %v", err)
	}
	if string(data) != string(body) {
		t.Fatalf("readCandidateFile data = %q, want %q", data, body)
	}
}

func TestSearchReturnsContextualErrorForUnreadableMemoryCandidate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	base := demandsDir(root)
	demandDir := filepath.Join(base, "broken-memory")
	if err := os.MkdirAll(filepath.Join(demandDir, memoryCandidatesFile), 0o755); err != nil {
		t.Fatalf("MkdirAll memory-candidates.md returned error: %v", err)
	}

	store := NewStore(root)
	_, err := store.Search("coupon")
	if err == nil {
		t.Fatal("Search returned nil error")
	}
	if !strings.Contains(err.Error(), "broken-memory") || !strings.Contains(err.Error(), memoryCandidatesFile) {
		t.Fatalf("Search error = %q, want contextual read failure", err)
	}
}

func demandsDir(root string) string {
	return filepath.Join(root, ".devflow", "demands")
}

func writeMemoryCandidate(t *testing.T, root, demandID, body string) {
	t.Helper()

	demandDir := filepath.Join(demandsDir(root), demandID)
	if err := os.MkdirAll(demandDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) returned error: %v", demandDir, err)
	}
	path := filepath.Join(demandDir, memoryCandidatesFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}

func createWindowsJunction(t *testing.T, linkPath, target string) {
	t.Helper()

	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("mklink /J unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	t.Cleanup(func() {
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			t.Fatalf("Remove(%s) returned error: %v", linkPath, err)
		}
	})
}

func TestStoreSearchStableMemory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir: %v", err)
	}
	body := `---
name: coupon-eligibility-policy
description: membership gates coupon eligibility
type: project
---

# coupon-eligibility-policy

Active membership must be checked before coupon discount rules.
`
	if err := os.WriteFile(filepath.Join(memDir, "coupon-eligibility-policy.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write memory file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("- [coupon](coupon-eligibility-policy.md)\n"), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	got, err := NewStore(root).SearchStable("membership coupon")
	if err != nil {
		t.Fatalf("SearchStable returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchStable returned %d results: %#v", len(got), got)
	}
	if got[0].Source != SourceStable {
		t.Fatalf("Source = %q, want stable", got[0].Source)
	}
	if got[0].DemandID != "" {
		t.Fatalf("DemandID = %q, want empty for stable memory", got[0].DemandID)
	}
	if !strings.Contains(got[0].Snippet, "membership gates coupon eligibility") {
		t.Fatalf("Snippet = %q, want description", got[0].Snippet)
	}
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestMemoryListShowsNumberedCandidates(t *testing.T) {
	t.Parallel()

	root := seedCLIMemoryDemand(t)
	var stdout bytes.Buffer

	err := Run([]string{"memory", "list", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run memory list returned error: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Memory candidates for add-coupon-check",
		"1. [pending] Active membership must be checked before coupon discount rules.",
		"2. [pending] Coupon errors should preserve the original order validation message.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory list missing %q:\n%s", want, output)
		}
	}
}

func TestMemoryPromoteWritesStableMemory(t *testing.T) {
	t.Parallel()

	root := seedCLIMemoryDemand(t)
	var stdout bytes.Buffer

	err := Run([]string{
		"memory", "promote",
		"--root", root,
		"--demand", "add-coupon-check",
		"--candidate", "1",
		"--name", "coupon-eligibility-policy",
		"--description", "membership gates coupon eligibility",
		"--by", "dd",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run memory promote returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "promoted candidate 1 for add-coupon-check") {
		t.Fatalf("promote output = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "memory", "coupon-eligibility-policy.md")); err != nil {
		t.Fatalf("stable memory file missing: %v", err)
	}
}

func TestMemoryRejectUpdatesListStatus(t *testing.T) {
	t.Parallel()

	root := seedCLIMemoryDemand(t)
	if err := Run([]string{
		"memory", "reject",
		"--root", root,
		"--demand", "add-coupon-check",
		"--candidate", "2",
		"--by", "dd",
		"--reason", "too specific",
	}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run memory reject returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"memory", "list", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run memory list returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "2. [rejected] Coupon errors should preserve the original order validation message.") {
		t.Fatalf("list output missing rejected status:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "reason: too specific") {
		t.Fatalf("list output missing rejection reason:\n%s", stdout.String())
	}
}

func TestMemoryCommandRequiresSubcommand(t *testing.T) {
	t.Parallel()

	err := Run([]string{"memory"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "memory subcommand is required") {
		t.Fatalf("Run memory error = %v, want subcommand required", err)
	}
}

func seedCLIMemoryDemand(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	body := `# Memory Candidates: coupon

## 稳定知识候选

- Active membership must be checked before coupon discount rules.
- Coupon errors should preserve the original order validation message.
`
	if err := store.WriteArtifact("add-coupon-check", artifacts.MemoryCandidatesFile, body); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	return root
}

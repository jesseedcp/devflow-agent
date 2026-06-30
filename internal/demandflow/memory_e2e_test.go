package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	memorystore "github.com/jesseedcp/devflow-agent/internal/memory"
)

func TestStableMemoryPromotedFromOneDemandAppearsInNextDemandPrompt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "first-coupon-demand", Title: "coupon flow", Description: "coupon membership", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create first demand: %v", err)
	}
	if err := store.WriteArtifact("first-coupon-demand", artifacts.MemoryCandidatesFile, "## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n"); err != nil {
		t.Fatalf("write first candidates: %v", err)
	}
	if _, err := memorystore.NewStore(root).PromoteCandidate(memorystore.PromoteOptions{
		DemandID:       "first-coupon-demand",
		CandidateIndex: 1,
		Name:           "coupon-membership-gate",
		Description:    "membership gates coupon discount rules",
		By:             "dd",
		Now:            func() time.Time { return time.Date(2026, 6, 30, 10, 30, 0, 0, time.UTC) },
	}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	if err := store.CreateDemand(artifacts.Demand{ID: "second-coupon-demand", Title: "coupon discount", Description: "coupon membership", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create second demand: %v", err)
	}
	snapshot, err := newContextLoader(root).Load("second-coupon-demand")
	if err != nil {
		t.Fatalf("load second context: %v", err)
	}
	prompt := requirementsPrompt(snapshot)
	for _, want := range []string{
		"Approved stable memory:",
		"membership gates coupon discount rules",
		"Unapproved candidate memory:",
		"first-coupon-demand",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("requirements prompt missing %q:\n%s", want, prompt)
		}
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "memory", "MEMORY.md")); err != nil {
		t.Fatalf("MEMORY.md missing: %v", err)
	}
}

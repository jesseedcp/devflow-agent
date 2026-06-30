package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestStoreListCandidatesShowsDecisionState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	store := NewStore(root)
	if _, err := store.PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}
	if _, err := store.RejectCandidate(RejectOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 2,
		By:             "dd",
		Reason:         "too specific to one test fixture",
		Now:            fixedMemoryTime,
	}); err != nil {
		t.Fatalf("RejectCandidate returned error: %v", err)
	}

	got, err := store.ListCandidates("add-coupon-check")
	if err != nil {
		t.Fatalf("ListCandidates returned error: %v", err)
	}
	if got[0].Status != CandidatePromoted || !strings.Contains(got[0].StablePath, "coupon-eligibility-policy.md") {
		t.Fatalf("candidate 1 decision = %#v, want promoted with stable path", got[0])
	}
	if got[1].Status != CandidateRejected || got[1].Reason != "too specific to one test fixture" {
		t.Fatalf("candidate 2 decision = %#v, want rejected with reason", got[1])
	}
}

func TestStorePromoteCandidateWritesStableMemoryAndIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	result, err := NewStore(root).PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	})
	if err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read stable memory: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"name: coupon-eligibility-policy",
		"description: membership gates coupon eligibility",
		"type: project",
		"source_demand: add-coupon-check",
		"promoted_by: dd",
		"Active membership must be checked before coupon discount rules.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("stable memory missing %q:\n%s", want, text)
		}
	}

	indexBody, err := os.ReadFile(result.IndexPath)
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(indexBody), "[coupon-eligibility-policy](coupon-eligibility-policy.md)") {
		t.Fatalf("MEMORY.md missing pointer:\n%s", string(indexBody))
	}

	events := readMemoryTestEvents(t, root, "add-coupon-check")
	if !memoryTestHasEvent(events, "memory.promoted") {
		t.Fatalf("events missing memory.promoted: %#v", events)
	}
}

func TestStorePromoteCandidateUsesDemandSuffixOnFileConflict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)
	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "coupon-eligibility-policy.md"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing memory: %v", err)
	}

	result, err := NewStore(root).PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	})
	if err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}
	if filepath.Base(result.Path) != "coupon-eligibility-policy-add-coupon-check.md" {
		t.Fatalf("stable path = %s, want demand suffix", result.Path)
	}
}

func TestStoreRejectCandidateRecordsEventWithoutStableFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	got, err := NewStore(root).RejectCandidate(RejectOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 2,
		By:             "dd",
		Reason:         "too specific to one test fixture",
		Now:            fixedMemoryTime,
	})
	if err != nil {
		t.Fatalf("RejectCandidate returned error: %v", err)
	}
	if got.Status != CandidateRejected || got.Reason != "too specific to one test fixture" {
		t.Fatalf("RejectCandidate = %#v, want rejected", got)
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".devflow", "memory")); err == nil && len(entries) != 0 {
		t.Fatalf("reject should not write stable memory, entries = %#v", entries)
	}

	events := readMemoryTestEvents(t, root, "add-coupon-check")
	if !memoryTestHasEvent(events, "memory.rejected") {
		t.Fatalf("events missing memory.rejected: %#v", events)
	}
}

func TestStoreCandidateErrorsAreClear(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact("add-coupon-check", artifacts.MemoryCandidatesFile, "# Memory Candidates\n\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	_, err := NewStore(root).PromoteCandidate(PromoteOptions{DemandID: "add-coupon-check", CandidateIndex: 1, By: "dd", Now: fixedMemoryTime})
	if err == nil || !strings.Contains(err.Error(), "no memory candidates found") {
		t.Fatalf("PromoteCandidate error = %v, want no memory candidates found", err)
	}
}

func TestStorePromoteCandidateRejectsUnsafeStableMemoryDirectory(t *testing.T) {
	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	outside := t.TempDir()
	devflowDir := filepath.Join(root, ".devflow")
	if err := os.MkdirAll(devflowDir, 0o755); err != nil {
		t.Fatalf("MkdirAll .devflow returned error: %v", err)
	}
	linkPath := filepath.Join(devflowDir, "memory")
	switch runtime.GOOS {
	case "windows":
		createWindowsJunction(t, linkPath, outside)
	default:
		if err := os.Symlink(outside, linkPath); err != nil {
			t.Skipf("symlink setup unavailable: %v", err)
		}
	}

	_, err := NewStore(root).PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	})
	if err == nil {
		t.Fatal("PromoteCandidate returned nil error for linked stable memory directory")
	}
	if !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("PromoteCandidate error = %q, want unsafe path error", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "coupon-eligibility-policy.md")); !os.IsNotExist(statErr) {
		t.Fatalf("outside stable memory file stat err = %v, want not exist", statErr)
	}
}

func TestStorePromoteCandidateRejectsUnsafeMemoryIndex(t *testing.T) {
	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir returned error: %v", err)
	}
	linkPath := filepath.Join(memDir, "MEMORY.md")
	switch runtime.GOOS {
	case "windows":
		createWindowsJunction(t, linkPath, t.TempDir())
	default:
		outside := filepath.Join(t.TempDir(), "outside-index.md")
		if err := os.WriteFile(outside, []byte("- outside\n"), 0o644); err != nil {
			t.Fatalf("WriteFile outside index returned error: %v", err)
		}
		if err := os.Symlink(outside, linkPath); err != nil {
			t.Skipf("symlink setup unavailable: %v", err)
		}
	}

	_, err := NewStore(root).PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	})
	if err == nil {
		t.Fatal("PromoteCandidate returned nil error for linked MEMORY.md")
	}
	if !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("PromoteCandidate error = %q, want unsafe path error", err)
	}
}

func seedDemandWithCandidates(t *testing.T, root string) {
	t.Helper()
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
}

func fixedMemoryTime() time.Time {
	return time.Date(2026, 6, 30, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
}

func readMemoryTestEvents(t *testing.T, root, demandID string) []artifacts.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demandID, artifacts.EventsFile))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var events []artifacts.Event
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event artifacts.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func memoryTestHasEvent(events []artifacts.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

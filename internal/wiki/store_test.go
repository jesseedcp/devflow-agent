package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateSlugRejectsUnsafeNames(t *testing.T) {
	for _, name := range []string{"", "..", "a/b", "a\b", "Upper", "has space", "with.dot"} {
		if err := ValidateSlug(name); err == nil {
			t.Errorf("ValidateSlug(%q) expected error", name)
		}
	}
}

func TestValidateSlugAcceptsSafeNames(t *testing.T) {
	for _, name := range []string{"coupon-membership-rule", "abc123_def", "x"} {
		if err := ValidateSlug(name); err != nil {
			t.Errorf("ValidateSlug(%q) unexpected error: %v", name, err)
		}
	}
}

func TestPromoteWritesEntryAndIndex(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	candidate := Candidate{Index: 1, Kind: KindBusiness, Text: "Active membership gates coupons."}
	opts := PromoteOptions{DemandID: "coupon", CandidateIndex: 1, Name: "coupon-rule", By: "tester", Now: func() time.Time { return fixed }}
	relPath, err := Promote(root, opts, candidate)
	if err != nil {
		t.Fatalf("Promote returned error: %v", err)
	}
	if relPath != ".devflow/wiki/coupon-rule.md" {
		t.Fatalf("relPath = %q", relPath)
	}
	entry, err := os.ReadFile(filepath.Join(root, ".devflow", "wiki", "coupon-rule.md"))
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	entryText := string(entry)
	for _, want := range []string{"# coupon-rule", "kind: business", "source_demand: coupon", "promoted_by: tester", "promoted_at: 2026-07-03T12:00:00Z", "Active membership gates coupons."} {
		if !strings.Contains(entryText, want) {
			t.Fatalf("entry missing %q:\n%s", want, entryText)
		}
	}
	index, err := os.ReadFile(filepath.Join(root, ".devflow", "wiki", "WIKI.md"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !strings.Contains(string(index), "- [coupon-rule](coupon-rule.md) - business - coupon") {
		t.Fatalf("index missing entry:\n%s", string(index))
	}
}

func TestMarkCandidateRejectDoesNotWriteWikiEntry(t *testing.T) {
	root := t.TempDir()
	cs := []Candidate{
		{Index: 1, Kind: KindBusiness, Text: "Fact.", Status: StatusPending},
	}
	if !MarkCandidate(cs, 1, StatusRejected, "", "too narrow") {
		t.Fatal("MarkCandidate returned false")
	}
	if cs[0].Status != StatusRejected || cs[0].Reason != "too narrow" {
		t.Fatalf("candidate = %#v", cs[0])
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "wiki")); !os.IsNotExist(err) {
		t.Fatalf("wiki directory should not exist after reject, err=%v", err)
	}
}
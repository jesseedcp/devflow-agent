package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSearchFindsPromotedEntry(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	candidate := Candidate{Index: 1, Kind: KindBusiness, Text: "Active membership must be checked before coupon discount rules."}
	opts := PromoteOptions{DemandID: "coupon", CandidateIndex: 1, Name: "coupon-membership-rule", By: "tester", Now: func() time.Time { return fixed }}
	if _, err := Promote(root, opts, candidate); err != nil {
		t.Fatal(err)
	}
	hits, err := Search(root, "coupon")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	hit := hits[0]
	if hit.Path != ".devflow/wiki/coupon-membership-rule.md" {
		t.Fatalf("path = %q", hit.Path)
	}
	if hit.Title != "coupon-membership-rule" {
		t.Fatalf("title = %q", hit.Title)
	}
	if !strings.Contains(strings.ToLower(hit.Snippet), "coupon") {
		t.Fatalf("snippet missing query term: %q", hit.Snippet)
	}
}

func TestSearchExcludesIndexFile(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	candidate := Candidate{Index: 1, Kind: KindBusiness, Text: "Active membership gates coupons."}
	opts := PromoteOptions{DemandID: "coupon", CandidateIndex: 1, Name: "coupon-rule", By: "tester", Now: func() time.Time { return fixed }}
	if _, err := Promote(root, opts, candidate); err != nil {
		t.Fatal(err)
	}
	hits, err := Search(root, "coupon")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	for _, hit := range hits {
		if strings.HasSuffix(hit.Path, "WIKI.md") {
			t.Fatalf("index file should be excluded: %q", hit.Path)
		}
	}
}

func TestSearchEmptyWhenNoWikiDir(t *testing.T) {
	root := t.TempDir()
	hits, err := Search(root, "anything")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits, got %d", len(hits))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	candidate := Candidate{Index: 1, Kind: KindBusiness, Text: "Membership check required."}
	opts := PromoteOptions{DemandID: "m", CandidateIndex: 1, Name: "membership-rule", By: "tester", Now: func() time.Time { return fixed }}
	if _, err := Promote(root, opts, candidate); err != nil {
		t.Fatal(err)
	}
	hits, err := Search(root, "MEMBERSHIP")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 case-insensitive hit, got %d", len(hits))
	}
}

func TestSearchNoMatchReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	fixed := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	candidate := Candidate{Index: 1, Kind: KindBusiness, Text: "Membership check required."}
	opts := PromoteOptions{DemandID: "m", CandidateIndex: 1, Name: "membership-rule", By: "tester", Now: func() time.Time { return fixed }}
	if _, err := Promote(root, opts, candidate); err != nil {
		t.Fatal(err)
	}
	// ensure WIKI.md exists but is excluded
	if _, err := os.Stat(filepath.Join(root, ".devflow", "wiki", "WIKI.md")); err != nil {
		t.Fatal(err)
	}
	hits, err := Search(root, "nonexistent-term-xyz")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits for nonexistent term, got %d", len(hits))
	}
}
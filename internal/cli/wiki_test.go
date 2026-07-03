package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestWikiDistillWritesArtifactsAndAppendsEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	memory := "# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n"
	if err := store.WriteArtifact(demand.ID, artifacts.MemoryCandidatesFile, memory); err != nil {
		t.Fatal(err)
	}
	implReview := "# Implementation Review: coupon\n\nRecommendation: `needs_implementation_diff`\n\n"
	if err := store.WriteArtifact(demand.ID, artifacts.ImplementationReviewFile, implReview); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "distill", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki distill returned error: %v", err)
	}

	demandDir := store.DemandDir(demand.ID)
	rawLog, err := os.ReadFile(filepath.Join(demandDir, artifacts.CloseoutRawLogFile))
	if err != nil {
		t.Fatalf("read closeout-raw-log: %v", err)
	}
	rawText := string(rawLog)
	for _, want := range []string{"# Closeout Raw Log: Coupon eligibility", "## Closeout", "## Memory Candidates", "## Implementation Review", "## Review And Verification Events", "Active membership must be checked before coupon discount rules.", "Recommendation: `needs_implementation_diff`"} {
		if !strings.Contains(rawText, want) {
			t.Fatalf("closeout-raw-log missing %q:\n%s", want, rawText)
		}
	}

	candidates, err := os.ReadFile(filepath.Join(demandDir, artifacts.WikiCandidatesFile))
	if err != nil {
		t.Fatalf("read wiki-candidates: %v", err)
	}
	candText := string(candidates)
	for _, want := range []string{"# Wiki Candidates: Coupon eligibility", "## Stable Business Knowledge", "- Active membership must be checked before coupon discount rules. (source: memory-candidates.md)", "## Process Improvement Candidates", "Implementation review recommended needs_implementation_diff.", "## Archive Only"} {
		if !strings.Contains(candText, want) {
			t.Fatalf("wiki-candidates missing %q:\n%s", want, candText)
		}
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Type == "wiki.candidates_distilled" {
			found = true
		}
	}
	if !found {
		t.Fatal("wiki.candidates_distilled event not appended")
	}
	if !strings.Contains(stdout.String(), "wiki candidates distilled for coupon") {
		t.Fatalf("stdout missing distill confirmation: %s", stdout.String())
	}
}

func TestWikiDistillPreservesEmptyTemplates(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "empty-wiki", Title: "Empty", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "distill", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki distill returned error: %v", err)
	}
	candidates, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.WikiCandidatesFile))
	if err != nil {
		t.Fatal(err)
	}
	candText := string(candidates)
	for _, want := range []string{"No stable business knowledge candidates distilled yet.", "No process improvement candidates distilled yet.", "No archive-only material distilled yet."} {
		if !strings.Contains(candText, want) {
			t.Fatalf("wiki-candidates missing empty template %q:\n%s", want, candText)
		}
	}
}
func distillForWikiTest(t *testing.T, root, demandID, memory, implReview string) {
	t.Helper()
	store := artifacts.NewStore(root)
	if memory != "" {
		if err := store.WriteArtifact(demandID, artifacts.MemoryCandidatesFile, memory); err != nil {
			t.Fatal(err)
		}
	}
	if implReview != "" {
		if err := store.WriteArtifact(demandID, artifacts.ImplementationReviewFile, implReview); err != nil {
			t.Fatal(err)
		}
	}
	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "distill", "--root", root, "--demand", demandID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki distill returned error: %v", err)
	}
}

func TestWikiListPrintsCandidates(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	distillForWikiTest(t, root, demand.ID,
		"# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n",
		"# Implementation Review: coupon\n\nRecommendation: `needs_implementation_diff`\n\n")
	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "list", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki list returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Wiki candidates for coupon") {
		t.Fatalf("list output missing header: %s", out)
	}
	if !strings.Contains(out, "1. [pending] business - Active membership must be checked before coupon discount rules.") {
		t.Fatalf("list output missing business candidate: %s", out)
	}
	if !strings.Contains(out, "2. [pending] process - Implementation review recommended needs_implementation_diff.") {
		t.Fatalf("list output missing process candidate: %s", out)
	}
}

func TestWikiPromoteWritesEntryIndexCandidateAndEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	distillForWikiTest(t, root, demand.ID,
		"# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n",
		"")

	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "promote", "--root", root, "--demand", demand.ID, "--candidate", "1", "--name", "coupon-membership-rule", "--by", "tester"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki promote returned error: %v", err)
	}

	entryPath := filepath.Join(root, ".devflow", "wiki", "coupon-membership-rule.md")
	entry, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("wiki entry not written: %v", err)
	}
	if !strings.Contains(string(entry), "Active membership must be checked before coupon discount rules.") {
		t.Fatalf("entry missing knowledge text:\n%s", string(entry))
	}

	index, err := os.ReadFile(filepath.Join(root, ".devflow", "wiki", "WIKI.md"))
	if err != nil {
		t.Fatalf("wiki index not written: %v", err)
	}
	if !strings.Contains(string(index), "coupon-membership-rule.md") {
		t.Fatalf("index missing entry:\n%s", string(index))
	}

	candidates, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.WikiCandidatesFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(candidates), "[promoted: .devflow/wiki/coupon-membership-rule.md]") {
		t.Fatalf("candidate not marked promoted:\n%s", string(candidates))
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type == "wiki.candidate_promoted" {
			found = true
			if event.Data["name"] != "coupon-membership-rule" {
				t.Fatalf("event name = %q", event.Data["name"])
			}
		}
	}
	if !found {
		t.Fatal("wiki.candidate_promoted event not appended")
	}
}

func TestWikiRejectUpdatesCandidateAndAppendsEventWithoutWikiFile(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	distillForWikiTest(t, root, demand.ID,
		"# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n",
		"")

	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "reject", "--root", root, "--demand", demand.ID, "--candidate", "1", "--by", "tester", "--reason", "too narrow"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki reject returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "wiki")); !os.IsNotExist(err) {
		t.Fatalf("wiki directory should not exist after reject, err=%v", err)
	}

	candidates, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.WikiCandidatesFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(candidates), "[rejected: too narrow]") {
		t.Fatalf("candidate not marked rejected:\n%s", string(candidates))
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type == "wiki.candidate_rejected" {
			found = true
			if event.Data["reason"] != "too narrow" {
				t.Fatalf("event reason = %q", event.Data["reason"])
			}
		}
	}
	if !found {
		t.Fatal("wiki.candidate_rejected event not appended")
	}
}

func TestWikiSearchFindsPromotedCouponKnowledge(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	distillForWikiTest(t, root, demand.ID,
		"# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n",
		"")
	if err := Run([]string{"wiki", "promote", "--root", root, "--demand", demand.ID, "--candidate", "1", "--name", "coupon-membership-rule", "--by", "tester"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "search", "--root", root, "coupon"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki search returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, ".devflow/wiki/coupon-membership-rule.md") {
		t.Fatalf("search output missing entry path: %s", out)
	}
	if !strings.Contains(out, "title: coupon-membership-rule") {
		t.Fatalf("search output missing title: %s", out)
	}
}

func TestWikiSearchNoMatchPrintsMessage(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	if err := Run([]string{"wiki", "search", "--root", root, "nothing"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("wiki search returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "No wiki entries matched") {
		t.Fatalf("search output missing no-match message: %s", stdout.String())
	}
}

func TestWikiPromoteUnsafeNameFails(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	distillForWikiTest(t, root, demand.ID,
		"# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n",
		"")
	unsafeName := ".." + `\secret`
	err := Run([]string{"wiki", "promote", "--root", root, "--demand", demand.ID, "--candidate", "1", "--name", unsafeName, "--by", "tester"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for unsafe name")
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "wiki")); !os.IsNotExist(err) {
		t.Fatalf("wiki directory should not exist after unsafe promote, err=%v", err)
	}
}

func TestWikiPromoteRequiresNameAndBy(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	distillForWikiTest(t, root, demand.ID,
		"# Memory Candidates: Coupon eligibility\n\n## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n\n",
		"")
	if err := Run([]string{"wiki", "promote", "--root", root, "--demand", demand.ID, "--candidate", "1", "--name", "ok-name"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error when --by is missing")
	}
}
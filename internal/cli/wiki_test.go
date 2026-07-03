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
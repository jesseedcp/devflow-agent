package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestContextLoaderLoadsDemandAndArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)

	demand := artifacts.Demand{
		ID:          "add-coupon-check",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       "created",
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	const requirementsBody = "# Requirements: coupon flow\n\ncoupon flow body text\n"
	if err := store.WriteArtifact("add-coupon-check", artifacts.RequirementsFile, requirementsBody); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	loader := newContextLoader(root)
	snapshot, err := loader.Load("add-coupon-check")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if snapshot.Demand.ID != "add-coupon-check" {
		t.Fatalf("demand id = %q want add-coupon-check", snapshot.Demand.ID)
	}
	if snapshot.Demand.Title != "coupon flow" {
		t.Fatalf("demand title = %q want coupon flow", snapshot.Demand.Title)
	}
	if !strings.Contains(snapshot.Artifacts.Requirements, "coupon flow body text") {
		t.Fatalf("requirements artifact = %q", snapshot.Artifacts.Requirements)
	}
	if snapshot.Artifacts.Plan == "" {
		t.Fatalf("plan artifact should be the template, got empty")
	}
}

func TestContextLoaderExcludesCurrentDemandFromMemory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)

	prior := artifacts.Demand{
		ID:          "prior-work",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       "created",
	}
	if err := store.CreateDemand(prior); err != nil {
		t.Fatalf("create prior: %v", err)
	}
	const priorMemory = "## stable knowledge\n\n- coupon flow knowledge\n"
	if err := store.WriteArtifact("prior-work", artifacts.MemoryCandidatesFile, priorMemory); err != nil {
		t.Fatalf("write prior memory: %v", err)
	}

	current := artifacts.Demand{
		ID:          "add-coupon-check",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       "created",
	}
	if err := store.CreateDemand(current); err != nil {
		t.Fatalf("create current: %v", err)
	}

	loader := newContextLoader(root)
	snapshot, err := loader.Load("add-coupon-check")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var priorHit *MemoryHit
	for index := range snapshot.Memories {
		hit := snapshot.Memories[index]
		if hit.DemandID == "add-coupon-check" {
			t.Fatalf("current demand should be excluded from memory hits, got %#v", hit)
		}
		if hit.DemandID == "prior-work" {
			priorHit = &snapshot.Memories[index]
		}
	}
	if priorHit == nil {
		t.Fatalf("expected prior-work memory hit, got %#v", snapshot.Memories)
	}
	if !strings.Contains(priorHit.Snippet, "coupon flow knowledge") {
		t.Fatalf("prior snippet = %q want coupon flow knowledge", priorHit.Snippet)
	}
}

func TestContextLoaderLoadsStableMemoryBeforeCandidateMemory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "prior-work", Title: "coupon flow", Description: "coupon flow", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create prior: %v", err)
	}
	if err := store.WriteArtifact("prior-work", artifacts.MemoryCandidatesFile, "## 稳定知识候选\n\n- coupon candidate knowledge\n"); err != nil {
		t.Fatalf("write prior memory: %v", err)
	}
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon flow", Description: "coupon flow", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create current: %v", err)
	}
	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir: %v", err)
	}
	stable := `---
name: coupon-eligibility-policy
description: stable coupon policy
type: project
---

Stable coupon memory body.
`
	if err := os.WriteFile(filepath.Join(memDir, "coupon-eligibility-policy.md"), []byte(stable), 0o644); err != nil {
		t.Fatalf("write stable memory: %v", err)
	}

	snapshot, err := newContextLoader(root).Load("add-coupon-check")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(snapshot.Memories) < 2 {
		t.Fatalf("expected stable and candidate memories, got %#v", snapshot.Memories)
	}
	if snapshot.Memories[0].Source != "stable" {
		t.Fatalf("first memory source = %q, want stable; memories=%#v", snapshot.Memories[0].Source, snapshot.Memories)
	}
	if snapshot.Memories[1].Source != "candidate" {
		t.Fatalf("second memory source = %q, want candidate; memories=%#v", snapshot.Memories[1].Source, snapshot.Memories)
	}
}

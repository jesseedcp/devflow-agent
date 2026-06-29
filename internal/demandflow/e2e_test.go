package demandflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func setDemandState(t *testing.T, store artifacts.Store, id string, state workflow.State) {
	t.Helper()
	demand, err := store.LoadDemand(id)
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	demand.State = string(state)
	if err := store.SaveDemand(demand); err != nil {
		t.Fatalf("save demand: %v", err)
	}
}

func assertArtifactContains(t *testing.T, dir, name, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s = %q want to contain %q", name, string(data), want)
	}
}

func readEventTypes(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var types []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event artifacts.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode event %q: %v", line, err)
		}
		types = append(types, event.Type)
	}
	return types
}

func TestDemandflowSlimLoopEndToEnd(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "add-coupon-check",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	engine := NewEngine(root)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}

	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageRequirements:   {Text: "# Requirements: coupon flow\n\n## 目标行为\n\nrequirements e2e body\n"},
		StagePlan:           {Text: "# Technical Plan: coupon flow\n\n## 目标设计\n\nplan e2e body\n"},
		StageImplementation: {Text: "## 实现摘要\n\nimplementation e2e body\n", ToolSummary: []string{"edit file"}},
		StageVerification:   {Text: "# Verification: coupon flow\n\n## 结论\n\nverification e2e body\n"},
		StageCloseout:       {Text: "# Closeout: coupon flow\n\n## 需求结果\n\ncloseout e2e body\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: coupon flow\n\n## 稳定知识候选\n\n- coupon flow stable knowledge\n"},
	}}

	opts := func(stage Stage, configure func(*Options)) Options {
		o := Options{
			Root:     root,
			DemandID: "add-coupon-check",
			Stage:    stage,
			Runner:   runner,
			Now:      fixedNow,
		}
		if configure != nil {
			configure(&o)
		}
		return o
	}

	ctx := context.Background()

	if err := engine.Run(ctx, opts(StageRequirements, nil)); err != nil {
		t.Fatalf("requirements: %v", err)
	}
	setDemandState(t, store, "add-coupon-check", workflow.PlanDrafting)

	if err := engine.Run(ctx, opts(StagePlan, nil)); err != nil {
		t.Fatalf("plan: %v", err)
	}
	setDemandState(t, store, "add-coupon-check", workflow.Implementation)

	if err := engine.Run(ctx, opts(StageImplementation, func(o *Options) {
		o.QualityCommands = []quality.Command{{Name: "go", Args: []string{"test"}}}
	})); err != nil {
		t.Fatalf("implementation: %v", err)
	}

	if err := engine.Run(ctx, opts(StageMRReview, func(o *Options) {
		o.Review = ReviewOptions{Adapter: fakeReviewAdapter{}, Ref: adapters.ReviewRef{Project: "group/project", MergeRequest: "1"}}
	})); err != nil {
		t.Fatalf("mr-review: %v", err)
	}

	if err := engine.Run(ctx, opts(StageVerification, func(o *Options) {
		o.QualityCommands = []quality.Command{{Name: "go", Args: []string{"test"}}}
	})); err != nil {
		t.Fatalf("verification: %v", err)
	}
	setDemandState(t, store, "add-coupon-check", workflow.Closeout)

	if err := engine.Run(ctx, opts(StageCloseout, nil)); err != nil {
		t.Fatalf("closeout: %v", err)
	}

	demand, err := store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if demand.State != string(workflow.Closeout) {
		t.Fatalf("final state = %q want closeout", demand.State)
	}

	dir := store.DemandDir("add-coupon-check")
	assertArtifactContains(t, dir, artifacts.RequirementsFile, "requirements e2e body")
	assertArtifactContains(t, dir, artifacts.PlanFile, "plan e2e body")
	assertArtifactContains(t, dir, artifacts.ProgressFile, "implementation e2e body")
	assertArtifactContains(t, dir, artifacts.VerificationFile, "verification e2e body")
	assertArtifactContains(t, dir, artifacts.CloseoutFile, "closeout e2e body")
	assertArtifactContains(t, dir, artifacts.MemoryCandidatesFile, "coupon flow stable knowledge")

	types := readEventTypes(t, filepath.Join(dir, artifacts.EventsFile))
	for _, want := range []string{
		"requirements.drafted",
		"plan.drafted",
		"implementation.completed",
		"mr_review.cleared",
		"verification.drafted",
		"closeout.drafted",
	} {
		found := false
		for _, got := range types {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("events missing %s; got %v", want, types)
		}
	}
}

package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/memory"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestBuildMemoryRecallIncludesStableAndCandidateMemory(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "coupon-new", Title: "Coupon eligibility", Description: "coupon active member", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand current returned error: %v", err)
	}
	if err := store.CreateDemand(artifacts.Demand{ID: "coupon-old", Title: "Old coupon", Description: "old", Source: "test", State: string(workflow.Completed)}); err != nil {
		t.Fatalf("CreateDemand old returned error: %v", err)
	}
	if err := store.WriteArtifact("coupon-old", artifacts.MemoryCandidatesFile, "# Memory Candidates\n\n## 稳定知识候选\n\n- coupon active member checks must happen before coupon claim writes\n"); err != nil {
		t.Fatalf("WriteArtifact memory candidates returned error: %v", err)
	}
	if _, err := memory.NewStore(root).PromoteCandidate(memory.PromoteOptions{
		DemandID:       "coupon-old",
		CandidateIndex: 1,
		Name:           "coupon-active-member",
		Description:    "coupon active member checks must happen before writes",
		By:             "tester",
	}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	recall, err := BuildMemoryRecall(root, "coupon-new")
	if err != nil {
		t.Fatalf("BuildMemoryRecall returned error: %v", err)
	}
	text := RenderMemoryRecall(recall)
	for _, want := range []string{
		"# Context: Coupon eligibility",
		"## Approved Stable Memory",
		"coupon active member checks must happen before writes",
		"## Historical Demand Candidates",
		"coupon-old",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("recall missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "coupon-new:") {
		t.Fatalf("recall should not include current demand as candidate:\n%s", text)
	}
}

func TestBuildMemoryRecallNoHitsStillRendersReviewableContext(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "empty-context", Title: "Empty context", Description: "nothing matches", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	recall, err := BuildMemoryRecall(root, "empty-context")
	if err != nil {
		t.Fatalf("BuildMemoryRecall returned error: %v", err)
	}
	text := RenderMemoryRecall(recall)
	for _, want := range []string{
		"# Context: Empty context",
		"No approved stable memory recalled.",
		"No historical candidate memory recalled.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("recall missing %q:\n%s", want, text)
		}
	}
}

func TestWriteMemoryRecallWritesContextAndEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "write-context", Title: "Write context", Description: "context", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	result, err := WriteMemoryRecall(root, "write-context")
	if err != nil {
		t.Fatalf("WriteMemoryRecall returned error: %v", err)
	}
	if result.DemandID != "write-context" {
		t.Fatalf("DemandID = %q", result.DemandID)
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "write-context", artifacts.ContextFile))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if !strings.Contains(string(body), "# Context: Write context") {
		t.Fatalf("context body = %s", string(body))
	}
	events, err := store.ReadEvents("write-context")
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Type == "context.recalled" {
			found = true
			if event.Data["stable"] != "0" || event.Data["candidates"] != "0" {
				t.Fatalf("event data = %#v", event.Data)
			}
		}
	}
	if !found {
		t.Fatalf("context.recalled event missing: %#v", events)
	}
}

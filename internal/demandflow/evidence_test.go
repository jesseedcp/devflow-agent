package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestAddManualEvidenceAppendsVerificationAndEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-demand", Title: "Evidence demand", Description: "Verify external behavior", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.VerificationFile, "# Verification: Evidence demand\n\n"); err != nil {
		t.Fatalf("WriteArtifact verification returned error: %v", err)
	}

	record, err := AddManualEvidence(AddManualEvidenceOptions{
		Root:      root,
		DemandID:  demand.ID,
		Type:      "api",
		Criterion: "Inactive users are blocked",
		Status:    "pass",
		Summary:   "POST /coupon/claim returned COUPON_USER_INACTIVE.",
		Link:      "https://example.test/log/123",
		By:        "dd",
	})
	if err != nil {
		t.Fatalf("AddManualEvidence returned error: %v", err)
	}
	if record.Status != "pass" || record.Type != "api" {
		t.Fatalf("record = %#v, want pass api", record)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.VerificationFile))
	if err != nil {
		t.Fatalf("read verification: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"## Manual Acceptance Evidence",
		"[PASS] api - Inactive users are blocked",
		"POST /coupon/claim returned COUPON_USER_INACTIVE.",
		"Link: https://example.test/log/123",
		"By: dd",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("verification.md missing %q:\n%s", want, text)
		}
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	event := findEvidenceTestEvent(events, "verification.evidence_recorded")
	if event == nil {
		t.Fatalf("events missing verification.evidence_recorded: %#v", events)
	}
	if event.Data["status"] != "pass" || event.Data["criterion"] != "Inactive users are blocked" || event.Data["evidence_file"] != artifacts.VerificationFile {
		t.Fatalf("event data = %#v", event.Data)
	}
}

func TestAddManualEvidenceRequiresVerificationState(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-wrong-state", Title: "Wrong state", Description: "Wrong state", Source: "test", State: string(workflow.PlanReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	_, err := AddManualEvidence(AddManualEvidenceOptions{
		Root:      root,
		DemandID:  demand.ID,
		Type:      "api",
		Criterion: "Inactive users are blocked",
		Status:    "pass",
		Summary:   "Observed expected error code.",
	})
	if err == nil || !strings.Contains(err.Error(), "requires current state verification") {
		t.Fatalf("err = %v, want verification state error", err)
	}
}

func TestAddManualEvidenceValidatesRequiredFields(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-validation", Title: "Validation", Description: "Validation", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	_, err := AddManualEvidence(AddManualEvidenceOptions{
		Root:     root,
		DemandID: demand.ID,
		Type:     "spreadsheet",
		Status:   "maybe",
	})
	if err == nil || !strings.Contains(err.Error(), "--type must be one of") {
		t.Fatalf("err = %v, want type validation", err)
	}
}

func TestListManualEvidenceReturnsEventsInOrder(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-list", Title: "Evidence list", Description: "List evidence", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	for _, opts := range []AddManualEvidenceOptions{
		{Root: root, DemandID: demand.ID, Type: "api", Criterion: "Active user succeeds", Status: "pass", Summary: "200 OK"},
		{Root: root, DemandID: demand.ID, Type: "log", Criterion: "Inactive user blocked", Status: "blocked", Summary: "Waiting for log access"},
	} {
		if _, err := AddManualEvidence(opts); err != nil {
			t.Fatalf("AddManualEvidence returned error: %v", err)
		}
	}

	records, err := ListManualEvidence(root, demand.ID)
	if err != nil {
		t.Fatalf("ListManualEvidence returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Criterion != "Active user succeeds" || records[1].Status != "blocked" {
		t.Fatalf("records = %#v", records)
	}
}

func findEvidenceTestEvent(events []artifacts.Event, eventType string) *artifacts.Event {
	for index := range events {
		if events[index].Type == eventType {
			return &events[index]
		}
	}
	return nil
}

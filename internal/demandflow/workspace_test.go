package demandflow

import (
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/memory"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestInspectWorkspaceSummarizesEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-evidence", Title: "Workspace evidence", Description: "Need an operator summary", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.RequirementsFile, "\n- requirement detail\n"); err != nil {
		t.Fatalf("AppendToArtifact requirements returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.PlanFile, "\n- plan detail\n"); err != nil {
		t.Fatalf("AppendToArtifact plan returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.ProgressFile, "\nMR: !12 open\nreview gate cleared\n"); err != nil {
		t.Fatalf("AppendToArtifact progress returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.VerificationFile, "\nPASS go test ./...\n"); err != nil {
		t.Fatalf("AppendToArtifact verification returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.MemoryCandidatesFile, "# Memory Candidates: Workspace evidence\n\n## 稳定知识候选\n\n- Reuse tenant validation rule\n- Reuse audit logging pattern\n\n## 流程改进候选\n"); err != nil {
		t.Fatalf("WriteArtifact memory candidates returned error: %v", err)
	}
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "stage.confirmed", Message: "requirements confirmed", Data: map[string]string{"stage": "requirements"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(time.Minute), Type: "stage.confirmed", Message: "plan confirmed", Data: map[string]string{"stage": "plan"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(2 * time.Minute), Type: "implementation.completed", Message: "implementation completed"})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(3 * time.Minute), Type: "mr_review.cleared", Message: "review gate cleared", Data: map[string]string{"mr": "!12"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(4 * time.Minute), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./...", "evidence_file": "verification.md"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(5 * time.Minute), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}})

	memStore := memory.NewStore(root)
	if _, err := memStore.PromoteCandidate(memory.PromoteOptions{DemandID: demand.ID, CandidateIndex: 1, Name: "tenant-validation", Description: "Tenant validation", By: "tester", Now: fixedWorkspaceTime}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}

	if summary.Demand.ID != demand.ID {
		t.Fatalf("Demand.ID = %q, want %q", summary.Demand.ID, demand.ID)
	}
	if summary.State != workflow.Verification {
		t.Fatalf("State = %q, want %q", summary.State, workflow.Verification)
	}
	assertStageStatus(t, summary, "requirements", "confirmed")
	assertStageStatus(t, summary, "plan", "confirmed")
	assertStageStatus(t, summary, "implementation", "completed")
	assertStageStatus(t, summary, "mr-review", "cleared")
	assertStageStatus(t, summary, "verification", "passed")
	assertArtifactStatus(t, summary, artifacts.RequirementsFile, "confirmed")
	assertArtifactStatus(t, summary, artifacts.VerificationFile, "has_pass_evidence")
	if summary.MergeRequest.Status != "cleared" {
		t.Fatalf("MergeRequest.Status = %q, want cleared", summary.MergeRequest.Status)
	}
	if summary.MergeRequest.Reference != "!12" {
		t.Fatalf("MergeRequest.Reference = %q, want !12", summary.MergeRequest.Reference)
	}
	if summary.Verification.Status != "pass" || summary.Verification.Command != "go test ./..." {
		t.Fatalf("Verification = %#v, want pass go test ./...", summary.Verification)
	}
	if summary.Memory.Pending != 1 || summary.Memory.Promoted != 1 || summary.Memory.Rejected != 0 {
		t.Fatalf("Memory = %#v, want 1 pending, 1 promoted, 0 rejected", summary.Memory)
	}
	if len(summary.Actions) == 0 || summary.Actions[0].Label != "Confirm verification" {
		t.Fatalf("Actions = %#v, want Confirm verification first despite pending memory", summary.Actions)
	}
	if summary.Attention != "ready to confirm verification" {
		t.Fatalf("Attention = %q, want ready to confirm verification", summary.Attention)
	}
}

func TestInspectWorkspaceTemplateAndMissingArtifacts(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-template", Title: "Workspace template", Description: "Template summary", Source: "test", State: string(workflow.RequirementsReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ProgressFile, ""); err != nil {
		t.Fatalf("WriteArtifact progress returned error: %v", err)
	}

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}

	assertStageStatus(t, summary, "requirements", "needs_confirmation")
	assertArtifactStatus(t, summary, artifacts.RequirementsFile, "template")
	assertArtifactStatus(t, summary, artifacts.ProgressFile, "template")
	assertArtifactStatus(t, summary, artifacts.IntakeFile, "template")
	if summary.Memory.Status != "none" {
		t.Fatalf("Memory.Status = %q, want none", summary.Memory.Status)
	}
	if summary.Attention == "" {
		t.Fatal("Attention is empty")
	}
}

func TestInspectWorkspaceVerificationFailure(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-fail", Title: "Workspace fail", Description: "Failure summary", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "verification.recorded", Message: "verification fail", Data: map[string]string{"status": "FAIL", "command": "go test ./...", "failure_kind": "exit_nonzero"}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}

	assertStageStatus(t, summary, "verification", "failed")
	assertArtifactStatus(t, summary, artifacts.VerificationFile, "has_fail_evidence")
	if summary.Verification.FailureKind != "exit_nonzero" {
		t.Fatalf("FailureKind = %q, want exit_nonzero", summary.Verification.FailureKind)
	}
	if len(summary.Actions) == 0 || summary.Actions[0].Label != "Retry implementation" {
		t.Fatalf("Actions = %#v, want Retry implementation first", summary.Actions)
	}
	if summary.Attention != "verification failed" {
		t.Fatalf("Attention = %q, want verification failed", summary.Attention)
	}
}

func TestListWorkspacesOrdersAttentionBeforeCompleted(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	createWorkspaceDemand(t, store, artifacts.Demand{ID: "z-completed", Title: "Completed", Description: "Done", Source: "test", State: string(workflow.Completed)})
	createWorkspaceDemand(t, store, artifacts.Demand{ID: "a-blocked", Title: "Blocked", Description: "Blocked", Source: "test", State: string(workflow.FailedQualityGate)})
	createWorkspaceDemand(t, store, artifacts.Demand{ID: "m-review", Title: "Review", Description: "Review", Source: "test", State: string(workflow.MRReview)})

	summaries, err := ListWorkspaces(root)
	if err != nil {
		t.Fatalf("ListWorkspaces returned error: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("len(ListWorkspaces) = %d, want 3", len(summaries))
	}
	got := []string{summaries[0].Demand.ID, summaries[1].Demand.ID, summaries[2].Demand.ID}
	want := []string{"a-blocked", "m-review", "z-completed"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %#v, want %#v", got, want)
		}
	}
}

func TestWorkspaceNextActionsMRReviewClearedDraftsVerification(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-mr-cleared", Title: "MR cleared", Description: "MR cleared", Source: "test", State: string(workflow.MRReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "mr_review.cleared", Message: "review gate cleared", Data: map[string]string{"mr": "!34"}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}
	if len(summary.Actions) == 0 || summary.Actions[0].Label != "Draft verification" {
		t.Fatalf("Actions = %#v, want Draft verification first", summary.Actions)
	}
	if summary.Attention != "ready for verification" {
		t.Fatalf("Attention = %q, want ready for verification", summary.Attention)
	}
}

func appendWorkspaceEvent(t *testing.T, store artifacts.Store, demandID string, event artifacts.Event) {
	t.Helper()
	if err := store.AppendEvent(demandID, event); err != nil {
		t.Fatalf("AppendEvent(%s, %s) returned error: %v", demandID, event.Type, err)
	}
}

func createWorkspaceDemand(t *testing.T, store artifacts.Store, demand artifacts.Demand) {
	t.Helper()
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
	}
}

func fixedWorkspaceTime() time.Time {
	return time.Date(2026, 6, 30, 1, 2, 3, 0, time.UTC)
}

func assertStageStatus(t *testing.T, summary WorkspaceSummary, name, want string) {
	t.Helper()
	for _, stage := range summary.Stages {
		if stage.Name == name {
			if stage.Status != want {
				t.Fatalf("stage %s status = %q, want %q", name, stage.Status, want)
			}
			return
		}
	}
	t.Fatalf("stage %s missing from %#v", name, summary.Stages)
}

func assertArtifactStatus(t *testing.T, summary WorkspaceSummary, name, want string) {
	t.Helper()
	for _, artifact := range summary.Artifacts {
		if artifact.Name == name {
			if artifact.Status != want {
				t.Fatalf("artifact %s status = %q, want %q", name, artifact.Status, want)
			}
			return
		}
	}
	t.Fatalf("artifact %s missing from %#v", name, summary.Artifacts)
}

func TestInspectWorkspaceSummarizesManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-evidence", Title: "Workspace evidence", Description: "Evidence", Source: "test", State: string(workflow.Verification)}
	createWorkspaceDemand(t, store, demand)
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(time.Minute), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}
	if summary.Evidence.Pass != 1 || summary.Evidence.Fail != 0 || summary.Evidence.Blocked != 0 {
		t.Fatalf("Evidence = %#v, want one pass", summary.Evidence)
	}
	if len(summary.Evidence.Latest) != 1 || summary.Evidence.Latest[0].Criterion != "Inactive users are blocked" {
		t.Fatalf("Latest evidence = %#v", summary.Evidence.Latest)
	}
}

func TestWorkspaceNextActionsPreferEvidenceBeforeVerificationConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-evidence-next", Title: "Workspace evidence next", Description: "Evidence", Source: "test", State: string(workflow.Verification)}
	createWorkspaceDemand(t, store, demand)
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}
	if len(summary.Actions) == 0 || summary.Actions[0].Label != "Add acceptance evidence" {
		t.Fatalf("Actions = %#v, want Add acceptance evidence first", summary.Actions)
	}
	if !strings.Contains(summary.Actions[0].Command, "devflow evidence add --demand workspace-evidence-next") {
		t.Fatalf("first command = %q", summary.Actions[0].Command)
	}
}

package cli

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestConsoleListRendersDemandAttention(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	for _, demand := range []artifacts.Demand{
		{ID: "z-complete", Title: "Z complete", Description: "Done", Source: "test", State: string(workflow.Completed)},
		{ID: "a-failed", Title: "A failed", Description: "Failed", Source: "test", State: string(workflow.FailedQualityGate)},
	} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
		}
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Demand Console", "a-failed", "quality gate failed", "z-complete", "Next:", "devflow console --demand a-failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
}

func TestConsoleDetailRendersOperatorView(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-detail", Title: "Console detail", Description: "Show detail", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: fixedConsoleCLITime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: fixedConsoleCLITime().Add(time.Minute), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console detail returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Demand Console: console-detail", "State: verification", "Stages:", "Evidence:", "verification   PASS go test ./...", "Quality:", "requirements", "Recommended:", "Confirm verification", "Run-ready:", "no safe runner action"} {
		if !strings.Contains(got, want) {
			t.Fatalf("console detail missing %q:\n%s", want, got)
		}
	}
}

func TestConsoleDetailPrintsContextAwareRequirementChecks(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-quality-context", Title: "Console quality context", Description: "Evaluate context", Source: "test", State: string(workflow.RequirementsReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, "# Intake\n\n## 验收标准\n- Inactive users are blocked.\n"); err != nil {
		t.Fatalf("WriteArtifact intake returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, "# Context\n\n## Approved Stable Memory\n\nNo approved stable memory recalled.\n\n## Historical Demand Candidates\n\n- `coupon-old`: Candidate needs confirmation.\n"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- User status must be active.\n\n## 验收标准\n\n- Active users can claim coupons.\n\n## 待确认问题\n\n- 待人工补充。\n"); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"requirements.intake_coverage",
		"requirements.candidate_guard",
		"Inactive users are blocked",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
}

func TestConsoleHelpIncludesCommand(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"devflow console [--demand <id>] [--run-next]", "console  Show the operator demand console"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q:\n%s", want, got)
		}
	}
}

func fixedConsoleCLITime() time.Time {
	return time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
}

func TestConsoleRunNextCallsRunnerForAgentStage(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-run", Title: "Console run", Description: "Run requirements", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var gotArgs []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		fmt.Fprintln(stdout, "stub runner called")
		return nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "stub runner called") {
		t.Fatalf("stdout = %q, want stub runner output", stdout.String())
	}
	wantArgs := []string{"--root", root, "--demand", demand.ID, "--stage", "requirements"}
	for _, want := range wantArgs {
		if !containsString(gotArgs, want) {
			t.Fatalf("runner args = %#v, missing %q", gotArgs, want)
		}
	}
}

func TestConsoleRunNextRefusesHumanConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-confirm", Title: "Console confirm", Description: "Confirm", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 1, 0, 0, time.UTC), Type: "verification.recorded", Message: "pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 2, 0, 0, time.UTC), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	var called bool
	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		called = true
		return nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	if called {
		t.Fatal("runner was called for human confirmation")
	}
	if !strings.Contains(stdout.String(), "next action is not runner-safe: Confirm verification") {
		t.Fatalf("stdout = %q, want runner-safe refusal", stdout.String())
	}
}

func TestConsoleRunNextPassesQualityAndRunnerFlags(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-implementation", Title: "Console implementation", Description: "Implement", Source: "test", State: string(workflow.Implementation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var gotArgs []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next", "--runner-root", root, "--quality-root", root, "--config", "devflow.yaml", "--permission-mode", "acceptEdits", "--quality-command", "go test ./..."}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	for _, want := range []string{"--runner-root", "--quality-root", "--config", "devflow.yaml", "--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(gotArgs, want) {
			t.Fatalf("runner args = %#v, missing %q", gotArgs, want)
		}
	}
}

func TestConsoleRunNextUsesGeneratedImplementationDefaults(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-defaults", Title: "Console defaults", Description: "Use next action defaults", Source: "test", State: string(workflow.Implementation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var gotArgs []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	for _, want := range []string{"--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(gotArgs, want) {
			t.Fatalf("runner args = %#v, missing generated default %q", gotArgs, want)
		}
	}
}

func TestConsoleRunNextRunsMRReviewWithGitLabFlags(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-mr", Title: "Console MR", Description: "Review MR", Source: "test", State: string(workflow.MRReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var gotArgs []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		gotArgs = append([]string(nil), args...)
		return nil
	}

	err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next", "--gitlab-project", "group/project", "--gitlab-mr", "12", "--gitlab-base-url", "https://gitlab.example"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("console --run-next returned error: %v", err)
	}
	for _, want := range []string{"--stage", "mr-review", "--gitlab-project", "group/project", "--gitlab-mr", "12", "--gitlab-base-url", "https://gitlab.example"} {
		if !containsString(gotArgs, want) {
			t.Fatalf("runner args = %#v, missing %q", gotArgs, want)
		}
	}
}

func TestConsoleRunNextRefusesMRReviewWithoutGitLabFlags(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-mr-missing", Title: "Console MR missing", Description: "Review MR", Source: "test", State: string(workflow.MRReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"console", "--root", root, "--demand", demand.ID, "--run-next"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--gitlab-project and --gitlab-mr are required for mr-review") {
		t.Fatalf("err = %v, stdout = %q, want missing GitLab flags error", err, stdout.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestConsoleRunNextUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "console-defaults", Title: "Console defaults", Source: "test", State: string(workflow.Implementation)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	configPath := writeBackendDemandDefaultsConfig(t, root)
	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var got []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		got = append([]string(nil), args...)
		return nil
	}

	if err := Run([]string{"console", "--root", root, "--config", configPath, "--demand", "console-defaults", "--run-next"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("console returned error: %v", err)
	}
	for _, want := range []string{"--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(got, want) {
			t.Fatalf("console args missing %q: %#v", want, got)
		}
	}
}

func TestConsoleDetailPrintsManualEvidenceSummary(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-evidence", Title: "Console evidence", Description: "Show evidence", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: fixedConsoleCLITime(), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "manual", "criterion": "QA accepted", "summary": "QA signed off"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console detail returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "manual         pass=1 fail=0 blocked=0") {
		t.Fatalf("console detail missing manual evidence:\n%s", stdout.String())
	}
}

func TestConsoleDetailPrintsCIGateStatus(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-ci", Title: "Console ci", Description: "Show ci", Source: "test", State: string(workflow.MRReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: fixedConsoleCLITime(), Type: "ci_gate.blocked", Message: "github ci pending", Data: map[string]string{"provider": "github", "repo": "owner/repo", "pr": "42", "status": "pending"}}); err != nil {
		t.Fatalf("AppendEvent ci returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console detail returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ci             owner/repo#42 pending") {
		t.Fatalf("console detail missing ci gate status:\n%s", stdout.String())
	}
}

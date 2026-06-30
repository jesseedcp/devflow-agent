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

package cli

import (
	"bytes"
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
	for _, want := range []string{"Demand Console: console-detail", "State: verification", "Stages:", "Evidence:", "verification   PASS go test ./...", "Recommended:", "Confirm verification", "Run-ready:", "no safe runner action"} {
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

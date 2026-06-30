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

func TestDriveDryRunPrintsNextAction(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-dry", Title: "Drive dry", Description: "Run requirements", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"drive", "--root", root, "--demand", demand.ID, "--dry-run"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive --dry-run returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Drive: drive-dry", "dry-run", "Draft requirements", "devflow run"} {
		if !strings.Contains(got, want) {
			t.Fatalf("drive dry-run missing %q:\n%s", want, got)
		}
	}
}

func TestDriveRequiresDemand(t *testing.T) {
	err := Run([]string{"drive"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v, want demand required", err)
	}
}

func TestDriveHelpIncludesCommand(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "devflow drive --demand <id>") {
		t.Fatalf("help missing drive:\n%s", stdout.String())
	}
}

func TestDriveStopsAtHumanConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-confirm", Title: "Drive confirm", Description: "Confirm", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 1, 0, 0, time.UTC), Type: "verification.recorded", Message: "pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		t.Fatal("runner should not be called for human confirmation")
		return nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"drive", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "reason: human_confirmation") {
		t.Fatalf("stdout = %q, want human stop", stdout.String())
	}
}

func TestDriveLoopRunsUntilManualGate(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-loop", Title: "Drive loop", Description: "Loop", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var calls int
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		calls++
		if calls != 1 {
			t.Fatalf("calls = %d, want one runner call before manual gate", calls)
		}
		loaded, err := store.LoadDemand(demand.ID)
		if err != nil {
			t.Fatalf("LoadDemand returned error: %v", err)
		}
		loaded.State = string(workflow.RequirementsReview)
		if err := store.SaveDemand(loaded); err != nil {
			t.Fatalf("SaveDemand returned error: %v", err)
		}
		fmt.Fprintln(stdout, "stub requirements")
		return nil
	}

	var stdout bytes.Buffer
	if err := Run([]string{"drive", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Step 1", "stub requirements", "reason: human_confirmation"} {
		if !strings.Contains(got, want) {
			t.Fatalf("drive output missing %q:\n%s", want, got)
		}
	}
}

func TestDriveStopsOnRunnerFailure(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-fail", Title: "Drive fail", Description: "Fail", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		return fmt.Errorf("runner boom")
	}

	var stdout bytes.Buffer
	err := Run([]string{"drive", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "runner boom") {
		t.Fatalf("err = %v, want runner error", err)
	}
	if !strings.Contains(stdout.String(), "reason: runner_failed") {
		t.Fatalf("stdout = %q, want runner_failed", stdout.String())
	}
}

func TestDriveUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "drive-defaults", Title: "Drive defaults", Source: "test", State: string(workflow.Implementation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	configPath := writeBackendDemandDefaultsConfig(t, root)
	old := runConsoleDemandStage
	defer func() { runConsoleDemandStage = old }()
	var got []string
	runConsoleDemandStage = func(args []string, stdout io.Writer, stderr io.Writer) error {
		got = append([]string(nil), args...)
		loaded, err := store.LoadDemand(demand.ID)
		if err != nil {
			return err
		}
		loaded.State = string(workflow.MRReview)
		return store.SaveDemand(loaded)
	}

	if err := Run([]string{"drive", "--root", root, "--config", configPath, "--demand", demand.ID, "--max-steps", "1"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("drive returned error: %v", err)
	}
	for _, want := range []string{"--permission-mode", "acceptEdits", "--quality-command", "go test ./..."} {
		if !containsString(got, want) {
			t.Fatalf("drive args missing %q: %#v", want, got)
		}
	}
}

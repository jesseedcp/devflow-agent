package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestStatusPrintsDemandStateArtifactsAndNextActions(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Created)

	var stdout bytes.Buffer
	if err := Run([]string{"status", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("status: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Demand: add-coupon-check",
		"State: created",
		"requirements.md",
		"Next actions:",
		"Draft requirements",
		"devflow run --demand add-coupon-check --stage requirements",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestNextPrintsFirstRecommendedCommandOnly(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Verification)

	var stdout bytes.Buffer
	if err := Run([]string{"next", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("next: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	want := `devflow run --demand add-coupon-check --stage verification --quality-command "go test ./..."`
	if got != want {
		t.Fatalf("next = %q want %q", got, want)
	}
	if strings.Contains(got, "Confirm verification") {
		t.Fatalf("next output included extra action: %q", got)
	}
}

func TestStatusRequiresDemand(t *testing.T) {
	var stdout bytes.Buffer
	err := Run([]string{"status"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v want --demand is required", err)
	}
}

func TestNextForCompletedDemandPrintsNoCommand(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Completed)

	var stdout bytes.Buffer
	if err := Run([]string{"next", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("next: %v", err)
	}
	if !strings.Contains(stdout.String(), "No next command for add-coupon-check in state completed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestHelpIncludesStatusAndNext(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"devflow status --demand <id>",
		"devflow next --demand <id>",
		"status    Show demand state, artifacts, and next actions",
		"next      Print the next recommended command for a demand",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}

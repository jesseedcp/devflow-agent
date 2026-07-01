package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestRecallCommandRewritesContext(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "recall-demand", Title: "Recall demand", Description: "memory", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact("recall-demand", artifacts.ContextFile, "stale"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"recall", "--root", root, "--demand", "recall-demand"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("recall returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "recall-demand", artifacts.ContextFile))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if !strings.Contains(string(body), "# Context: Recall demand") {
		t.Fatalf("context not rewritten:\n%s", string(body))
	}
	if !strings.Contains(stdout.String(), "context recalled for recall-demand") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRecallCommandRequiresDemand(t *testing.T) {
	err := Run([]string{"recall"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v, want --demand is required", err)
	}
}

func TestHelpIncludesRecall(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow recall --demand <id>", "recall   Rebuild reusable memory context for a demand"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestEvaluateCommandPrintsStageStatuses(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli", Title: "Eval CLI", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- rule\n\n## 验收标准\n\n- accept\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Evaluation: eval-cli", "requirements", "blockers", "warnings"} {
		if !strings.Contains(got, want) {
			t.Fatalf("evaluate output missing %q:\n%s", want, got)
		}
	}
}

func TestEvaluateCommandStrictReturnsErrorOnFailure(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-strict", Title: "Eval CLI strict", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- rule\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "requirements", "--strict"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("err = %v, want evaluation failed", err)
	}
}

func TestEvaluateCommandStageFilter(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-stage", Title: "Eval CLI stage", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "plan"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "plan") {
		t.Fatalf("evaluate output missing plan:\n%s", got)
	}
	if strings.Contains(got, "requirements") {
		t.Fatalf("evaluate output included unrequested requirements:\n%s", got)
	}
}

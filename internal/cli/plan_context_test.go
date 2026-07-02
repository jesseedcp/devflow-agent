package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestPlanContextRefreshCombinesRequirementsContextAndCodemap(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test", Description: "Inactive users are blocked"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## Acceptance Criteria\n\n- Inactive users are blocked.\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, "# Context\n\n## Approved Stable Memory\n\n- Reuse coupon eligibility rules.\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CodemapFile, "# Codemap Context\n\n- `internal/coupon/service.go:7` method `CheckEligibility` score=3\n"); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := Run([]string{"plan-context", "refresh", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("plan-context refresh returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.PlanContextFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"Inactive users are blocked", "Approved Stable Memory", "internal/coupon/service.go", "Plan Context"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan-context.md missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(stdout.String(), "plan context refreshed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestPlanContextRefreshRequiresDemand(t *testing.T) {
	err := Run([]string{"plan-context", "refresh"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v, want demand required", err)
	}
}

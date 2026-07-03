package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestImplementationReviewRefreshWritesArtifact(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ChangeScopeFile, "# Change Scope\n\n## Source Files\n\n- `internal/coupon/service.go`\n\n## Test Files\n\n- `internal/coupon/service_test.go`\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Data: map[string]string{"status": "pass", "type": "api"}}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := Run([]string{"implementation-review", "refresh", "--root", root, "--demand", demand.ID, "--changed", "internal/coupon/service.go", "--changed", "internal/coupon/service_test.go"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("implementation-review refresh returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.ImplementationReviewFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"# Implementation Review: coupon", "ready_for_closeout", "internal/coupon/service.go"} {
		if !strings.Contains(text, want) {
			t.Fatalf("implementation review missing %q:\n%s", want, text)
		}
	}
}

func TestImplementationReviewStatusMissingArtifactIncludesRefreshCommand(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "missing-review", Title: "Missing", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(store.DemandDir(demand.ID), artifacts.ImplementationReviewFile)); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"implementation-review", "status", "--root", root, "--demand", demand.ID}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when implementation-review.md is missing")
	}
	if !strings.Contains(err.Error(), "implementation-review refresh") {
		t.Fatalf("error missing refresh guidance: %v", err)
	}
}

func TestImplementationReviewRefreshTemplateScopeIncludesScopeDeclare(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "template-scope", Title: "Template Scope", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"implementation-review", "refresh", "--root", root, "--demand", demand.ID, "--changed", "internal/coupon/service.go"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error when change-scope.md is template-only")
	}
	if !strings.Contains(err.Error(), "scope declare") {
		t.Fatalf("error missing scope declare guidance: %v", err)
	}
}
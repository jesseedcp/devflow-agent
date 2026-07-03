package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestScopeDeclareWritesChangeScope(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := Run([]string{"scope", "declare", "--root", root, "--demand", demand.ID, "--source", "internal/coupon/service.go", "--test", "internal/coupon/service_test.go"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("scope declare returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.ChangeScopeFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"internal/coupon/service.go", "internal/coupon/service_test.go"} {
		if !strings.Contains(text, want) {
			t.Fatalf("change scope missing %q:\n%s", want, text)
		}
	}
	if !strings.Contains(stdout.String(), "scope declared") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestScopeDeclareRequiresDemand(t *testing.T) {
	err := Run([]string{"scope", "declare", "--source", "internal/coupon/service.go"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v, want demand required", err)
	}
}

func TestScopeDeclareRejectsEmptyScope(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	err := Run([]string{"scope", "declare", "--root", root, "--demand", demand.ID}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "at least one --source or --test file is required") {
		t.Fatalf("scope declare error = %v", err)
	}
}

func TestScopeDiffReportsOutOfScopeChanges(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"scope", "declare", "--root", root, "--demand", demand.ID, "--source", "internal/coupon/service.go", "--test", "internal/coupon/service_test.go"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := Run([]string{"scope", "diff", "--root", root, "--demand", demand.ID, "--changed", "internal/coupon/service.go", "--changed", "README.md"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "scope diff found") {
		t.Fatalf("scope diff error = %v, want scope diff found", err)
	}
	text := stdout.String()
	for _, want := range []string{"## In Scope", "internal/coupon/service.go", "## Out Of Scope", "README.md", "## Missing Declared Tests", "internal/coupon/service_test.go"} {
		if !strings.Contains(text, want) {
			t.Fatalf("scope diff output missing %q:\n%s", want, text)
		}
	}
}

func TestScopeDiffPrintsGroupedSectionsAndNextAction(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"scope", "declare", "--root", root, "--demand", demand.ID, "--source", "internal/coupon/service.go", "--test", "internal/coupon/service_test.go"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := Run([]string{"scope", "diff", "--root", root, "--demand", demand.ID, "--changed", "internal/coupon/service.go", "--changed", "README.md"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "scope diff found") {
		t.Fatalf("scope diff error = %v", err)
	}
	text := stdout.String()
	for _, want := range []string{
		"Summary: in_scope=1 out_of_scope=1 missing_tests=1",
		"## In Scope",
		"## Out Of Scope",
		"README.md",
		"## Missing Declared Tests",
		"internal/coupon/service_test.go",
		"Next: update change-scope.md or adjust implementation changes",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("scope diff output missing %q:\n%s", want, text)
		}
	}
}

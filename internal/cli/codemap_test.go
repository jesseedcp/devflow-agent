package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestCodemapIndexAndSearch(t *testing.T) {
	root := t.TempDir()
	writeCodemapFixture(t, filepath.Join(root, "internal", "coupon", "service.go"), `package coupon

func CheckEligibility() bool {
	route := "/coupon/claim"
	_ = route
	return true
}
`)
	var stdout bytes.Buffer
	if err := Run([]string{"codemap", "index", "--root", root}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("codemap index returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "codemap indexed") {
		t.Fatalf("index output = %q", stdout.String())
	}
	stdout.Reset()
	if err := Run([]string{"codemap", "search", "--root", root, "coupon eligibility"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("codemap search returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "internal/coupon/service.go") {
		t.Fatalf("search output missing service file:\n%s", stdout.String())
	}
}

func writeCodemapFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCodemapRefreshWritesDemandCodemap(t *testing.T) {
	root := t.TempDir()
	writeCodemapFixture(t, filepath.Join(root, "internal", "coupon", "service.go"), `package coupon

func CheckEligibility() bool {
	return true
}
`)
	if err := Run([]string{"codemap", "index", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("codemap index returned error: %v", err)
	}
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon-eligibility", Title: "Coupon Eligibility", Description: "Inactive users are blocked", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	var stdout bytes.Buffer
	if err := Run([]string{"codemap", "refresh", "--root", root, "--demand", demand.ID, "--query", "coupon eligibility inactive"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("codemap refresh returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "codemap refreshed") {
		t.Fatalf("refresh output = %q", stdout.String())
	}
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, "codemap.md"))
	if err != nil {
		t.Fatalf("read codemap.md: %v", err)
	}
	if !strings.Contains(string(data), "internal/coupon/service.go") {
		t.Fatalf("codemap.md missing service file:\n%s", string(data))
	}
}

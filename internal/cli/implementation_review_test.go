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

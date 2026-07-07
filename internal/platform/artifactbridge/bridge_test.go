package artifactbridge

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func seedDemand(t *testing.T, root, id, title, state string) {
	t.Helper()
	st := artifacts.NewStore(root)
	if err := st.CreateDemand(artifacts.Demand{ID: id, Title: title, Description: "demo", Source: "test", State: state}); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	if err := st.AppendEvent(id, artifacts.Event{Type: "note", Message: "started"}); err != nil {
		t.Fatalf("append event: %v", err)
	}
}

func hasArtifact(list []string, name string) bool {
	for _, a := range list {
		if a == name {
			return true
		}
	}
	return false
}

func TestScanDemands(t *testing.T) {
	root := t.TempDir()
	seedDemand(t, root, "alpha-1", "Alpha", "plan")

	got, err := ScanDemands(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 demand, got %d", len(got))
	}
	d := got[0]
	if d.DemandKey != "alpha-1" || d.Title != "Alpha" || d.State != "plan" {
		t.Fatalf("unexpected summary: %+v", d)
	}
	if d.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero updated_at")
	}
	names := make([]string, 0, len(d.Artifacts))
	for _, a := range d.Artifacts {
		names = append(names, a.Name)
	}
	if !hasArtifact(names, "requirements.md") {
		t.Fatalf("requirements.md not in artifacts: %v", names)
	}
	for _, a := range d.Artifacts {
		if a.Name == "requirements.md" && !a.Exists {
			t.Fatal("requirements.md should exist")
		}
	}
}

func TestScanDemandsEmptyRoot(t *testing.T) {
	got, err := ScanDemands(t.TempDir())
	if err != nil {
		t.Fatalf("scan empty root: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 demands, got %d", len(got))
	}
}

func TestGetDemand(t *testing.T) {
	root := t.TempDir()
	seedDemand(t, root, "beta-2", "Beta", "verification")

	detail, err := GetDemand(root, "beta-2")
	if err != nil {
		t.Fatalf("get demand: %v", err)
	}
	if detail.DemandKey != "beta-2" || detail.Title != "Beta" || detail.State != "verification" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if len(detail.Artifacts) == 0 {
		t.Fatal("expected artifacts in detail")
	}
}

func TestGetDemandMissing(t *testing.T) {
	if _, err := GetDemand(t.TempDir(), "nope"); err == nil {
		t.Fatal("expected error for missing demand")
	}
}

func TestReadArtifact(t *testing.T) {
	root := t.TempDir()
	seedDemand(t, root, "gamma-3", "Gamma", "plan")

	content, err := ReadArtifact(root, "gamma-3", "requirements.md")
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Fatal("expected non-empty requirements.md")
	}
}

func TestReadArtifactRejectsUnknownName(t *testing.T) {
	root := t.TempDir()
	seedDemand(t, root, "delta-4", "Delta", "plan")

	if _, err := ReadArtifact(root, "delta-4", "bogus.txt"); err == nil {
		t.Fatal("expected error for unknown artifact name")
	}
}

func TestReadArtifactMissingDemand(t *testing.T) {
	if _, err := ReadArtifact(t.TempDir(), "missing", "requirements.md"); err == nil {
		t.Fatal("expected error for missing demand")
	}
}
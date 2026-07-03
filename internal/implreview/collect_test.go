package implreview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestCollectBuildsRecommendationFromScopeAndEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	writeDemandFile(t, store.DemandDir(demand.ID), artifacts.ChangeScopeFile, "# Change Scope\n\n## Source Files\n\n- `internal/coupon/service.go`\n\n## Test Files\n\n- `internal/coupon/service_test.go`\n")
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Data: map[string]string{"status": "pass", "type": "api"}}); err != nil {
		t.Fatal(err)
	}
	review, err := Collect(root, demand.ID, []string{"internal/coupon/service.go", "README.md"})
	if err != nil {
		t.Fatal(err)
	}
	if review.Recommendation != "needs_scope_review" {
		t.Fatalf("Recommendation = %q", review.Recommendation)
	}
	if len(review.OutOfScope) != 1 || review.OutOfScope[0] != "README.md" {
		t.Fatalf("OutOfScope = %#v", review.OutOfScope)
	}
	if review.AcceptancePass != 1 {
		t.Fatalf("AcceptancePass = %d", review.AcceptancePass)
	}
}

func writeDemandFile(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecommendationReadyWhenSignalsClean(t *testing.T) {
	review := Review{ChangedFiles: []string{"internal/coupon/service.go"}, VerificationStatus: "pass", AcceptancePass: 1, MRStatus: "cleared"}
	if got := Recommend(review); got != "ready_for_closeout" {
		t.Fatalf("Recommend = %q", got)
	}
}

func TestRecommendationNeedsVerification(t *testing.T) {
	review := Review{ChangedFiles: []string{"internal/coupon/service.go"}, MRStatus: "cleared"}
	if got := Recommend(review); !strings.Contains(got, "verification") {
		t.Fatalf("Recommend = %q", got)
	}
}

func TestCollectFailsWhenChangeScopeMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "no-scope", Title: "No Scope", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(store.DemandDir(demand.ID), artifacts.ChangeScopeFile)); err != nil {
		t.Fatal(err)
	}
	_, err := Collect(root, demand.ID, []string{"internal/coupon/service.go"})
	if err == nil {
		t.Fatal("expected error when change-scope.md is missing")
	}
	if !strings.Contains(err.Error(), "scope declare") {
		t.Fatalf("error missing scope declare guidance: %v", err)
	}
}

func TestCollectFailsWhenChangeScopeTemplateOnly(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "template-scope", Title: "Template Scope", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	_, err := Collect(root, demand.ID, []string{"internal/coupon/service.go"})
	if err == nil {
		t.Fatal("expected error when change-scope.md is template-only")
	}
	if !strings.Contains(err.Error(), "scope declare") {
		t.Fatalf("error missing scope declare guidance: %v", err)
	}
}
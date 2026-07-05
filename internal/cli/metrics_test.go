package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestMetricsReportPrintsAndWritesMetricsArtifact(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "coupon-metrics", Title: "Coupon metrics", Description: "demo", State: "completed"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "stage.confirmed", Data: map[string]string{"stage": "requirements"}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Data: map[string]string{"status": "pass"}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"metrics", "report", "--root", root}, &stdout, &stderr); err != nil {
		t.Fatalf("metrics report returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Devflow Metrics") || !strings.Contains(stdout.String(), "coupon-metrics") {
		t.Fatalf("stdout missing metrics:\n%s", stdout.String())
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.MetricsFile))
	if err != nil {
		t.Fatalf("read metrics artifact: %v", err)
	}
	if !strings.Contains(string(body), "coupon-metrics") {
		t.Fatalf("metrics artifact missing demand:\n%s", string(body))
	}
}

func TestMetricsReportDemandFiltersToOneDemand(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	for _, demand := range []artifacts.Demand{
		{ID: "coupon-metrics", Title: "Coupon metrics", Description: "demo", State: "completed"},
		{ID: "refund-metrics", Title: "Refund metrics", Description: "demo", State: "completed"},
	} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand returned error: %v", err)
		}
	}

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"metrics", "report", "--root", root, "--demand", "coupon-metrics"}, &stdout, &stderr); err != nil {
		t.Fatalf("metrics report demand returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "coupon-metrics") {
		t.Fatalf("stdout missing selected demand:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "refund-metrics") {
		t.Fatalf("stdout included unselected demand:\n%s", stdout.String())
	}
}

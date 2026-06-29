package dogfood

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

var fixedDogfoodNow = func() time.Time { return time.Date(2026, 6, 29, 8, 0, 0, 0, time.UTC) }

func TestRunCompletesFullDeterministicLoop(t *testing.T) {
	root := t.TempDir()
	qualityRoot := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	result, err := Run(context.Background(), Options{
		Root:        root,
		QualityRoot: qualityRoot,
		QualityCommands: []quality.Command{{
			Name: testHelperExecutable(t),
			Args: []string{"-test.run=^TestDogfoodHelper$"},
		}},
		Now: fixedDogfoodNow,
	})
	if err != nil {
		t.Fatalf("dogfood run: %v", err)
	}
	if result.FinalState != workflow.Completed {
		t.Fatalf("final state = %s, want completed", result.FinalState)
	}
	if result.ReportPath == "" {
		t.Fatal("report path is empty")
	}
	report, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{"requirements", "confirm requirements", "plan", "implementation", "mr-review", "verification", "closeout", "completed"} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("report missing %q:\n%s", want, reportText)
		}
	}

	demandDir := filepath.Join(root, ".devflow", "demands", "dogfood-coupon-eligibility")
	for _, name := range []string{
		artifacts.RequirementsFile,
		artifacts.PlanFile,
		artifacts.ProgressFile,
		artifacts.VerificationFile,
		artifacts.CloseoutFile,
		artifacts.MemoryCandidatesFile,
		"dogfood-report.md",
	} {
		if _, err := os.Stat(filepath.Join(demandDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}

	progressData, err := os.ReadFile(filepath.Join(demandDir, artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress.md: %v", err)
	}
	if !strings.Contains(string(progressData), "!1") {
		t.Fatalf("progress.md missing MR evidence:\n%s", string(progressData))
	}
}

func TestRunRejectsUnknownScenario(t *testing.T) {
	_, err := Run(context.Background(), Options{Root: t.TempDir(), QualityRoot: t.TempDir(), ScenarioName: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported dogfood scenario") {
		t.Fatalf("err = %v, want unsupported scenario", err)
	}
}

func TestDogfoodHelper(t *testing.T) {
	if os.Getenv("DEVFLOW_DOGFOOD_HELPER") != "1" {
		return
	}
	os.Exit(0)
}

func testHelperExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return exe
}

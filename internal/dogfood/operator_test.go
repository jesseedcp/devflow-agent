package dogfood

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestRunOperatorCompletesAndWritesEvidence(t *testing.T) {
	root := t.TempDir()
	qualityRoot := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	result, err := RunOperator(context.Background(), OperatorOptions{
		Root:        root,
		QualityRoot: qualityRoot,
		QualityCommands: []quality.Command{{
			Name: testHelperExecutable(t),
			Args: []string{"-test.run=^TestDogfoodHelper$"},
		}},
		Now: fixedDogfoodNow,
	})
	if err != nil {
		t.Fatalf("RunOperator returned error: %v", err)
	}
	if result.FinalState != workflow.Completed {
		t.Fatalf("FinalState = %s, want completed", result.FinalState)
	}
	if result.ReportPath == "" {
		t.Fatal("ReportPath is empty")
	}
	evaluation, err := demandflow.EvaluateDemand(root, result.DemandID)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if evaluation.Overall != demandflow.EvaluationPass {
		t.Fatalf("final evaluation = %s, want pass; stages=%#v", evaluation.Overall, evaluation.Stages)
	}
	report, err := os.ReadFile(result.ReportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	reportText := string(report)
	for _, want := range []string{"Operator Dogfood Report", "Drive", "Evaluation", "Workbench Snapshot", "human_confirmation", "completed"} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("report missing %q:\n%s", want, reportText)
		}
	}
	demandDir := filepath.Join(root, ".devflow", "demands", result.DemandID)
	for _, name := range []string{artifacts.RequirementsFile, artifacts.PlanFile, artifacts.VerificationFile, artifacts.CloseoutFile, "operator-dogfood-report.md"} {
		if _, err := os.Stat(filepath.Join(demandDir, name)); err != nil {
			t.Fatalf("artifact %s missing: %v", name, err)
		}
	}
}

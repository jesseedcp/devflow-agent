package dogfood

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestRunLiveSandboxRequiresExplicitEnvironmentOptIn(t *testing.T) {
	t.Setenv("DEVFLOW_LIVE_DOGFOOD", "")
	_, err := RunLiveSandbox(context.Background(), LiveOptions{Root: t.TempDir(), Now: fixedDogfoodNow})
	if err == nil || !strings.Contains(err.Error(), "DEVFLOW_LIVE_DOGFOOD=1") {
		t.Fatalf("err = %v, want live opt-in error", err)
	}
}

func TestRenderLiveReportIncludesRootsAndSteps(t *testing.T) {
	report := renderLiveReport(LiveResult{
		Root:       "root",
		RepoRoot:   "root/repo",
		DemandRoot: "root/artifacts",
		DemandID:   "live-dogfood-coupon-eligibility",
		Steps:      []Step{{Name: "requirements", State: "requirements_review", Output: "drafted"}},
	})
	for _, want := range []string{"Live Dogfood Report", "RepoRoot", "DemandRoot", "requirements"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}

func TestLiveSandboxDogfood(t *testing.T) {
	if os.Getenv("DEVFLOW_LIVE_DOGFOOD") != "1" {
		t.Skip("set DEVFLOW_LIVE_DOGFOOD=1 with provider credentials to run live sandbox dogfood")
	}
	result, err := RunLiveSandbox(context.Background(), LiveOptions{
		Root: t.TempDir(),
		Now:  fixedDogfoodNow,
	})
	if err != nil {
		t.Fatalf("live sandbox dogfood: %v", err)
	}
	if result.FinalState != "completed" {
		t.Fatalf("final state = %s, want completed", result.FinalState)
	}
	if result.ReportPath == "" {
		t.Fatal("report path is empty")
	}
}

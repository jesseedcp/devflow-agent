package releasecontrol

import (
	"strings"
	"testing"
	"time"
)

func TestRenderDeploymentIncludesRunAndStatus(t *testing.T) {
	record := DeploymentRecord{
		Provider:   "github_actions",
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		RunID:      "123",
		RunURL:     "https://github.com/owner/repo/actions/runs/123",
		HeadSHA:    "abc123",
		Status:     StatusPassed,
		Conclusion: "success",
		Summary:    "release workflow passed",
		CreatedAt:  time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 7, 5, 10, 2, 0, 0, time.UTC),
	}

	body := RenderDeployment("Coupon release", record)

	for _, want := range []string{
		"# Deployment: Coupon release",
		"Provider: `github_actions`",
		"Repository: `owner/repo`",
		"Workflow: `release.yml`",
		"Run ID: `123`",
		"Status: `passed`",
		"Conclusion: `success`",
		"https://github.com/owner/repo/actions/runs/123",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("deployment body missing %q:\n%s", want, body)
		}
	}
}

func TestRenderRollbackDefaultsToPendingDecision(t *testing.T) {
	body := RenderRollback("Coupon release", RollbackRecord{
		Trigger:     "deployment failed",
		Impact:      "release did not complete",
		Recommended: "rerun deployment after fix",
	})

	for _, want := range []string{"# Rollback: Coupon release", "Decision: `pending`", "deployment failed"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rollback body missing %q:\n%s", want, body)
		}
	}
}

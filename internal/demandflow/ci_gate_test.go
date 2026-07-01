package demandflow

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func TestRenderCIGateEvidenceIncludesStatusAndChecks(t *testing.T) {
	body := renderCIGateEvidence(adapters.CIResult{
		Provider: "github",
		Repo:     "owner/repo",
		PR:       "42",
		HeadSHA:  "abc123",
		Status:   adapters.CIStatusFailed,
		Checks: []adapters.CICheck{{
			Name:       "Go verification",
			Status:     "completed",
			Conclusion: "failure",
			URL:        "https://github.test/checks/1",
		}},
	})
	for _, want := range []string{"## CI Gate", "github", "owner/repo#42", "failed", "Go verification", "failure"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered evidence missing %q:\n%s", want, body)
		}
	}
}

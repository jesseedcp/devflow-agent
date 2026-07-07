package releasecontrol

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/evidence"
)

func TestObservationFromHealthResultPass(t *testing.T) {
	record := ObservationFromHealthResult(evidence.FetchResult{
		Status:          "pass",
		URL:             "https://example.test/health",
		ActualStatus:    200,
		Summary:         "GET health returned 200",
		ResponseExcerpt: "ok",
	})

	if record.Status != StatusPassed {
		t.Fatalf("Status = %s, want passed", record.Status)
	}
	if len(record.Checks) != 1 || record.Checks[0].Status != StatusPassed {
		t.Fatalf("checks = %#v", record.Checks)
	}
}

func TestObservationFromHealthResultFailRedactsExcerpt(t *testing.T) {
	record := ObservationFromHealthResult(evidence.FetchResult{
		Status:          "fail",
		URL:             "https://example.test/health?token=abc",
		ActualStatus:    500,
		Summary:         "GET failed token=abc",
		ResponseExcerpt: "Authorization: Bearer secret-token",
	})

	if record.Status != StatusFailed {
		t.Fatalf("Status = %s, want failed", record.Status)
	}
	body := RenderObservation("Health", record)
	for _, leaked := range []string{"token=abc", "secret-token"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("observation leaked %q:\n%s", leaked, body)
		}
	}
}

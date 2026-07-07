package releasecontrol

import evidenceadapter "github.com/jesseedcp/devflow-agent/internal/evidence"

func ObservationFromHealthResult(result evidenceadapter.FetchResult) ObservationRecord {
	status := StatusFailed
	summary := result.Summary
	if result.Status == "pass" {
		status = StatusPassed
	}
	if result.Status == "blocked" {
		status = StatusBlocked
	}
	check := ObservationCheck{
		Name:            "http_health",
		Status:          status,
		Summary:         evidenceadapter.Redact(summary),
		URL:             evidenceadapter.Redact(result.URL),
		ActualStatus:    result.ActualStatus,
		ResponseExcerpt: evidenceadapter.Excerpt(result.ResponseExcerpt, 512),
	}
	record := ObservationRecord{
		Status:  status,
		Summary: evidenceadapter.Redact(summary),
		Checks:  []ObservationCheck{check},
	}
	if status != StatusPassed {
		record.BlockingFindings = []string{record.Summary}
	}
	if check.URL != "" {
		record.EvidenceLinks = []string{check.URL}
	}
	return record
}

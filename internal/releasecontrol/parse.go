package releasecontrol

import (
	"strings"
	"time"
)

func metadataValue(text, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		value = strings.Trim(value, "` ")
		return value
	}
	return ""
}

func parseTimestamp(text, key string) time.Time {
	value := metadataValue(text, key)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func normalizeStatus(value string) Status {
	value = strings.TrimSpace(value)
	if value == "" {
		return StatusNotStarted
	}
	switch Status(value) {
	case StatusNotStarted, StatusPending, StatusPassed, StatusFailed, StatusBlocked, StatusUnknown:
		return Status(value)
	}
	return StatusUnknown
}

func normalizeRollbackDecision(value string) RollbackDecision {
	value = strings.TrimSpace(value)
	if value == "" {
		return RollbackPending
	}
	switch RollbackDecision(value) {
	case RollbackPending, RollbackConfirmed, RollbackRiskAccepted, RollbackRedeployRequired:
		return RollbackDecision(value)
	}
	return RollbackPending
}

func ParseDeployment(text string) DeploymentRecord {
	return DeploymentRecord{
		Provider:    metadataValue(text, "Provider"),
		Repo:        metadataValue(text, "Repository"),
		WorkflowID:  metadataValue(text, "Workflow"),
		Ref:         metadataValue(text, "Ref"),
		Environment: metadataValue(text, "Environment"),
		RunID:       metadataValue(text, "Run ID"),
		RunURL:      metadataValue(text, "Run URL"),
		HeadSHA:     metadataValue(text, "Head SHA"),
		Status:      normalizeStatus(metadataValue(text, "Status")),
		Conclusion:  metadataValue(text, "Conclusion"),
		Summary:     metadataValue(text, "Summary"),
		TriggeredBy: metadataValue(text, "Triggered By"),
		CreatedAt:   parseTimestamp(text, "Created At"),
		UpdatedAt:   parseTimestamp(text, "Updated At"),
	}
}

func ParseObservation(text string) ObservationRecord {
	record := ObservationRecord{
		Provider:         metadataValue(text, "Provider"),
		Repo:             metadataValue(text, "Repository"),
		RunID:            metadataValue(text, "Run ID"),
		RunURL:           metadataValue(text, "Run URL"),
		DeploymentStatus: normalizeStatus(metadataValue(text, "Deployment Status")),
		Status:           normalizeStatus(metadataValue(text, "Status")),
		ObservedAt:       parseTimestamp(text, "Observed At"),
	}
	return record
}

func ParseRollback(text string) RollbackRecord {
	return RollbackRecord{
		Trigger:       metadataValue(text, "Trigger"),
		Impact:        metadataValue(text, "Impact"),
		Recommended:   metadataValue(text, "Recommended"),
		Decision:      normalizeRollbackDecision(metadataValue(text, "Decision")),
		DecisionBy:    metadataValue(text, "By"),
		DecisionNotes: metadataValue(text, "Notes"),
		RecordedAt:    parseTimestamp(text, "Recorded At"),
	}
}

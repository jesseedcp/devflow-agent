package releasecontrol

import (
	"fmt"
	"strings"
	"time"
)

func statusOrDefault(status Status) Status {
	if strings.TrimSpace(string(status)) == "" {
		return StatusNotStarted
	}
	return status
}

func decisionOrDefault(decision RollbackDecision) RollbackDecision {
	if strings.TrimSpace(string(decision)) == "" {
		return RollbackPending
	}
	return decision
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func metadataLine(key, value string) string {
	return fmt.Sprintf("%s: `%s`", key, value)
}

func timestampLine(key string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return metadataLine(key, t.Format(time.RFC3339))
}

func RenderDeployment(title string, record DeploymentRecord) string {
	status := statusOrDefault(record.Status)
	var b strings.Builder
	fmt.Fprintf(&b, "# Deployment: %s\n\n", title)
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", nonEmpty(record.Summary, "No deployment recorded yet."))
	fmt.Fprintf(&b, "## Provider\n\n%s\n\n", metadataLine("Provider", record.Provider))
	fmt.Fprintf(&b, "## Repository\n\n%s\n\n", metadataLine("Repository", record.Repo))
	fmt.Fprintf(&b, "## Workflow\n\n%s\n", metadataLine("Workflow", record.WorkflowID))
	if strings.TrimSpace(record.Ref) != "" {
		fmt.Fprintf(&b, "%s\n", metadataLine("Ref", record.Ref))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "## Run\n\n%s\n", metadataLine("Run ID", record.RunID))
	if strings.TrimSpace(record.RunURL) != "" {
		fmt.Fprintf(&b, "Run URL: %s\n", record.RunURL)
	}
	if strings.TrimSpace(record.HeadSHA) != "" {
		fmt.Fprintf(&b, "%s\n", metadataLine("Head SHA", record.HeadSHA))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "## Environment\n\n%s\n\n", metadataLine("Environment", record.Environment))
	fmt.Fprintf(&b, "## Status\n\n%s\n", metadataLine("Status", string(status)))
	fmt.Fprintf(&b, "%s\n\n", metadataLine("Conclusion", record.Conclusion))
	fmt.Fprintf(&b, "## Triggered By\n\n%s\n\n", metadataLine("Triggered By", record.TriggeredBy))
	b.WriteString("## Timestamps\n\n")
	if line := timestampLine("Created At", record.CreatedAt); line != "" {
		fmt.Fprintf(&b, "%s\n", line)
	}
	if line := timestampLine("Updated At", record.UpdatedAt); line != "" {
		fmt.Fprintf(&b, "%s\n", line)
	}
	b.WriteString("\n## Evidence Links\n\n## Events\n")
	return b.String()
}

func RenderObservation(title string, record ObservationRecord) string {
	status := statusOrDefault(record.Status)
	deploymentStatus := statusOrDefault(record.DeploymentStatus)
	var b strings.Builder
	fmt.Fprintf(&b, "# Observation: %s\n\n", title)
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", nonEmpty(record.Summary, "No post-release observation recorded yet."))
	fmt.Fprintf(&b, "## Deployment Evidence\n\n%s\n", metadataLine("Provider", record.Provider))
	fmt.Fprintf(&b, "%s\n", metadataLine("Repository", record.Repo))
	fmt.Fprintf(&b, "%s\n", metadataLine("Run ID", record.RunID))
	if strings.TrimSpace(record.RunURL) != "" {
		fmt.Fprintf(&b, "Run URL: %s\n", record.RunURL)
	}
	fmt.Fprintf(&b, "%s\n\n", metadataLine("Deployment Status", string(deploymentStatus)))
	b.WriteString("## Provider Checks\n\n")
	if len(record.Checks) == 0 {
		fmt.Fprintf(&b, "%s\n\n", metadataLine("Status", string(status)))
	} else {
		for _, check := range record.Checks {
			if strings.TrimSpace(check.Name) == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s\n", metadataLine("Name", check.Name))
			fmt.Fprintf(&b, "  %s\n", metadataLine("Status", string(statusOrDefault(check.Status))))
			if strings.TrimSpace(check.URL) != "" {
				fmt.Fprintf(&b, "  URL: %s\n", check.URL)
			}
			if check.ActualStatus != 0 {
				fmt.Fprintf(&b, "  Actual Status: `%d`\n", check.ActualStatus)
			}
			if strings.TrimSpace(check.Summary) != "" {
				fmt.Fprintf(&b, "  Summary: %s\n", check.Summary)
			}
			if strings.TrimSpace(check.ResponseExcerpt) != "" {
				fmt.Fprintf(&b, "  Response Excerpt: `%s`\n", check.ResponseExcerpt)
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "## Result\n\n%s\n\n", nonEmpty(record.Summary, "No result recorded."))
	b.WriteString("## Blocking Findings\n\n")
	for _, finding := range record.BlockingFindings {
		if strings.TrimSpace(finding) == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", finding)
	}
	b.WriteString("\n## Evidence Links\n\n")
	for _, link := range record.EvidenceLinks {
		if strings.TrimSpace(link) == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", link)
	}
	if !record.ObservedAt.IsZero() {
		fmt.Fprintf(&b, "\n%s\n", metadataLine("Observed At", record.ObservedAt.Format(time.RFC3339)))
	}
	return b.String()
}

func RenderRollback(title string, record RollbackRecord) string {
	decision := decisionOrDefault(record.Decision)
	var b strings.Builder
	fmt.Fprintf(&b, "# Rollback: %s\n\n", title)
	fmt.Fprintf(&b, "## Trigger\n\n%s\n\n", nonEmpty(record.Trigger, "No rollback trigger recorded yet."))
	fmt.Fprintf(&b, "## Impact\n\n%s\n\n", nonEmpty(record.Impact, ""))
	fmt.Fprintf(&b, "## Recommended Action\n\n%s\n\n", nonEmpty(record.Recommended, ""))
	fmt.Fprintf(&b, "## Manual Decision\n\n%s\n", metadataLine("Decision", string(decision)))
	if strings.TrimSpace(record.DecisionBy) != "" {
		fmt.Fprintf(&b, "%s\n", metadataLine("By", record.DecisionBy))
	}
	b.WriteString("\n## Decision Evidence\n\n")
	if strings.TrimSpace(record.DecisionNotes) != "" {
		fmt.Fprintf(&b, "%s\n", record.DecisionNotes)
	}
	if !record.RecordedAt.IsZero() {
		fmt.Fprintf(&b, "%s\n", metadataLine("Recorded At", record.RecordedAt.Format(time.RFC3339)))
	}
	return b.String()
}

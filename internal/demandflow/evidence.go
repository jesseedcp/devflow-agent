package demandflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	evidenceadapter "github.com/jesseedcp/devflow-agent/internal/evidence"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type EvidenceRecord struct {
	Type      string
	Criterion string
	Status    string
	Summary   string
	Link      string
	By        string
}

type AddEvidenceOptions struct {
	Root           string
	DemandID       string
	Type           string
	Criterion      string
	Status         string
	Summary        string
	Link           string
	By             string
	Source         string
	Method         string
	URL            string
	ExpectedStatus string
	ActualStatus   string
	ExpectContains string
	Now            func() time.Time
}

type AddManualEvidenceOptions = AddEvidenceOptions

func AddManualEvidence(opts AddManualEvidenceOptions) (EvidenceRecord, error) {
	return AddEvidence(opts)
}

func AddEvidence(opts AddEvidenceOptions) (EvidenceRecord, error) {
	record, err := normalizeEvidenceRecord(opts)
	if err != nil {
		return EvidenceRecord{}, err
	}
	source := strings.Join(strings.Fields(opts.Source), " ")
	if source == "" {
		source = "manual"
	}
	store := artifacts.NewStore(opts.Root)
	err = store.WithDemandLock(recordDemandID(opts.DemandID), func() error {
		demand, err := store.LoadDemand(recordDemandID(opts.DemandID))
		if err != nil {
			return err
		}
		if workflow.State(demand.State) != workflow.Verification {
			return fmt.Errorf("evidence add requires current state %s, got %s", workflow.Verification, demand.State)
		}
		if err := store.AppendToArtifact(demand.ID, artifacts.VerificationFile, renderEvidence(record)); err != nil {
			return err
		}
		now := time.Now().UTC()
		if opts.Now != nil {
			now = opts.Now().UTC()
		}
		return store.AppendEvent(demand.ID, artifacts.Event{
			Time:    now,
			Type:    "verification.evidence_recorded",
			Message: "verification evidence recorded",
			Data: map[string]string{
				"type":            record.Type,
				"criterion":       record.Criterion,
				"status":          record.Status,
				"summary":         record.Summary,
				"link":            record.Link,
				"by":              record.By,
				"source":          source,
				"method":          strings.ToUpper(strings.TrimSpace(opts.Method)),
				"url":             evidenceadapter.Redact(strings.TrimSpace(opts.URL)),
				"expected_status": strings.TrimSpace(opts.ExpectedStatus),
				"actual_status":   strings.TrimSpace(opts.ActualStatus),
				"expect_contains": evidenceadapter.Redact(strings.TrimSpace(opts.ExpectContains)),
				"evidence_file":   artifacts.VerificationFile,
			},
		})
	})
	if err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func ListManualEvidence(root, demandID string) ([]EvidenceRecord, error) {
	return ListEvidence(root, demandID)
}

func ListEvidence(root, demandID string) ([]EvidenceRecord, error) {
	store := artifacts.NewStore(root)
	events, err := store.ReadEvents(strings.TrimSpace(demandID))
	if err != nil {
		return nil, err
	}
	var out []EvidenceRecord
	for _, event := range events {
		if event.Type != "verification.evidence_recorded" {
			continue
		}
		out = append(out, EvidenceRecord{
			Type:      event.Data["type"],
			Criterion: event.Data["criterion"],
			Status:    normalizeEvidenceStatus(event.Data["status"]),
			Summary:   event.Data["summary"],
			Link:      event.Data["link"],
			By:        event.Data["by"],
		})
	}
	return out, nil
}

func normalizeEvidenceRecord(opts AddEvidenceOptions) (EvidenceRecord, error) {
	record := EvidenceRecord{
		Type:      normalizeEvidenceType(opts.Type),
		Criterion: evidenceadapter.Redact(strings.Join(strings.Fields(opts.Criterion), " ")),
		Status:    normalizeEvidenceStatus(opts.Status),
		Summary:   evidenceadapter.Redact(strings.Join(strings.Fields(opts.Summary), " ")),
		Link:      evidenceadapter.Redact(strings.TrimSpace(opts.Link)),
		By:        evidenceadapter.Redact(strings.Join(strings.Fields(opts.By), " ")),
	}
	if record.Type == "" {
		return EvidenceRecord{}, fmt.Errorf("--type must be one of api, log, monitor, manual, link")
	}
	if record.Status == "" {
		return EvidenceRecord{}, fmt.Errorf("--status must be one of pass, fail, blocked")
	}
	if record.Criterion == "" || record.Summary == "" {
		return EvidenceRecord{}, fmt.Errorf("--criterion and --summary are required")
	}
	return record, nil
}

func normalizeEvidenceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "api", "log", "monitor", "manual", "link":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeEvidenceStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "pass", "passed", "success", "ok":
		return "pass"
	case "fail", "failed", "failure", "error":
		return "fail"
	case "blocked", "blocker":
		return "blocked"
	default:
		return ""
	}
}

func renderManualEvidence(record EvidenceRecord) string {
	return renderEvidence(record)
}

func renderEvidence(record EvidenceRecord) string {
	var builder strings.Builder
	builder.WriteString("\n## Acceptance Evidence\n\n")
	fmt.Fprintf(&builder, "- [%s] %s - %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
	fmt.Fprintf(&builder, "  Summary: %s\n", record.Summary)
	if record.Link != "" {
		fmt.Fprintf(&builder, "  Link: %s\n", record.Link)
	}
	if record.By != "" {
		fmt.Fprintf(&builder, "  By: %s\n", record.By)
	}
	return builder.String()
}

func recordDemandID(value string) string {
	return strings.TrimSpace(value)
}

func summarizeEvidence(events []artifacts.Event) EvidenceSummary {
	var summary EvidenceSummary
	for _, event := range events {
		if event.Type != "verification.evidence_recorded" {
			continue
		}
		record := EvidenceRecord{
			Type:      event.Data["type"],
			Criterion: event.Data["criterion"],
			Status:    normalizeEvidenceStatus(event.Data["status"]),
			Summary:   event.Data["summary"],
			Link:      event.Data["link"],
			By:        event.Data["by"],
		}
		switch record.Status {
		case "pass":
			summary.Pass++
		case "fail":
			summary.Fail++
		case "blocked":
			summary.Blocked++
		}
		summary.Latest = append(summary.Latest, record)
	}
	if len(summary.Latest) > 3 {
		summary.Latest = summary.Latest[len(summary.Latest)-3:]
	}
	return summary
}

package demandflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
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

type AddManualEvidenceOptions struct {
	Root      string
	DemandID  string
	Type      string
	Criterion string
	Status    string
	Summary   string
	Link      string
	By        string
	Now       func() time.Time
}

func AddManualEvidence(opts AddManualEvidenceOptions) (EvidenceRecord, error) {
	record, err := normalizeEvidenceRecord(opts)
	if err != nil {
		return EvidenceRecord{}, err
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
		if err := store.AppendToArtifact(demand.ID, artifacts.VerificationFile, renderManualEvidence(record)); err != nil {
			return err
		}
		now := time.Now().UTC()
		if opts.Now != nil {
			now = opts.Now().UTC()
		}
		return store.AppendEvent(demand.ID, artifacts.Event{
			Time:    now,
			Type:    "verification.evidence_recorded",
			Message: "manual verification evidence recorded",
			Data: map[string]string{
				"type":          record.Type,
				"criterion":     record.Criterion,
				"status":        record.Status,
				"summary":       record.Summary,
				"link":          record.Link,
				"by":            record.By,
				"evidence_file": artifacts.VerificationFile,
			},
		})
	})
	if err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func ListManualEvidence(root, demandID string) ([]EvidenceRecord, error) {
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

func normalizeEvidenceRecord(opts AddManualEvidenceOptions) (EvidenceRecord, error) {
	record := EvidenceRecord{
		Type:      normalizeEvidenceType(opts.Type),
		Criterion: strings.Join(strings.Fields(opts.Criterion), " "),
		Status:    normalizeEvidenceStatus(opts.Status),
		Summary:   strings.Join(strings.Fields(opts.Summary), " "),
		Link:      strings.TrimSpace(opts.Link),
		By:        strings.Join(strings.Fields(opts.By), " "),
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
	var builder strings.Builder
	builder.WriteString("\n## Manual Acceptance Evidence\n\n")
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

func summarizeManualEvidence(events []artifacts.Event) EvidenceSummary {
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

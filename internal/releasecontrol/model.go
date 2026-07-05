package releasecontrol

import "time"

type Status string

const (
	StatusNotStarted Status = "not_started"
	StatusPending    Status = "pending"
	StatusPassed     Status = "passed"
	StatusFailed     Status = "failed"
	StatusBlocked    Status = "blocked"
	StatusUnknown    Status = "unknown"
)

type RollbackDecision string

const (
	RollbackPending          RollbackDecision = "pending"
	RollbackConfirmed        RollbackDecision = "rollback_confirmed"
	RollbackRiskAccepted     RollbackDecision = "risk_accepted"
	RollbackRedeployRequired RollbackDecision = "redeploy_required"
)

type DeploymentRecord struct {
	Provider    string
	Repo        string
	WorkflowID  string
	Ref         string
	Environment string
	RunID       string
	RunURL      string
	HeadSHA     string
	Status      Status
	Conclusion  string
	Summary     string
	TriggeredBy string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ObservationRecord struct {
	Provider         string
	Repo             string
	RunID            string
	RunURL           string
	DeploymentStatus Status
	Status           Status
	Summary          string
	BlockingFindings []string
	EvidenceLinks    []string
	ObservedAt       time.Time
}

type RollbackRecord struct {
	Trigger       string
	Impact        string
	Recommended   string
	Decision      RollbackDecision
	DecisionBy    string
	DecisionNotes string
	RecordedAt    time.Time
}

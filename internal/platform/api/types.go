package api

import "time"

// HealthResponse is returned by GET /api/health.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// CurrentUser is the dev-mode identity exposed to the frontend so role-aware
// views can disable actions the caller is not permitted to perform.
type CurrentUser struct {
	Email       string `json:"email"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

// Workspace is the API contract for a platform workspace.
type Workspace struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ArtifactRoot string    `json:"artifact_root"`
	CreatedAt    time.Time `json:"created_at"`
}

// ArtifactSummary describes a single .devflow artifact file by name and
// presence, without parsing its contents.
type ArtifactSummary struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
}

// DemandSummary is the list-view contract for a demand workspace.
type DemandSummary struct {
	DemandKey string            `json:"demand_key"`
	Title     string            `json:"title"`
	State     string            `json:"state"`
	Attention string            `json:"attention"`
	UpdatedAt time.Time         `json:"updated_at"`
	Artifacts []ArtifactSummary `json:"artifacts"`
}

// EvidenceSummary counts acceptance evidence outcomes for a demand.
type EvidenceSummary struct {
	Pass    int `json:"pass"`
	Fail    int `json:"fail"`
	Blocked int `json:"blocked"`
}

// ReleaseSummary is the release-control snapshot for a demand.
type ReleaseSummary struct {
	DeploymentStatus  string `json:"deployment_status"`
	ObservationStatus string `json:"observation_status"`
	RollbackDecision  string `json:"rollback_decision"`
	RunURL            string `json:"run_url"`
}

// DemandDetail is the single-demand view, richer than the list summary.
type DemandDetail struct {
	DemandKey   string            `json:"demand_key"`
	Title       string            `json:"title"`
	State       string            `json:"state"`
	Attention   string            `json:"attention"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	Artifacts   []ArtifactSummary `json:"artifacts"`
	Evidence    EvidenceSummary   `json:"evidence"`
	Release     ReleaseSummary    `json:"release"`
	Quality     QualitySummary    `json:"quality"`
	NextActions []NextAction      `json:"next_actions"`
}

// AuditEvent is the API contract for a compliance audit record. Metadata is
// parsed from the stored JSON so the frontend receives an object, not a string.
type AuditEvent struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ActorUserID string    `json:"actor_user_id"`
	Action      string    `json:"action"`
	SubjectType string    `json:"subject_type"`
	SubjectID   string    `json:"subject_id"`
	Metadata    any       `json:"metadata,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateDemandRequest is the body for POST /api/workspaces/{id}/demands.
type CreateDemandRequest struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// ActionResult is the envelope returned by demand lifecycle action endpoints.
// It carries a human-readable status plus the refreshed demand detail so the
// Web UI can update without a follow-up GET.
type ActionResult struct {
	Status    string       `json:"status"`
	Message   string       `json:"message"`
	Demand    DemandDetail `json:"demand"`
	AuditID   string       `json:"audit_id,omitempty"`
	NextState string       `json:"next_state,omitempty"`
}

// ConfirmDemandRequest is the body for the confirm endpoint.
type ConfirmDemandRequest struct {
	Stage   string `json:"stage"`
	Summary string `json:"summary"`
}

// AddEvidenceRequest is the body for the acceptance evidence endpoint. Status
// code fields are strings to match the demandflow options contract.
type AddEvidenceRequest struct {
	Type           string `json:"type"`
	Criterion      string `json:"criterion"`
	Status         string `json:"status"`
	Summary        string `json:"summary"`
	Link           string `json:"link,omitempty"`
	Source         string `json:"source,omitempty"`
	Method         string `json:"method,omitempty"`
	URL            string `json:"url,omitempty"`
	ExpectedStatus string `json:"expected_status,omitempty"`
	ActualStatus   string `json:"actual_status,omitempty"`
	ExpectContains string `json:"expect_contains,omitempty"`
}

// QualitySummary exposes per-stage quality status derived from demandflow. The
// stage summary maps stage name to status; blockers and warnings count stages
// that need attention.
type QualitySummary struct {
	StageSummary map[string]string `json:"stage_summary"`
	Blockers     int               `json:"blockers"`
	Warnings     int               `json:"warnings"`
}

// NextAction describes a recommended next step for a demand, mirroring the
// demandflow action list so the Web UI can show the same guidance as the CLI.
type NextAction struct {
	Label    string `json:"label"`
	Command  string `json:"command,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

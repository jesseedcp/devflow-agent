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
	DemandKey string            `json:"demand_key"`
	Title     string            `json:"title"`
	State     string            `json:"state"`
	Attention string            `json:"attention"`
	UpdatedAt time.Time         `json:"updated_at"`
	Artifacts []ArtifactSummary `json:"artifacts"`
	Evidence  EvidenceSummary   `json:"evidence"`
	Release   ReleaseSummary    `json:"release"`
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
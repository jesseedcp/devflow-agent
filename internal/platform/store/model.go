package store

import "time"

// Role is the platform RBAC role assigned to a user within a workspace.
type Role string

const (
	RoleViewer    Role = "Viewer"
	RoleDeveloper Role = "Developer"
	RoleReviewer  Role = "Reviewer"
	RoleAdmin     Role = "Admin"
)

// ParseRole maps a free-form string to a known Role. Unknown values fall back
// to the most restrictive role (Viewer) so a misconfigured dev identity can
// never accidentally gain elevated permissions.
func ParseRole(value string) Role {
	switch Role(value) {
	case RoleViewer, RoleDeveloper, RoleReviewer, RoleAdmin:
		return Role(value)
	default:
		return RoleViewer
	}
}

// Action is a protected platform action gated by RBAC.
type Action string

const (
	ActionReadDemand         Action = "read_demand"
	ActionAddEvidence        Action = "add_evidence"
	ActionRefreshPlanContext Action = "refresh_plan_context"
	ActionRefreshImplReview  Action = "refresh_implementation_review"
	ActionConfirmGate        Action = "confirm_gate"
	ActionPromoteWiki        Action = "promote_wiki"
	ActionRejectWiki         Action = "reject_wiki"
	ActionConfigureWorkspace Action = "configure_workspace"
	ActionTriggerDeploy      Action = "trigger_deploy"
	ActionTriggerRollback    Action = "trigger_rollback"
	ActionAcceptReleaseRisk  Action = "accept_release_risk"
)

// User is a platform identity row.
type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Workspace is a team workspace that owns an artifact root on disk.
type Workspace struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ArtifactRoot string    `json:"artifact_root"`
	CreatedAt    time.Time `json:"created_at"`
}

// WorkspaceMember links a user to a workspace with a role.
type WorkspaceMember struct {
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Role        Role      `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

// DemandIndex is the platform-side index row for a demand workspace. The
// durable artifact record stays on disk under .devflow; this row powers UI
// listing and search queries.
type DemandIndex struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	DemandKey    string    `json:"demand_key"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	Attention    string    `json:"attention"`
	ArtifactPath string    `json:"artifact_path"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AuditEvent records a protected action for compliance review.
type AuditEvent struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	ActorUserID  string    `json:"actor_user_id"`
	Action       string    `json:"action"`
	SubjectType  string    `json:"subject_type"`
	SubjectID    string    `json:"subject_id"`
	MetadataJSON string    `json:"metadata_json"`
	CreatedAt    time.Time `json:"created_at"`
}
package store

import "encoding/json"

// Audit action labels written to audit_events.action. These mirror the
// high-risk actions enumerated in the platformization spec.
const (
	AuditConfirmGate        = "confirm_gate"
	AuditTriggerDeploy      = "trigger_deploy"
	AuditTriggerRollback    = "trigger_rollback"
	AuditAcceptReleaseRisk  = "accept_release_risk"
	AuditPromoteWiki        = "promote_wiki"
	AuditRejectWiki         = "reject_wiki"
	AuditConfigureWorkspace = "configure_workspace"
)

// NewAuditEvent builds an AuditEvent with metadata marshaled to JSON. The ID
// and CreatedAt are left zero so the store can assign them on insert.
func NewAuditEvent(workspaceID, actorUserID, action, subjectType, subjectID string, metadata map[string]any) AuditEvent {
	metadataJSON := ""
	if metadata != nil {
		encoded, err := json.Marshal(metadata)
		if err == nil {
			metadataJSON = string(encoded)
		}
	}
	return AuditEvent{
		WorkspaceID:  workspaceID,
		ActorUserID:  actorUserID,
		Action:       action,
		SubjectType:  subjectType,
		SubjectID:    subjectID,
		MetadataJSON: metadataJSON,
	}
}
package store

// rolePermissions is the cumulative permission map. Each role inherits the
// permissions of the more restrictive role beneath it, matching the spec:
// Viewer < Developer < Reviewer < Admin.
var rolePermissions = map[Role]map[Action]bool{
	RoleViewer: {
		ActionReadDemand: true,
	},
	RoleDeveloper: {
		ActionReadDemand:         true,
		ActionAddEvidence:        true,
		ActionRefreshPlanContext: true,
		ActionRefreshImplReview:  true,
	},
	RoleReviewer: {
		ActionReadDemand:         true,
		ActionAddEvidence:        true,
		ActionRefreshPlanContext: true,
		ActionRefreshImplReview:  true,
		ActionConfirmGate:        true,
		ActionPromoteWiki:        true,
		ActionRejectWiki:         true,
	},
	RoleAdmin: {
		ActionReadDemand:         true,
		ActionAddEvidence:        true,
		ActionRefreshPlanContext: true,
		ActionRefreshImplReview:  true,
		ActionConfirmGate:        true,
		ActionPromoteWiki:        true,
		ActionRejectWiki:         true,
		ActionConfigureWorkspace: true,
		ActionTriggerDeploy:      true,
		ActionTriggerRollback:    true,
		ActionAcceptReleaseRisk:  true,
	},
}

// Can reports whether role is permitted to perform action.
func Can(role Role, action Action) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	return perms[action]
}
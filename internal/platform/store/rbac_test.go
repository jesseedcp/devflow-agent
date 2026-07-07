package store

import "testing"

func TestRBACPermissionMatrix(t *testing.T) {
	cases := []struct {
		role   Role
		action Action
		want   bool
	}{
		{RoleViewer, ActionTriggerRollback, false},
		{RoleViewer, ActionReadDemand, true},
		{RoleDeveloper, ActionAddEvidence, true},
		{RoleDeveloper, ActionPromoteWiki, false},
		{RoleDeveloper, ActionConfigureWorkspace, false},
		{RoleReviewer, ActionPromoteWiki, true},
		{RoleReviewer, ActionRejectWiki, true},
		{RoleReviewer, ActionConfigureWorkspace, false},
		{RoleAdmin, ActionTriggerRollback, true},
		{RoleAdmin, ActionConfigureWorkspace, true},
		{RoleAdmin, ActionAcceptReleaseRisk, true},
		{RoleAdmin, ActionPromoteWiki, true},
	}
	for _, c := range cases {
		if got := Can(c.role, c.action); got != c.want {
			t.Errorf("Can(%s, %s) = %v, want %v", c.role, c.action, got, c.want)
		}
	}
}

func TestParseRoleFallsBackToViewer(t *testing.T) {
	if got := ParseRole("Superuser"); got != RoleViewer {
		t.Fatalf("unknown role must fall back to Viewer, got %s", got)
	}
	if got := ParseRole("Reviewer"); got != RoleReviewer {
		t.Fatalf("Reviewer should parse, got %s", got)
	}
}
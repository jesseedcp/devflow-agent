package releasecontrol

import "testing"

func TestParseDeploymentStatus(t *testing.T) {
	record := ParseDeployment(`# Deployment: Coupon

## Status

Status: ` + "`passed`" + `
Conclusion: ` + "`success`" + `
Run ID: ` + "`123`" + `
`)

	if record.Status != StatusPassed || record.Conclusion != "success" || record.RunID != "123" {
		t.Fatalf("record = %#v", record)
	}
}

func TestParseRollbackDecision(t *testing.T) {
	record := ParseRollback(`# Rollback: Coupon

## Manual Decision

Decision: ` + "`risk_accepted`" + `
By: ` + "`release-manager`" + `
`)

	if record.Decision != RollbackRiskAccepted || record.DecisionBy != "release-manager" {
		t.Fatalf("record = %#v", record)
	}
}

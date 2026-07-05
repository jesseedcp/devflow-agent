package metrics

import "time"

type DemandMetrics struct {
	DemandID                string
	Title                   string
	State                   string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	FirstEventAt            time.Time
	LastEventAt             time.Time
	TotalDuration           time.Duration
	HumanConfirmations      int
	ReviewReturns           int
	RequirementsReturns     int
	PlanReturns             int
	ImplementationReturns   int
	VerificationRuns        int
	VerificationPasses      int
	VerificationFailures    int
	AcceptancePasses        int
	AcceptanceFailures      int
	AcceptanceBlocked       int
	WikiCandidatesDistilled int
	WikiPromoted            int
	WikiRejected            int
	BlockedEvents           int
	CIBlocked               int
	CIPassed                int
}

func (m DemandMetrics) VerificationPassRate() float64 {
	if m.VerificationRuns == 0 {
		return 0
	}
	return float64(m.VerificationPasses) / float64(m.VerificationRuns)
}

func (m DemandMetrics) WikiDecisionRate() float64 {
	total := m.WikiPromoted + m.WikiRejected
	if m.WikiCandidatesDistilled == 0 {
		return 0
	}
	return float64(total) / float64(m.WikiCandidatesDistilled)
}

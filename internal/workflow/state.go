package workflow

import "fmt"

type State string

const (
	Created                    State = "created"
	ContextLoaded              State = "context_loaded"
	RequirementsDrafting       State = "requirements_drafting"
	RequirementsReview         State = "requirements_review"
	PlanDrafting               State = "plan_drafting"
	PlanReview                 State = "plan_review"
	Implementation             State = "implementation"
	MRReview                   State = "mr_review"
	Verification               State = "verification"
	Deployment                 State = "deployment"
	Observation                State = "observation"
	Closeout                   State = "closeout"
	Completed                  State = "completed"
	BlockedNeedUser            State = "blocked_need_user"
	BlockedNeedReleaseDecision State = "blocked_need_release_decision"
	BlockedNeedPlatform        State = "blocked_need_platform"
	FailedQualityGate          State = "failed_quality_gate"
	ReturnedToRequirements     State = "returned_to_requirements"
	ReturnedToPlan             State = "returned_to_plan"
	Cancelled                  State = "cancelled"
)

var knownStates = map[State]struct{}{
	Created:                    {},
	ContextLoaded:              {},
	RequirementsDrafting:       {},
	RequirementsReview:         {},
	PlanDrafting:               {},
	PlanReview:                 {},
	Implementation:             {},
	MRReview:                   {},
	Verification:               {},
	Deployment:                 {},
	Observation:                {},
	Closeout:                   {},
	Completed:                  {},
	BlockedNeedUser:            {},
	BlockedNeedReleaseDecision: {},
	BlockedNeedPlatform:        {},
	FailedQualityGate:          {},
	ReturnedToRequirements:     {},
	ReturnedToPlan:             {},
	Cancelled:                  {},
}

var allowedTransitions = map[State]map[State]struct{}{
	Created: {
		ContextLoaded: {},
		Cancelled:     {},
	},
	ContextLoaded: {
		RequirementsDrafting: {},
		BlockedNeedUser:      {},
		Cancelled:            {},
	},
	RequirementsDrafting: {
		RequirementsReview: {},
		BlockedNeedUser:    {},
		Cancelled:          {},
	},
	RequirementsReview: {
		PlanDrafting:         {},
		RequirementsDrafting: {},
		Cancelled:            {},
	},
	PlanDrafting: {
		PlanReview:      {},
		BlockedNeedUser: {},
		Cancelled:       {},
	},
	PlanReview: {
		Implementation: {},
		PlanDrafting:   {},
		Cancelled:      {},
	},
	Implementation: {
		MRReview:            {},
		FailedQualityGate:   {},
		BlockedNeedPlatform: {},
		Cancelled:           {},
	},
	MRReview: {
		Implementation:         {},
		Verification:           {},
		ReturnedToRequirements: {},
		ReturnedToPlan:         {},
		BlockedNeedUser:        {},
		Cancelled:              {},
	},
	Verification: {
		Deployment:        {},
		Closeout:          {},
		FailedQualityGate: {},
		BlockedNeedUser:   {},
		Cancelled:         {},
	},
	Deployment: {
		Observation:                {},
		BlockedNeedReleaseDecision: {},
		Cancelled:                  {},
	},
	Observation: {
		Closeout:                   {},
		BlockedNeedReleaseDecision: {},
		Cancelled:                  {},
	},
	BlockedNeedReleaseDecision: {
		Deployment: {},
		Closeout:   {},
		Cancelled:  {},
	},
	Closeout: {
		Completed:       {},
		BlockedNeedUser: {},
		Cancelled:       {},
	},
	ReturnedToRequirements: {
		RequirementsDrafting: {},
		Cancelled:            {},
	},
	ReturnedToPlan: {
		PlanDrafting: {},
		Cancelled:    {},
	},
	FailedQualityGate: {
		Implementation: {},
		Cancelled:      {},
	},
	// Generic blocked states do not track their source, so they only resume at the earliest safe phase.
	BlockedNeedUser: {
		RequirementsDrafting: {},
		Cancelled:            {},
	},
	BlockedNeedPlatform: {
		Implementation: {},
		Cancelled:      {},
	},
}

func Advance(current, next State) (State, error) {
	if !isKnownState(current) || !isKnownState(next) {
		return current, fmt.Errorf("invalid workflow transition from %s to %s", current, next)
	}

	if current == next {
		return current, nil
	}

	if nextStates, ok := allowedTransitions[current]; ok {
		if _, ok := nextStates[next]; ok {
			return next, nil
		}
	}

	return current, fmt.Errorf("invalid workflow transition from %s to %s", current, next)
}

func isKnownState(state State) bool {
	_, ok := knownStates[state]
	return ok
}

func RequiresHumanConfirmation(state State) bool {
	switch state {
	case RequirementsReview, PlanReview, Verification, BlockedNeedReleaseDecision, Closeout:
		return true
	default:
		return false
	}
}

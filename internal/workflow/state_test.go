package workflow

import "testing"

func TestAdvanceHappyPath(t *testing.T) {
	t.Parallel()

	sequence := []State{
		Created,
		ContextLoaded,
		RequirementsDrafting,
		RequirementsReview,
		PlanDrafting,
		PlanReview,
		Implementation,
		MRReview,
		Verification,
		Deployment,
		Observation,
		Closeout,
		Completed,
	}

	current := sequence[0]
	for _, next := range sequence[1:] {
		advanced, err := Advance(current, next)
		if err != nil {
			t.Fatalf("advance %s -> %s returned error: %v", current, next, err)
		}
		current = advanced
	}

	if current != Completed {
		t.Fatalf("final state = %s, want %s", current, Completed)
	}
}

func TestAdvanceRejectsSkippedGate(t *testing.T) {
	t.Parallel()

	next, err := Advance(RequirementsReview, Implementation)
	if err == nil {
		t.Fatal("expected skipped gate transition to fail")
	}
	if next != RequirementsReview {
		t.Fatalf("state after invalid advance = %s, want %s", next, RequirementsReview)
	}
}

func TestReturnedStates(t *testing.T) {
	t.Parallel()

	next, err := Advance(MRReview, ReturnedToRequirements)
	if err != nil {
		t.Fatalf("advance %s -> %s returned error: %v", MRReview, ReturnedToRequirements, err)
	}
	if next != ReturnedToRequirements {
		t.Fatalf("state after advance = %s, want %s", next, ReturnedToRequirements)
	}

	next, err = Advance(ReturnedToRequirements, RequirementsDrafting)
	if err != nil {
		t.Fatalf("advance %s -> %s returned error: %v", ReturnedToRequirements, RequirementsDrafting, err)
	}
	if next != RequirementsDrafting {
		t.Fatalf("state after advance = %s, want %s", next, RequirementsDrafting)
	}
}

func TestAdvanceRejectsCrossStageRecoveryFromGenericBlockedStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		current State
		next    State
	}{
		{current: BlockedNeedUser, next: Closeout},
		{current: BlockedNeedPlatform, next: MRReview},
		{current: BlockedNeedPlatform, next: Verification},
		{current: FailedQualityGate, next: Verification},
	}

	for _, tc := range cases {
		next, err := Advance(tc.current, tc.next)
		if err == nil {
			t.Fatalf("expected advance %s -> %s to fail", tc.current, tc.next)
		}
		if next != tc.current {
			t.Fatalf("state after invalid advance = %s, want %s", next, tc.current)
		}
	}
}

func TestAdvanceAllowsConservativeRecovery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		current State
		next    State
	}{
		{current: BlockedNeedUser, next: RequirementsDrafting},
		{current: BlockedNeedPlatform, next: Implementation},
		{current: FailedQualityGate, next: Implementation},
	}

	for _, tc := range cases {
		next, err := Advance(tc.current, tc.next)
		if err != nil {
			t.Fatalf("advance %s -> %s returned error: %v", tc.current, tc.next, err)
		}
		if next != tc.next {
			t.Fatalf("state after advance = %s, want %s", next, tc.next)
		}
	}
}

func TestAdvanceRejectsUnknownStatesEvenWhenIdempotent(t *testing.T) {
	t.Parallel()

	cases := []State{
		State("bogus"),
		State(""),
	}

	for _, state := range cases {
		next, err := Advance(state, state)
		if err == nil {
			t.Fatalf("expected advance %q -> %q to fail", state, state)
		}
		if next != state {
			t.Fatalf("state after invalid advance = %q, want %q", next, state)
		}
	}
}

func TestRequiresHumanConfirmation(t *testing.T) {
	t.Parallel()

	if !RequiresHumanConfirmation(RequirementsReview) {
		t.Fatalf("RequiresHumanConfirmation(%s) = false, want true", RequirementsReview)
	}
	if !RequiresHumanConfirmation(PlanReview) {
		t.Fatalf("RequiresHumanConfirmation(%s) = false, want true", PlanReview)
	}
	if !RequiresHumanConfirmation(Verification) {
		t.Fatalf("RequiresHumanConfirmation(%s) = false, want true", Verification)
	}
	if !RequiresHumanConfirmation(Closeout) {
		t.Fatalf("RequiresHumanConfirmation(%s) = false, want true", Closeout)
	}
	if RequiresHumanConfirmation(Implementation) {
		t.Fatalf("RequiresHumanConfirmation(%s) = true, want false", Implementation)
	}
}

func TestAdvanceAllowsMRReviewBackToImplementation(t *testing.T) {
	t.Parallel()

	next, err := Advance(MRReview, Implementation)
	if err != nil {
		t.Fatalf("advance %s -> %s returned error: %v", MRReview, Implementation, err)
	}
	if next != Implementation {
		t.Fatalf("state after advance = %s, want %s", next, Implementation)
	}
}

func TestReleaseControlTransitions(t *testing.T) {
	cases := []struct {
		from State
		to   State
	}{
		{Verification, Deployment},
		{Deployment, Observation},
		{Deployment, BlockedNeedReleaseDecision},
		{Observation, Closeout},
		{Observation, BlockedNeedReleaseDecision},
		{BlockedNeedReleaseDecision, Deployment},
		{BlockedNeedReleaseDecision, Closeout},
	}
	for _, tc := range cases {
		t.Run(string(tc.from)+"_to_"+string(tc.to), func(t *testing.T) {
			got, err := Advance(tc.from, tc.to)
			if err != nil {
				t.Fatalf("Advance returned error: %v", err)
			}
			if got != tc.to {
				t.Fatalf("Advance = %s, want %s", got, tc.to)
			}
		})
	}
}

func TestReleaseDecisionRequiresHumanConfirmation(t *testing.T) {
	if !RequiresHumanConfirmation(BlockedNeedReleaseDecision) {
		t.Fatalf("%s should require human confirmation", BlockedNeedReleaseDecision)
	}
	for _, state := range []State{Deployment, Observation} {
		if RequiresHumanConfirmation(state) {
			t.Fatalf("%s should be an evidence gate, not a generic human confirmation gate", state)
		}
	}
}

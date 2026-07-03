package implreview

func Recommend(review Review) string {
	if len(review.OutOfScope) > 0 || len(review.MissingTests) > 0 {
		return "needs_scope_review"
	}
	if review.VerificationStatus != "pass" {
		return "needs_verification"
	}
	if review.AcceptanceFail > 0 || review.AcceptanceBlocked > 0 || review.AcceptancePass == 0 {
		return "needs_acceptance_evidence"
	}
	if review.MRStatus == "action_required" {
		return "needs_review_resolution"
	}
	return "ready_for_closeout"
}

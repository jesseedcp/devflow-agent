package implreview

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/scope"
)

func Collect(root, demandID string, changedFiles []string) (Review, error) {
	store := artifacts.NewStore(root)
	if _, err := store.LoadDemand(demandID); err != nil {
		return Review{}, err
	}
	demandDir := store.DemandDir(demandID)
	scopeText, err := os.ReadFile(filepath.Join(demandDir, artifacts.ChangeScopeFile))
	if err != nil {
		return Review{}, err
	}
	decl := scope.ParseDeclaration(string(scopeText))
	diff := scope.CompareChangedFiles(decl, changedFiles)
	events, err := store.ReadEvents(demandID)
	if err != nil {
		return Review{}, err
	}
	review := Review{
		DemandID:       demandID,
		DeclaredSource: decl.SourceFiles,
		DeclaredTests:  decl.TestFiles,
		ChangedFiles:   changedFiles,
		InScope:        diff.InScope,
		OutOfScope:     diff.OutOfScope,
		MissingTests:   diff.MissingTests,
		MRStatus:       "not_checked",
	}
	for _, event := range events {
		switch event.Type {
		case "verification.recorded":
			review.VerificationStatus = normalizeStatus(event.Data["status"])
			review.VerificationCommand = event.Data["command"]
		case "verification.evidence_recorded":
			switch normalizeStatus(event.Data["status"]) {
			case "pass":
				review.AcceptancePass++
			case "fail":
				review.AcceptanceFail++
			case "blocked":
				review.AcceptanceBlocked++
			}
		case "mr_review.cleared":
			review.MRStatus = "cleared"
		case "mr_review.action_required":
			review.MRStatus = "action_required"
		}
	}
	review.Recommendation = Recommend(review)
	return review, nil
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass", "passed", "success", "ok":
		return "pass"
	case "fail", "failed", "failure", "error":
		return "fail"
	case "blocked", "blocker":
		return "blocked"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

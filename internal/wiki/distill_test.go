package wiki

import (
	"strings"
	"testing"
)

func TestDistillBusinessCandidatesFromChineseAndEnglishHeadings(t *testing.T) {
	memory := "# Memory Candidates: demo\n\n" +
		"## 稳定知识候选\n\n" +
		"- Active membership must be checked before coupon discount rules.\n\n" +
		"## Stable Business Knowledge\n\n" +
		"- Coupons expire at end of billing cycle.\n\n" +
		"## 不进入长期知识的材料\n\n- throwaway note\n"
	result := Distill(DistillInput{Title: "demo", MemoryCandidates: memory})
	business := filterByKind(result.Candidates, KindBusiness)
	if len(business) != 2 {
		t.Fatalf("expected 2 business candidates, got %d", len(business))
	}
	if business[0].Text != "Active membership must be checked before coupon discount rules." {
		t.Fatalf("first business text = %q", business[0].Text)
	}
	if business[0].Source != "memory-candidates.md" {
		t.Fatalf("first business source = %q", business[0].Source)
	}
	if business[1].Text != "Coupons expire at end of billing cycle." {
		t.Fatalf("second business text = %q", business[1].Text)
	}
	if business[0].Index != 1 || business[1].Index != 2 {
		t.Fatalf("indexes = %d, %d", business[0].Index, business[1].Index)
	}
}

func TestDistillProcessCandidateFromImplementationReview(t *testing.T) {
	implReview := "# Implementation Review: demo\n\nRecommendation: `needs_implementation_diff`\n\n"
	result := Distill(DistillInput{Title: "demo", ImplementationReview: implReview})
	process := filterByKind(result.Candidates, KindProcess)
	if len(process) != 1 {
		t.Fatalf("expected 1 process candidate, got %d", len(process))
	}
	if process[0].Text != "Implementation review recommended needs_implementation_diff." {
		t.Fatalf("process text = %q", process[0].Text)
	}
	if process[0].Source != "implementation-review.md" {
		t.Fatalf("process source = %q", process[0].Source)
	}
}

func TestDistillIgnoresReadyForCloseoutRecommendation(t *testing.T) {
	implReview := "# Implementation Review: demo\n\nRecommendation: `ready_for_closeout`\n\n"
	result := Distill(DistillInput{Title: "demo", ImplementationReview: implReview})
	process := filterByKind(result.Candidates, KindProcess)
	if len(process) != 0 {
		t.Fatalf("expected 0 process candidates for ready_for_closeout, got %d", len(process))
	}
}

func TestDistillProcessCandidateFromMRActionEvent(t *testing.T) {
	events := []EventInput{
		{Type: "mr_review.action_required", Message: "unresolved review comments", Data: map[string]string{"next_state": "returned_to_plan"}},
	}
	result := Distill(DistillInput{Title: "demo", Events: events})
	process := filterByKind(result.Candidates, KindProcess)
	if len(process) != 1 {
		t.Fatalf("expected 1 process candidate, got %d", len(process))
	}
	if process[0].Text != "unresolved review comments" {
		t.Fatalf("process text = %q", process[0].Text)
	}
}

func TestDistillArchiveCandidateWhenMaterialExists(t *testing.T) {
	result := Distill(DistillInput{
		Title:    "demo",
		Closeout: "# Closeout: demo\n\n## 需求结果\n\ndelivered on time\n\n## 稳定知识候选\n\n- membership gates coupons\n",
	})
	archive := filterByKind(result.Candidates, KindArchive)
	if len(archive) != 1 {
		t.Fatalf("expected 1 archive candidate, got %d", len(archive))
	}
	if archive[0].Text != "Closeout raw material archived in closeout-raw-log.md." {
		t.Fatalf("archive text = %q", archive[0].Text)
	}
	if archive[0].Source != "closeout.md" {
		t.Fatalf("archive source = %q", archive[0].Source)
	}
}

func TestDistillArchiveCandidateFromEvents(t *testing.T) {
	result := Distill(DistillInput{
		Title:  "demo",
		Events: []EventInput{{Type: "verification.recorded", Message: "ran tests"}},
	})
	archive := filterByKind(result.Candidates, KindArchive)
	if len(archive) != 1 {
		t.Fatalf("expected 1 archive candidate from events, got %d", len(archive))
	}
}

func TestDistillNoArchiveCandidateForFreshDemand(t *testing.T) {
	result := Distill(DistillInput{
		Title:   "demo",
		Events:  []EventInput{{Type: "demand.created", Message: "demand workspace created"}},
		Closeout: "# Closeout: demo\n\n## 需求结果\n\n",
	})
	archive := filterByKind(result.Candidates, KindArchive)
	if len(archive) != 0 {
		t.Fatalf("expected 0 archive candidates for fresh demand, got %d", len(archive))
	}
}

func TestDistillPreservesEmptyTemplates(t *testing.T) {
	result := Distill(DistillInput{Title: "demo"})
	rendered := RenderCandidates("demo", result.Candidates)
	for _, want := range []string{
		"No stable business knowledge candidates distilled yet.",
		"No process improvement candidates distilled yet.",
		"No archive-only material distilled yet.",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered candidates missing %q:\n%s", want, rendered)
		}
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("expected 0 candidates for empty input, got %d", len(result.Candidates))
	}
}

func TestDistillCloseoutRawLogSections(t *testing.T) {
	result := Distill(DistillInput{
		Title:                "demo",
		Closeout:             "# Closeout: demo\n\ndelivered\n",
		MemoryCandidates:     "# Memory Candidates: demo\n\n- fact\n",
		ImplementationReview: "# Implementation Review: demo\n\nRecommendation: `needs_verification`\n\n",
		Events: []EventInput{
			{Type: "verification.recorded", Message: "ran tests"},
		},
	})
	log := result.CloseoutRawLog
	for _, want := range []string{"# Closeout Raw Log: demo", "## Closeout", "delivered", "## Memory Candidates", "- fact", "## Implementation Review", "Recommendation: `needs_verification`", "## Review And Verification Events", "- verification.recorded: ran tests"} {
		if !strings.Contains(log, want) {
			t.Fatalf("closeout raw log missing %q:\n%s", want, log)
		}
	}
}

func TestDistillCloseoutRawLogEmptyPlaceholders(t *testing.T) {
	result := Distill(DistillInput{Title: "demo"})
	log := result.CloseoutRawLog
	for _, want := range []string{"No closeout material captured yet.", "No memory candidate material captured yet.", "No implementation review captured yet.", "No events captured yet."} {
		if !strings.Contains(log, want) {
			t.Fatalf("closeout raw log missing placeholder %q:\n%s", want, log)
		}
	}
}

func filterByKind(candidates []Candidate, kind CandidateKind) []Candidate {
	var out []Candidate
	for _, c := range candidates {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}
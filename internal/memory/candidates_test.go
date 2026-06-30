package memory

import (
	"reflect"
	"testing"
)

func TestParseCandidatesFromChineseStableKnowledgeSection(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates: Coupon

## 稳定知识候选

- Active membership must be checked before coupon discount rules.
- Coupon errors should preserve the original order validation message.

## 流程改进候选

- Keep review comments grouped by category.
`

	got := ParseCandidates(input)
	want := []Candidate{
		{Index: 1, Text: "Active membership must be checked before coupon discount rules.", Status: CandidatePending},
		{Index: 2, Text: "Coupon errors should preserve the original order validation message.", Status: CandidatePending},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCandidates() = %#v, want %#v", got, want)
	}
}

func TestParseCandidatesFromEnglishSection(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates

## Stable Knowledge Candidates

- Persist merge request routing decisions as progress evidence.
`
	got := ParseCandidates(input)
	want := []Candidate{{Index: 1, Text: "Persist merge request routing decisions as progress evidence.", Status: CandidatePending}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCandidates() = %#v, want %#v", got, want)
	}
}

func TestParseCandidatesFallsBackToTopLevelBullets(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates

- Stable fallback one.
  - nested detail should not become its own candidate.
- Stable fallback two.
`
	got := ParseCandidates(input)
	want := []Candidate{
		{Index: 1, Text: "Stable fallback one.", Status: CandidatePending},
		{Index: 2, Text: "Stable fallback two.", Status: CandidatePending},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCandidates() = %#v, want %#v", got, want)
	}
}

func TestParseCandidatesReturnsEmptyForTemplateOnlyFile(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates: Empty

## 稳定知识候选

## 流程改进候选

- This process item is not a stable knowledge candidate.
`
	if got := ParseCandidates(input); len(got) != 0 {
		t.Fatalf("ParseCandidates() = %#v, want empty", got)
	}
}

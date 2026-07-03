package wiki

import "testing"

func TestParseCandidatesThreeKinds(t *testing.T) {
	text := `# Wiki Candidates: Test

## Stable Business Knowledge

- Active membership gates coupons. (source: memory-candidates.md)

## Process Improvement Candidates

- Implementation review needs work. (source: implementation-review.md)

## Archive Only

- Closeout archived. (source: closeout.md)
`
	cs := ParseCandidates(text)
	if len(cs) != 3 {
		t.Fatalf("got %d candidates, want 3", len(cs))
	}
	if cs[0].Index != 1 || cs[0].Kind != KindBusiness || cs[0].Text != "Active membership gates coupons." || cs[0].Source != "memory-candidates.md" {
		t.Fatalf("candidate 0 = %#v", cs[0])
	}
	if cs[1].Index != 2 || cs[1].Kind != KindProcess {
		t.Fatalf("candidate 1 = %#v", cs[1])
	}
	if cs[2].Index != 3 || cs[2].Kind != KindArchive {
		t.Fatalf("candidate 2 = %#v", cs[2])
	}
}

func TestParseCandidatesIgnoresTemplateLines(t *testing.T) {
	text := `# Wiki Candidates: Test

## Stable Business Knowledge

No stable business knowledge candidates distilled yet.

## Process Improvement Candidates

No process improvement candidates distilled yet.

## Archive Only

No archive-only material distilled yet.
`
	if cs := ParseCandidates(text); len(cs) != 0 {
		t.Fatalf("got %d candidates, want 0", len(cs))
	}
}

func TestParseCandidatesPreservesStatus(t *testing.T) {
	text := `# Wiki Candidates: Test

## Stable Business Knowledge

- Promoted fact. (source: x) [promoted: .devflow/wiki/foo.md]

## Process Improvement Candidates

- Rejected idea. (source: y) [rejected: too narrow]
`
	cs := ParseCandidates(text)
	if len(cs) != 2 {
		t.Fatalf("got %d candidates, want 2", len(cs))
	}
	if cs[0].Status != StatusPromoted || cs[0].WikiPath != ".devflow/wiki/foo.md" {
		t.Fatalf("candidate 0 = %#v", cs[0])
	}
	if cs[1].Status != StatusRejected || cs[1].Reason != "too narrow" {
		t.Fatalf("candidate 1 = %#v", cs[1])
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	cs := []Candidate{
		{Index: 1, Kind: KindBusiness, Text: "Fact one.", Source: "a.md", Status: StatusPending},
		{Index: 2, Kind: KindProcess, Text: "Idea two.", Source: "b.md", Status: StatusPromoted, WikiPath: ".devflow/wiki/two.md"},
	}
	rendered := RenderCandidates("Round", cs)
	parsed := ParseCandidates(rendered)
	if len(parsed) != 2 {
		t.Fatalf("round trip got %d, want 2", len(parsed))
	}
	if parsed[1].Status != StatusPromoted || parsed[1].WikiPath != ".devflow/wiki/two.md" {
		t.Fatalf("round trip candidate 1 = %#v", parsed[1])
	}
}
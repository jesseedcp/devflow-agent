package wiki

import (
	"fmt"
	"strings"
)

func RenderCandidates(title string, candidates []Candidate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Wiki Candidates: %s\n\n", title)
	writeCandidateSection(&b, "## Stable Business Knowledge", "No stable business knowledge candidates distilled yet.", KindBusiness, candidates)
	writeCandidateSection(&b, "## Process Improvement Candidates", "No process improvement candidates distilled yet.", KindProcess, candidates)
	writeCandidateSection(&b, "## Archive Only", "No archive-only material distilled yet.", KindArchive, candidates)
	return b.String()
}

func writeCandidateSection(b *strings.Builder, heading, emptyLine string, kind CandidateKind, candidates []Candidate) {
	fmt.Fprintf(b, "%s\n\n", heading)
	hasAny := false
	for _, c := range candidates {
		if c.Kind != kind {
			continue
		}
		hasAny = true
		fmt.Fprintf(b, "- %s", c.Text)
		if c.Source != "" {
			fmt.Fprintf(b, " (source: %s)", c.Source)
		}
		if c.Status == StatusPromoted && c.WikiPath != "" {
			fmt.Fprintf(b, " [promoted: %s]", c.WikiPath)
		} else if c.Status == StatusRejected && c.Reason != "" {
			fmt.Fprintf(b, " [rejected: %s]", c.Reason)
		}
		fmt.Fprintln(b)
	}
	if !hasAny {
		fmt.Fprintf(b, "%s\n", emptyLine)
	}
	fmt.Fprintln(b)
}
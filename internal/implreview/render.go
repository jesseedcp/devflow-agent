package implreview

import (
	"fmt"
	"strings"
)

func Render(review Review) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Implementation Review: %s\n\n", review.DemandID)
	fmt.Fprintf(&b, "Recommendation: `%s`\n\n", review.Recommendation)
	fmt.Fprintf(&b, "Summary: in_scope=%d out_of_scope=%d missing_tests=%d verification=%s acceptance_pass=%d acceptance_fail=%d acceptance_blocked=%d mr=%s\n\n",
		len(review.InScope), len(review.OutOfScope), len(review.MissingTests), review.VerificationStatus, review.AcceptancePass, review.AcceptanceFail, review.AcceptanceBlocked, review.MRStatus)
	writeSection(&b, "Declared Source", review.DeclaredSource)
	writeSection(&b, "Declared Tests", review.DeclaredTests)
	writeSection(&b, "Changed Files", review.ChangedFiles)
	writeSection(&b, "Out Of Scope Changes", review.OutOfScope)
	writeSection(&b, "Missing Declared Tests", review.MissingTests)
	b.WriteString("## Verification\n\n")
	fmt.Fprintf(&b, "- status: `%s`\n", review.VerificationStatus)
	fmt.Fprintf(&b, "- command: `%s`\n", review.VerificationCommand)
	b.WriteString("\n## Acceptance Evidence\n\n")
	fmt.Fprintf(&b, "- pass=%d fail=%d blocked=%d\n", review.AcceptancePass, review.AcceptanceFail, review.AcceptanceBlocked)
	b.WriteString("\n## MR Review\n\n")
	fmt.Fprintf(&b, "- status: `%s`\n", review.MRStatus)
	return b.String()
}

func writeSection(builder *strings.Builder, title string, values []string) {
	fmt.Fprintf(builder, "## %s\n\n", title)
	if len(values) == 0 {
		builder.WriteString("- none\n\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(builder, "- `%s`\n", value)
	}
	builder.WriteString("\n")
}

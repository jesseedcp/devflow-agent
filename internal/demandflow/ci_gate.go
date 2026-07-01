package demandflow

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func renderCIGateEvidence(result adapters.CIResult) string {
	var b strings.Builder
	b.WriteString("## CI Gate\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", result.Provider)
	fmt.Fprintf(&b, "Target: %s#%s\n", result.Repo, result.PR)
	if strings.TrimSpace(result.HeadSHA) != "" {
		fmt.Fprintf(&b, "Head SHA: %s\n", result.HeadSHA)
	}
	fmt.Fprintf(&b, "Status: %s\n\n", result.Status)
	if len(result.Checks) == 0 {
		b.WriteString("- no check runs found\n\n")
		return b.String()
	}
	for _, check := range result.Checks {
		name := strings.TrimSpace(check.Name)
		if name == "" {
			name = "(unnamed check)"
		}
		fmt.Fprintf(&b, "- %s: status=%s conclusion=%s", name, check.Status, check.Conclusion)
		if strings.TrimSpace(check.URL) != "" {
			fmt.Fprintf(&b, " url=%s", check.URL)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

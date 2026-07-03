package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func WikiRoot(root string) string {
	return filepath.Join(root, ".devflow", "wiki")
}

func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("wiki name is empty")
	}
	if filepath.IsAbs(slug) {
		return fmt.Errorf("wiki name %q must not be an absolute path", slug)
	}
	if strings.Contains(slug, "..") {
		return fmt.Errorf("wiki name %q must not contain ..", slug)
	}
	if strings.ContainsAny(slug, "/\\") {
		return fmt.Errorf("wiki name %q must not contain path separators", slug)
	}
	for _, r := range slug {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("wiki name %q must contain only lowercase letters, digits, hyphen, or underscore", slug)
		}
	}
	return nil
}

func Promote(root string, opts PromoteOptions, candidate Candidate) (string, error) {
	if err := ValidateSlug(opts.Name); err != nil {
		return "", err
	}
	if candidate.Index != opts.CandidateIndex {
		candidate.Index = opts.CandidateIndex
	}
	now := time.Now()
	if opts.Now != nil {
		now = opts.Now()
	}
	wikiRoot := WikiRoot(root)
	if err := os.MkdirAll(wikiRoot, 0o755); err != nil {
		return "", fmt.Errorf("create wiki directory: %w", err)
	}
	relPath := ".devflow/wiki/" + opts.Name + ".md"
	entryPath := filepath.Join(wikiRoot, opts.Name+".md")
	if err := os.WriteFile(entryPath, []byte(renderEntry(opts, candidate, now)), 0o644); err != nil {
		return "", fmt.Errorf("write wiki entry: %w", err)
	}
	if err := updateIndex(wikiRoot, opts, candidate.Kind); err != nil {
		return "", fmt.Errorf("update wiki index: %w", err)
	}
	return relPath, nil
}

func renderEntry(opts PromoteOptions, candidate Candidate, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", opts.Name)
	fmt.Fprintf(&b, "## Metadata\n\n")
	fmt.Fprintf(&b, "- kind: %s\n", candidate.Kind)
	fmt.Fprintf(&b, "- source_demand: %s\n", opts.DemandID)
	fmt.Fprintf(&b, "- promoted_by: %s\n", opts.By)
	fmt.Fprintf(&b, "- promoted_at: %s\n\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "## Knowledge\n\n")
	fmt.Fprintf(&b, "%s\n", candidate.Text)
	return b.String()
}

func updateIndex(wikiRoot string, opts PromoteOptions, kind CandidateKind) error {
	indexPath := filepath.Join(wikiRoot, "WIKI.md")
	existing := ""
	if data, err := os.ReadFile(indexPath); err == nil {
		existing = string(data)
	}
	line := fmt.Sprintf("- [%s](%s.md) - %s - %s", opts.Name, opts.Name, kind, opts.DemandID)
	if strings.Contains(existing, line) {
		return nil
	}
	var b strings.Builder
	if !strings.Contains(existing, "# Devflow Wiki") {
		b.WriteString("# Devflow Wiki\n\n## Entries\n\n")
	} else {
		b.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			b.WriteString("\n")
		}
	}
	b.WriteString(line + "\n")
	return os.WriteFile(indexPath, []byte(b.String()), 0o644)
}
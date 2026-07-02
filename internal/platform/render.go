package platform

import (
	"fmt"
	"sort"
	"strings"
)

func RenderIntakeSnapshot(snapshot IntakeSnapshot) string {
	var b strings.Builder
	title := strings.TrimSpace(snapshot.Title)
	if title == "" {
		title = string(snapshot.Kind)
	}
	fmt.Fprintf(&b, "# Intake: %s\n\n", title)
	fmt.Fprintf(&b, "Source: `%s`\n", snapshot.Kind)
	fmt.Fprintf(&b, "Provider: `%s`\n", snapshot.Provider)
	if snapshot.ExternalID != "" {
		fmt.Fprintf(&b, "External ID: `%s`\n", snapshot.ExternalID)
	}
	if snapshot.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", snapshot.URL)
	}
	if snapshot.Author != "" {
		fmt.Fprintf(&b, "Author: `%s`\n", snapshot.Author)
	}
	if !snapshot.FetchedAt.IsZero() {
		fmt.Fprintf(&b, "Fetched At: `%s`\n", snapshot.FetchedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	b.WriteString("\n")

	writeStringList(&b, "## Labels", snapshot.Labels)
	writeMetadata(&b, snapshot.Metadata)

	b.WriteString("## Body\n\n")
	body := strings.TrimSpace(snapshot.Body)
	if body == "" {
		body = "_empty body_"
	}
	b.WriteString(body)
	b.WriteString("\n\n")

	if len(snapshot.Comments) > 0 {
		b.WriteString("## Comments\n\n")
		for _, comment := range snapshot.Comments {
			created := ""
			if !comment.CreatedAt.IsZero() {
				created = " at " + comment.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			author := strings.TrimSpace(comment.Author)
			if author == "" {
				author = "unknown"
			}
			fmt.Fprintf(&b, "### %s%s\n\n", author, created)
			if comment.URL != "" {
				fmt.Fprintf(&b, "URL: %s\n\n", comment.URL)
			}
			text := strings.TrimSpace(comment.Body)
			if text == "" {
				text = "_empty comment_"
			}
			b.WriteString(text)
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

func writeStringList(b *strings.Builder, heading string, values []string) {
	if len(values) == 0 {
		return
	}
	b.WriteString(heading)
	b.WriteString("\n\n")
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			fmt.Fprintf(b, "- %s\n", strings.TrimSpace(value))
		}
	}
	b.WriteString("\n")
}

func writeMetadata(b *strings.Builder, values map[string]string) {
	if len(values) == 0 {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	b.WriteString("## Metadata\n\n")
	for _, key := range keys {
		if strings.TrimSpace(values[key]) != "" {
			fmt.Fprintf(b, "- `%s`: %s\n", key, strings.TrimSpace(values[key]))
		}
	}
	b.WriteString("\n")
}

func SyncMarker(demandID, stage string) string {
	return fmt.Sprintf("<!-- devflow-sync:demand=%s:stage=%s -->", strings.TrimSpace(demandID), strings.TrimSpace(stage))
}

func RenderProgressComment(update ProgressUpdate) string {
	marker := strings.TrimSpace(update.Marker)
	if marker == "" {
		marker = SyncMarker(update.DemandID, update.Stage)
	}
	var b strings.Builder
	b.WriteString(marker)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "## Devflow Update: %s\n\n", update.Stage)
	if update.State != "" {
		fmt.Fprintf(&b, "State: `%s`\n\n", update.State)
	}
	if strings.TrimSpace(update.Summary) != "" {
		b.WriteString("Summary:\n")
		b.WriteString(strings.TrimSpace(update.Summary))
		b.WriteString("\n\n")
	}
	if update.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", update.URL)
	}
	return b.String()
}

func RenderCloseoutComment(update CloseoutUpdate) string {
	marker := strings.TrimSpace(update.Marker)
	if marker == "" {
		marker = SyncMarker(update.DemandID, "closeout")
	}
	var b strings.Builder
	b.WriteString(marker)
	b.WriteString("\n\n")
	b.WriteString("## Devflow Closeout\n\n")
	if strings.TrimSpace(update.Result) != "" {
		b.WriteString("Result:\n")
		b.WriteString(strings.TrimSpace(update.Result))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(update.Verification) != "" {
		b.WriteString("Verification:\n")
		b.WriteString(strings.TrimSpace(update.Verification))
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(update.Knowledge) != "" {
		b.WriteString("Knowledge candidates:\n")
		b.WriteString(strings.TrimSpace(update.Knowledge))
		b.WriteString("\n\n")
	}
	if update.URL != "" {
		fmt.Fprintf(&b, "URL: %s\n", update.URL)
	}
	return b.String()
}

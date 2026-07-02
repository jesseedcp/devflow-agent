package platform

import (
	"strings"
	"testing"
	"time"
)

func TestRenderIntakeSnapshotIncludesSourceAndComments(t *testing.T) {
	snapshot := IntakeSnapshot{
		Provider:   ProviderGitHub,
		Kind:       SourceGitHubIssue,
		ExternalID: "owner/repo#123",
		Title:      "Coupon issue",
		Body:       "Users need coupon eligibility.",
		URL:        "https://github.com/owner/repo/issues/123",
		Author:     "alice",
		Labels:     []string{"backend", "priority-high"},
		FetchedAt:  time.Date(2026, 7, 2, 1, 2, 3, 0, time.UTC),
		Comments: []ExternalComment{{
			ID:        "c1",
			Author:    "bob",
			Body:      "Remember inactive users.",
			URL:       "https://github.com/owner/repo/issues/123#issuecomment-1",
			CreatedAt: time.Date(2026, 7, 2, 2, 3, 4, 0, time.UTC),
		}},
	}

	got := RenderIntakeSnapshot(snapshot)
	for _, want := range []string{
		"# Intake: Coupon issue",
		"Source: `github_issue`",
		"Provider: `github`",
		"External ID: `owner/repo#123`",
		"URL: https://github.com/owner/repo/issues/123",
		"## Labels",
		"- backend",
		"## Body",
		"Users need coupon eligibility.",
		"## Comments",
		"### bob at 2026-07-02T02:03:04Z",
		"Remember inactive users.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered intake missing %q:\n%s", want, got)
		}
	}
}

func TestRenderSyncMarker(t *testing.T) {
	got := SyncMarker("coupon", "verification")
	if got != "<!-- devflow-sync:demand=coupon:stage=verification -->" {
		t.Fatalf("marker = %q", got)
	}
}

func TestRenderProgressComment(t *testing.T) {
	got := RenderProgressComment(ProgressUpdate{
		DemandID: "coupon",
		Stage:    "verification",
		State:    "verification",
		Summary:  "All checks passed.",
		URL:      "https://github.com/owner/repo/pull/12",
		Marker:   SyncMarker("coupon", "verification"),
	})
	for _, want := range []string{
		"<!-- devflow-sync:demand=coupon:stage=verification -->",
		"## Devflow Update: verification",
		"State: `verification`",
		"All checks passed.",
		"https://github.com/owner/repo/pull/12",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("progress comment missing %q:\n%s", want, got)
		}
	}
}

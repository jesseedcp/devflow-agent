package version

import (
	"strings"
	"testing"
)

func TestCurrentUsesBuildVariablesAndRuntime(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		Date = originalDate
	})

	Version = "0.1.0"
	Commit = "abc123"
	Date = "2026-06-29T00:00:00Z"

	info := Current()
	if info.Version != "0.1.0" {
		t.Fatalf("Version = %q, want 0.1.0", info.Version)
	}
	if info.Commit != "abc123" {
		t.Fatalf("Commit = %q, want abc123", info.Commit)
	}
	if info.Date != "2026-06-29T00:00:00Z" {
		t.Fatalf("Date = %q, want 2026-06-29T00:00:00Z", info.Date)
	}
	if info.GoVersion == "" || info.OS == "" || info.Arch == "" {
		t.Fatalf("runtime fields missing: %#v", info)
	}
}

func TestInfoStringIsStableAndMultiline(t *testing.T) {
	info := Info{
		Version:   " 0.1.0 ",
		Commit:    "abc123",
		Date:      "2026-06-29T00:00:00Z",
		GoVersion: "go1.25.0",
		OS:        "windows",
		Arch:      "amd64",
	}

	text := info.String()
	for _, want := range []string{
		"version: 0.1.0",
		"commit: abc123",
		"date: 2026-06-29T00:00:00Z",
		"go: go1.25.0",
		"platform: windows/amd64",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("version text missing %q:\n%s", want, text)
		}
	}
}

func TestCleanFallsBackToUnknown(t *testing.T) {
	if got := clean(" \n\t "); got != "unknown" {
		t.Fatalf("clean blank = %q, want unknown", got)
	}
}

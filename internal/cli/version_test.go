package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/version"
)

func TestVersionCommandPrintsBuildMetadata(t *testing.T) {
	originalVersion := version.Version
	originalCommit := version.Commit
	originalDate := version.Date
	t.Cleanup(func() {
		version.Version = originalVersion
		version.Commit = originalCommit
		version.Date = originalDate
	})

	version.Version = "0.1.0-test"
	version.Commit = "testcommit"
	version.Date = "2026-06-29T00:00:00Z"

	var stdout bytes.Buffer
	if err := Run([]string{"version"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("version: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"version: 0.1.0-test",
		"commit: testcommit",
		"date: 2026-06-29T00:00:00Z",
		"go:",
		"platform:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("version output missing %q:\n%s", want, output)
		}
	}
}

func TestVersionHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"devflow version", "version   Show build version and platform metadata"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestEvidenceAddRecordsManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-cli", Title: "Evidence CLI", Description: "CLI evidence", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"evidence", "add",
		"--root", root,
		"--demand", demand.ID,
		"--type", "api",
		"--criterion", "Inactive users are blocked",
		"--status", "pass",
		"--summary", "POST /coupon/claim returned COUPON_USER_INACTIVE.",
		"--link", "https://example.test/log/123",
		"--by", "dd",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("evidence add returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "evidence recorded for evidence-cli: PASS api") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.VerificationFile))
	if err != nil {
		t.Fatalf("read verification: %v", err)
	}
	if !strings.Contains(string(body), "[PASS] api - Inactive users are blocked") {
		t.Fatalf("verification.md missing evidence:\n%s", string(body))
	}
}

func TestEvidenceListPrintsManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-list-cli", Title: "Evidence list CLI", Description: "List", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := Run([]string{"evidence", "add", "--root", root, "--demand", demand.ID, "--type", "manual", "--criterion", "QA accepted", "--summary", "QA signed off", "--by", "dd"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("evidence add returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evidence", "list", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evidence list returned error: %v", err)
	}
	for _, want := range []string{"Manual evidence: evidence-list-cli", "PASS manual QA accepted", "QA signed off"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("evidence list missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEvidenceRejectsUnknownSubcommand(t *testing.T) {
	err := Run([]string{"evidence", "delete", "--root", t.TempDir(), "--demand", "x"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown evidence command") {
		t.Fatalf("err = %v, want unknown evidence command", err)
	}
}

func TestHelpIncludesEvidence(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow evidence add --demand <id>", "evidence  Record and list manual verification evidence"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

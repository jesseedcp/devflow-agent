package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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
	for _, want := range []string{"Acceptance evidence: evidence-list-cli", "PASS manual QA accepted", "QA signed off"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("evidence list missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEvidenceFetchAPIRecordsPassAndRedactsSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"COUPON_USER_INACTIVE"}`))
	}))
	defer server.Close()
	root, demandID := createVerificationDemandForEvidenceCLI(t, "evidence-fetch-api")

	var stdout bytes.Buffer
	err := Run([]string{
		"evidence", "fetch",
		"--root", root,
		"--demand", demandID,
		"--type", "api",
		"--criterion", "Inactive users are blocked",
		"--method", "POST",
		"--url", server.URL + "?token=abc",
		"--header", "Authorization: Bearer secret-token",
		"--body", `{"password":"pw"}`,
		"--expect-status", "403",
		"--expect-contains", "COUPON_USER_INACTIVE",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("evidence fetch returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "evidence fetched for "+demandID+": PASS api") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	assertEvidenceCLIDoesNotLeak(t, root, demandID, []string{"secret-token", "token=abc", `"password":"pw"`})
}

func TestEvidenceFetchAPIRecordsFailAndReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	root, demandID := createVerificationDemandForEvidenceCLI(t, "evidence-fetch-fail")

	var stdout bytes.Buffer
	err := Run([]string{"evidence", "fetch", "--root", root, "--demand", demandID, "--type", "api", "--criterion", "Inactive users are blocked", "--url", server.URL, "--expect-status", "403"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "evidence fetch recorded fail") {
		t.Fatalf("err = %v, want recorded fail", err)
	}
	if !strings.Contains(stdout.String(), "evidence fetched for "+demandID+": FAIL api") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestEvidenceFetchLinkRecordsPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	root, demandID := createVerificationDemandForEvidenceCLI(t, "evidence-fetch-link")

	var stdout bytes.Buffer
	if err := Run([]string{"evidence", "fetch", "--root", root, "--demand", demandID, "--type", "link", "--criterion", "Report is reachable", "--url", server.URL, "--expect-status", "200"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evidence fetch returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "evidence fetched for "+demandID+": PASS link") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func createVerificationDemandForEvidenceCLI(t *testing.T, demandID string) (string, string) {
	t.Helper()
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: demandID, Title: demandID, Description: "Fetch evidence", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.VerificationFile, "# Verification\n\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	return root, demand.ID
}

func assertEvidenceCLIDoesNotLeak(t *testing.T, root, demandID string, forbidden []string) {
	t.Helper()
	store := artifacts.NewStore(root)
	body, err := os.ReadFile(filepath.Join(store.DemandDir(demandID), artifacts.VerificationFile))
	if err != nil {
		t.Fatalf("read verification: %v", err)
	}
	events, err := os.ReadFile(filepath.Join(store.DemandDir(demandID), "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	combined := string(body) + "\n" + string(events)
	for _, leaked := range forbidden {
		if strings.Contains(combined, leaked) {
			t.Fatalf("artifact/event leaked %q:\n%s", leaked, combined)
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
	for _, want := range []string{"devflow evidence add --demand <id>", "evidence  Record, fetch, and list acceptance evidence"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}

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

func TestIntakeFileCreatesReviewReadyDemand(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "coupon-eligibility.md")
	if err := os.WriteFile(prdPath, []byte(`# Coupon eligibility

## 目标
- Active members can claim coupons.

## 业务规则
- User status must be active.

## 验收标准
- Inactive users are blocked.
`), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"intake", "--root", root, "--file", prdPath}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v\n%s", err, stdout.String())
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand("coupon-eligibility")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("state = %q, want requirements_review", demand.State)
	}
	if demand.Source != "intake:file:"+prdPath {
		t.Fatalf("source = %q", demand.Source)
	}

	requirements, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.RequirementsFile))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	for _, want := range []string{"# Requirements: Coupon eligibility", "Active members can claim coupons.", "Inactive users are blocked."} {
		if !strings.Contains(string(requirements), want) {
			t.Fatalf("requirements missing %q:\n%s", want, string(requirements))
		}
	}

	intakeBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.IntakeFile))
	if err != nil {
		t.Fatalf("read intake: %v", err)
	}
	if !strings.Contains(string(intakeBody), "Source: `"+prdPath+"`") {
		t.Fatalf("intake snapshot missing source:\n%s", string(intakeBody))
	}
	contextBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.ContextFile))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if !strings.Contains(string(contextBody), "# Context: Coupon eligibility") {
		t.Fatalf("context missing heading:\n%s", string(contextBody))
	}
	if !strings.Contains(stdout.String(), "context: ") {
		t.Fatalf("stdout missing context path:\n%s", stdout.String())
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "context.recalled") {
		t.Fatalf("events missing context.recalled: %#v", events)
	}
	if !strings.Contains(stdout.String(), "next: devflow evaluate --demand coupon-eligibility --stage requirements --strict") {
		t.Fatalf("stdout missing next command:\n%s", stdout.String())
	}
}

func TestIntakeFileAllowsTitleAndDemandOverride(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "rough.md")
	if err := os.WriteFile(prdPath, []byte("Raw requirement body"), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"intake",
		"--root", root,
		"--file", prdPath,
		"--title", "Manual title",
		"--demand", "manual-id",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v", err)
	}

	demand, err := artifacts.NewStore(root).LoadDemand("manual-id")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.Title != "Manual title" {
		t.Fatalf("title = %q, want Manual title", demand.Title)
	}
}

func TestIntakeURLCreatesReviewReadyDemand(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html>
<head><title>Coupon URL PRD</title></head>
<body>
	<h1>Coupon URL PRD</h1>
	<h2>目标</h2>
	<p>Active members can claim URL coupons.</p>
	<h2>业务规则</h2>
	<p>User status must be active.</p>
	<h2>验收标准</h2>
	<p>Inactive users are blocked.</p>
</body>
</html>`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{"intake", "--root", root, "--url", server.URL + "/prd"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v\n%s", err, stdout.String())
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand("coupon-url-prd")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("state = %q, want requirements_review", demand.State)
	}
	if demand.Source != "intake:url:"+server.URL+"/prd" {
		t.Fatalf("source = %q", demand.Source)
	}

	requirements, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.RequirementsFile))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	for _, want := range []string{"# Requirements: Coupon URL PRD", "Active members can claim URL coupons.", "Inactive users are blocked."} {
		if !strings.Contains(string(requirements), want) {
			t.Fatalf("requirements missing %q:\n%s", want, string(requirements))
		}
	}

	intakeBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.IntakeFile))
	if err != nil {
		t.Fatalf("read intake: %v", err)
	}
	if !strings.Contains(string(intakeBody), "Source: `"+server.URL+"/prd`") {
		t.Fatalf("intake snapshot missing URL source:\n%s", string(intakeBody))
	}
	if !strings.Contains(stdout.String(), "url: "+server.URL+"/prd") {
		t.Fatalf("stdout missing URL:\n%s", stdout.String())
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "intake.created") || !cliTestHasEvent(events, "context.recalled") {
		t.Fatalf("events missing intake/context events: %#v", events)
	}
}

func TestIntakeFileRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	err := Run([]string{"intake", "--root", root, "--file", filepath.Join(root, "missing.md")}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "read intake file") {
		t.Fatalf("err = %v, want read intake file error", err)
	}
}

func TestIntakeRejectsMissingSource(t *testing.T) {
	err := Run([]string{"intake", "--root", t.TempDir()}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "exactly one intake source is required") {
		t.Fatalf("err = %v, want missing source error", err)
	}
}

func TestIntakeRejectsMultipleSources(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "duplicate-source.md")
	if err := os.WriteFile(prdPath, []byte("# Duplicate source"), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}

	err := Run([]string{"intake", "--root", root, "--file", prdPath, "--url", "https://example.test/prd"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "exactly one intake source is required") {
		t.Fatalf("err = %v, want mutual exclusion error", err)
	}
}

func TestIntakeFileRejectsDuplicateDemand(t *testing.T) {
	root := t.TempDir()
	prdPath := filepath.Join(root, "duplicate.md")
	if err := os.WriteFile(prdPath, []byte("# Duplicate"), 0o644); err != nil {
		t.Fatalf("write prd: %v", err)
	}
	if err := Run([]string{"intake", "--root", root, "--file", prdPath}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("first intake returned error: %v", err)
	}
	err := Run([]string{"intake", "--root", root, "--file", prdPath}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already exists", err)
	}
}

func TestHelpIncludesIntake(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow intake --file <path>", "devflow intake --url <url>", "devflow intake --github-issue <owner/repo#number>", "devflow intake --feishu-doc <doc-url-or-token>", "intake   Create a demand workspace from a PRD file, URL, GitHub Issue, Feishu Doc, or Feishu Bitable record"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
func cliTestHasEvent(events []artifacts.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func TestIntakeGitHubIssueCreatesReviewReadyDemand(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/issues/123":
			_, _ = w.Write([]byte(`{"number":123,"title":"Coupon issue","body":"Users need coupon eligibility.","html_url":"https://github.com/owner/repo/issues/123","state":"open","user":{"login":"alice"},"labels":[{"name":"backend"}]}`))
		case "/repos/owner/repo/issues/123/comments":
			_, _ = w.Write([]byte(`[{"id":10,"body":"Remember inactive users.","html_url":"https://github.com/owner/repo/issues/123#issuecomment-10","created_at":"2026-07-02T02:03:04Z","user":{"login":"bob"}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"intake",
		"--root", root,
		"--github-issue", "owner/repo#123",
		"--github-base-url", server.URL,
		"--github-token", "token",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v\n%s", err, stdout.String())
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand("coupon-issue")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.Source != "intake:github_issue:owner/repo#123" {
		t.Fatalf("source = %q", demand.Source)
	}
	intakeBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.IntakeFile))
	if err != nil {
		t.Fatalf("read intake: %v", err)
	}
	for _, want := range []string{"Source: `github_issue`", "External ID: `owner/repo#123`", "Remember inactive users."} {
		if !strings.Contains(string(intakeBody), want) {
			t.Fatalf("intake missing %q:\n%s", want, string(intakeBody))
		}
	}
	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "platform.intake_fetched") {
		t.Fatalf("events missing platform.intake_fetched: %#v", events)
	}
}

func TestIntakeFeishuDocCreatesReviewReadyDemand(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/docx/v1/documents/doc_token":
			_, _ = w.Write([]byte(`{"code":0,"data":{"document":{"title":"Coupon PRD"}}}`))
		case "/open-apis/docx/v1/documents/doc_token/blocks/doc_token/children":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"block_id":"b1","block_type":3,"heading1":{"elements":[{"text_run":{"content":"Goals"}}]}},{"block_id":"b2","block_type":2,"text":{"elements":[{"text_run":{"content":"Active users can claim coupons."}}]}}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"intake",
		"--root", root,
		"--feishu-doc", "doc_token",
		"--feishu-base-url", server.URL,
		"--feishu-app-id", "cli_test",
		"--feishu-app-secret", "secret",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v\n%s", err, stdout.String())
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand("coupon-prd")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.Source != "intake:feishu_doc:doc_token" {
		t.Fatalf("source = %q", demand.Source)
	}
	intakeBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.IntakeFile))
	if err != nil {
		t.Fatalf("read intake: %v", err)
	}
	for _, want := range []string{"Source: `feishu_doc`", "External ID: `doc_token`", "Active users can claim coupons."} {
		if !strings.Contains(string(intakeBody), want) {
			t.Fatalf("intake missing %q:\n%s", want, string(intakeBody))
		}
	}
	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "platform.intake_fetched") {
		t.Fatalf("events missing platform.intake_fetched: %#v", events)
	}
}

func TestIntakeFeishuBitableRecordCreatesDemand(t *testing.T) {
	root := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/bitable/v1/apps/app_token/tables/tbl/records/search":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"record_id":"rec1","fields":{"需求标题":"Coupon","需求描述":"Coupon eligibility","状态":"待澄清","优先级":"P1","负责人":"dd"}}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"intake",
		"--root", root,
		"--feishu-bitable", "app_token",
		"--table", "tbl",
		"--record", "rec1",
		"--feishu-base-url", server.URL,
		"--feishu-app-id", "cli_test",
		"--feishu-app-secret", "secret",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("intake returned error: %v\n%s", err, stdout.String())
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand("coupon")
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if demand.Source != "intake:feishu_bitable_record:rec1" {
		t.Fatalf("source = %q", demand.Source)
	}
	intakeBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.IntakeFile))
	if err != nil {
		t.Fatalf("read intake: %v", err)
	}
	for _, want := range []string{"Source: `feishu_bitable_record`", "External ID: `rec1`", "Coupon eligibility"} {
		if !strings.Contains(string(intakeBody), want) {
			t.Fatalf("intake missing %q:\n%s", want, string(intakeBody))
		}
	}
}

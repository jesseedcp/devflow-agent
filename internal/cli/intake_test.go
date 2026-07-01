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

func TestIntakeFileRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	err := Run([]string{"intake", "--root", root, "--file", filepath.Join(root, "missing.md")}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "read intake file") {
		t.Fatalf("err = %v, want read intake file error", err)
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
	for _, want := range []string{"devflow intake --file <path>", "intake   Create a demand workspace from a local PRD file"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorReportsConfigFailureWithoutConfig(t *testing.T) {
	var stdout bytes.Buffer
	err := Run([]string{"doctor", "--config", filepath.Join(t.TempDir(), "missing.yaml")}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "doctor found failing checks") {
		t.Fatalf("err = %v want failing checks", err)
	}
	if !strings.Contains(stdout.String(), "[FAIL] config:") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDoctorReportsOKWithoutPrintingAPIKey(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "gitlab-secret-token")
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	secret := "test-api-key-secret"
	body := "providers:\n  - name: test\n    protocol: openai\n    base_url: https://api.openai.com/v1\n    model: gpt-test\n    api_key: " + secret + "\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	_ = Run([]string{"doctor", "--config", cfgPath}, &stdout, &bytes.Buffer{})
	output := stdout.String()
	if !strings.Contains(output, "[OK] config: loaded provider test without printing secrets") {
		t.Fatalf("stdout = %q", output)
	}
	if strings.Contains(output, secret) || strings.Contains(output, "gitlab-secret-token") {
		t.Fatalf("doctor output leaked secret: %q", output)
	}
}

func TestCheckGitLabTokenReportsMissingWithoutLeakingValues(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	check := checkGitLabToken()
	if check.OK || check.Name != "gitlab" {
		t.Fatalf("check = %#v", check)
	}
	if strings.Contains(check.Message, "secret") {
		t.Fatalf("message leaked value: %q", check.Message)
	}
}

func TestDoctorHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"devflow doctor [--require-gitlab]", "doctor   Diagnose config, environment, git, and GitLab readiness"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}
func TestDoctorSkipsGitLabByDefault(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	body := "providers:\n  - name: test\n    protocol: openai\n    base_url: https://api.openai.com/v1\n    model: gpt-test\n    api_key: test-api-key\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"doctor", "--config", cfgPath}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("doctor: %v\nstdout:\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "[OK] gitlab: skipped; pass --require-gitlab to validate mr-review token setup") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDoctorRequiresGitLabWhenFlagSet(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	body := "providers:\n  - name: test\n    protocol: openai\n    base_url: https://api.openai.com/v1\n    model: gpt-test\n    api_key: test-api-key\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"doctor", "--config", cfgPath, "--require-gitlab"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "doctor found failing checks") {
		t.Fatalf("err = %v want failing checks", err)
	}
	if !strings.Contains(stdout.String(), "[FAIL] gitlab: GITLAB_TOKEN is not set") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDoctorReportsBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := writeBackendDemandDefaultsConfig(t, root)
	t.Setenv("OPENAI_API_KEY", "test-key")
	var stdout bytes.Buffer
	err := Run([]string{"doctor", "--config", configPath}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("doctor returned error: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "[OK] backend-demand: quality command defaults configured") {
		t.Fatalf("doctor output missing backend-demand defaults:\n%s", stdout.String())
	}
}

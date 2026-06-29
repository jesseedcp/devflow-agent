package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesNoSecretDefaultConfig(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer

	if err := Run([]string{"init", "--root", root}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("init: %v", err)
	}

	path := filepath.Join(root, ".devflow", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	body := string(data)
	for _, want := range []string{"openai-compat", "https://ark.cn-beijing.volces.com/api/coding/v3", "ark-code-latest"} {
		if !strings.Contains(body, want) {
			t.Fatalf("config missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(strings.ToLower(body), "api_key") {
		t.Fatalf("config contains api_key: %s", body)
	}
	if !strings.Contains(stdout.String(), "wrote") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInitRefusesExistingConfigUnlessForced(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".devflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("existing\n"), 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	err := Run([]string{"init", "--root", root}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v want already exists", err)
	}

	if err := Run([]string{"init", "--root", root, "--force", "--provider", "anthropic"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("init force: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read forced config: %v", err)
	}
	if !strings.Contains(string(data), "anthropic") || strings.Contains(string(data), "existing") {
		t.Fatalf("forced config = %q", string(data))
	}
}

func TestInitRejectsUnsupportedProvider(t *testing.T) {
	err := Run([]string{"init", "--root", t.TempDir(), "--provider", "bogus"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("err = %v want unsupported provider", err)
	}
}

func TestInitHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"devflow init --provider", "init      Create a no-secret .devflow/config.yaml"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}

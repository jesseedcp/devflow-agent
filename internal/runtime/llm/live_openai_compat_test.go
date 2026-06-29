package llm

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/conversation"
)

func TestLiveOpenAICompat(t *testing.T) {
	if os.Getenv("DEVFLOW_LIVE_LLM") != "1" {
		t.Skip("set DEVFLOW_LIVE_LLM=1 to run the real provider smoke test")
	}

	restore, err := chdirToRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	cfg, err := config.LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}

	provider := selectArkOpenAICompatProvider(cfg.Providers)
	if provider == nil {
		t.Fatal("no Ark openai-compatible provider configured; add one to .devflow/config.yaml or legacy .mewcode/config.yaml")
	}

	client, err := NewClient(provider, "Reply concisely and follow the requested exact output.")
	if err != nil {
		t.Fatal(err)
	}

	conversationManager := conversation.NewManager()
	conversationManager.AddUserMessage("Reply with exactly: DEVFLOW_RUNTIME_OK")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	events, errs := client.Stream(ctx, conversationManager, nil)

	var response strings.Builder
	for event := range events {
		if delta, ok := event.(TextDelta); ok {
			response.WriteString(delta.Text)
		}
	}
	for streamErr := range errs {
		if streamErr != nil {
			t.Fatal(streamErr)
		}
	}

	if got := strings.TrimSpace(response.String()); got != "DEVFLOW_RUNTIME_OK" {
		t.Fatalf("response = %q, want DEVFLOW_RUNTIME_OK", got)
	}
}

func TestChdirToRepoRootFindsGoModFromNestedDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "internal", "runtime", "llm")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWD)

	restore, err := chdirToRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()

	currentWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if currentWD != root {
		t.Fatalf("cwd = %q, want %q", currentWD, root)
	}
}

func TestSelectArkOpenAICompatProviderPrefersArkOverEarlierCompatProvider(t *testing.T) {
	providers := []config.ProviderConfig{
		{Name: "generic-compat", Protocol: "openai-compat", BaseURL: "https://example.invalid/v1", Model: "generic-model"},
		{Name: "ark", Protocol: "openai-compat", BaseURL: "https://ark.cn-beijing.volces.com/api/coding/v3", Model: "ark-code-latest"},
	}

	selected := selectArkOpenAICompatProvider(providers)
	if selected == nil {
		t.Fatal("expected Ark provider")
	}
	if selected.Name != "ark" {
		t.Fatalf("selected provider = %q, want ark", selected.Name)
	}
}

func TestSelectArkOpenAICompatProviderReturnsNilWithoutArkProvider(t *testing.T) {
	providers := []config.ProviderConfig{
		{Name: "generic-compat", Protocol: "openai-compat", BaseURL: "https://example.invalid/v1", Model: "generic-model"},
		{Name: "anthropic", Protocol: "anthropic", BaseURL: "https://api.anthropic.com", Model: "claude-sonnet"},
	}

	selected := selectArkOpenAICompatProvider(providers)
	if selected != nil {
		t.Fatalf("selected provider = %#v, want nil", selected)
	}
}

func TestSelectArkOpenAICompatProviderDoesNotMatchSparkBySubstring(t *testing.T) {
	providers := []config.ProviderConfig{
		{Name: "spark", Protocol: "openai-compat", BaseURL: "https://example.invalid/v1", Model: "spark-coder"},
		{Name: "ark", Protocol: "openai-compat", BaseURL: "https://ark.cn-beijing.volces.com/api/coding/v3", Model: "ark-code-latest"},
	}

	selected := selectArkOpenAICompatProvider(providers)
	if selected == nil {
		t.Fatal("expected Ark provider")
	}
	if selected.Name != "ark" {
		t.Fatalf("selected provider = %q, want ark", selected.Name)
	}
}

func chdirToRepoRoot() (func(), error) {
	originalWD, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	root, err := findRepoRoot(originalWD)
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(root); err != nil {
		return nil, err
	}
	return func() {
		_ = os.Chdir(originalWD)
	}, nil
}

func findRepoRoot(start string) (string, error) {
	current := start
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", &config.ConfigError{Message: "could not locate repo root containing go.mod for config discovery"}
		}
		current = parent
	}
}

func selectArkOpenAICompatProvider(providers []config.ProviderConfig) *config.ProviderConfig {
	for index := range providers {
		if isArkOpenAICompatProvider(providers[index]) {
			return &providers[index]
		}
	}
	return nil
}

func isArkOpenAICompatProvider(provider config.ProviderConfig) bool {
	if provider.Protocol != "openai-compat" {
		return false
	}

	if baseURLHostLooksLikeArk(provider.BaseURL) {
		return true
	}
	model := strings.ToLower(provider.Model)
	name := strings.ToLower(provider.Name)
	return strings.HasPrefix(model, "ark-") || name == "ark"
}

func baseURLHostLooksLikeArk(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "ark.cn-beijing.volces.com" || strings.HasSuffix(host, ".ark.cn-beijing.volces.com")
}

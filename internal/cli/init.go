package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root, provider, baseURL, model string
	var force bool
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&provider, "provider", "openai-compat", "provider protocol")
	fs.StringVar(&baseURL, "base-url", "", "provider base url")
	fs.StringVar(&model, "model", "", "provider model")
	fs.BoolVar(&force, "force", false, "overwrite existing config")

	if err := fs.Parse(args); err != nil {
		return err
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "openai-compat"
	}
	cfg, err := renderInitialConfig(provider, baseURL, model)
	if err != nil {
		return err
	}

	cfgDir := filepath.Join(root, ".devflow")
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if _, err := os.Stat(cfgPath); err == nil && !force {
		return fmt.Errorf("%s already exists; use --force to overwrite", cfgPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect config: %w", err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("create .devflow directory: %w", err)
	}
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintf(stdout, "wrote %s\n", cfgPath)
	fmt.Fprintln(stdout, "Set the required API key in your environment before running devflow chat or devflow run.")
	return nil
}

func renderInitialConfig(provider, baseURL, model string) (string, error) {
	switch provider {
	case "openai-compat":
		if strings.TrimSpace(baseURL) == "" {
			baseURL = "https://ark.cn-beijing.volces.com/api/coding/v3"
		}
		if strings.TrimSpace(model) == "" {
			model = "ark-code-latest"
		}
		return fmt.Sprintf(`providers:
  - name: ark
    protocol: openai-compat
    base_url: %s
    model: %s
    context_window: 128000
    max_output_tokens: 8192
permission_mode: default
`, baseURL, model), nil
	case "openai":
		if strings.TrimSpace(baseURL) == "" {
			baseURL = "https://api.openai.com/v1"
		}
		if strings.TrimSpace(model) == "" {
			model = "gpt-5.4"
		}
		return fmt.Sprintf(`providers:
  - name: openai
    protocol: openai
    base_url: %s
    model: %s
    context_window: 128000
    max_output_tokens: 8192
permission_mode: default
`, baseURL, model), nil
	case "anthropic":
		if strings.TrimSpace(baseURL) == "" {
			baseURL = "https://api.anthropic.com"
		}
		if strings.TrimSpace(model) == "" {
			model = "claude-sonnet-4-5"
		}
		return fmt.Sprintf(`providers:
  - name: anthropic
    protocol: anthropic
    base_url: %s
    model: %s
    context_window: 200000
    max_output_tokens: 8192
permission_mode: default
`, baseURL, model), nil
	default:
		return "", fmt.Errorf("unsupported provider %q", provider)
	}
}

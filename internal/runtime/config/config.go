package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/runtime/hooks"
	"gopkg.in/yaml.v3"
)

var envKeyMap = map[string]string{
	"anthropic":     "ANTHROPIC_API_KEY",
	"openai":        "OPENAI_API_KEY",
	"openai-compat": "OPENAI_API_KEY",
}

var validProtocols = map[string]bool{
	"anthropic":     true,
	"openai":        true,
	"openai-compat": true,
}

var statPath = os.Stat

type ConfigError struct {
	Message string
	Cause   error
}

func (e *ConfigError) Error() string { return e.Message }

func (e *ConfigError) Unwrap() error { return e.Cause }

type ProviderConfig struct {
	Name            string `yaml:"name"`
	Protocol        string `yaml:"protocol"`
	BaseURL         string `yaml:"base_url"`
	Model           string `yaml:"model"`
	APIKey          string `yaml:"api_key"`
	Thinking        bool   `yaml:"thinking"`
	ContextWindow   int    `yaml:"context_window"`
	MaxOutputTokens int    `yaml:"max_output_tokens"`
}

func (p *ProviderConfig) GetContextWindow() int {
	if p.ContextWindow > 0 {
		return p.ContextWindow
	}
	if strings.Contains(p.Model, "claude") {
		return 200000
	}
	return 128000
}

func (p *ProviderConfig) GetMaxOutputTokens() int {
	if p.MaxOutputTokens > 0 {
		return p.MaxOutputTokens
	}
	if p.Thinking {
		return 64000
	}
	return 8192
}

func (p *ProviderConfig) ResolveAPIKey() string {
	if p.APIKey != "" {
		return p.APIKey
	}
	envVar := envKeyMap[p.Protocol]
	if envVar == "" {
		return ""
	}
	return os.Getenv(envVar)
}

type MCPServerConfig struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	URL       string            `yaml:"url"`
	Transport string            `yaml:"transport"`
	Headers   map[string]string `yaml:"headers"`
	Env       map[string]string `yaml:"env"`
}

type AppConfig struct {
	Providers      []ProviderConfig  `yaml:"providers"`
	PermissionMode string            `yaml:"permission_mode"`
	MCPServers     []MCPServerConfig `yaml:"mcp_servers"`
	Hooks          []hooks.Hook      `yaml:"hooks"`
}

func loadSingleFile(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("Failed to parse config %s: %s", path, err)}
	}
	return cloneAppConfig(&cfg), nil
}

// Providers replace the earlier slice, MCP servers replace-by-name or append,
// and hooks preserve source behavior by appending later hooks after earlier ones.
func mergeConfig(base, override *AppConfig) *AppConfig {
	merged := cloneAppConfig(base)
	if merged == nil {
		merged = &AppConfig{}
	}
	if override == nil {
		return merged
	}
	if override.Providers != nil {
		merged.Providers = cloneProviderConfigs(override.Providers)
	}
	if override.PermissionMode != "" {
		merged.PermissionMode = override.PermissionMode
	}
	if len(override.MCPServers) > 0 {
		byName := make(map[string]int, len(merged.MCPServers))
		for i, server := range merged.MCPServers {
			byName[server.Name] = i
		}
		for _, server := range override.MCPServers {
			cloned := cloneMCPServer(server)
			if idx, ok := byName[server.Name]; ok {
				merged.MCPServers[idx] = cloned
				continue
			}
			merged.MCPServers = append(merged.MCPServers, cloned)
			byName[server.Name] = len(merged.MCPServers) - 1
		}
	}
	if len(override.Hooks) > 0 {
		merged.Hooks = append(merged.Hooks, cloneHooks(override.Hooks)...)
	}
	return merged
}

func validateProviderEntries(cfg *AppConfig) error {
	requiredFields := []string{"name", "protocol", "base_url", "model"}
	for i, provider := range cfg.Providers {
		var missing []string
		values := map[string]string{
			"name":     provider.Name,
			"protocol": provider.Protocol,
			"base_url": provider.BaseURL,
			"model":    provider.Model,
		}
		for _, field := range requiredFields {
			if values[field] == "" {
				missing = append(missing, field)
			}
		}
		if len(missing) > 0 {
			return &ConfigError{
				Message: fmt.Sprintf("Provider #%d: missing fields: %s", i+1, strings.Join(missing, ", ")),
			}
		}
		if !validProtocols[provider.Protocol] {
			return &ConfigError{
				Message: fmt.Sprintf("Provider #%d: invalid protocol '%s', must be one of: anthropic, openai, openai-compat", i+1, provider.Protocol),
			}
		}
	}
	return nil
}

func validateUniqueNames(cfg *AppConfig) error {
	providerIndexes := make(map[string]int, len(cfg.Providers))
	for i, provider := range cfg.Providers {
		if firstIndex, ok := providerIndexes[provider.Name]; ok {
			return &ConfigError{
				Message: fmt.Sprintf("duplicate provider name %q at providers[%d]; first declared at providers[%d]", provider.Name, i, firstIndex),
			}
		}
		providerIndexes[provider.Name] = i
	}

	serverIndexes := make(map[string]int, len(cfg.MCPServers))
	for i, server := range cfg.MCPServers {
		if strings.TrimSpace(server.Name) == "" {
			return &ConfigError{Message: fmt.Sprintf("mcp_servers[%d]: name is required", i)}
		}
		if firstIndex, ok := serverIndexes[server.Name]; ok {
			return &ConfigError{
				Message: fmt.Sprintf("duplicate mcp server name %q at mcp_servers[%d]; first declared at mcp_servers[%d]", server.Name, i, firstIndex),
			}
		}
		serverIndexes[server.Name] = i
	}

	return nil
}

func validateLayer(cfg *AppConfig) error {
	if err := validateProviderEntries(cfg); err != nil {
		return err
	}
	if err := validateUniqueNames(cfg); err != nil {
		return err
	}
	if err := hooks.Validate(cfg.Hooks); err != nil {
		return &ConfigError{Message: fmt.Sprintf("Invalid hooks configuration: %s", err)}
	}
	return nil
}

func validateFinal(cfg *AppConfig) error {
	if err := validateLayer(cfg); err != nil {
		return err
	}
	if len(cfg.Providers) == 0 {
		return &ConfigError{Message: "At least one provider must be configured"}
	}
	return nil
}

func preferredConfig(primary, legacy string) (string, error) {
	if _, err := statPath(primary); err == nil {
		return primary, nil
	} else if !os.IsNotExist(err) {
		return "", &ConfigError{
			Message: fmt.Sprintf("Failed to inspect preferred config %s before legacy %s: %s", primary, legacy, err),
			Cause:   err,
		}
	}
	return legacy, nil
}

func loadDiscoveredConfig(home, wd string) (*AppConfig, error) {
	userPath, err := preferredConfig(
		filepath.Join(home, ".devflow", "config.yaml"),
		filepath.Join(home, ".mewcode", "config.yaml"),
	)
	if err != nil {
		return nil, err
	}
	projectPath, err := preferredConfig(
		filepath.Join(wd, ".devflow", "config.yaml"),
		filepath.Join(wd, ".mewcode", "config.yaml"),
	)
	if err != nil {
		return nil, err
	}
	localPath, err := preferredConfig(
		filepath.Join(wd, ".devflow", "config.local.yaml"),
		filepath.Join(wd, ".mewcode", "config.local.yaml"),
	)
	if err != nil {
		return nil, err
	}

	candidates := []string{userPath, projectPath, localPath}
	var merged *AppConfig
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := statPath(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, &ConfigError{Message: fmt.Sprintf("Failed to access config %s: %s", path, err), Cause: err}
		}
		layer, err := loadSingleFile(path)
		if err != nil {
			return nil, err
		}
		if err := validateLayer(layer); err != nil {
			return nil, err
		}
		if merged == nil {
			merged = layer
			continue
		}
		merged = mergeConfig(merged, layer)
	}

	if merged == nil {
		return nil, &ConfigError{
			Message: "No config file found. Expected .devflow/config.yaml or .devflow/config.local.yaml, or legacy .mewcode/config.yaml or .mewcode/config.local.yaml in the project or user home",
		}
	}
	if err := validateFinal(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func LoadConfig(path string) (*AppConfig, error) {
	if path != "" {
		cfg, err := loadSingleFile(path)
		if err != nil {
			return nil, err
		}
		if err := validateFinal(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("Failed to get working directory: %s", err)}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("Failed to get user home directory: %s", err)}
	}
	return loadDiscoveredConfig(home, wd)
}

func cloneAppConfig(cfg *AppConfig) *AppConfig {
	if cfg == nil {
		return nil
	}
	return &AppConfig{
		Providers:      cloneProviderConfigs(cfg.Providers),
		PermissionMode: cfg.PermissionMode,
		MCPServers:     cloneMCPServers(cfg.MCPServers),
		Hooks:          cloneHooks(cfg.Hooks),
	}
}

func cloneProviderConfigs(in []ProviderConfig) []ProviderConfig {
	if in == nil {
		return nil
	}
	out := make([]ProviderConfig, len(in))
	copy(out, in)
	return out
}

func cloneMCPServers(in []MCPServerConfig) []MCPServerConfig {
	if in == nil {
		return nil
	}
	out := make([]MCPServerConfig, len(in))
	for i, server := range in {
		out[i] = cloneMCPServer(server)
	}
	return out
}

func cloneMCPServer(server MCPServerConfig) MCPServerConfig {
	cloned := server
	if server.Args != nil {
		cloned.Args = append([]string(nil), server.Args...)
	}
	if server.Headers != nil {
		cloned.Headers = cloneStringMap(server.Headers)
	}
	if server.Env != nil {
		cloned.Env = cloneStringMap(server.Env)
	}
	return cloned
}

func cloneHooks(in []hooks.Hook) []hooks.Hook {
	if in == nil {
		return nil
	}
	out := make([]hooks.Hook, len(in))
	for i, hook := range in {
		out[i] = cloneHook(hook)
	}
	return out
}

func cloneHook(hook hooks.Hook) hooks.Hook {
	cloned := hook
	if hook.Action.Headers != nil {
		cloned.Action.Headers = cloneStringMap(hook.Action.Headers)
	}
	return cloned
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

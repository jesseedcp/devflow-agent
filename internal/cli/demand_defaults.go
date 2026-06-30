package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
)

type demandCommandDefaults struct {
	RunnerRoot           string
	QualityRoot          string
	QualityCommands      []string
	PermissionMode       string
	GitLabProject        string
	GitLabBaseURL        string
	CreateMRTargetBranch string
}

func resolveDemandDefaults(configPath string) (demandCommandDefaults, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		if configPath != "" && errors.Is(err, os.ErrNotExist) {
			return demandCommandDefaults{}, nil
		}
		if configPath == "" {
			return demandCommandDefaults{}, nil
		}
		return demandCommandDefaults{}, err
	}
	backend := cfg.BackendDemand
	return demandCommandDefaults{
		RunnerRoot:           strings.TrimSpace(backend.RunnerRoot),
		QualityRoot:          strings.TrimSpace(backend.QualityRoot),
		QualityCommands:      trimStringSlice(backend.QualityCommands),
		PermissionMode:       strings.TrimSpace(backend.PermissionMode),
		GitLabProject:        strings.TrimSpace(backend.GitLab.Project),
		GitLabBaseURL:        strings.TrimSpace(backend.GitLab.BaseURL),
		CreateMRTargetBranch: firstNonEmpty(strings.TrimSpace(backend.CreateMRTargetBranch), strings.TrimSpace(backend.GitLab.DefaultTargetBranch)),
	}, nil
}

func trimStringSlice(values []string) []string {
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

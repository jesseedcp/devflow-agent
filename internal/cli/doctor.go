package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/platform"
	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
)

type doctorCheck struct {
	Name    string
	OK      bool
	Message string
}

func runDoctor(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var requireGitLab bool
	var platformName string
	fs.StringVar(&configPath, "config", "", "config path")
	fs.BoolVar(&requireGitLab, "require-gitlab", false, "require GITLAB_TOKEN for mr-review readiness")
	fs.StringVar(&platformName, "platform", "", "platform to check: github, feishu, or all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(platformName) != "" {
		return printDoctorChecks(stdout, runPlatformDoctorChecks(platformName))
	}
	return printDoctorChecks(stdout, runDoctorChecks(context.Background(), configPath, requireGitLab))
}

func printDoctorChecks(stdout io.Writer, checks []doctorCheck) error {
	failed := false
	for _, check := range checks {
		mark := "OK"
		if !check.OK {
			mark = "FAIL"
			failed = true
		}
		fmt.Fprintf(stdout, "[%s] %s: %s\n", mark, check.Name, check.Message)
	}
	if failed {
		return fmt.Errorf("doctor found failing checks")
	}
	return nil
}

func runPlatformDoctorChecks(platformName string) []doctorCheck {
	env := map[string]string{
		"GITHUB_TOKEN":      os.Getenv("GITHUB_TOKEN"),
		"FEISHU_APP_ID":     os.Getenv("FEISHU_APP_ID"),
		"FEISHU_APP_SECRET": os.Getenv("FEISHU_APP_SECRET"),
	}
	var checks []platform.DoctorCheck
	switch strings.ToLower(strings.TrimSpace(platformName)) {
	case "github":
		checks = platform.CredentialChecks(platform.ProviderGitHub, env)
	case "feishu":
		checks = platform.CredentialChecks(platform.ProviderFeishu, env)
	case "all":
		checks = append(checks, platform.CredentialChecks(platform.ProviderGitHub, env)...)
		checks = append(checks, platform.CredentialChecks(platform.ProviderFeishu, env)...)
	default:
		checks = []platform.DoctorCheck{{Name: "platform", OK: false, Message: "unsupported platform " + platformName}}
	}
	out := make([]doctorCheck, len(checks))
	for i, check := range checks {
		out[i] = doctorCheck{Name: check.Name, OK: check.OK, Message: check.Message}
	}
	return out
}

func runDoctorChecks(ctx context.Context, configPath string, requireGitLab bool) []doctorCheck {
	checks := []doctorCheck{
		checkGit(ctx),
		checkConfig(configPath),
	}
	if requireGitLab {
		checks = append(checks, checkGitLabToken())
	} else {
		checks = append(checks, doctorCheck{Name: "gitlab", OK: true, Message: "skipped; pass --require-gitlab to validate mr-review token setup"})
	}
	checks = append(checks, checkBackendDemandDefaults(configPath))
	return checks
}

func checkBackendDemandDefaults(configPath string) doctorCheck {
	defaults, err := resolveDemandDefaults(configPath)
	if err != nil {
		return doctorCheck{Name: "backend-demand", OK: false, Message: err.Error()}
	}
	if len(defaults.QualityCommands) == 0 && defaults.PermissionMode == "" && defaults.GitLabProject == "" {
		return doctorCheck{Name: "backend-demand", OK: true, Message: "no defaults configured; CLI flags remain required"}
	}
	if len(defaults.QualityCommands) > 0 {
		return doctorCheck{Name: "backend-demand", OK: true, Message: "quality command defaults configured"}
	}
	return doctorCheck{Name: "backend-demand", OK: true, Message: "defaults configured"}
}

func checkGit(ctx context.Context) doctorCheck {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return doctorCheck{Name: "git", OK: false, Message: "not inside a git repository"}
	}
	return doctorCheck{Name: "git", OK: true, Message: strings.TrimSpace(string(out))}
}

func checkConfig(configPath string) doctorCheck {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return doctorCheck{Name: "config", OK: false, Message: err.Error()}
	}
	if len(cfg.Providers) == 0 {
		return doctorCheck{Name: "config", OK: false, Message: "no providers configured"}
	}
	provider := cfg.Providers[0]
	if provider.ResolveAPIKey() == "" {
		return doctorCheck{Name: "config", OK: false, Message: "provider " + provider.Name + " has no API key in config or environment"}
	}
	return doctorCheck{Name: "config", OK: true, Message: "loaded provider " + provider.Name + " without printing secrets"}
}

func checkGitLabToken() doctorCheck {
	if os.Getenv("GITLAB_TOKEN") == "" {
		return doctorCheck{Name: "gitlab", OK: false, Message: "GITLAB_TOKEN is not set; mr-review requires it unless a token is passed by adapter code"}
	}
	return doctorCheck{Name: "gitlab", OK: true, Message: "GITLAB_TOKEN is set"}
}

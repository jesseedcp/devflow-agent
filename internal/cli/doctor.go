package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	evidenceadapter "github.com/jesseedcp/devflow-agent/internal/evidence"
	"github.com/jesseedcp/devflow-agent/internal/platform"
	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
)

type doctorCheck struct {
	Name    string
	OK      bool
	Message string
}

var doctorObservationTimeout = 5 * time.Second

func runDoctor(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var requireGitLab bool
	var platformName string
	var observationURL string
	fs.StringVar(&configPath, "config", "", "config path")
	fs.BoolVar(&requireGitLab, "require-gitlab", false, "require GITLAB_TOKEN for mr-review readiness")
	fs.StringVar(&platformName, "platform", "", "platform to check: github, feishu, or all")
	fs.StringVar(&observationURL, "observation-url", "", "HTTP URL to check for post-deploy observation reachability")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(observationURL) != "" {
		return printDoctorChecks(stdout, []doctorCheck{checkObservationHTTP(context.Background(), observationURL)})
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

func checkObservationHTTP(ctx context.Context, rawURL string) doctorCheck {
	redactedURL := evidenceadapter.Redact(strings.TrimSpace(rawURL))
	ctx, cancel := context.WithTimeout(ctx, doctorObservationTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return doctorCheck{Name: "observation http", OK: false, Message: "invalid observation URL " + redactedURL + ": " + evidenceadapter.Redact(err.Error())}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return doctorCheck{Name: "observation http", OK: false, Message: observationProxyHint("request failed for " + redactedURL + ": " + evidenceadapter.Redact(err.Error()))}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return doctorCheck{Name: "observation http", OK: false, Message: fmt.Sprintf("GET %s returned %d; expected 2xx/3xx", redactedURL, resp.StatusCode)}
	}
	return doctorCheck{Name: "observation http", OK: true, Message: fmt.Sprintf("GET %s returned %d", redactedURL, resp.StatusCode)}
}

func observationProxyHint(message string) string {
	return message + "; if this URL works in a browser but not Devflow, set HTTPS_PROXY / HTTP_PROXY / NO_PROXY for the shell running Devflow"
}

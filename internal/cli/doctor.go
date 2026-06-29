package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

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
	fs.StringVar(&configPath, "config", "", "config path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	checks := runDoctorChecks(context.Background(), configPath)
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

func runDoctorChecks(ctx context.Context, configPath string) []doctorCheck {
	return []doctorCheck{
		checkGit(ctx),
		checkConfig(configPath),
		checkGitLabToken(),
	}
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

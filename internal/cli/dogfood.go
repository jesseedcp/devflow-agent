package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/dogfood"
	"github.com/jesseedcp/devflow-agent/internal/quality"
)

func runDogfood(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("dogfood", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var root, qualityRoot, scenario string
	var qualityCommands stringSliceFlag
	fs.StringVar(&root, "root", "", "demand artifact root; defaults to a new temp directory")
	fs.StringVar(&qualityRoot, "quality-root", ".", "working directory for quality commands")
	fs.StringVar(&scenario, "scenario", "coupon-eligibility", "dogfood scenario")
	fs.Var(&qualityCommands, "quality-command", "quality command as a quoted program and args (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var commands []quality.Command
	for _, raw := range qualityCommands {
		parts, err := parseCommandLine(raw)
		if err != nil {
			return fmt.Errorf("parse --quality-command %q: %w", raw, err)
		}
		if len(parts) == 0 {
			continue
		}
		commands = append(commands, quality.Command{Name: parts[0], Args: parts[1:]})
	}
	if strings.TrimSpace(qualityRoot) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		qualityRoot = wd
	}

	result, err := dogfood.Run(context.Background(), dogfood.Options{
		Root:            root,
		QualityRoot:     qualityRoot,
		ScenarioName:    scenario,
		QualityCommands: commands,
		Now:             time.Now,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "dogfood completed for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "state: %s\n", result.FinalState)
	fmt.Fprintf(stdout, "root: %s\n", result.Root)
	fmt.Fprintf(stdout, "quality-root: %s\n", result.QualityRoot)
	fmt.Fprintf(stdout, "report: %s\n", result.ReportPath)
	return nil
}

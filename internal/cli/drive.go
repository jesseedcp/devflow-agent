package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

type driveArgs struct {
	root           string
	demandID       string
	dryRun         bool
	maxSteps       int
	runnerRoot     string
	qualityRoot    string
	configPath     string
	permissionMode string
	gitlabProject  string
	gitlabMR       string
	gitlabBaseURL  string
	qualityCommand stringSliceFlag
}

func runDrive(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDriveArgs(args, stderr)
	if err != nil {
		return err
	}
	consoleOpts := consoleArgs{
		root:           opts.root,
		demandID:       opts.demandID,
		runnerRoot:     opts.runnerRoot,
		qualityRoot:    opts.qualityRoot,
		configPath:     opts.configPath,
		permissionMode: opts.permissionMode,
		gitlabProject:  opts.gitlabProject,
		gitlabMR:       opts.gitlabMR,
		gitlabBaseURL:  opts.gitlabBaseURL,
		qualityCommand: opts.qualityCommand,
	}
	if err := applyDefaultsToConsoleArgs(&consoleOpts); err != nil {
		return err
	}
	opts.runnerRoot = consoleOpts.runnerRoot
	opts.qualityRoot = consoleOpts.qualityRoot
	opts.permissionMode = consoleOpts.permissionMode
	opts.gitlabProject = consoleOpts.gitlabProject
	opts.gitlabBaseURL = consoleOpts.gitlabBaseURL
	opts.qualityCommand = consoleOpts.qualityCommand
	if opts.dryRun {
		return runDriveDryRun(opts, stdout)
	}
	return runDriveLoop(opts, stdout, stderr)
}

func parseDriveArgs(args []string, stderr io.Writer) (driveArgs, error) {
	fs := flag.NewFlagSet("drive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts driveArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print the next drive decision without running")
	fs.IntVar(&opts.maxSteps, "max-steps", 5, "maximum runner steps")
	fs.StringVar(&opts.runnerRoot, "runner-root", "", "working directory for agent tools; defaults to --root")
	fs.StringVar(&opts.qualityRoot, "quality-root", "", "working directory for quality commands; defaults to --root")
	fs.StringVar(&opts.configPath, "config", "", "devflow config path")
	fs.StringVar(&opts.permissionMode, "permission-mode", "", "permission mode for implementation")
	fs.StringVar(&opts.gitlabProject, "gitlab-project", "", "gitlab project path for mr-review")
	fs.StringVar(&opts.gitlabMR, "gitlab-mr", "", "gitlab merge request iid for mr-review")
	fs.StringVar(&opts.gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.Var(&opts.qualityCommand, "quality-command", "quality command as a quoted program and args (repeatable)")
	if err := fs.Parse(args); err != nil {
		return driveArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.demandID == "" {
		return driveArgs{}, fmt.Errorf("--demand is required")
	}
	if opts.maxSteps <= 0 {
		return driveArgs{}, fmt.Errorf("--max-steps must be greater than zero")
	}
	return opts, nil
}

func runDriveDryRun(opts driveArgs, stdout io.Writer) error {
	summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	decision := demandflow.DecideDriveStop(summary, 0, opts.maxSteps)
	fmt.Fprintf(stdout, "Drive: %s\n", opts.demandID)
	fmt.Fprintln(stdout, "Mode: dry-run")
	if decision.ShouldStop {
		printDriveStop(stdout, decision)
		return nil
	}
	fmt.Fprintln(stdout, "Next runner-safe action:")
	printDriveAction(stdout, decision.Action)
	return nil
}

func runDriveLoop(opts driveArgs, stdout io.Writer, stderr io.Writer) error {
	for step := 0; step < opts.maxSteps; step++ {
		summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
		if err != nil {
			return err
		}
		decision := demandflow.DecideDriveStop(summary, step, opts.maxSteps)
		if decision.ShouldStop {
			printDriveStop(stdout, decision)
			return nil
		}
		fmt.Fprintf(stdout, "Step %d\n", step+1)
		printDriveAction(stdout, decision.Action)
		consoleOpts := consoleArgs{
			root:           opts.root,
			demandID:       opts.demandID,
			runnerRoot:     opts.runnerRoot,
			qualityRoot:    opts.qualityRoot,
			configPath:     opts.configPath,
			permissionMode: opts.permissionMode,
			gitlabProject:  opts.gitlabProject,
			gitlabMR:       opts.gitlabMR,
			gitlabBaseURL:  opts.gitlabBaseURL,
			qualityCommand: opts.qualityCommand,
		}
		action := consoleRunnableAction(consoleOpts, decision.Action)
		if !action.Runnable {
			printDriveStop(stdout, demandflow.DriveDecision{ShouldStop: true, Reason: demandflow.DriveStopManualAction, Action: action, Message: action.BlockReason})
			return nil
		}
		if err := runConsoleStageAction(consoleOpts, action, stdout, stderr); err != nil {
			printDriveStop(stdout, demandflow.DriveDecision{ShouldStop: true, Reason: demandflow.DriveStopRunnerFailed, Action: action, Message: err.Error()})
			return err
		}
	}
	summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	printDriveStop(stdout, demandflow.DriveDecision{ShouldStop: true, Reason: demandflow.DriveStopMaxStepsReached, Action: summary.PrimaryAction})
	return nil
}

func printDriveAction(stdout io.Writer, action demandflow.ConsoleAction) {
	if action.Label != "" {
		fmt.Fprintf(stdout, "action: %s\n", action.Label)
	}
	if strings.TrimSpace(action.Command) != "" {
		fmt.Fprintf(stdout, "command: %s\n", action.Command)
	}
	if action.BlockReason != "" && !action.Runnable {
		fmt.Fprintf(stdout, "blocked: %s\n", action.BlockReason)
	}
}

func printDriveStop(stdout io.Writer, decision demandflow.DriveDecision) {
	fmt.Fprintln(stdout, "\nStopped")
	fmt.Fprintf(stdout, "reason: %s\n", decision.Reason)
	if decision.Action.Label != "" {
		fmt.Fprintf(stdout, "next: %s\n", decision.Action.Label)
	}
	if strings.TrimSpace(decision.Action.Command) != "" {
		fmt.Fprintf(stdout, "command: %s\n", decision.Action.Command)
	}
	if decision.Message != "" {
		fmt.Fprintf(stdout, "message: %s\n", decision.Message)
	}
}

package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

var runConsoleDemandStage = runDemandStage

type consoleArgs struct {
	root           string
	demandID       string
	runNext        bool
	runnerRoot     string
	qualityRoot    string
	configPath     string
	permissionMode string
	gitlabProject  string
	gitlabMR       string
	gitlabBaseURL  string
	qualityCommand stringSliceFlag
}

func runConsole(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseConsoleArgs(args, stderr)
	if err != nil {
		return err
	}
	if opts.runNext {
		return runConsoleNext(opts, stdout, stderr)
	}
	if opts.demandID != "" {
		summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
		if err != nil {
			return err
		}
		printConsoleDetail(stdout, summary)
		return nil
	}
	summaries, err := demandflow.ListConsole(opts.root)
	if err != nil {
		return err
	}
	printConsoleList(stdout, summaries)
	return nil
}

func parseConsoleArgs(args []string, stderr io.Writer) (consoleArgs, error) {
	fs := flag.NewFlagSet("console", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts consoleArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.BoolVar(&opts.runNext, "run-next", false, "run the next safe agent stage")
	fs.StringVar(&opts.runnerRoot, "runner-root", "", "working directory for agent tools; defaults to --root")
	fs.StringVar(&opts.qualityRoot, "quality-root", "", "working directory for quality commands; defaults to --root")
	fs.StringVar(&opts.configPath, "config", "", "devflow config path")
	fs.StringVar(&opts.permissionMode, "permission-mode", "", "permission mode for implementation")
	fs.StringVar(&opts.gitlabProject, "gitlab-project", "", "gitlab project path for mr-review")
	fs.StringVar(&opts.gitlabMR, "gitlab-mr", "", "gitlab merge request iid for mr-review")
	fs.StringVar(&opts.gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.Var(&opts.qualityCommand, "quality-command", "quality command as a quoted program and args (repeatable)")
	if err := fs.Parse(args); err != nil {
		return consoleArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.runNext && opts.demandID == "" {
		return consoleArgs{}, fmt.Errorf("--demand is required for --run-next")
	}
	return opts, nil
}

func printConsoleList(stdout io.Writer, summaries []demandflow.ConsoleSummary) {
	fmt.Fprintln(stdout, "Demand Console")
	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "\nNo demands found")
		return
	}
	fmt.Fprintln(stdout)
	for _, summary := range summaries {
		workspace := summary.Workspace
		fmt.Fprintf(stdout, "  %-24s %-22s %s\n", workspace.Demand.ID, workspace.State, workspace.Attention)
	}
	fmt.Fprintln(stdout, "\nNext:")
	fmt.Fprintf(stdout, "  devflow console --demand %s\n", summaries[0].Workspace.Demand.ID)
}

func printConsoleDetail(stdout io.Writer, summary demandflow.ConsoleSummary) {
	workspace := summary.Workspace
	fmt.Fprintf(stdout, "Demand Console: %s\n", workspace.Demand.ID)
	fmt.Fprintf(stdout, "State: %s\n", workspace.State)
	fmt.Fprintf(stdout, "Attention: %s\n\n", workspace.Attention)

	fmt.Fprintln(stdout, "Stages:")
	for _, stage := range workspace.Stages {
		fmt.Fprintf(stdout, "  %-14s %s\n", stage.Name, humanStatus(stage.Status))
	}

	fmt.Fprintln(stdout, "\nEvidence:")
	printConsoleEvidence(stdout, workspace)

	fmt.Fprintln(stdout, "\nRecommended:")
	printConsoleAction(stdout, summary.PrimaryAction)

	fmt.Fprintln(stdout, "\nRun-ready:")
	if summary.RunReadyAction.Runnable {
		printConsoleAction(stdout, summary.RunReadyAction)
	} else {
		fmt.Fprintf(stdout, "  %s\n", summary.RunReadyAction.BlockReason)
	}
}

func printConsoleEvidence(stdout io.Writer, workspace demandflow.WorkspaceSummary) {
	switch workspace.Verification.Status {
	case "pass":
		fmt.Fprintf(stdout, "  %-14s PASS %s\n", "verification", workspace.Verification.Command)
	case "fail":
		fmt.Fprintf(stdout, "  %-14s FAIL %s\n", "verification", workspace.Verification.Command)
	default:
		fmt.Fprintf(stdout, "  %-14s none\n", "verification")
	}
	fmt.Fprintf(stdout, "  %-14s %d pending, %d promoted, %d rejected\n", "memory", workspace.Memory.Pending, workspace.Memory.Promoted, workspace.Memory.Rejected)
	mr := humanStatus(workspace.MergeRequest.Status)
	if workspace.MergeRequest.Reference != "" {
		mr = workspace.MergeRequest.Reference + " " + mr
	}
	fmt.Fprintf(stdout, "  %-14s %s\n", "mr", mr)
}

func printConsoleAction(stdout io.Writer, action demandflow.ConsoleAction) {
	if action.Label != "" {
		fmt.Fprintf(stdout, "  %s\n", action.Label)
	}
	if strings.TrimSpace(action.Command) != "" {
		fmt.Fprintf(stdout, "  %s\n", action.Command)
	}
	if action.BlockReason != "" && !action.Runnable {
		fmt.Fprintf(stdout, "  blocked: %s\n", action.BlockReason)
	}
}

func runConsoleNext(opts consoleArgs, stdout io.Writer, stderr io.Writer) error {
	summary, err := demandflow.InspectConsole(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	action := consoleRunnableAction(opts, summary.PrimaryAction)
	if !action.Runnable {
		if action.Kind == demandflow.ConsoleActionMRReview {
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required for mr-review")
		}
		fmt.Fprintf(stdout, "next action is not runner-safe: %s\n", action.Label)
		if action.BlockReason != "" {
			fmt.Fprintf(stdout, "%s\n", action.BlockReason)
		}
		if strings.TrimSpace(action.Command) != "" {
			fmt.Fprintf(stdout, "manual command:\n%s\n", action.Command)
		}
		return nil
	}
	return runConsoleStageAction(opts, action, stdout, stderr)
}

func consoleRunnableAction(opts consoleArgs, action demandflow.ConsoleAction) demandflow.ConsoleAction {
	if action.Kind != demandflow.ConsoleActionMRReview {
		return action
	}
	if strings.TrimSpace(opts.gitlabProject) == "" || strings.TrimSpace(opts.gitlabMR) == "" {
		return action
	}
	action.Runnable = true
	action.BlockReason = ""
	return action
}

func runConsoleStageAction(opts consoleArgs, action demandflow.ConsoleAction, stdout io.Writer, stderr io.Writer) error {
	if action.Stage == "" {
		return fmt.Errorf("console action %q has no runnable stage", action.Label)
	}
	args := []string{
		"--root", opts.root,
		"--demand", opts.demandID,
		"--stage", string(action.Stage),
	}
	if strings.TrimSpace(opts.runnerRoot) != "" {
		args = append(args, "--runner-root", strings.TrimSpace(opts.runnerRoot))
	}
	if strings.TrimSpace(opts.qualityRoot) != "" {
		args = append(args, "--quality-root", strings.TrimSpace(opts.qualityRoot))
	}
	if strings.TrimSpace(opts.configPath) != "" {
		args = append(args, "--config", strings.TrimSpace(opts.configPath))
	}
	if strings.TrimSpace(opts.permissionMode) != "" {
		args = append(args, "--permission-mode", strings.TrimSpace(opts.permissionMode))
	}
	for _, command := range opts.qualityCommand {
		if strings.TrimSpace(command) != "" {
			args = append(args, "--quality-command", strings.TrimSpace(command))
		}
	}
	if action.Stage == demandflow.StageMRReview {
		if strings.TrimSpace(opts.gitlabProject) == "" || strings.TrimSpace(opts.gitlabMR) == "" {
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required for mr-review")
		}
		args = append(args, "--gitlab-project", strings.TrimSpace(opts.gitlabProject), "--gitlab-mr", strings.TrimSpace(opts.gitlabMR))
		if strings.TrimSpace(opts.gitlabBaseURL) != "" {
			args = append(args, "--gitlab-base-url", strings.TrimSpace(opts.gitlabBaseURL))
		}
	}
	return runConsoleDemandStage(args, stdout, stderr)
}

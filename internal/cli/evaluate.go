package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

type evaluateArgs struct {
	root     string
	demandID string
	stage    string
	strict   bool
}

func runEvaluate(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseEvaluateArgs(args, stderr)
	if err != nil {
		return err
	}
	var stages []demandflow.Stage
	if opts.stage != "" {
		stage, err := demandflow.ParseStage(opts.stage)
		if err != nil {
			return err
		}
		stages = append(stages, stage)
	}
	evaluation, err := demandflow.EvaluateDemand(opts.root, opts.demandID, stages...)
	if err != nil {
		return err
	}
	printEvaluation(stdout, evaluation)
	if opts.strict && evaluation.Overall == demandflow.EvaluationFail {
		return fmt.Errorf("evaluation failed for %s", opts.demandID)
	}
	return nil
}

func parseEvaluateArgs(args []string, stderr io.Writer) (evaluateArgs, error) {
	fs := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts evaluateArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.StringVar(&opts.stage, "stage", "", "stage to evaluate")
	fs.BoolVar(&opts.strict, "strict", false, "return non-zero on failed evaluation")
	if err := fs.Parse(args); err != nil {
		return evaluateArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.demandID == "" {
		return evaluateArgs{}, fmt.Errorf("--demand is required")
	}
	opts.stage = strings.TrimSpace(opts.stage)
	return opts, nil
}

func printEvaluation(stdout io.Writer, evaluation demandflow.DemandEvaluation) {
	fmt.Fprintf(stdout, "Evaluation: %s\n", evaluation.DemandID)
	fmt.Fprintf(stdout, "Overall: %s\n\n", evaluation.Overall)
	for _, stage := range evaluation.Stages {
		fmt.Fprintf(stdout, "%-14s %-8s blockers=%d warnings=%d\n", stage.Stage, stage.Status, stage.Blockers, stage.Warnings)
		for _, check := range stage.Checks {
			fmt.Fprintf(stdout, "  %-28s %-8s %s\n", check.ID, check.Status, check.Label)
		}
	}
}

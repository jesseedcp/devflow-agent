package cli

import (
	"flag"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type workbenchOptions struct {
	root           string
	configPath     string
	noAltScreen    bool
	qualityCommand stringSliceFlag
}

var runWorkbenchProgram = func(opts workbenchOptions) error {
	model := newWorkbenchModel(opts)
	options := []tea.ProgramOption{}
	if !opts.noAltScreen {
		options = append(options, tea.WithAltScreen())
	}
	program := tea.NewProgram(model, options...)
	_, err := program.Run()
	return err
}

func runWorkbench(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseWorkbenchArgs(args, stderr)
	if err != nil {
		return err
	}
	return runWorkbenchProgram(opts)
}

func parseWorkbenchArgs(args []string, stderr io.Writer) (workbenchOptions, error) {
	fs := flag.NewFlagSet("workbench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts workbenchOptions
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.configPath, "config", "", "devflow config path")
	fs.BoolVar(&opts.noAltScreen, "no-alt-screen", false, "disable alternate screen")
	fs.Var(&opts.qualityCommand, "quality-command", "quality command for run actions")
	if err := fs.Parse(args); err != nil {
		return workbenchOptions{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	return opts, nil
}

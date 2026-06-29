package quality

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

type Command struct {
	Name string
	Args []string
}

type Result struct {
	Command  string
	Args     []string
	Dir      string
	ExitCode int
	Stdout   string
	Stderr   string
}

type GateResult struct {
	Passed  bool
	Results []Result
}

type Runner interface {
	Run(ctx context.Context, dir string, name string, args ...string) Result
}

type ExecRunner struct{}

type Gate struct {
	Runner Runner
}

func (r ExecRunner) Run(ctx context.Context, dir string, name string, args ...string) Result {
	if ctx == nil {
		ctx = context.Background()
	}

	result := Result{
		Command: name,
		Args:    append([]string(nil), args...),
		Dir:     dir,
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err == nil {
		return result
	}

	result.ExitCode = 1

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	}

	if result.Stderr == "" {
		result.Stderr = err.Error()
	}

	return result
}

func (g Gate) Run(ctx context.Context, dir string, commands ...Command) GateResult {
	if ctx == nil {
		ctx = context.Background()
	}

	runner := g.Runner
	if runner == nil {
		runner = ExecRunner{}
	}

	results := make([]Result, 0, len(commands))
	passed := true

	for _, command := range commands {
		result := runner.Run(ctx, dir, command.Name, command.Args...)
		results = append(results, result)
		if result.ExitCode != 0 {
			passed = false
		}
	}

	return GateResult{
		Passed:  passed,
		Results: results,
	}
}

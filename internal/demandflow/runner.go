package demandflow

import (
	"context"
	"time"
)

type RunnerRequest struct {
	Stage    Stage
	Root     string
	DemandID string
	Prompt   string
	Context  ContextSnapshot
	ToolMode ToolMode
}

type ToolMode string

const (
	ToolModeReadOnly     ToolMode = "read-only"
	ToolModeEdit         ToolMode = "edit"
	ToolModeEditAndShell ToolMode = "edit-and-shell"
)

type RuntimeCompletionMode string

const (
	RuntimeCompletionUnknown                RuntimeCompletionMode = ""
	RuntimeCompletionModelText              RuntimeCompletionMode = "model_text"
	RuntimeCompletionDeterministicFinalizer RuntimeCompletionMode = "deterministic_finalizer"
	RuntimeCompletionFailed                 RuntimeCompletionMode = "failed"
)

type RuntimeToolTrace struct {
	ToolID   string
	ToolName string
	Desc     string
	Output   string
	IsError  bool
	Elapsed  time.Duration
}

type RuntimeSummary struct {
	Stage            Stage
	Model            string
	CompletionMode   RuntimeCompletionMode
	MaxIterationsHit bool
	ToolCalls        int
	EditCalls        int
	BashCalls        int
	ErrorCalls       int
	LastTools        []string
	ChangedFiles     []string
	TestCommands     []string
}

type RunnerResponse struct {
	Text        string
	ToolSummary []string
	Runtime     RuntimeSummary
}

type Runner interface {
	Run(ctx context.Context, req RunnerRequest) (RunnerResponse, error)
}

type StaticRunner struct {
	Responses map[Stage]RunnerResponse
	Requests  []RunnerRequest
}

func (r *StaticRunner) Run(_ context.Context, req RunnerRequest) (RunnerResponse, error) {
	r.Requests = append(r.Requests, req)
	if resp, ok := r.Responses[req.Stage]; ok {
		return resp, nil
	}
	return RunnerResponse{Text: "# " + string(req.Stage) + "\n\nstatic response\n"}, nil
}

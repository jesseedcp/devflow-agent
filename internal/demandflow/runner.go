package demandflow

import "context"

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

type RunnerResponse struct {
	Text        string
	ToolSummary []string
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

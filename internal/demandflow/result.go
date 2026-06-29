package demandflow

import "github.com/jesseedcp/devflow-agent/internal/workflow"

type RunResult struct {
	DemandID      string
	Stage         Stage
	PreviousState workflow.State
	CurrentState  workflow.State
	Artifacts     []string
	QualityPassed *bool
	Message       string
	NextActions   []NextAction
}

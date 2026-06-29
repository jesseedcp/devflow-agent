package demandflow

import (
	"fmt"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
)

type Stage string

const (
	StageRequirements   Stage = "requirements"
	StagePlan           Stage = "plan"
	StageImplementation Stage = "implementation"
	StageMRReview       Stage = "mr-review"
	StageVerification   Stage = "verification"
	StageCloseout       Stage = "closeout"
)

func ParseStage(value string) (Stage, error) {
	switch Stage(value) {
	case StageRequirements, StagePlan, StageImplementation, StageMRReview, StageVerification, StageCloseout:
		return Stage(value), nil
	default:
		return "", fmt.Errorf("unsupported stage %q", value)
	}
}

type ArtifactSnapshot struct {
	Requirements     string
	Plan             string
	Progress         string
	Verification     string
	Closeout         string
	MemoryCandidates string
}

type ContextSnapshot struct {
	Demand    artifacts.Demand
	Artifacts ArtifactSnapshot
	Memories  []MemoryHit
}

type MemoryHit struct {
	DemandID string
	Path     string
	Snippet  string
}

type Options struct {
	Root            string
	RunnerRoot      string
	QualityRoot     string
	DemandID        string
	Stage           Stage
	QualityCommands []quality.Command
	Runner          Runner
	Review          ReviewOptions
	Now             func() time.Time
}

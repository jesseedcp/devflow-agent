package artifacts

import "time"

const (
	DemandFile               = "demand.json"
	IntakeFile               = "intake.md"
	ContextFile              = "context.md"
	CodemapFile              = "codemap.md"
	PlanContextFile          = "plan-context.md"
	ChangeScopeFile          = "change-scope.md"
	ImplementationReviewFile = "implementation-review.md"
	RequirementsFile         = "requirements.md"
	PlanFile                 = "plan.md"
	ProgressFile             = "progress.md"
	VerificationFile         = "verification.md"
	CloseoutFile             = "closeout.md"
	MemoryCandidatesFile     = "memory-candidates.md"
	EventsFile               = "events.jsonl"
)

type Demand struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Source      string    `json:"source"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Event struct {
	Time    time.Time         `json:"time"`
	Type    string            `json:"type"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data,omitempty"`
}

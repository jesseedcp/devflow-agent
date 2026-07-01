package adapters

import "context"

type CIStatus string

const (
	CIStatusPassed  CIStatus = "passed"
	CIStatusFailed  CIStatus = "failed"
	CIStatusPending CIStatus = "pending"
	CIStatusUnknown CIStatus = "unknown"
)

type CIRef struct {
	Provider string
	Repo     string
	PR       string
	BaseURL  string
	Token    string
}

type CICheck struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
}

type CIResult struct {
	Provider string
	Repo     string
	PR       string
	HeadSHA  string
	Status   CIStatus
	Checks   []CICheck
	Message  string
}

type CIGateAdapter interface {
	Check(ctx context.Context, ref CIRef) (CIResult, error)
}

package wiki

import "time"

type CandidateKind string

const (
	KindBusiness CandidateKind = "business"
	KindProcess  CandidateKind = "process"
	KindArchive  CandidateKind = "archive"
)

type CandidateStatus string

const (
	StatusPending  CandidateStatus = "pending"
	StatusPromoted CandidateStatus = "promoted"
	StatusRejected CandidateStatus = "rejected"
)

type Candidate struct {
	Index    int
	Kind     CandidateKind
	Text     string
	Source   string
	Status   CandidateStatus
	WikiPath string
	Reason   string
}

type PromoteOptions struct {
	DemandID       string
	CandidateIndex int
	Name           string
	By             string
	Now            func() time.Time
}

type RejectOptions struct {
	DemandID       string
	CandidateIndex int
	By             string
	Reason         string
	Now            func() time.Time
}
package implreview

type Review struct {
	DemandID            string
	DeclaredSource      []string
	DeclaredTests       []string
	ChangedFiles        []string
	InScope             []string
	OutOfScope          []string
	MissingTests        []string
	VerificationStatus  string
	VerificationCommand string
	AcceptancePass      int
	AcceptanceFail      int
	AcceptanceBlocked   int
	MRStatus            string
	Recommendation      string
}

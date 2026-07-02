package codemap

type Index struct {
	Version string     `json:"version"`
	Root    string     `json:"root"`
	Facts   []CodeFact `json:"facts"`
}

type CodeFact struct {
	Kind      string   `json:"kind"`
	Package   string   `json:"package,omitempty"`
	File      string   `json:"file"`
	Name      string   `json:"name"`
	Receiver  string   `json:"receiver,omitempty"`
	Line      int      `json:"line"`
	Signature string   `json:"signature,omitempty"`
	Text      string   `json:"text,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type SearchResult struct {
	Fact  CodeFact `json:"fact"`
	Score int      `json:"score"`
}

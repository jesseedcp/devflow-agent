package evidence

type FetchResult struct {
	Status          string
	Summary         string
	URL             string
	Method          string
	ActualStatus    int
	RequestExcerpt  string
	ResponseExcerpt string
}

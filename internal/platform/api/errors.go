package api

// ErrorResponse is the standard JSON error envelope returned by platform API
// handlers on failure.
type ErrorResponse struct {
	Error string `json:"error"`
}
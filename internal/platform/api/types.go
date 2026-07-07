package api

import "time"

// HealthResponse is returned by GET /api/health.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// CurrentUser is the dev-mode identity exposed to the frontend so role-aware
// views can disable actions the caller is not permitted to perform.
type CurrentUser struct {
	Email       string `json:"email"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
}

// Workspace is the API contract for a platform workspace.
type Workspace struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ArtifactRoot string    `json:"artifact_root"`
	CreatedAt    time.Time `json:"created_at"`
}
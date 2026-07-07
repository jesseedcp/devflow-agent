package server

import (
	"net/http"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.store.Ping(r.Context()); err != nil {
		dbStatus = "down"
	}
	status := "ok"
	code := http.StatusOK
	if dbStatus != "ok" {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, api.HealthResponse{Status: status, Database: dbStatus})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.CurrentUser{
		Email:       s.cfg.DevUserEmail,
		Role:        string(s.cfg.DevUserRole),
		DisplayName: s.cfg.DevUserEmail,
	})
}

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	ws, err := s.store.ListWorkspaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list workspaces failed")
		return
	}
	out := make([]api.Workspace, 0, len(ws))
	for _, item := range ws {
		out = append(out, toAPIWorkspace(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ArtifactRoot string `json:"artifact_root"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.ArtifactRoot == "" {
		req.ArtifactRoot = s.cfg.ArtifactRoot
	}
	if req.ID == "" {
		req.ID = newID()
	}
	ws := store.Workspace{
		ID:           req.ID,
		Name:         req.Name,
		ArtifactRoot: req.ArtifactRoot,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.store.UpsertWorkspace(r.Context(), ws); err != nil {
		writeError(w, http.StatusInternalServerError, "create workspace failed")
		return
	}
	writeJSON(w, http.StatusCreated, toAPIWorkspace(ws))
}

func toAPIWorkspace(w store.Workspace) api.Workspace {
	return api.Workspace{
		ID:           w.ID,
		Name:         w.Name,
		ArtifactRoot: w.ArtifactRoot,
		CreatedAt:    w.CreatedAt,
	}
}
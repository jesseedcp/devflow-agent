package server

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/artifactbridge"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func (s *Server) handleListDemands(w http.ResponseWriter, r *http.Request) {
	ws, err := s.workspaceForRequest(r)
	if err != nil {
		s.writeWorkspaceError(w, err)
		return
	}
	demands, err := artifactbridge.ScanDemands(ws.ArtifactRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scan demands failed")
		return
	}
	writeJSON(w, http.StatusOK, demands)
}

func (s *Server) handleCreateDemand(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.workspaceForRequest(r)
	if err != nil {
		s.writeWorkspaceError(w, err)
		return
	}

	var req api.CreateDemandRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Key = strings.TrimSpace(req.Key)
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Source == "" {
		req.Source = "web"
	}
	if req.Key == "" || req.Title == "" {
		writeError(w, http.StatusBadRequest, "key and title are required")
		return
	}

	now := time.Now().UTC()
	if err := artifacts.NewStore(workspace.ArtifactRoot).CreateDemand(artifacts.Demand{
		ID:          req.Key,
		Title:       req.Title,
		Description: req.Description,
		Source:      req.Source,
		State:       string(workflow.RequirementsReview),
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.recordAudit(r, workspace.ID, "create_demand", "demand", req.Key, map[string]any{"title": req.Title, "source": req.Source})
	detail, err := s.demandDetail(workspace, req.Key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, api.ActionResult{Status: "created", Message: "Demand created", Demand: detail})
}

func (s *Server) handleGetDemand(w http.ResponseWriter, r *http.Request) {
	ws, err := s.workspaceForRequest(r)
	if err != nil {
		s.writeWorkspaceError(w, err)
		return
	}
	detail, err := s.demandDetail(ws, r.PathValue("demandKey"))
	if err != nil {
		writeError(w, http.StatusNotFound, "demand not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// demandDetail is the single mapping path from a demandflow workspace summary
// to the API demand detail. All demand handlers use it so lifecycle state is
// interpreted in one place.
func (s *Server) demandDetail(workspace store.Workspace, demandKey string) (api.DemandDetail, error) {
	return artifactbridge.GetDemand(workspace.ArtifactRoot, demandKey)
}

func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	ws, err := s.workspaceForRequest(r)
	if err != nil {
		s.writeWorkspaceError(w, err)
		return
	}
	content, err := artifactbridge.ReadArtifact(ws.ArtifactRoot, r.PathValue("demandKey"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func (s *Server) workspaceForRequest(r *http.Request) (store.Workspace, error) {
	return s.store.GetWorkspace(r.Context(), r.PathValue("workspaceID"))
}

func (s *Server) writeWorkspaceError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "workspace lookup failed")
}

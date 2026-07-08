package server

import (
	"errors"
	"net/http"

	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/artifactbridge"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
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

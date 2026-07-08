package server

import "github.com/jesseedcp/devflow-agent/internal/platform/store"

// registerRoutes wires all platform API endpoints. Protected write endpoints
// are wrapped with requireAction so RBAC is enforced before the handler runs.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/me", s.handleMe)
	s.mux.HandleFunc("GET /api/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("POST /api/workspaces", s.requireAction(store.ActionConfigureWorkspace, s.handleCreateWorkspace))

	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/demands", s.handleListDemands)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/demands", s.requireAction(store.ActionConfigureWorkspace, s.handleCreateDemand))
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/demands/{demandKey}", s.handleGetDemand)
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/demands/{demandKey}/confirm", s.requireAction(store.ActionConfirmGate, s.handleConfirmDemand))
	s.mux.HandleFunc("POST /api/workspaces/{workspaceID}/demands/{demandKey}/evidence", s.requireAction(store.ActionAddEvidence, s.handleAddDemandEvidence))
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/demands/{demandKey}/artifacts/{name}", s.handleGetArtifact)

	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/audit", s.handleListAudit)
}

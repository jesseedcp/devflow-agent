package server

// registerRoutes wires all platform API endpoints. Wiki, release, and audit
// routes are added in later tasks.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/me", s.handleMe)
	s.mux.HandleFunc("GET /api/workspaces", s.handleListWorkspaces)
	s.mux.HandleFunc("POST /api/workspaces", s.handleCreateWorkspace)

	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/demands", s.handleListDemands)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/demands/{demandKey}", s.handleGetDemand)
	s.mux.HandleFunc("GET /api/workspaces/{workspaceID}/demands/{demandKey}/artifacts/{name}", s.handleGetArtifact)
}
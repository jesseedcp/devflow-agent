package server

import (
	"net/http"

	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

// requireAction wraps a handler so it only runs if the current dev-mode role
// is permitted to perform action. Forbidden requests get 403.
func (s *Server) requireAction(action store.Action, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := s.cfg.DevUserRole
		if !store.Can(role, action) {
			writeError(w, http.StatusForbidden, "forbidden: role "+string(role)+" may not "+string(action))
			return
		}
		next(w, r)
	}
}

// recordAudit appends an audit event for the current dev user. Audit write
// failures are logged but do not undo an already-completed action, because the
// mutation has already happened by the time audit is recorded.
func (s *Server) recordAudit(r *http.Request, workspaceID, action, subjectType, subjectID string, metadata map[string]any) {
	ev := store.NewAuditEvent(workspaceID, s.cfg.DevUserID, action, subjectType, subjectID, metadata)
	if err := s.store.AppendAudit(r.Context(), ev); err != nil {
		s.cfg.Logger.Printf("audit append failed for %s: %v", action, err)
	}
}
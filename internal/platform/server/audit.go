package server

import (
	"encoding/json"
	"net/http"

	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	ws, err := s.workspaceForRequest(r)
	if err != nil {
		s.writeWorkspaceError(w, err)
		return
	}
	events, err := s.store.ListAudit(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list audit failed")
		return
	}
	out := make([]api.AuditEvent, 0, len(events))
	for _, e := range events {
		out = append(out, toAPIAudit(e))
	}
	writeJSON(w, http.StatusOK, out)
}

func toAPIAudit(e store.AuditEvent) api.AuditEvent {
	var meta map[string]any
	if e.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(e.MetadataJSON), &meta)
	}
	return api.AuditEvent{
		ID:          e.ID,
		WorkspaceID: e.WorkspaceID,
		ActorUserID: e.ActorUserID,
		Action:      e.Action,
		SubjectType: e.SubjectType,
		SubjectID:   e.SubjectID,
		Metadata:    meta,
		CreatedAt:   e.CreatedAt,
	}
}
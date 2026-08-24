package httpapi

import (
	"net/http"

	"task218-wafercausal/internal/ingest"
)

// untrustedRequest PUT /api/events/{id}/untrusted 请求体。
type untrustedRequest struct {
	Untrusted bool `json:"untrusted"`
}

// handleImportEvents POST /api/batches/{id}/events
func (s *Server) handleImportEvents(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var inputs []ingest.EventInput
	if err := decodeBody(r, &inputs); err != nil {
		writeErr(w, err)
		return
	}
	res, err := s.app.ImportEvents(r.Context(), batchID, inputs)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleListEvents GET /api/batches/{id}/events
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	events, err := s.app.ListEvents(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

// handleMarkUntrusted PUT /api/events/{id}/untrusted
func (s *Server) handleMarkUntrusted(w http.ResponseWriter, r *http.Request) {
	eventID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var req untrustedRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.app.MarkEventUntrusted(r.Context(), eventID, req.Untrusted); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "event_id": eventID, "untrusted": req.Untrusted})
}

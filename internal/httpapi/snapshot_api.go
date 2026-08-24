package httpapi

import (
	"net/http"
)

// publishSnapshotRequest POST /api/batches/{id}/snapshots 请求体。
type publishSnapshotRequest struct {
	Name string `json:"name"`
}

// handlePublishSnapshot POST /api/batches/{id}/snapshots
func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var req publishSnapshotRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	snap, err := s.app.PublishSnapshot(r.Context(), batchID, req.Name)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// handleListSnapshots GET /api/batches/{id}/snapshots
func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	snaps, err := s.app.ListSnapshots(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps, "count": len(snaps)})
}

// handleGetSnapshot GET /api/snapshots/{id}
func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	snap, err := s.app.GetSnapshot(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

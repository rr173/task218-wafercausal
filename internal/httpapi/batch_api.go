package httpapi

import (
	"net/http"
)

// createBatchRequest POST /api/batches 请求体。
type createBatchRequest struct {
	Name       string  `json:"name"`
	WaferID    string  `json:"wafer_id"`
	DiameterMM float64 `json:"diameter_mm"`
}

// handleCreateBatch POST /api/batches
func (s *Server) handleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var req createBatchRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.app.CreateBatch(r.Context(), req.Name, req.WaferID, req.DiameterMM)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// handleListBatches GET /api/batches
func (s *Server) handleListBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := s.app.ListBatches(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": batches, "count": len(batches)})
}

// handleGetBatch GET /api/batches/{id}
func (s *Server) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.app.GetBatch(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleFreezeBatch POST /api/batches/{id}/freeze
func (s *Server) handleFreezeBatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.app.FreezeBatch(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// handleArchiveBatch POST /api/batches/{id}/archive
func (s *Server) handleArchiveBatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.app.ArchiveBatch(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

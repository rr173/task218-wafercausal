package httpapi

import (
	"net/http"
)

// handleGenerateCandidates POST /api/batches/{id}/causal
func (s *Server) handleGenerateCandidates(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cands, err := s.app.GenerateCandidates(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"candidates": cands, "count": len(cands)})
}

// handleListCandidates GET /api/batches/{id}/causal
func (s *Server) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cands, err := s.app.ListCandidates(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": cands, "count": len(cands)})
}

// handleGetCandidate GET /api/causal/{id}
func (s *Server) handleGetCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cand, err := s.app.GetCandidate(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cand)
}

package httpapi

import (
	"net/http"
)

// rulingRequest POST /api/causal/{id}/confirm|exclude 请求体。
type rulingRequest struct {
	Reason string `json:"reason"`
}

// handleConfirmCandidate POST /api/causal/{id}/confirm
func (s *Server) handleConfirmCandidate(w http.ResponseWriter, r *http.Request) {
	candidateID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cand, err := s.app.GetCandidate(r.Context(), candidateID)
	if err != nil {
		writeErr(w, err)
		return
	}
	var req rulingRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	rule, err := s.app.ConfirmCandidate(r.Context(), cand.BatchID, candidateID, actor(r), req.Reason)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// handleExcludeCandidate POST /api/causal/{id}/exclude
func (s *Server) handleExcludeCandidate(w http.ResponseWriter, r *http.Request) {
	candidateID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cand, err := s.app.GetCandidate(r.Context(), candidateID)
	if err != nil {
		writeErr(w, err)
		return
	}
	var req rulingRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	rule, err := s.app.ExcludeCandidate(r.Context(), cand.BatchID, candidateID, actor(r), req.Reason)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// handleListRulings GET /api/batches/{id}/rulings
func (s *Server) handleListRulings(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	rulings, err := s.app.ListRulings(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rulings": rulings, "count": len(rulings)})
}

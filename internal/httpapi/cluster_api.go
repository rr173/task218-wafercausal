package httpapi

import (
	"net/http"
)

// handleRunClustering POST /api/batches/{id}/clusters?threshold_um=5000
func (s *Server) handleRunClustering(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	threshold := queryFloat(r, "threshold_um", 5000)
	clusters, err := s.app.RunClustering(r.Context(), batchID, threshold)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clusters": clusters, "count": len(clusters)})
}

// handleListClusters GET /api/batches/{id}/clusters
func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	clusters, err := s.app.ListClusters(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clusters": clusters, "count": len(clusters)})
}

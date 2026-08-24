package httpapi

import (
	"net/http"

	"task218-wafercausal/internal/ingest"
)

// handleImportDefects POST /api/batches/{id}/defects
// 请求体为缺陷输入数组，逐条幂等去重。
func (s *Server) handleImportDefects(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var inputs []ingest.DefectInput
	if err := decodeBody(r, &inputs); err != nil {
		writeErr(w, err)
		return
	}
	res, err := s.app.ImportDefects(r.Context(), batchID, inputs)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleListDefects GET /api/batches/{id}/defects
func (s *Server) handleListDefects(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	defects, err := s.app.ListDefects(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"defects": defects, "count": len(defects)})
}

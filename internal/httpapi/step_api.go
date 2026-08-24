package httpapi

import (
	"net/http"
)

// addStepRequest POST /api/batches/{id}/steps 请求体。
type addStepRequest struct {
	Name string `json:"name"`
	Tool string `json:"tool"`
	Seq  int    `json:"seq"`
}

// addStepChainRequest POST /api/batches/{id}/steps/chain 请求体。
type addStepChainRequest struct {
	Names []string `json:"names"`
}

// setParentRequest PUT /api/steps/{id}/parent 请求体。
type setParentRequest struct {
	ParentStepID int64 `json:"parent_step_id"`
}

// handleAddStep POST /api/batches/{id}/steps
func (s *Server) handleAddStep(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var req addStepRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	st, err := s.app.AddStep(r.Context(), batchID, req.Name, req.Tool, req.Seq)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, st)
}

// handleAddStepChain POST /api/batches/{id}/steps/chain
func (s *Server) handleAddStepChain(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var req addStepChainRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	steps, err := s.app.AddStepChain(r.Context(), batchID, req.Names)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"steps": steps, "count": len(steps)})
}

// handleListSteps GET /api/batches/{id}/steps
func (s *Server) handleListSteps(w http.ResponseWriter, r *http.Request) {
	batchID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	steps, err := s.app.ListSteps(r.Context(), batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "count": len(steps)})
}

// handleSetStepParent PUT /api/steps/{id}/parent
func (s *Server) handleSetStepParent(w http.ResponseWriter, r *http.Request) {
	stepID, err := parseID(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var req setParentRequest
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.app.SetStepParent(r.Context(), stepID, req.ParentStepID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "step_id": stepID})
}

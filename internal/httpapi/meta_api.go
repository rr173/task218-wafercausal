package httpapi

import (
	"net/http"
)

// handleHealthz GET /healthz
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "task218-wafercausal"})
}

// handleSelfCheck GET /api/selfcheck
// 自检：DB 完整性、关键表计数。
func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	report, err := s.app.SelfCheck(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	status := http.StatusOK
	if report["integrity_check"] != 1 {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, report)
}

// handleStats GET /api/stats
// 返回各表行数与完整性标志。
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.app.Repos().Stats(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// handleAuditEvents GET /api/audit/events?limit=N
func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	events, err := s.app.Repos().ListAuditEvents(r.Context(), int(limit))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
}

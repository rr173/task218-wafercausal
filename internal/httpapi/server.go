// Package httpapi 提供 HTTP 接口层，路由统一以 /api 前缀。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/service"
)

// Server 封装 HTTP 服务。
type Server struct {
	app *service.Service
	mux *http.ServeMux
}

// New 构造 HTTP 服务并注册全部路由。
func New(app *service.Service) *Server {
	s := &Server{app: app, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回 http.Handler。
func (s *Server) Handler() http.Handler {
	return logMiddleware(s.mux)
}

// routes 注册路由（统一 /api 前缀）。
func (s *Server) routes() {
	// 元信息
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/selfcheck", s.handleSelfCheck)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/audit/events", s.handleAuditEvents)

	// 晶圆批次
	s.mux.HandleFunc("POST /api/batches", s.handleCreateBatch)
	s.mux.HandleFunc("GET /api/batches", s.handleListBatches)
	s.mux.HandleFunc("GET /api/batches/{id}", s.handleGetBatch)
	s.mux.HandleFunc("POST /api/batches/{id}/freeze", s.handleFreezeBatch)
	s.mux.HandleFunc("POST /api/batches/{id}/archive", s.handleArchiveBatch)

	// 工艺步骤
	s.mux.HandleFunc("POST /api/batches/{id}/steps", s.handleAddStep)
	s.mux.HandleFunc("POST /api/batches/{id}/steps/chain", s.handleAddStepChain)
	s.mux.HandleFunc("GET /api/batches/{id}/steps", s.handleListSteps)
	s.mux.HandleFunc("PUT /api/steps/{id}/parent", s.handleSetStepParent)

	// 缺陷记录
	s.mux.HandleFunc("POST /api/batches/{id}/defects", s.handleImportDefects)
	s.mux.HandleFunc("GET /api/batches/{id}/defects", s.handleListDefects)

	// 工艺事件
	s.mux.HandleFunc("POST /api/batches/{id}/events", s.handleImportEvents)
	s.mux.HandleFunc("GET /api/batches/{id}/events", s.handleListEvents)
	s.mux.HandleFunc("PUT /api/events/{id}/untrusted", s.handleMarkUntrusted)

	// 空间聚类
	s.mux.HandleFunc("POST /api/batches/{id}/clusters", s.handleRunClustering)
	s.mux.HandleFunc("GET /api/batches/{id}/clusters", s.handleListClusters)

	// 因果候选
	s.mux.HandleFunc("POST /api/batches/{id}/causal", s.handleGenerateCandidates)
	s.mux.HandleFunc("GET /api/batches/{id}/causal", s.handleListCandidates)
	s.mux.HandleFunc("GET /api/causal/{id}", s.handleGetCandidate)

	// 证据裁决
	s.mux.HandleFunc("POST /api/causal/{id}/confirm", s.handleConfirmCandidate)
	s.mux.HandleFunc("POST /api/causal/{id}/exclude", s.handleExcludeCandidate)
	s.mux.HandleFunc("GET /api/batches/{id}/rulings", s.handleListRulings)

	// 因果快照
	s.mux.HandleFunc("POST /api/batches/{id}/snapshots", s.handlePublishSnapshot)
	s.mux.HandleFunc("GET /api/batches/{id}/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("GET /api/snapshots/{id}", s.handleGetSnapshot)
}

// ---------------------------------------------------------------------------
// 工具
// ---------------------------------------------------------------------------

// writeJSON 写出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// writeErr 依据领域错误映射 HTTP 状态码。
func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrConflict), errors.Is(err, model.ErrDuplicate):
		status = http.StatusConflict
	case errors.Is(err, model.ErrArchived), errors.Is(err, model.ErrStateMachine):
		status = http.StatusConflict
	case errors.Is(err, model.ErrInvalid):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// parseID 解析路径参数 ID。
func parseID(r *http.Request, key string) (int64, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return 0, model.ErrInvalid
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, model.ErrInvalid
	}
	return id, nil
}

// decodeBody 解析 JSON 请求体（限制大小，禁止未知字段）。
func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 8<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// queryInt 读取查询参数整数。
func queryInt(r *http.Request, key string, def int64) int64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return v
	}
	return def
}

// queryFloat 读取查询参数浮点数。
func queryFloat(r *http.Request, key string, def float64) float64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v
	}
	return def
}

// logMiddleware 请求日志中间件。
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Round(time.Microsecond))
	})
}

// actor 提取请求方标识（缺省 system）。
func actor(r *http.Request) string {
	a := strings.TrimSpace(r.Header.Get("X-Actor"))
	if a == "" {
		return "system"
	}
	return a
}

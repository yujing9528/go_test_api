package stats

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	store  *Store
	logger *log.Logger
}

func NewHandler(store *Store, logger *log.Logger) *Handler {
	return &Handler{
		store:  store,
		logger: logger,
	}
}

func (h *Handler) Routes() http.Handler {
	// 注册路由与中间件
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", h.handleHealth)
	r.Get("/stats", h.handleStats)

	return r
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	// 健康检查
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	// 统计汇总
	summary, err := h.store.Summary(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load stats")
		return
	}
	h.writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	// 统一 JSON 响应输出
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Printf("json encode error: %v", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	// 错误响应包装
	h.writeJSON(w, status, map[string]string{"error": message})
}

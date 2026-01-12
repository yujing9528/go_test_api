package todo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
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

	r.Route("/todos", func(r chi.Router) {
		r.Get("/", h.handleListTodos)
		r.Post("/", h.handleCreateTodo)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGetTodo)
			r.Put("/", h.handleUpdateTodo)
			r.Delete("/", h.handleDeleteTodo)
		})
	})

	r.Route("/practice", func(r chi.Router) {
		r.Get("/concurrency", h.handlePracticeConcurrency)
		r.Get("/interface", h.handlePracticeInterface)
		r.Get("/range", h.handlePracticeRange)
		r.Get("/slice", h.handlePracticeSlice)
		r.Get("/map", h.handlePracticeMap)
	})

	return r
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	// 健康检查
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleListTodos(w http.ResponseWriter, r *http.Request) {
	// 列表查询
	ctx := r.Context()
	items, err := h.store.List(ctx)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load todos")
		return
	}
	response := map[string]any{"todos": items}
	h.writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleGetTodo(w http.ResponseWriter, r *http.Request) {
	// 单条查询
	id, err := readIDParam(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	todo, err := h.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.writeError(w, http.StatusNotFound, "todo not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to load todo")
		return
	}

	h.writeJSON(w, http.StatusOK, todo)
}

func (h *Handler) handleCreateTodo(w http.ResponseWriter, r *http.Request) {
	// 创建资源
	var input createTodoRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		h.writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	todo, err := h.store.Create(r.Context(), input)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create todo")
		return
	}

	h.writeJSON(w, http.StatusCreated, todo)
}

func (h *Handler) handleUpdateTodo(w http.ResponseWriter, r *http.Request) {
	// 更新资源（支持部分字段）
	id, err := readIDParam(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var input updateTodoRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Title == nil && input.Done == nil {
		h.writeError(w, http.StatusBadRequest, "provide title or done")
		return
	}

	if input.Title != nil {
		trimmed := strings.TrimSpace(*input.Title)
		if trimmed == "" {
			h.writeError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		input.Title = &trimmed
	}

	todo, err := h.store.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.writeError(w, http.StatusNotFound, "todo not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to update todo")
		return
	}

	h.writeJSON(w, http.StatusOK, todo)
}

func (h *Handler) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	// 删除资源
	id, err := readIDParam(r)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	deleted, err := h.store.Delete(r.Context(), id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to delete todo")
		return
	}
	if !deleted {
		h.writeError(w, http.StatusNotFound, "todo not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// 限制请求体大小并严格解析 JSON
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("body must contain a single JSON object")
	}
	return nil
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

func readIDParam(r *http.Request) (int64, error) {
	// 解析并校验路径参数
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

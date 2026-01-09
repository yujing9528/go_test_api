package main

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

type app struct {
	cfg    Config
	db     *sql.DB
	store  *todoStore
	logger *log.Logger
}

func newApp(cfg Config, db *sql.DB, logger *log.Logger) *app {
	return &app{
		cfg:    cfg,
		db:     db,
		store:  newTodoStore(db),
		logger: logger,
	}
}

func (a *app) routes() http.Handler {
	// 注册路由与中间件
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", a.handleHealth)

	r.Route("/todos", func(r chi.Router) {
		r.Get("/", a.handleListTodos)
		r.Post("/", a.handleCreateTodo)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", a.handleGetTodo)
			r.Put("/", a.handleUpdateTodo)
			r.Delete("/", a.handleDeleteTodo)
		})
	})

	return r
}

func (a *app) handleHealth(w http.ResponseWriter, r *http.Request) {
	// 健康检查
	a.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) handleListTodos(w http.ResponseWriter, r *http.Request) {
	// 列表查询
	ctx := r.Context()
	items, err := a.store.List(ctx)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to load todos")
		return
	}
	response := map[string]any{"todos": items}
	a.writeJSON(w, http.StatusOK, response)
}

func (a *app) handleGetTodo(w http.ResponseWriter, r *http.Request) {
	// 单条查询
	id, err := readIDParam(r)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	todo, err := a.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			a.writeError(w, http.StatusNotFound, "todo not found")
			return
		}
		a.writeError(w, http.StatusInternalServerError, "failed to load todo")
		return
	}

	a.writeJSON(w, http.StatusOK, todo)
}

func (a *app) handleCreateTodo(w http.ResponseWriter, r *http.Request) {
	// 创建资源
	var input createTodoRequest
	if err := a.decodeJSON(w, r, &input); err != nil {
		a.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		a.writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	todo, err := a.store.Create(r.Context(), input)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to create todo")
		return
	}

	a.writeJSON(w, http.StatusCreated, todo)
}

func (a *app) handleUpdateTodo(w http.ResponseWriter, r *http.Request) {
	// 更新资源（支持部分字段）
	id, err := readIDParam(r)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var input updateTodoRequest
	if err := a.decodeJSON(w, r, &input); err != nil {
		a.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Title == nil && input.Done == nil {
		a.writeError(w, http.StatusBadRequest, "provide title or done")
		return
	}

	if input.Title != nil {
		trimmed := strings.TrimSpace(*input.Title)
		if trimmed == "" {
			a.writeError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		input.Title = &trimmed
	}

	todo, err := a.store.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			a.writeError(w, http.StatusNotFound, "todo not found")
			return
		}
		a.writeError(w, http.StatusInternalServerError, "failed to update todo")
		return
	}

	a.writeJSON(w, http.StatusOK, todo)
}

func (a *app) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	// 删除资源
	id, err := readIDParam(r)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	deleted, err := a.store.Delete(r.Context(), id)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "failed to delete todo")
		return
	}
	if !deleted {
		a.writeError(w, http.StatusNotFound, "todo not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *app) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
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

func (a *app) writeJSON(w http.ResponseWriter, status int, v any) {
	// 统一 JSON 响应输出
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		a.logger.Printf("json encode error: %v", err)
	}
}

func (a *app) writeError(w http.ResponseWriter, status int, message string) {
	// 错误响应包装
	a.writeJSON(w, status, map[string]string{"error": message})
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

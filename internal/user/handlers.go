package user

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var errUnauthorized = errors.New("unauthorized")

const (
	minPasswordLength = 8
)

type Handler struct {
	store      *Store
	logger     *log.Logger
	sessionTTL time.Duration
	resetTTL   time.Duration
}

func NewHandler(store *Store, logger *log.Logger) *Handler {
	return &Handler{
		store:      store,
		logger:     logger,
		sessionTTL: 24 * time.Hour,
		resetTTL:   30 * time.Minute,
	}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", h.handleHealth)

	r.Route("/users", func(r chi.Router) {
		r.Post("/register", h.handleRegister)
		r.Post("/login", h.handleLogin)
		r.Post("/password/forgot", h.handleForgotPassword)
		r.Post("/password/reset", h.handleResetPassword)
		r.Get("/me", h.handleGetProfile)
		r.Put("/me", h.handleUpdateProfile)
	})

	return r
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := normalizeEmail(input.Email)
	name := strings.TrimSpace(input.Name)

	if !isValidEmail(email) {
		h.writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(input.Password) < minPasswordLength {
		h.writeError(w, http.StatusBadRequest, "password too short")
		return
	}

	hash, err := hashPassword(input.Password)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.store.CreateUser(r.Context(), email, name, hash)
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			h.writeError(w, http.StatusConflict, "email already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	h.writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input LoginRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := normalizeEmail(input.Email)
	if !isValidEmail(email) {
		h.writeError(w, http.StatusBadRequest, "invalid email")
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}

	if err := verifyPassword(user.PasswordHash, input.Password); err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := generateToken(32)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	expiresAt := time.Now().Add(h.sessionTTL)
	session, err := h.store.CreateSession(r.Context(), user.ID, token, expiresAt)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	h.writeJSON(w, http.StatusOK, LoginResponse{User: user, Session: session})
}

func (h *Handler) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var input ForgotPasswordRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := normalizeEmail(input.Email)
	if !isValidEmail(email) {
		h.writeError(w, http.StatusBadRequest, "invalid email")
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.writeJSON(w, http.StatusOK, map[string]string{"message": "if the account exists, a reset token was generated"})
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	token, err := generateToken(32)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	expiresAt := time.Now().Add(h.resetTTL)
	if err := h.store.CreatePasswordReset(r.Context(), user.ID, token, expiresAt); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	response := map[string]any{
		"token":      token,
		"expires_at": expiresAt,
		"message":    "use token to reset password",
	}
	h.writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var input ResetPasswordRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	token := strings.TrimSpace(input.Token)
	if token == "" {
		h.writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if len(input.NewPassword) < minPasswordLength {
		h.writeError(w, http.StatusBadRequest, "password too short")
		return
	}

	hash, err := hashPassword(input.NewPassword)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if _, err := h.store.ConsumePasswordReset(r.Context(), token, hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.writeError(w, http.StatusBadRequest, "invalid or expired token")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

func (h *Handler) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	user, err := h.authenticate(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	currentUser, err := h.authenticate(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input UpdateProfileRequest
	if err := h.decodeJSON(w, r, &input); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if input.Email == nil && input.Name == nil {
		h.writeError(w, http.StatusBadRequest, "provide email or name")
		return
	}

	if input.Email != nil {
		value := normalizeEmail(*input.Email)
		if !isValidEmail(value) {
			h.writeError(w, http.StatusBadRequest, "invalid email")
			return
		}
		input.Email = &value
	}

	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" {
			h.writeError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		input.Name = &value
	}

	user, err := h.store.UpdateUser(r.Context(), currentUser.ID, input)
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			h.writeError(w, http.StatusConflict, "email already exists")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			h.writeError(w, http.StatusNotFound, "user not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	h.writeJSON(w, http.StatusOK, user)
}

func (h *Handler) authenticate(r *http.Request) (User, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return User{}, errUnauthorized
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return User{}, errUnauthorized
	}

	user, err := h.store.GetUserBySessionToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, errUnauthorized
		}
		return User{}, err
	}

	return user, nil
}

func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Printf("json encode error: %v", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isValidEmail(value string) bool {
	return value != "" && strings.Contains(value, "@")
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func verifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func generateToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

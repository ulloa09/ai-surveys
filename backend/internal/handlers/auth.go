package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// AuthServicer is the interface the auth handlers depend on.
// *services.AuthService satisfies it.
type AuthServicer interface {
	Register(ctx context.Context, email, password, displayName string) (*models.User, error)
	Login(ctx context.Context, email, password string) (*models.User, string, error)
	Logout(ctx context.Context, token string) error
}

// Register handles POST /api/auth/register.
// 201 on success, 409 on duplicate email, 400 on invalid input.
func Register(svc AuthServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email       string `json:"email"`
			Password    string `json:"password"`
			DisplayName string `json:"display_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		body.Email = strings.TrimSpace(body.Email)
		body.Password = strings.TrimSpace(body.Password)

		if body.Email == "" || body.Password == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
			return
		}

		user, err := svc.Register(r.Context(), body.Email, body.Password, body.DisplayName)
		if err != nil {
			if errors.Is(err, services.ErrEmailTaken) {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, user)
	}
}

// Login handles POST /api/auth/login.
// Sets an HTTP-only session cookie and returns the user on success.
// Returns 401 on invalid credentials.
func Login(svc AuthServicer, appEnv string, maxAgeSec int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		user, token, err := svc.Login(r.Context(), body.Email, body.Password)
		if err != nil {
			if errors.Is(err, services.ErrInvalidCredentials) {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   appEnv == "production",
			MaxAge:   maxAgeSec,
		})

		writeJSON(w, http.StatusOK, user)
	}
}

// Logout handles POST /api/auth/logout.
// Clears the session cookie and deletes the session from the database.
// Idempotent: returns 200 even if no cookie is present.
func Logout(svc AuthServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("session"); err == nil {
			_ = svc.Logout(r.Context(), cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// Me handles GET /api/admin/me.
// Requires the Authenticate middleware to have run — user is read from context.
func Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		writeJSON(w, http.StatusOK, user)
	}
}

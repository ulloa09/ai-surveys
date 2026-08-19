package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

// SessionValidator is the minimal interface Authenticate needs.
// *services.AuthService satisfies it.
type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (*models.User, error)
}

// Authenticate validates the session cookie and stores the user in the request
// context. Returns 401 and aborts without calling next on failure.
func Authenticate(svc SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			user, err := svc.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthenticate resolves the session cookie like Authenticate but never
// rejects the request: a missing or invalid session simply leaves the context
// without a user. Lo usan las rutas de respondiente, donde el requisito de
// sesión depende del anonymity_level de la encuesta — un dato que solo se
// conoce después de resolver el token, ya dentro del handler. Las encuestas
// 'full' (anónimas) se contestan sin cuenta; las demás siguen exigiendo sesión
// y membresía del equipo, y son los handlers quienes devuelven el 401.
func OptionalAuthenticate(svc SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			user, err := svc.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the authenticated user stored by Authenticate.
// Returns nil if the middleware was not applied or auth failed.
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

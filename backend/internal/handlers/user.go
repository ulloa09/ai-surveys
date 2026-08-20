package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// UserServicer es la interfaz que los handlers de gestion de usuarios necesitan.
// *services.UserService la satisface. Uso exclusivo de super_admin — la
// autorizacion se aplica con appmw.RequireSuperAdmin() en las rutas.
type UserServicer interface {
	List(ctx context.Context, search, role string) ([]models.User, error)
	Get(ctx context.Context, id string) (*models.User, error)
	Create(ctx context.Context, email, password, displayName, role string) (*models.User, error)
	Update(ctx context.Context, id, email, displayName string) (*models.User, error)
	ChangeRole(ctx context.Context, id, newRole, callerID string) (*models.User, error)
	ResetPassword(ctx context.Context, id, newPassword string) error
	Delete(ctx context.Context, id, callerID string) error
}

// ListUsers handles GET /api/admin/users.
// Acepta ?search=texto (email o nombre) y ?role=super_admin|admin|viewer.
func ListUsers(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		role := r.URL.Query().Get("role")

		users, err := svc.List(r.Context(), search, role)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list users"})
			return
		}
		if users == nil {
			users = []models.User{}
		}
		writeJSON(w, http.StatusOK, users)
	}
}

// GetUser handles GET /api/admin/users/{userID}.
func GetUser(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUIDParam(w, r, "userID", "user id")
		if !ok {
			return
		}

		user, err := svc.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, services.ErrUserNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not get user"})
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}

// CreateUser handles POST /api/admin/users.
// El super_admin elige el rol explicitamente al crear la cuenta.
func CreateUser(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Email       string `json:"email"`
			Password    string `json:"password"`
			DisplayName string `json:"display_name"`
			Role        string `json:"role"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		body.Email = strings.TrimSpace(body.Email)
		body.DisplayName = strings.TrimSpace(body.DisplayName)

		if body.Email == "" || body.Password == "" || body.DisplayName == "" || body.Role == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, password, display_name and role are required"})
			return
		}

		user, err := svc.Create(r.Context(), body.Email, body.Password, body.DisplayName, body.Role)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrEmailTaken):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			case errors.Is(err, services.ErrInvalidRole):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be super_admin, admin, profesor, or alumno"})
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return
		}

		writeJSON(w, http.StatusCreated, user)
	}
}

// UpdateUser handles PATCH /api/admin/users/{userID}.
// Actualiza email y nombre. El rol y la contraseña se cambian con endpoints separados.
func UpdateUser(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUIDParam(w, r, "userID", "user id")
		if !ok {
			return
		}

		var body struct {
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		user, err := svc.Update(r.Context(), id, body.Email, body.DisplayName)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			case errors.Is(err, services.ErrEmailTaken):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}

// ChangeUserRole handles PATCH /api/admin/users/{userID}/role.
// No permite que el super_admin autenticado se cambie su propio rol,
// ni degradar al ultimo super_admin de la plataforma.
func ChangeUserRole(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUIDParam(w, r, "userID", "user id")
		if !ok {
			return
		}

		var body struct {
			Role string `json:"role"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		caller := appmw.UserFromContext(r.Context())

		user, err := svc.ChangeRole(r.Context(), id, body.Role, caller.ID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			case errors.Is(err, services.ErrInvalidRole):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be super_admin, admin, profesor, or alumno"})
			case errors.Is(err, services.ErrCannotModifySelf):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot change your own role"})
			case errors.Is(err, services.ErrLastSuperAdmin):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot remove the last super admin"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not change role"})
			}
			return
		}

		writeJSON(w, http.StatusOK, user)
	}
}

// ResetUserPassword handles PATCH /api/admin/users/{userID}/password.
// El super_admin establece la contraseña nueva directamente; invalida las
// sesiones activas del usuario para forzar un nuevo login.
func ResetUserPassword(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUIDParam(w, r, "userID", "user id")
		if !ok {
			return
		}

		var body struct {
			NewPassword string `json:"new_password"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if err := svc.ResetPassword(r.Context(), id, body.NewPassword); err != nil {
			if errors.Is(err, services.ErrUserNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// DeleteUser handles DELETE /api/admin/users/{userID}.
// No permite que el super_admin autenticado se elimine a si mismo, eliminar
// al ultimo super_admin, ni eliminar usuarios que son dueños de encuestas o
// que crearon equipos.
func DeleteUser(svc UserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUIDParam(w, r, "userID", "user id")
		if !ok {
			return
		}

		caller := appmw.UserFromContext(r.Context())

		if err := svc.Delete(r.Context(), id, caller.ID); err != nil {
			switch {
			case errors.Is(err, services.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			case errors.Is(err, services.ErrCannotModifySelf):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot delete your own account"})
			case errors.Is(err, services.ErrLastSuperAdmin):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot remove the last super admin"})
			case errors.Is(err, services.ErrUserHasOwnedSurveys):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot delete a user who owns surveys"})
			case errors.Is(err, services.ErrUserHasCreatedTeams):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "cannot delete a user who created teams"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete user"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

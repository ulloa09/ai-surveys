package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// teamRoleKey es la clave con la que guardamos el rol del equipo en el context.
// Usamos un tipo propio para evitar colisiones con otras claves del context.
type teamRoleKey struct{}

// TeamRoleChecker es la interfaz mínima que necesita el middleware.
// *services.TeamService la satisface.
// Usar interfaz en vez del struct directo permite testear sin DB real.
type TeamRoleChecker interface {
	GetMemberRole(ctx context.Context, teamID, userID string) (string, error)
}

// RequireTeamMember verifica que el usuario autenticado pertenezca al equipo
// indicado en el parámetro de URL {teamID}.
//
// Si el usuario es super_admin, bypasea la verificación — puede acceder a todo.
// Si pertenece al equipo, guarda su rol en el context para que
// RequireTeamRole lo pueda leer sin otra query a la DB.
// Si no pertenece, devuelve 403.

func RequireTeamMember(svc TeamRoleChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				// Authenticate no corrió — error de configuración
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}

			// Super admin y admin (Coordinador) bypasean el chequeo de equipo —
			// el Coordinador administra equipos/grupos de forma transversal,
			// no solo los que el mismo creó o donde es owner.
			if user.Role == "super_admin" || user.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			teamID := chi.URLParam(r, "teamID")
			if teamID == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "teamID is required"})
				return
			}

			// Buscar el rol del usuario en este equipo
			role, err := svc.GetMemberRole(r.Context(), teamID, user.ID)
			if err != nil {
				// ErrMemberNotFound o cualquier error → 403
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
				return
			}

			// Guardamos el rol en el context para no repetir la query en el handler
			ctx := context.WithValue(r.Context(), teamRoleKey{}, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTeamRole verifica que el usuario tenga uno de los roles permitidos
// dentro del equipo. Debe encadenarse DESPUÉS de RequireTeamMember.
//
// Ejemplo — solo profesores pueden ver el detalle:
//
//	r.Get("/surveys/{id}", appmw.RequireTeamRole("profesor"), handlers.GetSurvey)
func RequireTeamRole(allowed ...string) func(http.Handler) http.Handler {
	// Convertimos a mapa para búsqueda O(1)
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())

			// Super admin y admin (Coordinador) siempre pueden
			if user != nil && (user.Role == "super_admin" || user.Role == "admin") {
				next.ServeHTTP(w, r)
				return
			}

			// Leer el rol que RequireTeamMember guardó en el context
			role, ok := r.Context().Value(teamRoleKey{}).(string)
			if !ok || role == "" {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
				return
			}

			if _, allowed := allowedSet[role]; !allowed {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireSuperAdmin es un shortcut para endpoints que solo super_admin puede usar.
func RequireSuperAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || user.Role != "super_admin" {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "super admin required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TeamRoleFromContext lee el rol de equipo guardado por RequireTeamMember.
// Devuelve "" si no está en el context.
func TeamRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(teamRoleKey{}).(string)
	return role
}

// RequireAdminRole permite el acceso solo a usuarios con rol admin o super_admin.
// Lo usan los endpoints de creacion de equipos y gestion de la plataforma.
func RequireAdminRole() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			if user.Role != "admin" && user.Role != "super_admin" {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole permite el acceso solo a usuarios cuyo rol este en la lista
// indicada. super_admin siempre puede pasar, sin importar la lista — es el
// gate generico para la matriz de permisos de la Fase 3 (RBAC), usado para
// scoping fino por endpoint (ej. profesor puede leer encuestas pero no
// crearlas; alumno no puede tocar ninguna ruta de gestion).
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			if user.Role == "super_admin" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := allowedSet[user.Role]; !ok {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

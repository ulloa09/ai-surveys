package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// TeamServicer es la interfaz que los handlers de teams necesitan.
// *services.TeamService la satisface.
type TeamServicer interface {
	Create(ctx context.Context, name, createdByID string) (*models.Team, error)
	List(ctx context.Context, name, order, year, month string) ([]models.Team, error)
	ListForUser(ctx context.Context, userID, name, order, year, month string) ([]models.Team, error)
	GetWithMembers(ctx context.Context, teamID, member string) (*models.TeamWithMembers, error)
	AddMember(ctx context.Context, teamID, email string) (*models.TeamMember, error)
	RemoveMember(ctx context.Context, teamID, userID string) error
	MoveMember(ctx context.Context, fromTeamID, toTeamID, userID string) (*models.TeamMember, error)
}

// CreateTeam handles POST /api/admin/teams
// Solo admins y super_admins pueden crear equipos. El creador no se agrega
// como miembro — administra el equipo vía su rol global (ver Rbac.go).
func CreateTeam(svc TeamServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}

		user := appmw.UserFromContext(r.Context())

		team, err := svc.Create(r.Context(), body.Name, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create team"})
			return
		}

		writeJSON(w, http.StatusCreated, team)
	}
}

// ListTeams handles GET /api/admin/teams.
// Acepta ?name=texto para filtrar equipos por nombre (busqueda parcial).
// Super admins y admins (Coordinador) ven todos los equipos — el Coordinador
// administra grupos de forma transversal. Profesores y alumnos solo ven
// los equipos/grupos de los que son miembros.
func ListTeams(svc TeamServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		name := r.URL.Query().Get("name")
		order := r.URL.Query().Get("order")
		year := r.URL.Query().Get("year")
		month := r.URL.Query().Get("month")

		var (
			teams []models.Team
			err   error
		)

		if user.Role == "super_admin" || user.Role == "admin" {
			teams, err = svc.List(r.Context(), name, order, year, month)
		} else {
			teams, err = svc.ListForUser(r.Context(), user.ID, name, order, year, month)
		}

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list teams"})
			return
		}
		if teams == nil {
			teams = []models.Team{}
		}
		writeJSON(w, http.StatusOK, teams)
	}
}

// GetTeam handles GET /api/admin/teams/{teamID}.
// Acepta ?member=texto para filtrar miembros por email o nombre (busqueda parcial).
// RequireTeamMember ya verifico que el usuario tiene acceso.
func GetTeam(svc TeamServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "teamID")
		member := r.URL.Query().Get("member")

		team, err := svc.GetWithMembers(r.Context(), teamID, member)
		if err != nil {
			if errors.Is(err, services.ErrTeamNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "team not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not get team"})
			return
		}

		writeJSON(w, http.StatusOK, team)
	}
}

// AddMember handles POST /api/admin/teams/{teamID}/members
// Invita a un usuario por email. Su rol dentro del equipo se toma
// automáticamente de su rol de cuenta (profesor o alumno) — no se elige a
// mano, así nunca queda desalineado con lo que esa persona puede hacer en el
// resto de la plataforma. Solo admin/super_admin pueden invitar (ver
// RequireAdminRole en main.go).
func AddMember(svc TeamServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "teamID")

		var body struct {
			Email string `json:"email"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Email == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
			return
		}

		member, err := svc.AddMember(r.Context(), teamID, body.Email)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			case errors.Is(err, services.ErrAlreadyMember):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "user is already a member"})
			case errors.Is(err, services.ErrInvalidMemberRole):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "solo se pueden agregar cuentas con rol Profesor o Alumno a un equipo"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not add member"})
			}
			return
		}

		writeJSON(w, http.StatusCreated, member)
	}
}

// RemoveMember handles DELETE /api/admin/teams/{teamID}/members/{userID}
// Solo admin/super_admin pueden remover miembros (ver RequireAdminRole).
func RemoveMember(svc TeamServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "teamID")
		userID := chi.URLParam(r, "userID")

		if err := svc.RemoveMember(r.Context(), teamID, userID); err != nil {
			switch {
			case errors.Is(err, services.ErrMemberNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not remove member"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// MoveMember handles POST /api/admin/teams/{teamID}/members/{userID}/move
// Mueve a un miembro del equipo {teamID} (origen) a otro equipo (destino),
// preservando su rol dentro del equipo. Pensado para que un Coordinador
// mueva alumnos entre grupos de forma atómica. Solo admin/super_admin
// pueden moverlo (ver RequireAdminRole).
func MoveMember(svc TeamServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fromTeamID := chi.URLParam(r, "teamID")
		userID := chi.URLParam(r, "userID")

		var body struct {
			ToTeamID string `json:"to_team_id"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.ToTeamID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to_team_id is required"})
			return
		}
		if body.ToTeamID == fromTeamID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to_team_id must be different from the current team"})
			return
		}

		member, err := svc.MoveMember(r.Context(), fromTeamID, body.ToTeamID, userID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrMemberNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found in the source team"})
			case errors.Is(err, services.ErrAlreadyMember):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "user is already a member of the destination team"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not move member"})
			}
			return
		}

		writeJSON(w, http.StatusOK, member)
	}
}

// decodeJSON decodifica el body JSON del request en v.
func decodeJSON(r *http.Request, v any) error {
	return jsonDecoder(r).Decode(v)
}

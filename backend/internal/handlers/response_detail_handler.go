// internal/handlers/response_detail_handler.go
// Endpoints autenticados de reanudación del lado del servidor (issue #14):
//
//	GET  /api/surveys/{id}/my-response  — respuesta reanudable del usuario
//	GET  /api/responses/{id}            — vista consolidada, con control de acceso
//	POST /api/responses/{id}/abandon    — descartar la sesión previa
//
// A diferencia de los endpoints públicos que identifican al respondiente por
// response_id, estos derivan la identidad de la sesión y aplican control de
// acceso por dueño / equipo, sin tocar el flujo anónimo existente.
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// MyResponseServicer expone la búsqueda de la respuesta reanudable del usuario.
type MyResponseServicer interface {
	GetMyResponse(ctx context.Context, surveyID, userID string) (*models.Response, error)
}

// ResponseDetailServicer expone la vista consolidada y el abandono de respuestas.
type ResponseDetailServicer interface {
	GetResponseDetail(ctx context.Context, responseID string) (*services.ResponseDetail, *models.Survey, error)
	GetResponseSurvey(ctx context.Context, responseID string) (*models.Response, *models.Survey, error)
	AbandonResponse(ctx context.Context, responseID string) error
}

// TeamRoleReader es la porción de *services.TeamService que necesita el control
// de acceso para saber el rol del usuario dentro del equipo de la encuesta.
type TeamRoleReader interface {
	GetMemberRole(ctx context.Context, teamID, userID string) (string, error)
}

// GetMyResponse maneja GET /api/surveys/{id}/my-response.
// Devuelve la respuesta en progreso o pendiente de envío del usuario autenticado
// para esta encuesta, o {"response": null} si no tiene ninguna reanudable.
func GetMyResponse(svc MyResponseServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		surveyID := chi.URLParam(r, "id")

		response, err := svc.GetMyResponse(r.Context(), surveyID, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"response": response})
	}
}

// GetResponseDetail maneja GET /api/responses/{id}.
// Devuelve la vista consolidada de la respuesta (transcript, preguntas
// contestadas, índice actual y status). Aplica control de acceso: solo el dueño
// de la respuesta, el profesor del equipo de la encuesta, el dueño de la
// encuesta, un admin o un super_admin pueden leerla; cualquier otro recibe 403.
func GetResponseDetail(svc ResponseDetailServicer, teamSvc TeamRoleReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		responseID := chi.URLParam(r, "id")

		detail, survey, err := svc.GetResponseDetail(r.Context(), responseID)
		if err != nil {
			if errors.Is(err, services.ErrResponseNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		if !canAccessResponse(r.Context(), teamSvc, user, detail.Response, survey) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}

		writeJSON(w, http.StatusOK, detail)
	}
}

// AbandonResponse maneja POST /api/responses/{id}/abandon.
// Marca la respuesta como abandonada. Aplica el mismo control de acceso que la
// lectura consolidada. Es idempotente sobre sesiones ya terminadas.
func AbandonResponse(svc ResponseDetailServicer, teamSvc TeamRoleReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		responseID := chi.URLParam(r, "id")

		response, survey, err := svc.GetResponseSurvey(r.Context(), responseID)
		if err != nil {
			if errors.Is(err, services.ErrResponseNotFound) || errors.Is(err, services.ErrResponseSurveyNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		if !canAccessResponse(r.Context(), teamSvc, user, response, survey) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}

		if err := svc.AbandonResponse(r.Context(), responseID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// canAccessResponse decide si el usuario puede leer/mutar una respuesta.
// Permite: super_admin, admin (Coordinador), el dueño de la respuesta, el
// dueño de la encuesta, o el profesor del equipo dueño de la encuesta (para
// que pueda trackear el status de las respuestas de su grupo). Un alumno
// miembro del equipo NO obtiene acceso por esta vía — solo a su propia
// respuesta, ya cubierto arriba.
func canAccessResponse(ctx context.Context, teamSvc TeamRoleReader, user *models.User, response *models.Response, survey *models.Survey) bool {
	if user.Role == "super_admin" || user.Role == "admin" {
		return true
	}
	if response.UserID != nil && *response.UserID == user.ID {
		return true
	}
	if survey.OwnerID == user.ID {
		return true
	}
	if role, err := teamSvc.GetMemberRole(ctx, survey.TeamID, user.ID); err == nil {
		if role == "profesor" {
			return true
		}
	}
	return false
}

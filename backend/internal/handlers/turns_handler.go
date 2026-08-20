// internal/handlers/turns_handler.go
// Devuelve el historial de turnos de una respuesta para que el frontend
// pueda reconstruir el chat al reanudar una sesión (allow_revisit).
package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

type TurnsServicer interface {
	GetResponseStatus(ctx context.Context, responseID string) (string, error)
	GetTurns(ctx context.Context, responseID string) ([]services.Turn, error)
}

func GetTurns(svc TurnsServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")
		turns, err := svc.GetTurns(r.Context(), responseID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"turns": turns})
	}
}

// Busca la respuesta más reciente del usuario autenticado para una encuesta
// específica. Lo usa el landing cuando allow_revisit = true y el usuario ya
// contestó, para redirigirlo al chat en vez de mostrar error.
//
// El user_id se deriva SIEMPRE de la sesión autenticada, nunca de un query
// param — un endpoint público que confiara en un ?user_id= provisto por el
// cliente permitiría a cualquiera (sin sesión) leer la respuesta de otro
// usuario con solo adivinar su id (IDOR).
type ResponseByUserServicer interface {
	GetResponseByUser(ctx context.Context, surveyID, userID string) (*models.Response, error)
}

func GetResponseByUser(svc ResponseByUserServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		surveyID := r.URL.Query().Get("survey_id")
		if surveyID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "survey_id es requerido"})
			return
		}

		response, err := svc.GetResponseByUser(r.Context(), surveyID, user.ID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}

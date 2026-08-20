// internal/handlers/turns_handler.go
// Devuelve el historial de turnos de una respuesta para que el frontend
// pueda reconstruir el chat al reanudar una sesión (allow_revisit).
package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

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

// internal/handlers/response_status.go
// Handler para GET /api/responses/{id}/status.
// Devuelve el status actual de la respuesta para que el frontend
// detecte si el engine la marcó como pending_submission.
package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ResponseStatusServicer es la interfaz mínima que necesita este handler.
type ResponseStatusServicer interface {
	GetResponseStatus(ctx context.Context, responseID string) (string, error)
}

// GetResponseStatus maneja GET /api/responses/{id}/status.
func GetResponseStatus(svc ResponseStatusServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")
		status, err := svc.GetResponseStatus(r.Context(), responseID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"status": "unknown"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": status})
	}
}

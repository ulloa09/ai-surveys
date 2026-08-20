// internal/handlers/open_answer.go
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

type OpenAnswerServicer interface {
	RecordOpenEndedAnswer(ctx context.Context, responseID, questionID, value string) error
}

// RecordOpenAnswer maneja POST /api/responses/{id}/open-answer
// Registra respuesta open_ended sin avanzar el índice.
// El engine avanza después de recibir el followup del usuario.
//
// Respuestas basura devuelven códigos que el frontend traduce a UX amigable:
//   - 422 low_quality_answer  → pedir al usuario que reformule (una vez)
//   - 409 response_blocked    → la sesión se cerró por respuestas inválidas repetidas
func RecordOpenAnswer(svc OpenAnswerServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")
		var body struct {
			QuestionID string `json:"question_id"`
			Value      string `json:"value"`
		}
		if err := decodeJSON(r, &body); err != nil || body.QuestionID == "" || body.Value == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question_id y value son requeridos"})
			return
		}
		if err := svc.RecordOpenEndedAnswer(r.Context(), responseID, body.QuestionID, body.Value); err != nil {
			switch {
			case errors.Is(err, services.ErrLowQualityAnswer):
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "low_quality_answer"})
			case errors.Is(err, services.ErrResponseBlocked):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "response_blocked"})
			case errors.Is(err, services.ErrResponseNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			case errors.Is(err, services.ErrResponseNotInProgress):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "response is not in progress"})
			case errors.Is(err, services.ErrInvalidAnswer):
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid answer"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "no se pudo registrar"})
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

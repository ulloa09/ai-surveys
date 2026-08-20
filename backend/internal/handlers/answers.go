// internal/handlers/answers.go
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// AnswerServicer agrupa los métodos que necesitan los handlers de answers.
type AnswerServicer interface {
	GetAnsweredQuestionIDs(ctx context.Context, responseID string) ([]string, error)
	GetAnswersWithValues(ctx context.Context, responseID string) ([]map[string]string, error)
	RecordAnswerDirect(ctx context.Context, responseID, questionID, value string) error
}

// GetAnswers maneja GET /api/responses/{id}/answers
// Si withValues=true devuelve los valores además de los IDs (para allow_revisit).
// Público — el respondiente lo usa para sincronizar su progreso.
func GetAnswers(svc AnswerServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")
		withValues := r.URL.Query().Get("withValues") == "true"

		if withValues {
			answers, err := svc.GetAnswersWithValues(r.Context(), responseID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			if answers == nil {
				answers = []map[string]string{}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"answers": answers})
			return
		}

		ids, err := svc.GetAnsweredQuestionIDs(r.Context(), responseID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if ids == nil {
			ids = []string{}
		}
		writeJSON(w, http.StatusOK, map[string][]string{"answered": ids})
	}
}

// RecordAnswer maneja POST /api/responses/{id}/answers
// El frontend manda el valor de una pregunta estructurada directamente.
// No pasa por el engine conversacional — se registra de inmediato.
func RecordAnswer(svc AnswerServicer) http.HandlerFunc {
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

		if err := svc.RecordAnswerDirect(r.Context(), responseID, body.QuestionID, body.Value); err != nil {
			switch {
			case errors.Is(err, services.ErrResponseNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			case errors.Is(err, services.ErrResponseNotInProgress):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "response is not in progress"})
			case errors.Is(err, services.ErrSurveyNotOpen):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "survey is not open"})
			case errors.Is(err, services.ErrInvalidAnswer):
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid answer"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

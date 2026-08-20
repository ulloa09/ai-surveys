package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// EngineServicer es la interfaz que los handlers del engine necesitan.
type EngineServicer interface {
	ProcessTurn(ctx context.Context, responseID, userMessage string) (*services.TurnResult, error)
	Submit(ctx context.Context, responseID string) error
	ExpireResponse(ctx context.Context, responseID string) (string, error)
}

// ProcessTurn maneja POST /api/responses/{id}/turns.
// Invoca el engine y hace streaming de la respuesta vía SSE. Al terminar el
// stream emite un evento `status` con el estado real de la respuesta (evaluado
// después de persistir el turno) para que el cliente sepa si la sesión terminó.
func ProcessTurn(engineSvc EngineServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")

		var body struct {
			Message string `json:"message"`
		}
		if err := decodeJSON(r, &body); err != nil || body.Message == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
			return
		}

		ctx := r.Context()
		result, err := engineSvc.ProcessTurn(ctx, responseID, body.Message)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrResponseNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			case errors.Is(err, services.ErrResponseNotInProgress):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "response is not in progress"})
			case errors.Is(err, services.ErrAIProviderUnavailable):
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI provider unavailable"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
			return
		}

		// SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
			return
		}

		for chunk := range result.TokenCh {
			if chunk.Error != nil {
				// Nunca exponer el error crudo del proveedor al respondiente.
				fmt.Fprintf(w, "event: error\ndata: AI provider unavailable\n\n")
				flusher.Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", chunk.Token)
			flusher.Flush()
		}

		// Estado final real, evaluado después de persistir el turno.
		if result.FinalStatus != nil {
			if finalStatus := <-result.FinalStatus; finalStatus != "" {
				fmt.Fprintf(w, "event: status\ndata: %s\n\n", finalStatus)
			}
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// SubmitResponse maneja POST /api/responses/{id}/submit.
// Valida cobertura de preguntas requeridas y transiciona a submitted.
// Devuelve 422 si hay preguntas requeridas sin respuesta.
func SubmitResponse(engineSvc EngineServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")
		ctx := r.Context()

		if err := engineSvc.Submit(ctx, responseID); err != nil {
			switch {
			case errors.Is(err, services.ErrResponseNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			case errors.Is(err, services.ErrResponseNotInProgress):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "response cannot be submitted in its current state"})
			case errors.Is(err, services.ErrAlreadySubmitted):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "response already submitted"})
			case errors.Is(err, services.ErrEmptyResponse):
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "response has no answers"})
			case errors.Is(err, services.ErrRequiredQuestionsOpen):
				var missingErr *services.MissingRequiredQuestionsError
				if errors.As(err, &missingErr) {
					writeJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
						"error":                      "required questions still unanswered",
						"missing_required_questions": missingErr.Missing,
					})
					return
				}
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "required questions still unanswered"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// ExpireResponse maneja POST /api/responses/{id}/expire.
// Lo invoca el cliente cuando el temporizador de time_estimate llega a cero.
// Si las preguntas requeridas están cubiertas, transiciona a pending_submission;
// si no, la sesión sigue en in_progress y el cliente debe explicar al usuario
// que aún faltan respuestas obligatorias. Devuelve el status resultante.
func ExpireResponse(engineSvc EngineServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseID := chi.URLParam(r, "id")
		ctx := r.Context()

		status, err := engineSvc.ExpireResponse(ctx, responseID)
		if err != nil {
			switch {
			case errors.Is(err, services.ErrResponseNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "status": status})
	}
}

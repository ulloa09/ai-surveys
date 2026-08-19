package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// QuestionServicer es la interfaz que los handlers de preguntas necesitan.
// *services.QuestionService la satisface.
type QuestionServicer interface {
	List(ctx context.Context, surveyID string) ([]models.Question, error)
	Create(ctx context.Context, surveyID string, in services.CreateQuestionInput) (*models.Question, error)
	Update(ctx context.Context, surveyID, questionID string, in services.UpdateQuestionInput) (*models.Question, error)
	Delete(ctx context.Context, surveyID, questionID string) error
	Reorder(ctx context.Context, surveyID string, orderedIDs []string) error
}

// ListQuestions maneja GET /api/surveys/{id}/questions.
// Devuelve todas las preguntas de la encuesta en orden.
// Antes de listar verifica que el usuario tenga acceso de lectura a la encuesta.
func ListQuestions(svc QuestionServicer, surveys SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		surveyID, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		// Verificar que la encuesta existe y que el usuario tiene acceso.
		if _, err := surveys.Get(r.Context(), user, surveyID); err != nil {
			writeSurveyError(w, err)
			return
		}

		questions, err := svc.List(r.Context(), surveyID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		if questions == nil {
			questions = []models.Question{}
		}

		writeJSON(w, http.StatusOK, questions)
	}
}

// CreateQuestion maneja POST /api/surveys/{id}/questions.
// Agrega una pregunta al final de la encuesta.
// Requiere permiso de escritura sobre el equipo de la encuesta.
func CreateQuestion(svc QuestionServicer, surveys SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		surveyID, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		// Verificar acceso de escritura: Get + el servicio de encuestas
		// ya aplica la lógica de roles internamente.
		if err := surveys.CheckWriteAccess(r.Context(), user, surveyID); err != nil {
			writeSurveyError(w, err)
			return
		}

		var body struct {
			Type       string          `json:"type"`
			Text       string          `json:"text"`
			Required   *bool           `json:"required"`
			AIFollowup *bool           `json:"ai_followup"`
			Options    json.RawMessage `json:"options"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		question, err := svc.Create(r.Context(), surveyID, services.CreateQuestionInput{
			Type:       body.Type,
			Text:       body.Text,
			Required:   body.Required,
			AIFollowup: body.AIFollowup,
			Options:    []byte(body.Options),
		})
		if err != nil {
			writeQuestionError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, question)
	}
}

// UpdateQuestion maneja PATCH /api/surveys/{id}/questions/{qid}.
// Aplica un PATCH parcial: solo se modifican los campos presentes en el body.
// Devuelve 409 si la encuesta ya tiene respuestas.
func UpdateQuestion(svc QuestionServicer, surveys SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		surveyID, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}
		questionID, ok := pathUUIDParam(w, r, "qid", "question id")
		if !ok {
			return
		}

		if err := surveys.CheckWriteAccess(r.Context(), user, surveyID); err != nil {
			writeSurveyError(w, err)
			return
		}

		var body struct {
			Type       *string         `json:"type"`
			Text       *string         `json:"text"`
			Required   *bool           `json:"required"`
			AIFollowup *bool           `json:"ai_followup"`
			Options    json.RawMessage `json:"options"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		question, err := svc.Update(r.Context(), surveyID, questionID, services.UpdateQuestionInput{
			Type:       body.Type,
			Text:       body.Text,
			Required:   body.Required,
			AIFollowup: body.AIFollowup,
			Options:    []byte(body.Options),
		})
		if err != nil {
			writeQuestionError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, question)
	}
}

// DeleteQuestion maneja DELETE /api/surveys/{id}/questions/{qid}.
// Devuelve 409 si la encuesta ya tiene respuestas.
func DeleteQuestion(svc QuestionServicer, surveys SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		surveyID, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}
		questionID, ok := pathUUIDParam(w, r, "qid", "question id")
		if !ok {
			return
		}

		if err := surveys.CheckWriteAccess(r.Context(), user, surveyID); err != nil {
			writeSurveyError(w, err)
			return
		}

		if err := svc.Delete(r.Context(), surveyID, questionID); err != nil {
			writeQuestionError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// ReorderQuestions maneja PUT /api/surveys/{id}/questions/order.
// Recibe la lista completa de ids en el nuevo orden y actualiza order_index de cada una.
func ReorderQuestions(svc QuestionServicer, surveys SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		surveyID, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		if err := surveys.CheckWriteAccess(r.Context(), user, surveyID); err != nil {
			writeSurveyError(w, err)
			return
		}

		var body struct {
			Order []string `json:"order"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if err := svc.Reorder(r.Context(), surveyID, body.Order); err != nil {
			writeQuestionError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// writeQuestionError traduce los errores del servicio de preguntas al código HTTP correspondiente.
func writeQuestionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrQuestionNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "question not found"})
	case errors.Is(err, services.ErrQuestionsLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "questions cannot be modified after the first response"})
	case errors.Is(err, services.ErrInvalidQuestion):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

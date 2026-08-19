package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// SurveyServicer es la interfaz que los handlers de encuestas necesitan.
// *services.SurveyService la satisface. Usar una interfaz permite inyectar
// un mock en los tests sin tocar la base de datos.
type SurveyServicer interface {
	Create(ctx context.Context, user *models.User, in services.CreateSurveyInput) (*models.Survey, error)
	List(ctx context.Context, user *models.User) ([]models.Survey, error)
	Get(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	Update(ctx context.Context, user *models.User, id string, in services.UpdateSurveyInput) (*models.Survey, error)
	Delete(ctx context.Context, user *models.User, id string) error
	Duplicate(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	CheckWriteAccess(ctx context.Context, user *models.User, surveyID string) error
}

// CreateSurvey maneja POST /api/surveys. El usuario debe ser admin/super_admin
// (ver RequireRole("admin") en main.go).
func CreateSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title                string  `json:"title"`
			Description          *string `json:"description"`
			TeamID               string  `json:"team_id"`
			AnonymityLevel       string  `json:"anonymity_level"`
			AllowRevisit         bool    `json:"allow_revisit"`
			OptionalRegistration bool    `json:"optional_registration"`
			Mode                 string  `json:"mode"`
			SystemPrompt         *string `json:"system_prompt"`
			TerminationMode      string    `json:"termination_mode"`
			TurnLimit            *int      `json:"turn_limit"`
			TimeEstimateMinutes  *int      `json:"time_estimate_minutes"`
			AvailableLanguages   *[]string `json:"available_languages"`
			DefaultLanguage      string    `json:"default_language"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if body.Title == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
			return
		}
		if body.TeamID == "" || !isUUID(body.TeamID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "team_id must be a valid uuid"})
			return
		}

		// El nivel de anonimato por defecto del schema es "none".
		if body.AnonymityLevel == "" {
			body.AnonymityLevel = "none"
		}
		if !services.ValidAnonymityLevels[body.AnonymityLevel] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "anonymity_level must be none, partial, or full"})
			return
		}

		// El modo por defecto es conversational (Mode C — hibrido recomendado).
		if body.Mode == "" {
			body.Mode = "conversational"
		}
		if !services.ValidModes[body.Mode] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be conversational, form, or prompt_only"})
			return
		}
		// Mode B (prompt_only) y Mode C (conversational) necesitan un system
		// prompt; Mode A (form, preguntas fijas) es el único donde es opcional.
		if body.Mode != "form" && (body.SystemPrompt == nil || strings.TrimSpace(*body.SystemPrompt) == "") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "system_prompt is required for prompt_only and conversational modes"})
			return
		}

		// El modo de terminación por defecto es turn_limit con 12 turnos.
		if body.TerminationMode == "" {
			body.TerminationMode = "turn_limit"
		}
		if !services.ValidTerminationModes[body.TerminationMode] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "termination_mode must be turn_limit, question_coverage, time_estimate, or combination"})
			return
		}
		if (body.TerminationMode == "turn_limit" || body.TerminationMode == "combination") && body.TurnLimit == nil {
			defaultTurnLimit := 12
			body.TurnLimit = &defaultTurnLimit
		}
		if (body.TerminationMode == "time_estimate" || body.TerminationMode == "combination") && body.TimeEstimateMinutes == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "time_estimate_minutes is required for time_estimate and combination termination modes"})
			return
		}

		user := appmw.UserFromContext(r.Context())
		survey, err := svc.Create(r.Context(), user, services.CreateSurveyInput{
			Title:                body.Title,
			Description:          body.Description,
			TeamID:               body.TeamID,
			AnonymityLevel:       body.AnonymityLevel,
			AllowRevisit:         body.AllowRevisit,
			OptionalRegistration: body.OptionalRegistration,
			Mode:                 body.Mode,
			SystemPrompt:         body.SystemPrompt,
			TerminationMode:      body.TerminationMode,
			TurnLimit:            body.TurnLimit,
			TimeEstimateMinutes:  body.TimeEstimateMinutes,
			AvailableLanguages:   body.AvailableLanguages,
			DefaultLanguage:      body.DefaultLanguage,
		})
		if err != nil {
			writeSurveyError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, survey)
	}
}

// ListSurveys maneja GET /api/surveys. Devuelve las encuestas visibles para
// el usuario (todas si es super_admin/admin, solo las de sus equipos si es profesor).
func ListSurveys(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())

		surveys, err := svc.List(r.Context(), user)
		if err != nil {
			writeSurveyError(w, err)
			return
		}
		if surveys == nil {
			surveys = []models.Survey{}
		}

		writeJSON(w, http.StatusOK, surveys)
	}
}

// GetSurvey maneja GET /api/surveys/{id}. Cualquier miembro del equipo de la
// encuesta puede leerla.
func GetSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		id, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		survey, err := svc.Get(r.Context(), user, id)
		if err != nil {
			writeSurveyError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, survey)
	}
}

// UpdateSurvey maneja PATCH /api/surveys/{id}. Aplica un PATCH parcial: solo
// se modifican los campos presentes en el body. Cambiar anonymity_level
// después de la primera respuesta devuelve 409.
func UpdateSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title                *string `json:"title"`
			Description          *string `json:"description"`
			AnonymityLevel       *string `json:"anonymity_level"`
			AllowRevisit         *bool   `json:"allow_revisit"`
			OptionalRegistration *bool   `json:"optional_registration"`
			Mode                 *string `json:"mode"`
			SystemPrompt         *string `json:"system_prompt"`
			TerminationMode      *string   `json:"termination_mode"`
			TurnLimit            *int      `json:"turn_limit"`
			TimeEstimateMinutes  *int      `json:"time_estimate_minutes"`
			AvailableLanguages   *[]string `json:"available_languages"`
			DefaultLanguage      *string   `json:"default_language"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if body.Title != nil && *body.Title == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title cannot be empty"})
			return
		}
		if body.AnonymityLevel != nil && !services.ValidAnonymityLevels[*body.AnonymityLevel] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "anonymity_level must be none, partial, or full"})
			return
		}
		if body.Mode != nil && !services.ValidModes[*body.Mode] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be conversational, form, or prompt_only"})
			return
		}
		if body.Mode != nil && *body.Mode != "form" &&
			(body.SystemPrompt == nil || strings.TrimSpace(*body.SystemPrompt) == "") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "system_prompt is required for prompt_only and conversational modes"})
			return
		}
		if body.TerminationMode != nil && !services.ValidTerminationModes[*body.TerminationMode] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "termination_mode must be turn_limit, question_coverage, time_estimate, or combination"})
			return
		}

		user := appmw.UserFromContext(r.Context())
		id, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		survey, err := svc.Update(r.Context(), user, id, services.UpdateSurveyInput{
			Title:                body.Title,
			Description:          body.Description,
			AnonymityLevel:       body.AnonymityLevel,
			AllowRevisit:         body.AllowRevisit,
			OptionalRegistration: body.OptionalRegistration,
			Mode:                 body.Mode,
			SystemPrompt:         body.SystemPrompt,
			TerminationMode:      body.TerminationMode,
			TurnLimit:            body.TurnLimit,
			TimeEstimateMinutes:  body.TimeEstimateMinutes,
			AvailableLanguages:   body.AvailableLanguages,
			DefaultLanguage:      body.DefaultLanguage,
		})
		if err != nil {
			writeSurveyError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, survey)
	}
}

// DeleteSurvey maneja DELETE /api/surveys/{id}. Se rechaza con 409 si la
// encuesta ya tiene respuestas.
func DeleteSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		id, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		if err := svc.Delete(r.Context(), user, id); err != nil {
			writeSurveyError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// DuplicateSurvey maneja POST /api/surveys/{id}/duplicate. Crea una copia en
// draft con " (copia)" en el título y sin respuestas.
func DuplicateSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		id, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		survey, err := svc.Duplicate(r.Context(), user, id)
		if err != nil {
			writeSurveyError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, survey)
	}
}

// writeSurveyError traduce los errores tipados del servicio de encuestas al
// código HTTP correspondiente. Cualquier otro error se trata como 500.
func writeSurveyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrSurveyNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "survey not found"})
	case errors.Is(err, services.ErrSurveyForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
	case errors.Is(err, services.ErrAnonymityLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "anonymity level is locked after the first response"})
	case errors.Is(err, services.ErrSurveyHasResponses):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "survey has responses and cannot be deleted"})
	case errors.Is(err, services.ErrInvalidLanguage):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

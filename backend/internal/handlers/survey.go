package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// SurveyServicer es la interfaz que los handlers de encuestas necesitan.
// *services.SurveyService la satisface. Usar una interfaz permite inyectar
// un mock en los tests sin tocar la base de datos.
type SurveyServicer interface {
	Create(ctx context.Context, user *models.User, in services.CreateSurveyInput) (*models.Survey, error)
	List(ctx context.Context, user *models.User, includeArchived bool) ([]models.Survey, error)
	Get(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	Update(ctx context.Context, user *models.User, id string, in services.UpdateSurveyInput) (*models.Survey, error)
	Delete(ctx context.Context, user *models.User, id string) error
	Duplicate(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	CheckWriteAccess(ctx context.Context, user *models.User, surveyID string) error
	// Transiciones de ciclo de vida (#08).
	Activate(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	Close(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	Reopen(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	Archive(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	// RetryAnalysis re-encola el Analysis Engine (#15) para una encuesta
	// atascada en 'failed' o 'analysing'.
	RetryAnalysis(ctx context.Context, user *models.User, id string) (*models.Survey, error)
}

// CreateSurvey maneja POST /api/surveys. El usuario debe ser admin/super_admin
// (ver RequireRole("admin") en main.go).
func CreateSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title                string    `json:"title"`
			Description          *string   `json:"description"`
			TeamID               string    `json:"team_id"`
			AnonymityLevel       string    `json:"anonymity_level"`
			AllowRevisit         bool      `json:"allow_revisit"`
			OptionalRegistration bool      `json:"optional_registration"`
			Mode                 string    `json:"mode"`
			SystemPrompt         *string   `json:"system_prompt"`
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

		// ?include_archived=true muestra también las archivadas (ocultas por defecto).
		includeArchived := r.URL.Query().Get("include_archived") == "true"

		surveys, err := svc.List(r.Context(), user, includeArchived)
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

// parseNullableTime interpreta un campo json.RawMessage opcional con semántica
// de tres estados: ausente (nil → no tocar), null (**T apuntando a nil → limpiar),
// o un valor (**T apuntando al valor → fijar).
func parseNullableTime(raw json.RawMessage) (**time.Time, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if string(raw) == "null" {
		var cleared *time.Time
		return &cleared, nil
	}
	var t time.Time
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	value := &t
	return &value, nil
}

// parseNullableInt es el equivalente de parseNullableTime para enteros (cap).
func parseNullableInt(raw json.RawMessage) (**int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if string(raw) == "null" {
		var cleared *int
		return &cleared, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, err
	}
	value := &n
	return &value, nil
}

// UpdateSurvey maneja PATCH /api/surveys/{id}. Aplica un PATCH parcial: solo
// se modifican los campos presentes en el body. Cambiar anonymity_level
// después de la primera respuesta devuelve 409.
func UpdateSurvey(svc SurveyServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title                *string   `json:"title"`
			Description          *string   `json:"description"`
			AnonymityLevel       *string   `json:"anonymity_level"`
			AllowRevisit         *bool     `json:"allow_revisit"`
			OptionalRegistration *bool     `json:"optional_registration"`
			Mode                 *string   `json:"mode"`
			SystemPrompt         *string   `json:"system_prompt"`
			TerminationMode      *string   `json:"termination_mode"`
			TurnLimit            *int      `json:"turn_limit"`
			TimeEstimateMinutes  *int      `json:"time_estimate_minutes"`
			AvailableLanguages   *[]string `json:"available_languages"`
			DefaultLanguage      *string   `json:"default_language"`
			// Programación (#08). json.RawMessage para distinguir tres casos:
			// campo ausente (no tocar), null (limpiar), o un valor (fijar).
			OpensAt     json.RawMessage `json:"opens_at"`
			ClosesAt    json.RawMessage `json:"closes_at"`
			ResponseCap json.RawMessage `json:"response_cap"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		opensAt, err := parseNullableTime(body.OpensAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "opens_at must be a valid timestamp or null"})
			return
		}
		closesAt, err := parseNullableTime(body.ClosesAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "closes_at must be a valid timestamp or null"})
			return
		}
		responseCap, err := parseNullableInt(body.ResponseCap)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "response_cap must be a positive integer or null"})
			return
		}
		if responseCap != nil && *responseCap != nil && **responseCap <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "response_cap must be greater than 0"})
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
			OpensAt:              opensAt,
			ClosesAt:             closesAt,
			ResponseCap:          responseCap,
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

// lifecycleTransition es la firma común de Activate/Close/Reopen/Archive.
type lifecycleTransition func(ctx context.Context, user *models.User, id string) (*models.Survey, error)

// surveyTransitionHandler genera un handler para una transición de ciclo de
// vida. Las cuatro comparten la misma forma: validar id, ejecutar la
// transición y devolver la encuesta actualizada.
func surveyTransitionHandler(transition lifecycleTransition) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())
		id, ok := pathUUIDParam(w, r, "id", "survey id")
		if !ok {
			return
		}

		survey, err := transition(r.Context(), user, id)
		if err != nil {
			writeSurveyError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, survey)
	}
}

// ActivateSurvey maneja POST /api/surveys/{id}/activate (draft → open).
func ActivateSurvey(svc SurveyServicer) http.HandlerFunc {
	return surveyTransitionHandler(svc.Activate)
}

// CloseSurvey maneja POST /api/surveys/{id}/close (open → closed).
func CloseSurvey(svc SurveyServicer) http.HandlerFunc {
	return surveyTransitionHandler(svc.Close)
}

// ReopenSurvey maneja POST /api/surveys/{id}/reopen (closed → open).
func ReopenSurvey(svc SurveyServicer) http.HandlerFunc {
	return surveyTransitionHandler(svc.Reopen)
}

// ArchiveSurvey maneja POST /api/surveys/{id}/archive (draft/closed → archived).
// El router lo restringe a super_admin.
func ArchiveSurvey(svc SurveyServicer) http.HandlerFunc {
	return surveyTransitionHandler(svc.Archive)
}

// RetrySurveyAnalysis maneja POST /api/surveys/{id}/analysis/retry.
// Vuelve a encolar el Analysis Engine (#15) para una encuesta que quedó en
// 'failed' o 'analysing' sin avanzar nunca — ver AnalysisService.AnalyseSurvey.
func RetrySurveyAnalysis(svc SurveyServicer) http.HandlerFunc {
	return surveyTransitionHandler(svc.RetryAnalysis)
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
	case errors.Is(err, services.ErrInvalidStatusTransition):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "invalid status transition for this survey"})
	case errors.Is(err, services.ErrInvalidSurveyConfig):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, services.ErrAIProviderUnavailable):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "ai provider not configured"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

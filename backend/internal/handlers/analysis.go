// internal/handlers/analysis.go
// Endpoints de lectura del dashboard de resultados (#16):
//
//	GET /api/surveys/stats         — agregados de todas las encuestas visibles
//	GET /api/surveys/{id}/stats    — agregados de una encuesta
//	GET /api/surveys/{id}/analysis — resultado del Analysis Engine por pregunta
//	GET /api/surveys/{id}/responses — respuestas individuales, sin agregar
//
// Los cuatro delegan el control de acceso en SurveyServicer: List ya devuelve
// solo las encuestas del equipo del usuario, y Get devuelve 404/403 antes de
// que se lea nada del análisis. El router los restringe además a
// admin/profesor (ver main.go), así que un alumno nunca llega hasta aquí.
package handlers

import (
	"context"
	"net/http"

	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// AnalysisReader es la porción de *services.AnalysisService que necesita el
// dashboard: solo lectura, sobre encuestas cuyo acceso ya se autorizó.
type AnalysisReader interface {
	StatsForSurveys(ctx context.Context, surveyIDs []string) ([]models.SurveyStats, error)
	SurveyStats(ctx context.Context, surveyID string) (*models.SurveyStats, error)
	SurveyAnalysis(ctx context.Context, surveyID string) (*models.SurveyAnalysis, error)
	SurveyResponses(ctx context.Context, surveyID string) (*models.SurveyResponses, error)
}

// analysableStatuses son los estados en los que la vista de análisis existe.
// En cualquier otro, el endpoint responde 404 — no hay nada que mostrar, y el
// dashboard esconde el link (#16). 'failed' cuenta porque el job puede haber
// alcanzado a analizar algunas preguntas antes de fallar — el admin ve ese
// avance parcial junto con el aviso de error y el botón de reintentar.
var analysableStatuses = map[string]bool{"analysing": true, "complete": true, "failed": true}

// ListSurveyStats maneja GET /api/surveys/stats.
// Devuelve los agregados de las encuestas que el usuario puede ver, en el
// mismo orden que GET /api/surveys — el listado los cruza por survey_id sin
// hacer un fetch por encuesta.
func ListSurveyStats(surveySvc SurveyServicer, reader AnalysisReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := appmw.UserFromContext(r.Context())

		includeArchived := r.URL.Query().Get("include_archived") == "true"
		surveys, err := surveySvc.List(r.Context(), user, includeArchived)
		if err != nil {
			writeSurveyError(w, err)
			return
		}

		ids := make([]string, 0, len(surveys))
		for _, survey := range surveys {
			ids = append(ids, survey.ID)
		}

		stats, err := reader.StatsForSurveys(r.Context(), ids)
		if err != nil {
			writeSurveyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}

// GetSurveyStats maneja GET /api/surveys/{id}/stats.
// A diferencia del análisis, está disponible en cualquier estado: el detalle
// muestra respuestas y tasa de completado desde que la encuesta abre.
func GetSurveyStats(surveySvc SurveyServicer, reader AnalysisReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		survey, ok := authorizedSurvey(w, r, surveySvc)
		if !ok {
			return
		}

		stats, err := reader.SurveyStats(r.Context(), survey.ID)
		if err != nil {
			writeSurveyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}

// GetSurveyAnalysis maneja GET /api/surveys/{id}/analysis.
// Responde 404 mientras la encuesta no esté en 'analysing', 'complete' o
// 'failed', para que el análisis no se pueda leer antes de que el Analysis
// Engine (#15) haya corrido. La verificación de acceso va primero: un
// no-miembro recibe 403 aunque la encuesta esté sin analizar, y así no puede
// distinguir por el código de estado si la encuesta existe.
func GetSurveyAnalysis(surveySvc SurveyServicer, reader AnalysisReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		survey, ok := authorizedSurvey(w, r, surveySvc)
		if !ok {
			return
		}
		if !analysableStatuses[survey.Status] {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "analysis not available for this survey"})
			return
		}

		analysis, err := reader.SurveyAnalysis(r.Context(), survey.ID)
		if err != nil {
			writeSurveyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, analysis)
	}
}

// GetSurveyResponses maneja GET /api/surveys/{id}/responses.
// A diferencia del análisis, no valida survey.Status: la data cruda existe
// desde que hay respuestas enviadas, sin depender de que el Analysis Engine
// haya corrido (ver AnalysisService.SurveyResponses).
func GetSurveyResponses(surveySvc SurveyServicer, reader AnalysisReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		survey, ok := authorizedSurvey(w, r, surveySvc)
		if !ok {
			return
		}

		responses, err := reader.SurveyResponses(r.Context(), survey.ID)
		if err != nil {
			writeSurveyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, responses)
	}
}

// authorizedSurvey valida el {id} de la ruta y carga la encuesta aplicando el
// control de acceso de SurveyService (404 si no existe, 403 si el usuario no
// pertenece al equipo). Devuelve ok=false si ya escribió la respuesta de error.
func authorizedSurvey(w http.ResponseWriter, r *http.Request, surveySvc SurveyServicer) (*models.Survey, bool) {
	id, ok := pathUUIDParam(w, r, "id", "survey id")
	if !ok {
		return nil, false
	}

	user := appmw.UserFromContext(r.Context())
	survey, err := surveySvc.Get(r.Context(), user, id)
	if err != nil {
		writeSurveyError(w, err)
		return nil, false
	}
	return survey, true
}

// ensure *services.AnalysisService satisfies AnalysisReader at compile time
var _ AnalysisReader = (*services.AnalysisService)(nil)

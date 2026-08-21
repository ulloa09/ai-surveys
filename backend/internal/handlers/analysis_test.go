package handlers_test

// Tests de los endpoints de lectura del dashboard de resultados (#16):
//   - el análisis solo existe en 'analysing'/'complete'/'failed'; en
//     cualquier otro estado el endpoint responde 404 aunque la encuesta exista
//   - el control de acceso (403 para no-miembros, 404 para inexistentes) lo
//     hereda de SurveyService y se propaga tal cual
//   - /stats está disponible en cualquier estado y respeta include_archived

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

const testCopySurveyID = "22222222-2222-2222-2222-222222222222"

// --- mock AnalysisReader ---

type fakeAnalysisReader struct {
	statsForSurveysFn func(ctx context.Context, surveyIDs []string) ([]models.SurveyStats, error)
	surveyStatsFn     func(ctx context.Context, surveyID string) (*models.SurveyStats, error)
	surveyAnalysisFn  func(ctx context.Context, surveyID string) (*models.SurveyAnalysis, error)
	surveyResponsesFn func(ctx context.Context, surveyID string) (*models.SurveyResponses, error)
}

func (f *fakeAnalysisReader) StatsForSurveys(ctx context.Context, surveyIDs []string) ([]models.SurveyStats, error) {
	return f.statsForSurveysFn(ctx, surveyIDs)
}
func (f *fakeAnalysisReader) SurveyStats(ctx context.Context, surveyID string) (*models.SurveyStats, error) {
	return f.surveyStatsFn(ctx, surveyID)
}
func (f *fakeAnalysisReader) SurveyAnalysis(ctx context.Context, surveyID string) (*models.SurveyAnalysis, error) {
	return f.surveyAnalysisFn(ctx, surveyID)
}
func (f *fakeAnalysisReader) SurveyResponses(ctx context.Context, surveyID string) (*models.SurveyResponses, error) {
	return f.surveyResponsesFn(ctx, surveyID)
}

var _ handlers.AnalysisReader = (*fakeAnalysisReader)(nil)

// surveyWithStatus arma el fake de SurveyServicer para una encuesta que existe
// y a la que el usuario tiene acceso.
func surveyWithStatus(status string) *fakeSurveySvc {
	return &fakeSurveySvc{
		getFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id, Title: "Encuesta", Status: status}, nil
		},
	}
}

func analysisReaderFor(surveyID string) *fakeAnalysisReader {
	return &fakeAnalysisReader{
		surveyAnalysisFn: func(_ context.Context, id string) (*models.SurveyAnalysis, error) {
			return &models.SurveyAnalysis{
				SurveyID:  id,
				Status:    "complete",
				Questions: []models.QuestionAnalysis{{QuestionID: testQuestionID, Text: "¿Qué tal?", SummaryText: "resumen"}},
			}, nil
		},
		surveyStatsFn: func(_ context.Context, id string) (*models.SurveyStats, error) {
			return &models.SurveyStats{SurveyID: id, ResponseCount: 4, CompletedCount: 3, CompletionRate: 0.75}, nil
		},
	}
}

// --- GetSurveyAnalysis ---

func TestGetSurveyAnalysis_AvailableWhileAnalysing(t *testing.T) {
	handler := handlers.GetSurveyAnalysis(surveyWithStatus("analysing"), analysisReaderFor(testSurveyID))

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/analysis", nil),
		adminUser(), map[string]string{"id": testSurveyID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body models.SurveyAnalysis
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Questions) != 1 || body.Questions[0].SummaryText != "resumen" {
		t.Fatalf("questions = %+v, want the analysed question", body.Questions)
	}
}

func TestGetSurveyAnalysis_AvailableWhenComplete(t *testing.T) {
	handler := handlers.GetSurveyAnalysis(surveyWithStatus("complete"), analysisReaderFor(testSurveyID))

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/analysis", nil),
		adminUser(), map[string]string{"id": testSurveyID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// El link a la vista de análisis se esconde hasta que la encuesta se analiza;
// el endpoint tiene que respaldar eso con un 404 para quien lo pida a mano.
func TestGetSurveyAnalysis_NotFoundBeforeAnalysis(t *testing.T) {
	for _, status := range []string{"draft", "open", "closed", "archived"} {
		t.Run(status, func(t *testing.T) {
			reader := &fakeAnalysisReader{
				surveyAnalysisFn: func(context.Context, string) (*models.SurveyAnalysis, error) {
					t.Fatal("no se debe leer el análisis de una encuesta sin analizar")
					return nil, nil
				},
			}
			handler := handlers.GetSurveyAnalysis(surveyWithStatus(status), reader)

			rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/analysis", nil),
				adminUser(), map[string]string{"id": testSurveyID})

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
		})
	}
}

// Un no-miembro recibe 403 antes de que se evalúe el status de la encuesta —
// así no puede deducir por el código de estado si la encuesta ya se analizó.
func TestGetSurveyAnalysis_NonMemberForbidden(t *testing.T) {
	svc := &fakeSurveySvc{
		getFn: func(context.Context, *models.User, string) (*models.Survey, error) {
			return nil, services.ErrSurveyForbidden
		},
	}
	handler := handlers.GetSurveyAnalysis(svc, &fakeAnalysisReader{})

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/analysis", nil),
		adminUser(), map[string]string{"id": testSurveyID})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestGetSurveyAnalysis_SurveyNotFound(t *testing.T) {
	svc := &fakeSurveySvc{
		getFn: func(context.Context, *models.User, string) (*models.Survey, error) {
			return nil, services.ErrSurveyNotFound
		},
	}
	handler := handlers.GetSurveyAnalysis(svc, &fakeAnalysisReader{})

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/analysis", nil),
		adminUser(), map[string]string{"id": testSurveyID})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGetSurveyAnalysis_InvalidIDReturnsBadRequest(t *testing.T) {
	svc := &fakeSurveySvc{
		getFn: func(context.Context, *models.User, string) (*models.Survey, error) {
			t.Fatal("no se debe consultar el servicio con un id inválido")
			return nil, nil
		},
	}
	handler := handlers.GetSurveyAnalysis(svc, &fakeAnalysisReader{})

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/not-a-uuid/analysis", nil),
		adminUser(), map[string]string{"id": "not-a-uuid"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- GetSurveyStats ---

// Las cifras del detalle no dependen del análisis: se muestran desde que la
// encuesta abre, mucho antes de que el Analysis Engine corra.
func TestGetSurveyStats_AvailableBeforeAnalysis(t *testing.T) {
	handler := handlers.GetSurveyStats(surveyWithStatus("open"), analysisReaderFor(testSurveyID))

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/stats", nil),
		adminUser(), map[string]string{"id": testSurveyID})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body models.SurveyStats
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.ResponseCount != 4 || body.CompletionRate != 0.75 {
		t.Fatalf("stats = %+v, want 4 respuestas y 0.75 de completado", body)
	}
}

func TestGetSurveyStats_NonMemberForbidden(t *testing.T) {
	svc := &fakeSurveySvc{
		getFn: func(context.Context, *models.User, string) (*models.Survey, error) {
			return nil, services.ErrSurveyForbidden
		},
	}
	handler := handlers.GetSurveyStats(svc, &fakeAnalysisReader{})

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/"+testSurveyID+"/stats", nil),
		adminUser(), map[string]string{"id": testSurveyID})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// --- ListSurveyStats ---

// El listado pide los agregados de todas las encuestas visibles de una sola
// vez: el handler pasa exactamente los ids que devolvió List, sin re-scopear.
func TestListSurveyStats_RequestsStatsForVisibleSurveys(t *testing.T) {
	svc := &fakeSurveySvc{
		listFn: func(context.Context, *models.User, bool) ([]models.Survey, error) {
			return []models.Survey{{ID: testSurveyID}, {ID: testCopySurveyID}}, nil
		},
	}
	var gotIDs []string
	reader := &fakeAnalysisReader{
		statsForSurveysFn: func(_ context.Context, ids []string) ([]models.SurveyStats, error) {
			gotIDs = ids
			return []models.SurveyStats{{SurveyID: testSurveyID}, {SurveyID: testCopySurveyID}}, nil
		},
	}
	handler := handlers.ListSurveyStats(svc, reader)

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/stats", nil), adminUser(), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(gotIDs) != 2 || gotIDs[0] != testSurveyID || gotIDs[1] != testCopySurveyID {
		t.Fatalf("ids = %v, want los dos ids del listado en orden", gotIDs)
	}
}

func TestListSurveyStats_PropagatesIncludeArchived(t *testing.T) {
	var gotIncludeArchived bool
	svc := &fakeSurveySvc{
		listFn: func(_ context.Context, _ *models.User, includeArchived bool) ([]models.Survey, error) {
			gotIncludeArchived = includeArchived
			return nil, nil
		},
	}
	reader := &fakeAnalysisReader{
		statsForSurveysFn: func(context.Context, []string) ([]models.SurveyStats, error) {
			return []models.SurveyStats{}, nil
		},
	}
	handler := handlers.ListSurveyStats(svc, reader)

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/stats?include_archived=true", nil), adminUser(), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !gotIncludeArchived {
		t.Fatal("include_archived=true no llegó al servicio")
	}
}

func TestListSurveyStats_EmptyReturnsArray(t *testing.T) {
	svc := &fakeSurveySvc{
		listFn: func(context.Context, *models.User, bool) ([]models.Survey, error) { return nil, nil },
	}
	reader := &fakeAnalysisReader{
		statsForSurveysFn: func(context.Context, []string) ([]models.SurveyStats, error) {
			return []models.SurveyStats{}, nil
		},
	}
	handler := handlers.ListSurveyStats(svc, reader)

	rec := serveAuthed(handler, httptest.NewRequest(http.MethodGet, "/api/surveys/stats", nil), adminUser(), nil)

	if got := rec.Body.String(); got != "[]\n" && got != "[]" {
		t.Fatalf("body = %q, want un array vacío", got)
	}
}

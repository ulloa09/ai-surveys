package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// --- mock SurveyServicer ---

type fakeSurveySvc struct {
	createFn           func(ctx context.Context, user *models.User, in services.CreateSurveyInput) (*models.Survey, error)
	listFn             func(ctx context.Context, user *models.User, includeArchived bool) ([]models.Survey, error)
	getFn              func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	updateFn           func(ctx context.Context, user *models.User, id string, in services.UpdateSurveyInput) (*models.Survey, error)
	deleteFn           func(ctx context.Context, user *models.User, id string) error
	duplicateFn        func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	checkWriteAccessFn func(ctx context.Context, user *models.User, surveyID string) error
	activateFn         func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	closeFn            func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	reopenFn           func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	archiveFn          func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
	retryAnalysisFn    func(ctx context.Context, user *models.User, id string) (*models.Survey, error)
}

func (f *fakeSurveySvc) Create(ctx context.Context, user *models.User, in services.CreateSurveyInput) (*models.Survey, error) {
	return f.createFn(ctx, user, in)
}
func (f *fakeSurveySvc) List(ctx context.Context, user *models.User, includeArchived bool) ([]models.Survey, error) {
	return f.listFn(ctx, user, includeArchived)
}
func (f *fakeSurveySvc) Get(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.getFn(ctx, user, id)
}
func (f *fakeSurveySvc) Update(ctx context.Context, user *models.User, id string, in services.UpdateSurveyInput) (*models.Survey, error) {
	return f.updateFn(ctx, user, id, in)
}
func (f *fakeSurveySvc) Delete(ctx context.Context, user *models.User, id string) error {
	return f.deleteFn(ctx, user, id)
}
func (f *fakeSurveySvc) Duplicate(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.duplicateFn(ctx, user, id)
}
func (f *fakeSurveySvc) CheckWriteAccess(ctx context.Context, user *models.User, surveyID string) error {
	if f.checkWriteAccessFn != nil {
		return f.checkWriteAccessFn(ctx, user, surveyID)
	}
	return nil
}
func (f *fakeSurveySvc) Activate(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.activateFn(ctx, user, id)
}
func (f *fakeSurveySvc) Close(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.closeFn(ctx, user, id)
}
func (f *fakeSurveySvc) Reopen(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.reopenFn(ctx, user, id)
}
func (f *fakeSurveySvc) Archive(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.archiveFn(ctx, user, id)
}
func (f *fakeSurveySvc) RetryAnalysis(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	return f.retryAnalysisFn(ctx, user, id)
}

// ensure fakeSurveySvc satisfies handlers.SurveyServicer at compile time
var _ handlers.SurveyServicer = (*fakeSurveySvc)(nil)

const testSurveyID = "11111111-1111-1111-1111-111111111111"
const testTeamID = "22222222-2222-2222-2222-222222222222"
const testQuestionID = "33333333-3333-3333-3333-333333333333"

// serveAuthed corre el handler envuelto en Authenticate para inyectar un
// usuario en el contexto, agregando opcionalmente parámetros de ruta de chi.
func serveAuthed(handler http.Handler, req *http.Request, user *models.User, params map[string]string) *httptest.ResponseRecorder {
	if len(params) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok"})

	rec := httptest.NewRecorder()
	middleware.Authenticate(&fakeSessionValidator{user: user})(handler).ServeHTTP(rec, req)
	return rec
}

func adminUser() *models.User {
	return &models.User{ID: "u1", Role: "admin"}
}

func jsonReq(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// --- CreateSurvey ---

func TestCreateSurveyHandler_Created(t *testing.T) {
	svc := &fakeSurveySvc{
		createFn: func(_ context.Context, _ *models.User, in services.CreateSurveyInput) (*models.Survey, error) {
			return &models.Survey{ID: "s1", Title: in.Title, Status: "draft"}, nil
		},
	}
	body := `{"title":"Encuesta semana 12","team_id":"` + testTeamID + `","system_prompt":"Sondea la experiencia del semestre."}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusCreated, rr.Body.String())
	}
	var got models.Survey
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Title != "Encuesta semana 12" {
		t.Errorf("title = %q, want %q", got.Title, "Encuesta semana 12")
	}
}

func TestCreateSurveyHandler_MissingTitle(t *testing.T) {
	svc := &fakeSurveySvc{}
	body := `{"team_id":"` + testTeamID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateSurveyHandler_InvalidTeamID(t *testing.T) {
	svc := &fakeSurveySvc{}
	body := `{"title":"x","team_id":"not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateSurveyHandler_InvalidAnonymityLevel(t *testing.T) {
	svc := &fakeSurveySvc{}
	body := `{"title":"x","team_id":"` + testTeamID + `","anonymity_level":"bogus"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateSurveyHandler_DefaultsToConversationalMode(t *testing.T) {
	// Rubrica: la creación de encuestas por defecto usa Mode C (conversational)
	// y termination turn_limit con límite de 12.
	svc := &fakeSurveySvc{
		createFn: func(_ context.Context, _ *models.User, in services.CreateSurveyInput) (*models.Survey, error) {
			if in.Mode != "conversational" {
				t.Errorf("mode = %q, want conversational", in.Mode)
			}
			if in.TerminationMode != "turn_limit" {
				t.Errorf("termination_mode = %q, want turn_limit", in.TerminationMode)
			}
			if in.TurnLimit == nil || *in.TurnLimit != 12 {
				t.Errorf("turn_limit = %v, want 12", in.TurnLimit)
			}
			return &models.Survey{ID: "s1", Title: in.Title, Mode: in.Mode, Status: "draft"}, nil
		},
	}
	body := `{"title":"x","team_id":"` + testTeamID + `","system_prompt":"Sé breve y directo."}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestCreateSurveyHandler_SystemPromptRequiredForConversational(t *testing.T) {
	// Rubrica: system prompt es requerido para Mode B y C, opcional para Mode A.
	svc := &fakeSurveySvc{}
	body := `{"title":"x","team_id":"` + testTeamID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateSurveyHandler_FormModeDoesNotRequireSystemPrompt(t *testing.T) {
	// Rubrica: Mode A (form, preguntas fijas) no requiere system prompt.
	svc := &fakeSurveySvc{
		createFn: func(_ context.Context, _ *models.User, in services.CreateSurveyInput) (*models.Survey, error) {
			return &models.Survey{ID: "s1", Title: in.Title, Mode: in.Mode, Status: "draft"}, nil
		},
	}
	body := `{"title":"x","team_id":"` + testTeamID + `","mode":"form"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestCreateSurveyHandler_InvalidTerminationMode(t *testing.T) {
	svc := &fakeSurveySvc{}
	body := `{"title":"x","team_id":"` + testTeamID + `","mode":"form","termination_mode":"bogus"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateSurveyHandler_TimeEstimateRequiresMinutes(t *testing.T) {
	svc := &fakeSurveySvc{}
	body := `{"title":"x","team_id":"` + testTeamID + `","mode":"form","termination_mode":"time_estimate"}`
	req := httptest.NewRequest(http.MethodPost, "/api/surveys", strings.NewReader(body))
	rr := serveAuthed(handlers.CreateSurvey(svc), req, adminUser(), nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- ListSurveys ---

func TestListSurveysHandler_EmptyReturnsArray(t *testing.T) {
	svc := &fakeSurveySvc{
		listFn: func(_ context.Context, _ *models.User, _ bool) ([]models.Survey, error) { return nil, nil },
	}
	req := httptest.NewRequest(http.MethodGet, "/api/surveys", nil)
	rr := serveAuthed(handlers.ListSurveys(svc), req, adminUser(), nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Errorf("body = %q, want []", rr.Body.String())
	}
}

// --- UpdateSurvey ---

func TestUpdateSurveyHandler_AnonymityLocked(t *testing.T) {
	svc := &fakeSurveySvc{
		updateFn: func(_ context.Context, _ *models.User, _ string, _ services.UpdateSurveyInput) (*models.Survey, error) {
			return nil, services.ErrAnonymityLocked
		},
	}
	body := `{"anonymity_level":"full"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/surveys/"+testSurveyID, strings.NewReader(body))
	rr := serveAuthed(handlers.UpdateSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusConflict, rr.Body.String())
	}
}

func TestUpdateSurveyHandler_EmptyTitleRejected(t *testing.T) {
	svc := &fakeSurveySvc{}
	body := `{"title":""}`
	req := httptest.NewRequest(http.MethodPatch, "/api/surveys/"+testSurveyID, strings.NewReader(body))
	rr := serveAuthed(handlers.UpdateSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// --- DeleteSurvey ---

func TestDeleteSurveyHandler_ConflictWhenResponsesExist(t *testing.T) {
	svc := &fakeSurveySvc{
		deleteFn: func(_ context.Context, _ *models.User, _ string) error {
			return services.ErrSurveyHasResponses
		},
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/surveys/"+testSurveyID, nil)
	rr := serveAuthed(handlers.DeleteSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
	}
}

// --- DuplicateSurvey ---

func TestDuplicateSurveyHandler_Created(t *testing.T) {
	svc := &fakeSurveySvc{
		duplicateFn: func(_ context.Context, _ *models.User, _ string) (*models.Survey, error) {
			return &models.Survey{ID: "s2", Title: "Original (copia)", Status: "draft"}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/duplicate", nil)
	rr := serveAuthed(handlers.DuplicateSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	var got models.Survey
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasSuffix(got.Title, "(copia)") {
		t.Errorf("title = %q, want suffix (copia)", got.Title)
	}
}

// --- Lifecycle transitions (#08) ---

func TestActivateSurveyHandler_DraftToOpen(t *testing.T) {
	svc := &fakeSurveySvc{
		activateFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id, Status: "open"}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/activate", nil)
	rr := serveAuthed(handlers.ActivateSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestActivateSurveyHandler_RejectsInvalidTransition(t *testing.T) {
	svc := &fakeSurveySvc{
		activateFn: func(_ context.Context, _ *models.User, _ string) (*models.Survey, error) {
			return nil, services.ErrInvalidStatusTransition
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/activate", nil)
	rr := serveAuthed(handlers.ActivateSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
	}
}

func TestActivateSurveyHandler_RejectsSurveyWithoutQuestions(t *testing.T) {
	svc := &fakeSurveySvc{
		activateFn: func(_ context.Context, _ *models.User, _ string) (*models.Survey, error) {
			return nil, services.ErrInvalidSurveyConfig
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/activate", nil)
	rr := serveAuthed(handlers.ActivateSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCloseSurveyHandler_OpenToClosed(t *testing.T) {
	svc := &fakeSurveySvc{
		closeFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id, Status: "closed"}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/close", nil)
	rr := serveAuthed(handlers.CloseSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestReopenSurveyHandler_ClosedToOpen(t *testing.T) {
	svc := &fakeSurveySvc{
		reopenFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id, Status: "open"}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/reopen", nil)
	rr := serveAuthed(handlers.ReopenSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestArchiveSurveyHandler_OK(t *testing.T) {
	svc := &fakeSurveySvc{
		archiveFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id, Status: "archived"}, nil
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/surveys/"+testSurveyID+"/archive", nil)
	rr := serveAuthed(handlers.ArchiveSurvey(svc), req, adminUser(), map[string]string{"id": testSurveyID})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

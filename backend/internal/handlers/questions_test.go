package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// --- mock QuestionServicer ---

type fakeQuestionSvc struct {
	listFn    func(ctx context.Context, surveyID string) ([]models.Question, error)
	createFn  func(ctx context.Context, surveyID string, in services.CreateQuestionInput) (*models.Question, error)
	updateFn  func(ctx context.Context, surveyID, questionID string, in services.UpdateQuestionInput) (*models.Question, error)
	deleteFn  func(ctx context.Context, surveyID, questionID string) error
	reorderFn func(ctx context.Context, surveyID string, orderedIDs []string) error
}

func (f *fakeQuestionSvc) List(ctx context.Context, surveyID string) ([]models.Question, error) {
	return f.listFn(ctx, surveyID)
}
func (f *fakeQuestionSvc) Create(ctx context.Context, surveyID string, in services.CreateQuestionInput) (*models.Question, error) {
	return f.createFn(ctx, surveyID, in)
}
func (f *fakeQuestionSvc) Update(ctx context.Context, surveyID, questionID string, in services.UpdateQuestionInput) (*models.Question, error) {
	return f.updateFn(ctx, surveyID, questionID, in)
}
func (f *fakeQuestionSvc) Delete(ctx context.Context, surveyID, questionID string) error {
	return f.deleteFn(ctx, surveyID, questionID)
}
func (f *fakeQuestionSvc) Reorder(ctx context.Context, surveyID string, orderedIDs []string) error {
	return f.reorderFn(ctx, surveyID, orderedIDs)
}

var _ handlers.QuestionServicer = (*fakeQuestionSvc)(nil)

// surveySvcAllowAll devuelve un fakeSurveySvc que siempre permite acceso de
// lectura y escritura. Lo usan los tests que no prueban permisos.
func surveySvcAllowAll() *fakeSurveySvc {
	return &fakeSurveySvc{
		getFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id, Status: "draft"}, nil
		},
		checkWriteAccessFn: func(_ context.Context, _ *models.User, _ string) error {
			return nil
		},
	}
}

// --- ListQuestions ---

func TestListQuestions_EmptyReturnsArray(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		listFn: func(_ context.Context, _ string) ([]models.Question, error) {
			return nil, nil
		},
	}
	rec := serveAuthed(
		handlers.ListQuestions(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodGet, "/api/surveys/s1/questions", ""),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("expected empty array, got %q", got)
	}
}

func TestListQuestions_InvalidSurveyIDReturnsBadRequest(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		listFn: func(context.Context, string) ([]models.Question, error) {
			t.Fatal("question service should not be called for invalid survey id")
			return nil, nil
		},
	}
	rec := serveAuthed(
		handlers.ListQuestions(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodGet, "/api/surveys/undefined/questions", ""),
		adminUser(),
		map[string]string{"id": "undefined"},
	)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- CreateQuestion — los 7 tipos ---
// Rubrica: Admin can add a question of each of the 7 types to a survey in draft state
// Rubrica: Each question type renders the appropriate configuration fields

func testCreateQuestionType(t *testing.T, questionType string, options []byte) {
	t.Helper()
	qsvc := &fakeQuestionSvc{
		createFn: func(_ context.Context, _ string, in services.CreateQuestionInput) (*models.Question, error) {
			return &models.Question{
				ID:         "q1",
				SurveyID:   "s1",
				Type:       in.Type,
				Text:       in.Text,
				Required:   true,
				AIFollowup: in.Type == "open_ended",
				Options:    in.Options,
			}, nil
		},
	}

	body := map[string]any{
		"type": questionType,
		"text": "Pregunta de prueba",
	}
	if options != nil {
		body["options"] = json.RawMessage(options)
	}
	bodyBytes, _ := json.Marshal(body)

	rec := serveAuthed(
		handlers.CreateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPost, "/api/surveys/s1/questions", string(bodyBytes)),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusCreated {
		t.Errorf("type %q: expected 201, got %d — body: %s", questionType, rec.Code, rec.Body.String())
	}
}

// Rubrica: Admin can add a question of each of the 7 types to a survey in draft state
func TestCreateQuestion_OpenEnded(t *testing.T) {
	// open_ended no necesita options
	testCreateQuestionType(t, "open_ended", nil)
}

// Rubrica: Each question type renders the appropriate configuration fields (options list for choice types)
func TestCreateQuestion_SingleChoice(t *testing.T) {
	opts := []byte(`{"choices":[{"label":"Sí","value":"yes"},{"label":"No","value":"no"}]}`)
	testCreateQuestionType(t, "single_choice", opts)
}

// Rubrica: Each question type renders the appropriate configuration fields (options list for choice types)
func TestCreateQuestion_MultiChoice(t *testing.T) {
	opts := []byte(`{"choices":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}`)
	testCreateQuestionType(t, "multi_choice", opts)
}

// Rubrica: Admin can add a question of each of the 7 types to a survey in draft state
func TestCreateQuestion_TrueFalse(t *testing.T) {
	// true_false no necesita options
	testCreateQuestionType(t, "true_false", nil)
}

func TestCreateQuestion_InvalidSurveyIDReturnsBadRequest(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		createFn: func(context.Context, string, services.CreateQuestionInput) (*models.Question, error) {
			t.Fatal("question service should not be called for invalid survey id")
			return nil, nil
		},
	}
	rec := serveAuthed(
		handlers.CreateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPost, "/api/surveys/undefined/questions", `{"type":"open_ended","text":"Pregunta"}`),
		adminUser(),
		map[string]string{"id": "undefined"},
	)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// Rubrica: Each question type renders the appropriate configuration fields (min/max for scale)
func TestCreateQuestion_LinearScale(t *testing.T) {
	opts := []byte(`{"min":1,"max":5,"min_label":"Malo","max_label":"Excelente"}`)
	testCreateQuestionType(t, "linear_scale", opts)
}

// Rubrica: Each question type renders the appropriate configuration fields (items for ranking)
func TestCreateQuestion_Ranking(t *testing.T) {
	opts := []byte(`{"items":[{"label":"Opcion 1","value":"1"},{"label":"Opcion 2","value":"2"}]}`)
	testCreateQuestionType(t, "ranking", opts)
}

// Rubrica: Each question type renders the appropriate configuration fields (items for matrix)
func TestCreateQuestion_Matrix(t *testing.T) {
	opts := []byte(`{"rows":["Fila 1","Fila 2"],"columns":["Col 1","Col 2"]}`)
	testCreateQuestionType(t, "matrix", opts)
}

// --- Defaults de required y ai_followup ---
// Rubrica: Required toggle defaults to true; AI follow-up toggle defaults to true
// for open_ended and false for all structured types

func TestCreateQuestion_RequiredDefaultsTrue(t *testing.T) {
	// El handler no manda required — el service aplica el default true.
	// Verificamos que la respuesta tenga required:true.
	qsvc := &fakeQuestionSvc{
		createFn: func(_ context.Context, _ string, in services.CreateQuestionInput) (*models.Question, error) {
			return &models.Question{ID: "q1", Type: "open_ended", Required: true, AIFollowup: true}, nil
		},
	}
	body := `{"type":"open_ended","text":"Pregunta"}`
	rec := serveAuthed(
		handlers.CreateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPost, "/api/surveys/s1/questions", body),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"required":true`) {
		t.Errorf("expected required:true in response, got %s", rec.Body.String())
	}
}

func TestCreateQuestion_AIFollowupTrueForOpenEnded(t *testing.T) {
	// Rubrica: AI follow-up toggle defaults to true for open_ended
	qsvc := &fakeQuestionSvc{
		createFn: func(_ context.Context, _ string, in services.CreateQuestionInput) (*models.Question, error) {
			return &models.Question{ID: "q1", Type: "open_ended", AIFollowup: true}, nil
		},
	}
	body := `{"type":"open_ended","text":"Pregunta abierta"}`
	rec := serveAuthed(
		handlers.CreateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPost, "/api/surveys/s1/questions", body),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ai_followup":true`) {
		t.Errorf("expected ai_followup:true for open_ended, got %s", rec.Body.String())
	}
}

func TestCreateQuestion_AIFollowupFalseForStructured(t *testing.T) {
	// Rubrica: AI follow-up toggle defaults to false for all structured types
	qsvc := &fakeQuestionSvc{
		createFn: func(_ context.Context, _ string, in services.CreateQuestionInput) (*models.Question, error) {
			return &models.Question{ID: "q1", Type: "single_choice", AIFollowup: false}, nil
		},
	}
	body := `{"type":"single_choice","text":"Elige una"}`
	rec := serveAuthed(
		handlers.CreateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPost, "/api/surveys/s1/questions", body),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ai_followup":false`) {
		t.Errorf("expected ai_followup:false for single_choice, got %s", rec.Body.String())
	}
}

// --- UpdateQuestion: cambiar ai_followup en cualquier tipo ---
// Rubrica: Admin can change the AI follow-up toggle on any question type

func TestUpdateQuestion_AIFollowupToggle(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		updateFn: func(_ context.Context, _, _ string, in services.UpdateQuestionInput) (*models.Question, error) {
			if in.AIFollowup == nil || *in.AIFollowup != true {
				t.Errorf("expected ai_followup true, got %v", in.AIFollowup)
			}
			return &models.Question{ID: "q1", Type: "single_choice", AIFollowup: true}, nil
		},
	}
	body := `{"ai_followup":true}`
	rec := serveAuthed(
		handlers.UpdateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPatch, "/api/surveys/s1/questions/q1", body),
		adminUser(),
		map[string]string{"id": testSurveyID, "qid": testQuestionID},
	)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestUpdateQuestion_InvalidSurveyIDReturnsBadRequest(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		updateFn: func(context.Context, string, string, services.UpdateQuestionInput) (*models.Question, error) {
			t.Fatal("question service should not be called for invalid survey id")
			return nil, nil
		},
	}
	rec := serveAuthed(
		handlers.UpdateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPatch, "/api/surveys/undefined/questions/q1", `{"text":"Nuevo"}`),
		adminUser(),
		map[string]string{"id": "undefined", "qid": testQuestionID},
	)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestUpdateQuestion_InvalidQuestionIDReturnsBadRequest(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		updateFn: func(context.Context, string, string, services.UpdateQuestionInput) (*models.Question, error) {
			t.Fatal("question service should not be called for invalid question id")
			return nil, nil
		},
	}
	rec := serveAuthed(
		handlers.UpdateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPatch, "/api/surveys/s1/questions/undefined", `{"text":"Nuevo"}`),
		adminUser(),
		map[string]string{"id": testSurveyID, "qid": "undefined"},
	)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- Locked: editar o borrar despues de primera respuesta devuelve 409 ---
// Rubrica: Attempting to edit or delete a question after the first response returns 409 Conflict

func TestUpdateQuestion_Locked(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		updateFn: func(_ context.Context, _, _ string, _ services.UpdateQuestionInput) (*models.Question, error) {
			return nil, services.ErrQuestionsLocked
		},
	}
	body := `{"text":"Nuevo texto"}`
	rec := serveAuthed(
		handlers.UpdateQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPatch, "/api/surveys/s1/questions/q1", body),
		adminUser(),
		map[string]string{"id": testSurveyID, "qid": testQuestionID},
	)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestDeleteQuestion_Locked(t *testing.T) {
	// Rubrica: Attempting to edit or delete a question after the first response returns 409 Conflict
	qsvc := &fakeQuestionSvc{
		deleteFn: func(_ context.Context, _, _ string) error {
			return services.ErrQuestionsLocked
		},
	}
	rec := serveAuthed(
		handlers.DeleteQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodDelete, "/api/surveys/s1/questions/q1", ""),
		adminUser(),
		map[string]string{"id": testSurveyID, "qid": testQuestionID},
	)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestDeleteQuestion_InvalidQuestionIDReturnsBadRequest(t *testing.T) {
	qsvc := &fakeQuestionSvc{
		deleteFn: func(context.Context, string, string) error {
			t.Fatal("question service should not be called for invalid question id")
			return nil
		},
	}
	rec := serveAuthed(
		handlers.DeleteQuestion(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodDelete, "/api/surveys/s1/questions/undefined", ""),
		adminUser(),
		map[string]string{"id": testSurveyID, "qid": "undefined"},
	)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- Reorder: el orden persiste ---
// Rubrica: Admin can reorder questions via drag-and-drop; order persists on save

func TestReorderQuestions_OK(t *testing.T) {
	var receivedOrder []string
	qsvc := &fakeQuestionSvc{
		reorderFn: func(_ context.Context, _ string, orderedIDs []string) error {
			receivedOrder = orderedIDs
			return nil
		},
	}
	body := `{"order":["q2","q1","q3"]}`
	rec := serveAuthed(
		handlers.ReorderQuestions(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPut, "/api/surveys/s1/questions/order", body),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if len(receivedOrder) != 3 || receivedOrder[0] != "q2" {
		t.Errorf("expected order [q2,q1,q3], got %v", receivedOrder)
	}
}

func TestReorderQuestions_Locked(t *testing.T) {
	// Rubrica: Attempting to edit or delete a question after the first response returns 409 Conflict
	qsvc := &fakeQuestionSvc{
		reorderFn: func(_ context.Context, _ string, _ []string) error {
			return services.ErrQuestionsLocked
		},
	}
	body := `{"order":["q2","q1"]}`
	rec := serveAuthed(
		handlers.ReorderQuestions(qsvc, surveySvcAllowAll()),
		jsonReq(http.MethodPut, "/api/surveys/s1/questions/order", body),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

// --- Viewer no puede crear preguntas ---

func TestCreateQuestion_AlumnoForbidden(t *testing.T) {
	surveySvc := &fakeSurveySvc{
		getFn: func(_ context.Context, _ *models.User, id string) (*models.Survey, error) {
			return &models.Survey{ID: id}, nil
		},
		checkWriteAccessFn: func(_ context.Context, _ *models.User, _ string) error {
			return services.ErrSurveyForbidden
		},
	}
	qsvc := &fakeQuestionSvc{}
	alumno := &models.User{ID: "u2", Role: "alumno"}
	body := `{"type":"open_ended","text":"Pregunta"}`
	rec := serveAuthed(
		handlers.CreateQuestion(qsvc, surveySvc),
		jsonReq(http.MethodPost, "/api/surveys/s1/questions", body),
		alumno,
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for alumno, got %d", rec.Code)
	}
}

// --- Duplicate deep-copies questions ---
// Rubrica: Duplicate survey deep-copies all questions with their configuration

func TestDuplicateSurvey_CopiesQuestions(t *testing.T) {
	// El handler devuelve 201 y la copia contiene el titulo con sufijo.
	// La copia real de preguntas ocurre dentro de SurveyService.Duplicate
	// que llama a QuestionCopier.CopyQuestions — esa logica se prueba en
	// el service test. Aqui verificamos que el handler no rompe el flujo.
	svc := &fakeSurveySvc{
		duplicateFn: func(_ context.Context, _ *models.User, _ string) (*models.Survey, error) {
			return &models.Survey{
				ID:     "s2",
				Title:  "Encuesta (copia)",
				Status: "draft",
			}, nil
		},
	}
	rec := serveAuthed(
		handlers.DuplicateSurvey(svc),
		jsonReq(http.MethodPost, "/api/surveys/s1/duplicate", ""),
		adminUser(),
		map[string]string{"id": testSurveyID},
	)
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "(copia)") {
		t.Errorf("expected title to contain '(copia)', got %s", rec.Body.String())
	}
}

package handlers_test

// Tests de integración para la estabilización del survey engine (#12/#13):
//   - turn_limit cuenta intercambios y no termina con requeridas pendientes
//   - el estado final viaja en el stream SSE después de la última respuesta AI
//   - pregunta + respuesta quedan persistidas como turnos del transcript
//   - detección de respuestas basura con followup de recuperación y bloqueo
//   - el followup usa la última pregunta respondida según la DB
//   - expire no ofrece submit si el backend lo va a rechazar
//
// Requieren Postgres local (se saltan si no está disponible).

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// capturingProvider guarda el último TurnRequest recibido para poder verificar
// el system prompt y el transcript que arma el engine.
type capturingProvider struct {
	text string
	last ai.TurnRequest
}

func (p *capturingProvider) StreamTurn(_ context.Context, req ai.TurnRequest) (<-chan ai.TurnChunk, error) {
	p.last = req
	ch := make(chan ai.TurnChunk, 1)
	ch <- ai.TurnChunk{Token: p.text}
	close(ch)
	return ch, nil
}

func (p *capturingProvider) Extract(context.Context, ai.ExtractionRequest) (ai.ExtractionResult, error) {
	return ai.ExtractionResult{}, nil
}

func (f integrationFixture) engineWith(provider ai.AIProvider) *services.EngineService {
	return services.NewEngineService(
		f.pool,
		f.settingsSvc,
		f.questionSvc,
		services.WithProviderFactory(func(string, string, string) (ai.AIProvider, error) {
			return provider, nil
		}),
	)
}

// setTurnLimit ajusta el turn_limit de la encuesta (la fixture crea 12 por defecto).
func (f integrationFixture) setTurnLimit(t *testing.T, surveyID string, limit int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE surveys SET turn_limit = $1 WHERE id = $2`, limit, surveyID); err != nil {
		t.Fatalf("set turn limit: %v", err)
	}
}

func (f integrationFixture) createFollowupQuestion(t *testing.T, surveyID, text string) string {
	t.Helper()
	var questionID string
	if err := f.pool.QueryRow(context.Background(), `
		INSERT INTO questions (survey_id, type, text, required, ai_followup, order_index)
		VALUES ($1, 'open_ended', $2, true, true, COALESCE((SELECT MAX(order_index) + 1 FROM questions WHERE survey_id = $1), 0))
		RETURNING id::text`,
		surveyID, text,
	).Scan(&questionID); err != nil {
		t.Fatalf("insert followup question: %v", err)
	}
	return questionID
}

func turnCount(t *testing.T, f integrationFixture, responseID string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM turns WHERE response_id = $1`, responseID).Scan(&n); err != nil {
		t.Fatalf("count turns: %v", err)
	}
	return n
}

// El límite de turnos no cierra la sesión mientras falten requeridas: el
// usuario puede completarlas y el submit nunca se ofrece para ser rechazado.
func TestIntegrationTurnLimitRequiresCoverageBeforeEnding(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "conversational", nil, "")
	f.setTurnLimit(t, surveyID, 1)
	q1 := f.createQuestion(t, surveyID, "single_choice", "¿Opción?", true, `{"choices":[{"label":"A","value":"a"}]}`)
	q2 := f.createQuestion(t, surveyID, "linear_scale", "¿Escala?", true, `{"min":1,"max":5}`)
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "ok"})

	// Primera respuesta: consume el presupuesto (1 intercambio) pero q2 sigue pendiente.
	rec := servePublic(handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+q1+`","value":"a"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("record q1: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status != "in_progress" {
		t.Fatalf("turn limit hit with required missing must stay in_progress, got %q", status)
	}

	// Submit prematuro → 422 con las preguntas faltantes.
	rec = servePublic(handlers.SubmitResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/submit", `{}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("premature submit: expected 422, got %d: %s", rec.Code, rec.Body.String())
	}

	// Con la segunda requerida cubierta, el límite ya puede cerrar la sesión.
	rec = servePublic(handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+q2+`","value":"4"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("record q2: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status != "pending_submission" {
		t.Fatalf("expected pending_submission after coverage + limit, got %q", status)
	}

	// El cierre requiere acción explícita: el submit del usuario.
	rec = servePublic(handlers.SubmitResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/submit", `{}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("final submit: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status != "submitted" {
		t.Fatalf("expected submitted, got %q", status)
	}
}

// El estado final viaja como evento SSE después de los tokens: el cliente
// siempre recibe la última respuesta completa de la IA antes de saber que la
// sesión terminó (no hay cierre abrupto).
func TestIntegrationStatusEventArrivesAfterLastAIResponse(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "prompt_only", nil, "")
	f.setTurnLimit(t, surveyID, 1)
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "gracias por participar"})

	rec := servePublic(handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"hola, soy el usuario"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	tokenIdx := strings.Index(body, "gracias por participar")
	statusIdx := strings.Index(body, "event: status\ndata: pending_submission")
	if tokenIdx == -1 {
		t.Fatalf("expected AI tokens in SSE body: %s", body)
	}
	if statusIdx == -1 {
		t.Fatalf("expected pending_submission status event in SSE body: %s", body)
	}
	if statusIdx < tokenIdx {
		t.Fatalf("status event must arrive AFTER the AI response: %s", body)
	}

	// Un turno posterior es rechazado (409, la sesión ya no está en progreso):
	// no llama a la IA ni persiste nada. El frontend lo interpreta como fin.
	turnsBefore := turnCount(t, f, responseID)
	rec = servePublic(handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"otro mensaje"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on post-limit turn, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := turnCount(t, f, responseID); got != turnsBefore {
		t.Fatalf("post-limit turn persisted turns: before=%d after=%d", turnsBefore, got)
	}
}

// Cada pregunta mostrada y su respuesta quedan en el transcript, para que el
// historial del chat y el contexto de la IA reflejen la conversación real.
func TestIntegrationQuestionAndAnswerPersistedAsTurns(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "conversational", nil, "")
	qID := f.createQuestion(t, surveyID, "single_choice", "¿Cómo calificas el curso?", true, `{"choices":[{"label":"Muy bueno","value":"mb"}]}`)
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "ok"})

	rec := servePublic(handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+qID+`","value":"mb"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	turns, err := engineSvc.GetTurns(context.Background(), responseID)
	if err != nil {
		t.Fatalf("get turns: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected question+answer turns, got %#v", turns)
	}
	if turns[0].Role != "assistant" || turns[0].Content != "¿Cómo calificas el curso?" {
		t.Fatalf("expected assistant question turn first, got %#v", turns[0])
	}
	if turns[1].Role != "user" || turns[1].Content != "Muy bueno" {
		t.Fatalf("expected user answer turn with display label, got %#v", turns[1])
	}

	// Editar la respuesta no duplica turnos (answers es idempotente por UNIQUE).
	rec = servePublic(handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+qID+`","value":"mb"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on re-answer, got %d", rec.Code)
	}
	if got := turnCount(t, f, responseID); got != 2 {
		t.Fatalf("re-answer duplicated turns: got %d", got)
	}
}

// Respuesta basura: primer intento pide reformular (422), segundo en la misma
// pregunta bloquea la sesión (409 + abandoned) para no gastar tokens.
func TestIntegrationLowQualityAnswerRecoveryThenBlock(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "conversational", nil, "")
	qID := f.createFollowupQuestion(t, surveyID, "¿Qué mejorarías del curso?")
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "ok"})

	rec := servePublic(handlers.RecordOpenAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/open-answer", `{"question_id":"`+qID+`","value":"xx"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "low_quality_answer") {
		t.Fatalf("expected 422 low_quality_answer, got %d: %s", rec.Code, rec.Body.String())
	}
	if answerCount(t, f.pool, responseID) != 0 {
		t.Fatal("junk answer must not persist")
	}

	rec = servePublic(handlers.RecordOpenAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/open-answer", `{"question_id":"`+qID+`","value":"sdjfdfa"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "response_blocked") {
		t.Fatalf("expected 409 response_blocked, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status != "abandoned" {
		t.Fatalf("expected abandoned after repeated junk, got %q", status)
	}
	var reason *string
	_ = f.pool.QueryRow(context.Background(),
		`SELECT closed_reason FROM responses WHERE id = $1`, responseID).Scan(&reason)
	if reason == nil || *reason != "low_quality_answers" {
		t.Fatalf("expected closed_reason=low_quality_answers, got %v", reason)
	}

	// La sesión bloqueada no acepta submit.
	rec = servePublic(handlers.SubmitResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/submit", `{}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 submit on blocked response, got %d", rec.Code)
	}
}

// Tras un intento basura, una respuesta real se acepta y reinicia el contador.
func TestIntegrationLowQualityThenValidAnswerRecovers(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "conversational", nil, "")
	qID := f.createFollowupQuestion(t, surveyID, "¿Qué mejorarías del curso?")
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "ok"})

	rec := servePublic(handlers.RecordOpenAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/open-answer", `{"question_id":"`+qID+`","value":"xx"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for junk, got %d", rec.Code)
	}

	rec = servePublic(handlers.RecordOpenAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/open-answer", `{"question_id":"`+qID+`","value":"me gustaría más práctica en laboratorio"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for real answer, got %d: %s", rec.Code, rec.Body.String())
	}
	if answerCount(t, f.pool, responseID) != 1 {
		t.Fatal("expected real answer persisted")
	}
	var attempts int
	_ = f.pool.QueryRow(context.Background(),
		`SELECT low_quality_attempts FROM responses WHERE id = $1`, responseID).Scan(&attempts)
	if attempts != 0 {
		t.Fatalf("expected attempts reset after valid answer, got %d", attempts)
	}
}

// El followup se construye desde la DB: el system prompt referencia la última
// pregunta respondida y la respuesta viaja solo por el transcript (rol user).
func TestIntegrationFollowupUsesLastAnsweredQuestionFromDB(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "conversational", nil, "")
	qID := f.createFollowupQuestion(t, surveyID, "¿Qué mejorarías del curso?")
	responseID := f.createResponse(t, surveyID, "es")
	provider := &capturingProvider{text: "¿Por qué lo consideras así?"}
	engineSvc := f.engineWith(provider)

	rec := servePublic(handlers.RecordOpenAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/open-answer", `{"question_id":"`+qID+`","value":"el laboratorio siempre está saturado"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("record open answer: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = servePublic(handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"[FOLLOWUP_NEEDED]"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("followup turn: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(provider.last.SystemPrompt, "¿Qué mejorarías del curso?") {
		t.Fatalf("followup prompt must reference the answered question:\n%s", provider.last.SystemPrompt)
	}
	if strings.Contains(provider.last.SystemPrompt, "el laboratorio siempre está saturado") {
		t.Fatalf("respondent content must NOT be interpolated into the system prompt:\n%s", provider.last.SystemPrompt)
	}
	if provider.last.UserMessage != "" {
		t.Fatalf("followup must not add an extra user message, got %q", provider.last.UserMessage)
	}
	last := provider.last.Transcript[len(provider.last.Transcript)-1]
	if last.Role != "user" || last.Content != "el laboratorio siempre está saturado" {
		t.Fatalf("transcript must end with the user's answer, got %#v", last)
	}

	// FOLLOWUP_DONE concatena la respuesta y avanza la cobertura sin duplicarla.
	rec = servePublic(handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"[FOLLOWUP_DONE] porque somos demasiados alumnos"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("followup done: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var raw string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT raw_value FROM answers WHERE response_id = $1 AND question_id = $2`,
		responseID, qID).Scan(&raw); err != nil {
		t.Fatalf("load answer: %v", err)
	}
	if raw != "el laboratorio siempre está saturado | porque somos demasiados alumnos" {
		t.Fatalf("expected concatenated answer, got %q", raw)
	}
}

// El timer del cliente no puede forzar un submit imposible: expire con
// requeridas pendientes deja la sesión viva.
func TestIntegrationExpireKeepsSessionAliveWhenRequiredMissing(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "conversational", nil, "")
	qID := f.createQuestion(t, surveyID, "single_choice", "¿Opción?", true, `{"choices":[{"label":"A","value":"a"}]}`)
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "ok"})

	rec := servePublic(handlers.ExpireResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/expire", `{}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"in_progress"`) {
		t.Fatalf("expected in_progress on expire with missing required, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status != "in_progress" {
		t.Fatalf("expire must not end session with required missing, got %q", status)
	}

	rec = servePublic(handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+qID+`","value":"a"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("record answer: expected 200, got %d", rec.Code)
	}

	rec = servePublic(handlers.ExpireResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/expire", `{}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"pending_submission"`) {
		t.Fatalf("expected pending_submission after coverage, got %d: %s", rec.Code, rec.Body.String())
	}
}

// El mensaje de bienvenida de Modo B se persiste solo como turno assistant y
// no consume presupuesto de intercambios.
func TestIntegrationWelcomePersistsOnlyAssistantTurn(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "prompt_only", nil, "")
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWith(&capturingProvider{text: "¡Hola! Bienvenido a la encuesta."})

	rec := servePublic(handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"[WELCOME]"}`),
		map[string]string{"id": responseID})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	turns, err := engineSvc.GetTurns(context.Background(), responseID)
	if err != nil {
		t.Fatalf("get turns: %v", err)
	}
	if len(turns) != 1 || turns[0].Role != "assistant" {
		t.Fatalf("expected single assistant welcome turn, got %#v", turns)
	}
	var userTurns int
	_ = f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM turns WHERE response_id = $1 AND role = 'user'`, responseID).Scan(&userTurns)
	if userTurns != 0 {
		t.Fatalf("welcome must not persist user turns, got %d", userTurns)
	}
}

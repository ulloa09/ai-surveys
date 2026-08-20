package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/config"
	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

var (
	integrationMigrateOnce sync.Once
	integrationMigrateErr  error
)

type integrationProvider struct {
	mode string
	text string
}

func (p integrationProvider) StreamTurn(context.Context, ai.TurnRequest) (<-chan ai.TurnChunk, error) {
	if p.mode == "error" {
		return nil, errors.New("provider 500 raw body with sk-secret")
	}
	ch := make(chan ai.TurnChunk, 1)
	if p.mode != "empty" {
		text := p.text
		if text == "" {
			text = "ok"
		}
		ch <- ai.TurnChunk{Token: text}
	}
	close(ch)
	return ch, nil
}

func (p integrationProvider) Extract(context.Context, ai.ExtractionRequest) (ai.ExtractionResult, error) {
	return ai.ExtractionResult{}, nil
}

type integrationFixture struct {
	pool        *pgxpool.Pool
	questionSvc *services.QuestionService
	settingsSvc *services.SettingsService
}

// integrationDSN decide contra qué base corren estos tests, que TRUNCAN todas
// las tablas. Por seguridad NO usan por defecto el DSN de la app (.env): un
// `go test ./...` descuidado apuntaría a la base de desarrollo/Docker y la
// borraría. Reglas:
//   - TEST_DATABASE_URL definido → se usa esa base aislada (camino recomendado).
//   - RUN_DB_TESTS=1 → opt-in explícito para usar el DSN de la app (config .env).
//   - ninguno → se omiten (skip), para que la suite normal nunca trunque datos.
func integrationDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	if os.Getenv("RUN_DB_TESTS") == "1" {
		return config.Load().DB.DSN()
	}
	t.Skip("db integration test skipped: set TEST_DATABASE_URL (isolated db) or RUN_DB_TESTS=1 to run; these tests TRUNCATE all tables")
	return ""
}

func newIntegrationFixture(t *testing.T) integrationFixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, integrationDSN(t))
	if err != nil {
		t.Skipf("postgres integration unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres integration unavailable: %v", err)
	}

	integrationMigrateOnce.Do(func() {
		sqlDB := stdlib.OpenDBFromPool(pool)
		_ = goose.SetDialect("postgres")
		integrationMigrateErr = goose.RunContext(ctx, "up", sqlDB, filepath.Join("..", "..", "migrations"))
	})
	if integrationMigrateErr != nil {
		pool.Close()
		t.Fatalf("migrate test database: %v", integrationMigrateErr)
	}

	truncateIntegrationDB(t, pool)
	settingsSvc, err := services.NewSettingsService(pool, []byte("dev-encryption-key-32-chars-pad!"))
	if err != nil {
		pool.Close()
		t.Fatalf("settings service: %v", err)
	}
	fixture := integrationFixture{
		pool:        pool,
		questionSvc: services.NewQuestionService(pool),
		settingsSvc: settingsSvc,
	}
	t.Cleanup(pool.Close)
	return fixture
}

func truncateIntegrationDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE analysis_results, answers, turns, survey_access_tokens, responses,
		         questions, surveys, team_members, teams, sessions,
		         users, platform_settings
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate integration db: %v", err)
	}
}

func (f integrationFixture) configureAI(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := f.settingsSvc.Set(ctx, services.SettingActiveProvider, "claude"); err != nil {
		t.Fatalf("set active provider: %v", err)
	}
	if err := f.settingsSvc.Set(ctx, services.SettingClaudeKey, "test-key"); err != nil {
		t.Fatalf("set claude key: %v", err)
	}
	if err := f.settingsSvc.Set(ctx, services.SettingClaudeModel, "test-model"); err != nil {
		t.Fatalf("set claude model: %v", err)
	}
}

func (f integrationFixture) engineWithProvider(provider integrationProvider) *services.EngineService {
	return services.NewEngineService(
		f.pool,
		f.settingsSvc,
		f.questionSvc,
		services.WithProviderFactory(func(string, string, string) (ai.AIProvider, error) {
			return provider, nil
		}),
	)
}

// createOwnerTeam crea un admin y un equipo que administra. El admin no se
// agrega como team_member — administra el equipo vía su rol global (ver
// authorizeTeam en services/survey.go), igual que en producción.
func (f integrationFixture) createOwnerTeam(t *testing.T) (models.User, string) {
	t.Helper()
	ctx := context.Background()
	var user models.User
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, role, password_hash)
		VALUES ($1, 'Integration Admin', 'admin', 'hash')
		RETURNING id::text, email, display_name, role, created_at`,
		fmt.Sprintf("integration-%d@test.com", time.Now().UnixNano()),
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.CreatedAt); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	var teamID string
	if err := f.pool.QueryRow(ctx,
		`INSERT INTO teams (name, created_by) VALUES ('Integration Team', $1) RETURNING id::text`,
		user.ID,
	).Scan(&teamID); err != nil {
		t.Fatalf("insert team: %v", err)
	}
	return user, teamID
}

func (f integrationFixture) createSurvey(t *testing.T, mode string, available []string, defaultLanguage string) string {
	t.Helper()
	owner, teamID := f.createOwnerTeam(t)
	if len(available) == 0 {
		available = []string{"es"}
	}
	if defaultLanguage == "" {
		defaultLanguage = available[0]
	}
	var surveyID string
	if err := f.pool.QueryRow(context.Background(), `
		INSERT INTO surveys
			(title, owner_id, team_id, status, mode, anonymity_level,
			 available_languages, default_language, termination_mode, turn_limit)
		VALUES
			('Integration Survey', $1, $2, 'open', $3, 'none', $4, $5, 'turn_limit', 12)
		RETURNING id::text`,
		owner.ID, teamID, mode, available, defaultLanguage,
	).Scan(&surveyID); err != nil {
		t.Fatalf("insert survey: %v", err)
	}
	return surveyID
}

// createResponse inserta una respuesta para la encuesta creada por createSurvey.
func (f integrationFixture) createResponse(t *testing.T, surveyID string, language string) string {
	t.Helper()
	if language == "" {
		language = "es"
	}
	var responseID string
	if err := f.pool.QueryRow(context.Background(), `
		INSERT INTO responses (survey_id, language, status)
		VALUES ($1, $2, 'in_progress')
		RETURNING id::text`,
		surveyID, language,
	).Scan(&responseID); err != nil {
		t.Fatalf("insert response: %v", err)
	}
	return responseID
}

func (f integrationFixture) createQuestion(t *testing.T, surveyID, qType, text string, required bool, options string) string {
	t.Helper()
	var questionID string
	var err error
	if options == "" {
		err = f.pool.QueryRow(context.Background(), `
			INSERT INTO questions (survey_id, type, text, required, order_index)
			VALUES ($1, $2, $3, $4, COALESCE((SELECT MAX(order_index) + 1 FROM questions WHERE survey_id = $1), 0))
			RETURNING id::text`,
			surveyID, qType, text, required,
		).Scan(&questionID)
	} else {
		err = f.pool.QueryRow(context.Background(), `
			INSERT INTO questions (survey_id, type, text, required, options, order_index)
			VALUES ($1, $2, $3, $4, $5::jsonb, COALESCE((SELECT MAX(order_index) + 1 FROM questions WHERE survey_id = $1), 0))
			RETURNING id::text`,
			surveyID, qType, text, required, options,
		).Scan(&questionID)
	}
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}
	return questionID
}

func answerCount(t *testing.T, pool *pgxpool.Pool, responseID string) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM answers WHERE response_id = $1`, responseID,
	).Scan(&count); err != nil {
		t.Fatalf("count answers: %v", err)
	}
	return count
}

func responseStatus(t *testing.T, pool *pgxpool.Pool, responseID string) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM responses WHERE id = $1`, responseID,
	).Scan(&status); err != nil {
		t.Fatalf("load response status: %v", err)
	}
	return status
}

func TestIntegrationProviderFailureDoesNotPersistOrphanUserTurn(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "prompt_only", nil, "")
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWithProvider(integrationProvider{mode: "error"})

	rec := servePublic(
		handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": responseID},
	)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AI provider unavailable") {
		t.Fatalf("expected normalized provider error, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-secret") || strings.Contains(rec.Body.String(), "provider 500") {
		t.Fatalf("raw provider error leaked: %s", rec.Body.String())
	}
	var turnCount int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM turns WHERE response_id = $1`, responseID,
	).Scan(&turnCount); err != nil {
		t.Fatalf("count turns: %v", err)
	}
	if turnCount != 0 {
		t.Fatalf("expected no persisted turns after provider failure, got %d", turnCount)
	}
}

func TestIntegrationProviderSuccessPersistsCompleteExchange(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "prompt_only", nil, "")
	responseID := f.createResponse(t, surveyID, "es")
	engineSvc := f.engineWithProvider(integrationProvider{text: "respuesta IA"})

	rec := servePublic(
		handlers.ProcessTurn(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": responseID},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rows, err := f.pool.Query(context.Background(), `
		SELECT role, content
		FROM turns
		WHERE response_id = $1
		ORDER BY created_at ASC, CASE role WHEN 'user' THEN 0 ELSE 1 END`,
		responseID,
	)
	if err != nil {
		t.Fatalf("load turns: %v", err)
	}
	defer rows.Close()
	var turns []services.Turn
	for rows.Next() {
		var turn services.Turn
		if err := rows.Scan(&turn.Role, &turn.Content); err != nil {
			t.Fatalf("scan turn: %v", err)
		}
		turns = append(turns, turn)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate turns: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected exactly 2 turns, got %#v", turns)
	}
	if turns[0].Role != "user" || turns[0].Content != "hola" {
		t.Fatalf("unexpected user turn: %#v", turns[0])
	}
	if turns[1].Role != "assistant" || turns[1].Content != "respuesta IA" {
		t.Fatalf("unexpected assistant turn: %#v", turns[1])
	}
}

func TestIntegrationStructuredAnswerRejectsQuestionFromAnotherSurvey(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyA := f.createSurvey(t, "form", nil, "")
	responseA := f.createResponse(t, surveyA, "es")
	surveyB := f.createSurvey(t, "form", nil, "")
	questionB := f.createQuestion(t, surveyB, "linear_scale", "Q from B", true, `{"min":1,"max":5}`)
	engineSvc := f.engineWithProvider(integrationProvider{})

	rec := servePublic(
		handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseA+"/answers", `{"question_id":"`+questionB+`","value":"3"}`),
		map[string]string{"id": responseA},
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if answerCount(t, f.pool, responseA) != 0 {
		t.Fatal("expected no answer saved for question from another survey")
	}
}

func TestIntegrationStructuredInvalidAnswersDoNotPersist(t *testing.T) {
	tests := []struct {
		name    string
		qType   string
		options string
		value   string
	}{
		{name: "linear scale out of range", qType: "linear_scale", options: `{"min":1,"max":5}`, value: "999"},
		{name: "single choice outside options", qType: "single_choice", options: `{"choices":[{"label":"A","value":"a"}]}`, value: "z"},
		{name: "multi choice one invalid", qType: "multi_choice", options: `{"choices":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}`, value: "a, z"},
		{name: "open ended through structured endpoint", qType: "open_ended", value: "texto libre"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newIntegrationFixture(t)
			surveyID := f.createSurvey(t, "form", nil, "")
			responseID := f.createResponse(t, surveyID, "es")
			questionID := f.createQuestion(t, surveyID, tt.qType, tt.name, true, tt.options)
			engineSvc := f.engineWithProvider(integrationProvider{})

			rec := servePublic(
				handlers.RecordAnswer(engineSvc),
				jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+questionID+`","value":"`+tt.value+`"}`),
				map[string]string{"id": responseID},
			)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
			}
			if answerCount(t, f.pool, responseID) != 0 {
				t.Fatal("expected invalid structured answer not to persist")
			}
		})
	}
}

func TestIntegrationSubmitGatePersistsOnlyWhenRequiredComplete(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	responseID := f.createResponse(t, surveyID, "es")
	q1 := f.createQuestion(t, surveyID, "linear_scale", "Required scale", true, `{"min":1,"max":5}`)
	q2 := f.createQuestion(t, surveyID, "single_choice", "Required choice", true, `{"choices":[{"label":"A","value":"a"}]}`)
	engineSvc := f.engineWithProvider(integrationProvider{})

	rec := servePublic(
		handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+q1+`","value":"4"}`),
		map[string]string{"id": responseID},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("record first answer: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = servePublic(
		handlers.SubmitResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/submit", `{}`),
		map[string]string{"id": responseID},
	)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("incomplete submit: expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status == "submitted" {
		t.Fatal("response changed to submitted despite missing required question")
	}

	rec = servePublic(
		handlers.RecordAnswer(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/answers", `{"question_id":"`+q2+`","value":"a"}`),
		map[string]string{"id": responseID},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("record second answer: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = servePublic(
		handlers.SubmitResponse(engineSvc),
		jsonReq(http.MethodPost, "/api/responses/"+responseID+"/submit", `{}`),
		map[string]string{"id": responseID},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete submit: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if status := responseStatus(t, f.pool, responseID); status != "submitted" {
		t.Fatalf("expected submitted status, got %q", status)
	}
}

// TestIntegrationActivate_RejectsFormAndConversationalSurveysWithNoQuestions
// cubre el bug reportado: una encuesta 'form'/'conversational' activada sin
// ninguna pregunta deja el link público abriendo una pantalla en blanco para
// siempre (no hay pregunta que mostrar y el modo no dispara el flujo de IA
// libre de prompt_only). Activate ahora rechaza esa transición de entrada.
// prompt_only sigue permitido sin preguntas — no las necesita.
func TestIntegrationActivate_RejectsFormAndConversationalSurveysWithNoQuestions(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := context.Background()
	user, teamID := f.createOwnerTeam(t)
	questionSvc := services.NewQuestionService(f.pool)
	surveySvc := services.NewSurveyService(f.pool, questionSvc, "http://localhost:5173")

	for _, mode := range []string{"form", "conversational"} {
		t.Run(mode, func(t *testing.T) {
			systemPrompt := "Conversar con respeto"
			survey, err := surveySvc.Create(ctx, &user, services.CreateSurveyInput{
				Title:           "Sin preguntas (" + mode + ")",
				TeamID:          teamID,
				AnonymityLevel:  "none",
				Mode:            mode,
				SystemPrompt:    &systemPrompt,
				TerminationMode: "turn_limit",
			})
			if err != nil {
				t.Fatalf("create survey: %v", err)
			}
			if _, err := surveySvc.Activate(ctx, &user, survey.ID); !errors.Is(err, services.ErrInvalidSurveyConfig) {
				t.Fatalf("activate with zero questions: got %v, want ErrInvalidSurveyConfig", err)
			}
			reloaded, err := surveySvc.Get(ctx, &user, survey.ID)
			if err != nil {
				t.Fatalf("reload survey: %v", err)
			}
			if reloaded.Status != "draft" {
				t.Fatalf("status = %q after rejected activation, want draft", reloaded.Status)
			}

			// Agregar una pregunta desbloquea la activación normalmente.
			if _, err := questionSvc.Create(ctx, survey.ID, services.CreateQuestionInput{
				Type: "open_ended", Text: "¿Qué tal?",
			}); err != nil {
				t.Fatalf("create question: %v", err)
			}
			if _, err := surveySvc.Activate(ctx, &user, survey.ID); err != nil {
				t.Fatalf("activate after adding a question: %v", err)
			}
		})
	}
}

// prompt_only no depende de preguntas fijas — activar sin ninguna debe seguir
// funcionando, la IA arranca la conversación con el prompt del sistema.
func TestIntegrationActivate_AllowsPromptOnlySurveysWithNoQuestions(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := context.Background()
	user, teamID := f.createOwnerTeam(t)
	questionSvc := services.NewQuestionService(f.pool)
	surveySvc := services.NewSurveyService(f.pool, questionSvc, "http://localhost:5173")

	systemPrompt := "Entrevista libre"
	survey, err := surveySvc.Create(ctx, &user, services.CreateSurveyInput{
		Title:           "Prompt only sin preguntas",
		TeamID:          teamID,
		AnonymityLevel:  "none",
		Mode:            "prompt_only",
		SystemPrompt:    &systemPrompt,
		TerminationMode: "turn_limit",
	})
	if err != nil {
		t.Fatalf("create survey: %v", err)
	}
	if _, err := surveySvc.Activate(ctx, &user, survey.ID); err != nil {
		t.Fatalf("activate prompt_only with zero questions: %v", err)
	}
}

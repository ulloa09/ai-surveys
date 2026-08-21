package handlers_test

// Tests de integración para el Analysis Engine (#15):
//   - AnalyseResponse extrae sentimiento/tags de una respuesta (1 llamada)
//   - una respuesta sin answers no hace ninguna llamada (no-op)
//   - AnalyseSurvey hace exactamente una llamada por pregunta, no por
//     respuesta × pregunta, y se asegura primero de que cada respuesta
//     enviada tenga su análisis incremental
//   - is_outlier se marca sólo en las respuestas señaladas por el proveedor
//   - re-ejecutar sobrescribe sin error (idempotente) y no duplica filas
//   - una encuesta sin respuestas termina de inmediato, sin llamadas a IA
//
// Requieren Postgres local (se saltan si no está disponible).

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// fakeAnalysisProvider implementa ai.AnalysisProvider para tests de
// integración, contando llamadas para verificar "exactamente una llamada
// por pregunta" sin pegarle a un proveedor real.
type fakeAnalysisProvider struct {
	mu             sync.Mutex
	analyseCalls   int
	aggregateCalls int
	aggregateFn    func(ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error)
}

func (f *fakeAnalysisProvider) AnalyseAnswers(_ context.Context, req ai.AnswerAnalysisRequest) (ai.AnswerAnalysisResult, error) {
	f.mu.Lock()
	f.analyseCalls++
	f.mu.Unlock()
	answers := make([]ai.AnalysedAnswer, 0, len(req.Answers))
	for _, a := range req.Answers {
		answers = append(answers, ai.AnalysedAnswer{
			QuestionID:     a.QuestionID,
			SentimentLabel: "positive",
			SentimentScore: 0.9,
			Tags:           []string{"topic-a", "topic-b"},
		})
	}
	return ai.AnswerAnalysisResult{Answers: answers}, nil
}

func (f *fakeAnalysisProvider) AggregateQuestion(_ context.Context, req ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error) {
	f.mu.Lock()
	f.aggregateCalls++
	f.mu.Unlock()
	if f.aggregateFn != nil {
		return f.aggregateFn(req)
	}
	return ai.QuestionAggregationResult{SummaryText: "auto summary"}, nil
}

func (f integrationFixture) analysisSvcWith(provider ai.AnalysisProvider) *services.AnalysisService {
	return services.NewAnalysisService(f.pool, f.settingsSvc,
		services.WithAnalysisProviderFactory(func(string, string, string) (ai.AnalysisProvider, error) {
			return provider, nil
		}),
	)
}

func (f integrationFixture) createAnswer(t *testing.T, responseID, questionID, rawValue string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`INSERT INTO answers (response_id, question_id, raw_value) VALUES ($1, $2, $3)`,
		responseID, questionID, rawValue,
	); err != nil {
		t.Fatalf("insert answer: %v", err)
	}
}

func (f integrationFixture) markSubmitted(t *testing.T, responseID string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE responses SET status = 'submitted', submitted_at = NOW() WHERE id = $1`, responseID,
	); err != nil {
		t.Fatalf("mark submitted: %v", err)
	}
}

func (f integrationFixture) closeSurveyDirect(t *testing.T, surveyID string) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE surveys SET status = 'closed' WHERE id = $1`, surveyID,
	); err != nil {
		t.Fatalf("close survey: %v", err)
	}
}

func (f integrationFixture) surveyStatus(t *testing.T, surveyID string) string {
	t.Helper()
	var status string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT status FROM surveys WHERE id = $1`, surveyID,
	).Scan(&status); err != nil {
		t.Fatalf("load survey status: %v", err)
	}
	return status
}

func (f integrationFixture) analysisResult(t *testing.T, surveyID, questionID string) (string, models.SentimentDistribution, []models.TopicCluster) {
	t.Helper()
	var summary string
	var distBytes, clustersBytes []byte
	if err := f.pool.QueryRow(context.Background(), `
		SELECT summary_text, sentiment_distribution, topic_clusters
		FROM analysis_results WHERE survey_id = $1 AND question_id = $2`,
		surveyID, questionID,
	).Scan(&summary, &distBytes, &clustersBytes); err != nil {
		t.Fatalf("load analysis result: %v", err)
	}
	var dist models.SentimentDistribution
	if err := json.Unmarshal(distBytes, &dist); err != nil {
		t.Fatalf("unmarshal distribution: %v", err)
	}
	var clusters []models.TopicCluster
	if err := json.Unmarshal(clustersBytes, &clusters); err != nil {
		t.Fatalf("unmarshal clusters: %v", err)
	}
	return summary, dist, clusters
}

func (f integrationFixture) analysisResultCount(t *testing.T, surveyID string) int {
	t.Helper()
	var count int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM analysis_results WHERE survey_id = $1`, surveyID,
	).Scan(&count); err != nil {
		t.Fatalf("count analysis results: %v", err)
	}
	return count
}

func (f integrationFixture) isOutlier(t *testing.T, responseID, questionID string) bool {
	t.Helper()
	var flag bool
	if err := f.pool.QueryRow(context.Background(),
		`SELECT is_outlier FROM answers WHERE response_id = $1 AND question_id = $2`,
		responseID, questionID,
	).Scan(&flag); err != nil {
		t.Fatalf("load is_outlier: %v", err)
	}
	return flag
}

func (f integrationFixture) answerSentiment(t *testing.T, responseID, questionID string) (label string, score float64, tags []string) {
	t.Helper()
	var l *string
	var s *float64
	if err := f.pool.QueryRow(context.Background(), `
		SELECT sentiment_label, sentiment_score, topic_tags
		FROM answers WHERE response_id = $1 AND question_id = $2`,
		responseID, questionID,
	).Scan(&l, &s, &tags); err != nil {
		t.Fatalf("load answer sentiment: %v", err)
	}
	if l != nil {
		label = *l
	}
	if s != nil {
		score = *s
	}
	return label, score, tags
}

func TestIntegrationAnalyseResponse_PopulatesSentimentAndTags(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "How was it?", true, "")
	responseID := f.createResponse(t, surveyID, "es")
	f.createAnswer(t, responseID, q1, "fue genial")
	f.markSubmitted(t, responseID)

	fake := &fakeAnalysisProvider{}
	analysisSvc := f.analysisSvcWith(fake)

	if err := analysisSvc.AnalyseResponse(context.Background(), responseID); err != nil {
		t.Fatalf("analyse response: %v", err)
	}
	if fake.analyseCalls != 1 {
		t.Fatalf("expected exactly 1 analyse call, got %d", fake.analyseCalls)
	}
	label, score, tags := f.answerSentiment(t, responseID, q1)
	if label != "positive" || score != 0.9 {
		t.Fatalf("unexpected sentiment: label=%q score=%v", label, score)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %#v", tags)
	}
}

func TestIntegrationAnalyseResponse_NoAnswersIsNoopWithoutLLMCall(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	responseID := f.createResponse(t, surveyID, "es")

	fake := &fakeAnalysisProvider{}
	analysisSvc := f.analysisSvcWith(fake)

	if err := analysisSvc.AnalyseResponse(context.Background(), responseID); err != nil {
		t.Fatalf("expected no error for response with zero answers, got %v", err)
	}
	if fake.analyseCalls != 0 {
		t.Fatalf("expected zero LLM calls, got %d", fake.analyseCalls)
	}
}

func TestIntegrationAnalyseSurvey_OneCallPerQuestionOutliersAndIdempotent(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "What did you think?", true, "")
	q2 := f.createQuestion(t, surveyID, "open_ended", "Anything else?", false, "")

	var responseIDs []string
	for i := 0; i < 3; i++ {
		r := f.createResponse(t, surveyID, "es")
		f.createAnswer(t, r, q1, "respuesta a q1")
		f.createAnswer(t, r, q2, "respuesta a q2")
		f.markSubmitted(t, r)
		responseIDs = append(responseIDs, r)
	}
	f.closeSurveyDirect(t, surveyID)

	outlier := responseIDs[0]
	fake := &fakeAnalysisProvider{
		aggregateFn: func(req ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error) {
			return ai.QuestionAggregationResult{
				SummaryText:        "summary for " + req.QuestionText,
				OutlierResponseIDs: []string{outlier},
			}, nil
		},
	}
	analysisSvc := f.analysisSvcWith(fake)

	if err := analysisSvc.AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("analyse survey: %v", err)
	}

	if fake.analyseCalls != 3 {
		t.Fatalf("expected 3 per-response analysis calls (one per response, not per answer), got %d", fake.analyseCalls)
	}
	if fake.aggregateCalls != 2 {
		t.Fatalf("expected exactly one aggregation call per question (2 questions), got %d", fake.aggregateCalls)
	}
	if status := f.surveyStatus(t, surveyID); status != "complete" {
		t.Fatalf("expected survey to transition to complete, got %q", status)
	}

	summary1, dist1, _ := f.analysisResult(t, surveyID, q1)
	if summary1 != "summary for What did you think?" {
		t.Fatalf("unexpected summary: %q", summary1)
	}
	if dist1.Positive != 1.0 || dist1.Neutral != 0 || dist1.Negative != 0 {
		t.Fatalf("expected all-positive distribution from fake provider, got %#v", dist1)
	}

	if !f.isOutlier(t, outlier, q1) {
		t.Fatal("expected flagged response to be marked outlier on q1")
	}
	if !f.isOutlier(t, outlier, q2) {
		t.Fatal("expected flagged response to be marked outlier on q2")
	}
	for _, r := range responseIDs[1:] {
		if f.isOutlier(t, r, q1) {
			t.Fatalf("expected non-flagged response %s not to be outlier on q1", r)
		}
	}

	// Idempotencia: re-ejecutar sobrescribe sin error. El paso incremental no
	// vuelve a llamar al proveedor (ya no hay respuestas con sentiment NULL),
	// pero la agregación sí — siempre produce resultados frescos.
	if err := analysisSvc.AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("re-run analyse survey: %v", err)
	}
	if fake.analyseCalls != 3 {
		t.Fatalf("expected no extra per-response calls on re-run, got %d", fake.analyseCalls)
	}
	if fake.aggregateCalls != 4 {
		t.Fatalf("expected 2 more aggregation calls on re-run (4 total), got %d", fake.aggregateCalls)
	}
	if count := f.analysisResultCount(t, surveyID); count != 2 {
		t.Fatalf("expected exactly 2 analysis_results rows (one per question) after re-run, got %d", count)
	}
}

func TestIntegrationAnalyseSurvey_EmptyResponseSetCompletesWithoutLLMCall(t *testing.T) {
	f := newIntegrationFixture(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "Unanswered question", false, "")
	f.closeSurveyDirect(t, surveyID)

	// Deliberadamente sin f.configureAI(t): si el job intentara llamar al
	// proveedor, fallaría por falta de API key configurada.
	fake := &fakeAnalysisProvider{}
	analysisSvc := f.analysisSvcWith(fake)

	if err := analysisSvc.AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("expected no crash on empty response set, got %v", err)
	}
	if fake.analyseCalls != 0 || fake.aggregateCalls != 0 {
		t.Fatalf("expected zero LLM calls for empty response set, got analyse=%d aggregate=%d",
			fake.analyseCalls, fake.aggregateCalls)
	}
	if status := f.surveyStatus(t, surveyID); status != "complete" {
		t.Fatalf("expected survey to complete immediately, got %q", status)
	}
	summary, dist, clusters := f.analysisResult(t, surveyID, q1)
	if summary != "" {
		t.Fatalf("expected empty summary, got %q", summary)
	}
	if (dist != models.SentimentDistribution{}) {
		t.Fatalf("expected zero distribution, got %#v", dist)
	}
	if len(clusters) != 0 {
		t.Fatalf("expected no topic clusters, got %#v", clusters)
	}
}

// TestIntegrationAnalyseSurvey_ProviderErrorMarksSurveyFailedInsteadOfStuck
// cubre la regresión reportada: antes de este fix, cualquier error del
// proveedor a mitad de AnalyseSurvey (JSON inválido, rate limit, etc.) dejaba
// la encuesta en 'analysing' para siempre, sin salida — la UI mostraba
// "Analizando" indefinidamente. Ahora debe aterrizar en 'failed', y
// reintentar (llamando AnalyseSurvey de nuevo, como hace RetryAnalysis) debe
// poder destrabarla sin pasar por cerrar/reabrir.
func TestIntegrationAnalyseSurvey_ProviderErrorMarksSurveyFailedInsteadOfStuck(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "What did you think?", true, "")
	r := f.createResponse(t, surveyID, "es")
	f.createAnswer(t, r, q1, "respuesta a q1")
	f.markSubmitted(t, r)
	f.closeSurveyDirect(t, surveyID)

	boom := errors.New("boom: provider returned malformed JSON")
	failing := &fakeAnalysisProvider{
		aggregateFn: func(ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error) {
			return ai.QuestionAggregationResult{}, boom
		},
	}
	analysisSvc := f.analysisSvcWith(failing)

	if err := analysisSvc.AnalyseSurvey(context.Background(), surveyID); err == nil {
		t.Fatal("expected AnalyseSurvey to propagate the provider error")
	}
	if status := f.surveyStatus(t, surveyID); status != "failed" {
		t.Fatalf("expected survey to land on 'failed' instead of stuck 'analysing', got %q", status)
	}

	working := &fakeAnalysisProvider{}
	retrySvc := f.analysisSvcWith(working)
	if err := retrySvc.AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if status := f.surveyStatus(t, surveyID); status != "complete" {
		t.Fatalf("expected survey to complete after retry, got %q", status)
	}
}

// TestIntegrationSubmit_TriggersIncrementalAnalysisAsynchronously verifica el
// cableado real (no solo AnalyseResponse en aislamiento): EngineService.Submit
// debe encolar el job y el worker de AnalysisService.Run debe procesarlo,
// igual que en producción (main.go arranca Run() una sola vez).
func TestIntegrationSubmit_TriggersIncrementalAnalysisAsynchronously(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "How was it?", true, "")
	responseID := f.createResponse(t, surveyID, "es")
	f.createAnswer(t, responseID, q1, "great")

	fake := &fakeAnalysisProvider{}
	analysisSvc := f.analysisSvcWith(fake)
	engineSvc := services.NewEngineService(f.pool, f.settingsSvc, f.questionSvc,
		services.WithEngineAnalysisService(analysisSvc))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go analysisSvc.Run(ctx)

	if err := engineSvc.Submit(context.Background(), responseID); err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var label string
	for time.Now().Before(deadline) {
		label, _, _ = f.answerSentiment(t, responseID, q1)
		if label != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if label != "positive" {
		t.Fatalf("expected async incremental analysis to populate sentiment within 2s, got label=%q", label)
	}
}

// TestIntegrationSurveyClose_TriggersAnalysisJobAsynchronously verifica que
// cerrar una encuesta (SurveyService.Close) encola y el worker en segundo
// plano la lleva hasta 'complete', igual que en producción.
func TestIntegrationSurveyClose_TriggersAnalysisJobAsynchronously(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	user, teamID := f.createOwnerTeam(t)
	questionSvc := services.NewQuestionService(f.pool)

	fake := &fakeAnalysisProvider{}
	analysisSvc := f.analysisSvcWith(fake)
	surveySvc := services.NewSurveyService(f.pool, questionSvc, "http://localhost:5173",
		services.WithSurveyAnalysisService(analysisSvc))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go analysisSvc.Run(ctx)

	turnLimit := 5
	survey, err := surveySvc.Create(context.Background(), &user, services.CreateSurveyInput{
		Title:           "Close trigger survey",
		TeamID:          teamID,
		AnonymityLevel:  "none",
		Mode:            "form",
		TerminationMode: "turn_limit",
		TurnLimit:       &turnLimit,
	})
	if err != nil {
		t.Fatalf("create survey: %v", err)
	}
	if _, err := questionSvc.Create(context.Background(), survey.ID, services.CreateQuestionInput{
		Type: "open_ended", Text: "Q1",
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
	if _, err := surveySvc.Activate(context.Background(), &user, survey.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := surveySvc.Close(context.Background(), &user, survey.ID); err != nil {
		t.Fatalf("close: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		status = f.surveyStatus(t, survey.ID)
		if status == "complete" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if status != "complete" {
		t.Fatalf("expected survey to reach complete via async trigger within 2s, got %q", status)
	}
}

// Los temas y los outliers solo tienen sentido donde hay texto libre. En una
// escala o una opción múltiple, pedirlos hacía que el proveedor "detectara"
// como atípica cualquier calificación baja: el dashboard terminaba mostrando
// una bandera roja junto a un "1" pelón. Ahora el engine no los pide, e ignora
// los que el proveedor mande de todas formas.
func TestIntegrationAnalyseSurvey_StructuredQuestionSkipsTopicsAndOutliers(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	open := f.createQuestion(t, surveyID, "open_ended", "¿Qué cambiarías?", true, "")
	scale := f.createQuestion(t, surveyID, "linear_scale", "¿Qué tan claro fue?", true, `{"min":1,"max":5}`)

	r := f.createResponse(t, surveyID, "es")
	f.createAnswer(t, r, open, "el ritmo de las entregas es brutal")
	f.createAnswer(t, r, scale, "1")
	f.markSubmitted(t, r)
	f.closeSurveyDirect(t, surveyID)

	// El fake devuelve outliers y clusters SIEMPRE, se los pidan o no: así el
	// test verifica que el engine los descarta en la pregunta estructurada en
	// vez de simplemente no recibirlos.
	byQuestion := map[string]ai.QuestionAggregationRequest{}
	fake := &fakeAnalysisProvider{
		aggregateFn: func(req ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error) {
			byQuestion[req.QuestionText] = req
			return ai.QuestionAggregationResult{
				SummaryText:        "resumen",
				TopicClusters:      []ai.TopicCluster{{Tag: "ritmo", Count: 1}},
				OutlierResponseIDs: []string{r},
			}, nil
		},
	}
	if err := f.analysisSvcWith(fake).AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("analyse survey: %v", err)
	}

	if req := byQuestion["¿Qué tan claro fue?"]; req.DetectOutliers || req.ClusterTopics {
		t.Fatalf("structured question must not request outliers/topics, got DetectOutliers=%v ClusterTopics=%v",
			req.DetectOutliers, req.ClusterTopics)
	}
	if req := byQuestion["¿Qué cambiarías?"]; !req.DetectOutliers || !req.ClusterTopics {
		t.Fatalf("open question must request outliers/topics, got DetectOutliers=%v ClusterTopics=%v",
			req.DetectOutliers, req.ClusterTopics)
	}

	if f.isOutlier(t, r, scale) {
		t.Fatal("a low rating is not an outlier: structured answers must never be flagged")
	}
	if !f.isOutlier(t, r, open) {
		t.Fatal("expected the open-ended answer flagged by the provider to be an outlier")
	}

	if _, _, clusters := f.analysisResult(t, surveyID, scale); len(clusters) != 0 {
		t.Fatalf("expected no topic clusters on a structured question, got %#v", clusters)
	}
}

// Los clusters los agrupa el proveedor (que ve todas las respuestas de la
// pregunta en una sola llamada), no el conteo local por igualdad exacta de
// tags: el fake devuelve tags idénticos en cada respuesta ("topic-a"/"topic-b"),
// así que si el engine cayera al conteo local se verían esos y no los del
// proveedor.
func TestIntegrationAnalyseSurvey_UsesProviderTopicClusters(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q := f.createQuestion(t, surveyID, "open_ended", "¿Qué opinas?", true, "")

	for i := 0; i < 3; i++ {
		r := f.createResponse(t, surveyID, "es")
		f.createAnswer(t, r, q, "los laboratorios prácticos son lo mejor")
		f.markSubmitted(t, r)
	}
	f.closeSurveyDirect(t, surveyID)

	fake := &fakeAnalysisProvider{
		aggregateFn: func(ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error) {
			return ai.QuestionAggregationResult{
				SummaryText: "resumen",
				TopicClusters: []ai.TopicCluster{
					{Tag: "Laboratorios", Count: 3},
					{Tag: "profesor", Count: 1},
				},
			}, nil
		},
	}
	if err := f.analysisSvcWith(fake).AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("analyse survey: %v", err)
	}

	_, _, clusters := f.analysisResult(t, surveyID, q)
	if len(clusters) != 2 {
		t.Fatalf("expected the provider's 2 clusters, got %#v", clusters)
	}
	// Normalizado a minúsculas y ordenado por frecuencia descendente.
	if clusters[0].Tag != "laboratorios" || clusters[0].Count != 3 {
		t.Fatalf("expected most-mentioned provider cluster first, got %#v", clusters[0])
	}
	for _, c := range clusters {
		if c.Tag == "topic-a" || c.Tag == "topic-b" {
			t.Fatalf("clusters fell back to local tag counting instead of the provider: %#v", clusters)
		}
	}
}

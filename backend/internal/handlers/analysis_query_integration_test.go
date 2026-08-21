package handlers_test

// Tests de integración del lado de lectura del Analysis Engine (#16):
//   - StatsForSurveys agrega respuestas por encuesta en una sola consulta, e
//     incluye con ceros a las encuestas sin respuestas
//   - la tasa de completado, la duración promedio y la distribución de idioma
//     salen de las respuestas reales
//   - SurveyAnalysis devuelve las preguntas en orden, con su resumen,
//     sentimiento, temas y outliers
//   - una pregunta que el job todavía no agregó aparece con agregados en cero
//
// Requieren Postgres local (se saltan si no está disponible).

import (
	"context"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
)

// submitWithDuration marca una respuesta como enviada con una duración exacta,
// para poder verificar el promedio sin depender del reloj del test.
func (f integrationFixture) submitWithDuration(t *testing.T, responseID string, seconds int) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE responses
		SET status = 'submitted', started_at = NOW() - make_interval(secs => $2), submitted_at = NOW()
		WHERE id = $1`, responseID, seconds,
	); err != nil {
		t.Fatalf("submit with duration: %v", err)
	}
}

func TestIntegrationStatsForSurveys_AggregatesResponsesPerSurvey(t *testing.T) {
	f := newIntegrationFixture(t)
	analysisSvc := f.analysisSvcWith(&fakeAnalysisProvider{})

	withResponses := f.createSurvey(t, "form", []string{"es", "en"}, "es")
	// Dos enviadas (60s y 120s) y una en progreso: 2/3 de completado y 90s de
	// promedio, que solo considera las enviadas.
	r1 := f.createResponse(t, withResponses, "es")
	f.submitWithDuration(t, r1, 60)
	r2 := f.createResponse(t, withResponses, "en")
	f.submitWithDuration(t, r2, 120)
	f.createResponse(t, withResponses, "es")

	// Una segunda encuesta sin ninguna respuesta: tiene que aparecer igual,
	// con ceros, y no desaparecer del resultado por el LEFT JOIN.
	empty := f.createSurvey(t, "form", nil, "")

	stats, err := analysisSvc.StatsForSurveys(context.Background(), []string{withResponses, empty})
	if err != nil {
		t.Fatalf("stats for surveys: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected one entry per requested survey, got %d", len(stats))
	}

	// El orden de salida sigue al de entrada, para que el listado pueda cruzar
	// por índice o por id sin reordenar.
	if stats[0].SurveyID != withResponses || stats[1].SurveyID != empty {
		t.Fatalf("expected stats in requested order, got %s then %s", stats[0].SurveyID, stats[1].SurveyID)
	}

	got := stats[0]
	if got.ResponseCount != 3 {
		t.Fatalf("response_count = %d, want 3 (incluye la respuesta en progreso)", got.ResponseCount)
	}
	if got.CompletedCount != 2 {
		t.Fatalf("completed_count = %d, want 2", got.CompletedCount)
	}
	if got.CompletionRate < 0.66 || got.CompletionRate > 0.67 {
		t.Fatalf("completion_rate = %v, want ~0.667", got.CompletionRate)
	}
	if got.AvgDurationSeconds == nil {
		t.Fatal("avg_duration_seconds = nil, want ~90s promediando solo las enviadas")
	}
	if *got.AvgDurationSeconds < 89 || *got.AvgDurationSeconds > 91 {
		t.Fatalf("avg_duration_seconds = %v, want ~90", *got.AvgDurationSeconds)
	}

	// Dos respuestas en español, una en inglés — de mayor a menor.
	if len(got.LanguageDistribution) != 2 {
		t.Fatalf("language_distribution = %+v, want dos idiomas", got.LanguageDistribution)
	}
	if got.LanguageDistribution[0].Language != "es" || got.LanguageDistribution[0].Count != 2 {
		t.Fatalf("language_distribution[0] = %+v, want es con 2", got.LanguageDistribution[0])
	}
	if got.LanguageDistribution[1].Language != "en" || got.LanguageDistribution[1].Count != 1 {
		t.Fatalf("language_distribution[1] = %+v, want en con 1", got.LanguageDistribution[1])
	}

	blank := stats[1]
	if blank.ResponseCount != 0 || blank.CompletedCount != 0 || blank.CompletionRate != 0 {
		t.Fatalf("encuesta sin respuestas = %+v, want todo en cero", blank)
	}
	if blank.AvgDurationSeconds != nil {
		t.Fatalf("avg_duration_seconds = %v, want nil sin respuestas enviadas", *blank.AvgDurationSeconds)
	}
	if blank.LanguageDistribution == nil {
		t.Fatal("language_distribution = nil, want un array vacío para serializar como []")
	}
}

func TestIntegrationSurveyAnalysis_ReturnsQuestionsInOrderWithOutliers(t *testing.T) {
	f := newIntegrationFixture(t)
	f.configureAI(t)
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "¿Qué te pareció?", true, "")
	q2 := f.createQuestion(t, surveyID, "open_ended", "¿Algo más?", false, "")

	var responseIDs []string
	for i := 0; i < 3; i++ {
		r := f.createResponse(t, surveyID, "es")
		f.createAnswer(t, r, q1, "respuesta a q1")
		f.createAnswer(t, r, q2, "respuesta a q2")
		f.submitWithDuration(t, r, 30)
		responseIDs = append(responseIDs, r)
	}
	f.closeSurveyDirect(t, surveyID)

	outlier := responseIDs[0]
	// Solo q1 tiene outlier: así se verifica que se agrupan por pregunta y no
	// se derraman a las demás secciones.
	analysisSvc := f.analysisSvcWith(&fakeAnalysisProvider{
		aggregateFn: func(req ai.QuestionAggregationRequest) (ai.QuestionAggregationResult, error) {
			result := ai.QuestionAggregationResult{SummaryText: "resumen de " + req.QuestionText}
			if req.QuestionText == "¿Qué te pareció?" {
				result.OutlierResponseIDs = []string{outlier}
			}
			return result, nil
		},
	})
	if err := analysisSvc.AnalyseSurvey(context.Background(), surveyID); err != nil {
		t.Fatalf("analyse survey: %v", err)
	}

	analysis, err := analysisSvc.SurveyAnalysis(context.Background(), surveyID)
	if err != nil {
		t.Fatalf("survey analysis: %v", err)
	}

	if analysis.Status != "complete" {
		t.Fatalf("status = %q, want complete", analysis.Status)
	}
	if analysis.AnalysedAt == nil {
		t.Fatal("analysed_at = nil, want la marca de tiempo del análisis más reciente")
	}
	if analysis.Stats.CompletedCount != 3 {
		t.Fatalf("stats.completed_count = %d, want 3", analysis.Stats.CompletedCount)
	}

	if len(analysis.Questions) != 2 {
		t.Fatalf("questions = %d, want 2", len(analysis.Questions))
	}
	first, second := analysis.Questions[0], analysis.Questions[1]
	if first.QuestionID != q1 || second.QuestionID != q2 {
		t.Fatal("las preguntas no salieron en orden de order_index")
	}

	if first.SummaryText != "resumen de ¿Qué te pareció?" {
		t.Fatalf("summary_text = %q", first.SummaryText)
	}
	if first.AnswerCount != 3 {
		t.Fatalf("answer_count = %d, want 3", first.AnswerCount)
	}
	// El fake etiqueta todo como positive y devuelve dos tags por respuesta.
	if first.SentimentDistribution.Positive != 1.0 {
		t.Fatalf("sentiment_distribution = %+v, want 100%% positivo", first.SentimentDistribution)
	}
	if len(first.TopicClusters) != 2 {
		t.Fatalf("topic_clusters = %+v, want los dos tags del proveedor", first.TopicClusters)
	}

	if len(first.Outliers) != 1 {
		t.Fatalf("outliers de q1 = %+v, want exactamente 1", first.Outliers)
	}
	if first.Outliers[0].ResponseID != outlier || first.Outliers[0].RawValue != "respuesta a q1" {
		t.Fatalf("outlier = %+v, want la respuesta marcada con su texto original", first.Outliers[0])
	}
	if len(second.Outliers) != 0 {
		t.Fatalf("outliers de q2 = %+v, want ninguno", second.Outliers)
	}
}

// Mientras el job corre ('analysing'), la vista tiene que ser legible: las
// preguntas que todavía no tienen fila en analysis_results aparecen con los
// agregados en cero en vez de romper el LEFT JOIN.
func TestIntegrationSurveyAnalysis_UnanalysedQuestionHasZeroedAggregates(t *testing.T) {
	f := newIntegrationFixture(t)
	analysisSvc := f.analysisSvcWith(&fakeAnalysisProvider{})
	surveyID := f.createSurvey(t, "form", nil, "")
	q1 := f.createQuestion(t, surveyID, "open_ended", "Sin analizar todavía", false, "")

	analysis, err := analysisSvc.SurveyAnalysis(context.Background(), surveyID)
	if err != nil {
		t.Fatalf("survey analysis: %v", err)
	}
	if len(analysis.Questions) != 1 {
		t.Fatalf("questions = %d, want 1", len(analysis.Questions))
	}

	q := analysis.Questions[0]
	if q.QuestionID != q1 || q.Text != "Sin analizar todavía" {
		t.Fatalf("question = %+v, want la pregunta sin analizar", q)
	}
	if q.AnalysedAt != nil {
		t.Fatalf("analysed_at = %v, want nil", q.AnalysedAt)
	}
	if q.SummaryText != "" {
		t.Fatalf("summary_text = %q, want vacío", q.SummaryText)
	}
	if q.AnswerCount != 0 {
		t.Fatalf("answer_count = %d, want 0", q.AnswerCount)
	}
	if q.SentimentDistribution.Positive != 0 || q.SentimentDistribution.Negative != 0 {
		t.Fatalf("sentiment_distribution = %+v, want en cero", q.SentimentDistribution)
	}
	// Slices no-nil para que el JSON sea [] y no null.
	if q.TopicClusters == nil || q.Outliers == nil {
		t.Fatal("topic_clusters y outliers deben ser arrays vacíos, no nil")
	}
	if analysis.AnalysedAt != nil {
		t.Fatal("analysed_at de la encuesta debe ser nil si ninguna pregunta se analizó")
	}
}

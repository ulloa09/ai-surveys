package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

// AnalysisService implementa el Analysis Engine (#15): sentimiento/tags por
// respuesta (incremental, disparado por survey.submitted) y resumen/topic
// clusters/outliers por pregunta (batch, disparado al cerrar la encuesta).
//
// El job runner es un goroutine + channel (no polling): TriggerResponseAnalysis
// y TriggerSurveyAnalysis encolan trabajo sin bloquear al caller (Submit /
// Close), y Run() lo procesa en segundo plano. Es idempotente: volver a
// correr AnalyseResponse o AnalyseSurvey sobrescribe los resultados sin
// error, así que perder un job encolado ante un crash es aceptable — el
// próximo submit/close (o un re-trigger manual) lo repone.
type AnalysisService struct {
	db              *pgxpool.Pool
	settingsSvc     *SettingsService
	providerFactory func(providerName, apiKey, model string) (ai.AnalysisProvider, error)
	jobs            chan analysisJob
}

type AnalysisOption func(*AnalysisService)

// WithAnalysisProviderFactory inyecta un factory de proveedor distinto —
// lo usan los tests para pasar un fake sin llamar a un proveedor real.
func WithAnalysisProviderFactory(factory func(providerName, apiKey, model string) (ai.AnalysisProvider, error)) AnalysisOption {
	return func(a *AnalysisService) {
		if factory != nil {
			a.providerFactory = factory
		}
	}
}

// analysisJobQueueSize acota cuántos jobs pendientes se bufferizan antes de
// empezar a descartarlos (ver Trigger*). Si se llena es señal de que el
// worker está trabado — no vale la pena bloquear al request HTTP por eso.
const analysisJobQueueSize = 256

func NewAnalysisService(db *pgxpool.Pool, settingsSvc *SettingsService, opts ...AnalysisOption) *AnalysisService {
	a := &AnalysisService{
		db:              db,
		settingsSvc:     settingsSvc,
		providerFactory: ai.NewAnalysisProvider,
		jobs:            make(chan analysisJob, analysisJobQueueSize),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type analysisJobKind int

const (
	analysisJobResponse analysisJobKind = iota
	analysisJobSurvey
)

type analysisJob struct {
	kind       analysisJobKind
	responseID string
	surveyID   string
}

// TriggerResponseAnalysis encola el análisis incremental de una respuesta
// recién enviada (survey.submitted). No bloquea al caller.
func (a *AnalysisService) TriggerResponseAnalysis(responseID string) {
	select {
	case a.jobs <- analysisJob{kind: analysisJobResponse, responseID: responseID}:
	default:
		log.Printf("analysis: job queue full, dropping response job %s", responseID)
	}
}

// TriggerSurveyAnalysis encola el job batch disparado al cerrar una encuesta
// (manual o por RunScheduledTransitions). No bloquea al caller.
func (a *AnalysisService) TriggerSurveyAnalysis(surveyID string) {
	select {
	case a.jobs <- analysisJob{kind: analysisJobSurvey, surveyID: surveyID}:
	default:
		log.Printf("analysis: job queue full, dropping survey job %s", surveyID)
	}
}

// Run consume la cola de jobs hasta que ctx se cancela. Se arranca una sola
// vez desde main(), en su propia goroutine — mismo patrón que startScheduler.
func (a *AnalysisService) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-a.jobs:
			a.process(job)
		}
	}
}

func (a *AnalysisService) process(job analysisJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("analysis: panic recovered: %v", r)
		}
	}()
	switch job.kind {
	case analysisJobResponse:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.AnalyseResponse(ctx, job.responseID); err != nil {
			log.Printf("analysis: response %s: %v", job.responseID, err)
		}
	case analysisJobSurvey:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := a.AnalyseSurvey(ctx, job.surveyID); err != nil {
			log.Printf("analysis: survey %s: %v", job.surveyID, err)
		}
	}
}

// newProvider arma el proveedor activo leyendo platform_settings, igual que
// EngineService.
func (a *AnalysisService) newProvider(ctx context.Context) (ai.AnalysisProvider, error) {
	cfg, err := a.settingsSvc.GetAIConfig(ctx)
	if err != nil || cfg.APIKey == "" {
		return nil, ErrAIProviderUnavailable
	}
	return a.providerFactory(cfg.Provider, cfg.APIKey, cfg.Model)
}

// AnalyseResponse analiza sentimiento y tags de todas las respuestas de un
// participante en una sola llamada al proveedor (una llamada por respuesta,
// no por pregunta). Idempotente: sobrescribe cualquier resultado previo.
// Si la respuesta no tiene answers todavía, no hace nada (no hay nada que
// analizar, no es un error).
func (a *AnalysisService) AnalyseResponse(ctx context.Context, responseID string) error {
	if !isUUID(responseID) {
		return ErrResponseNotFound
	}

	rows, err := a.db.Query(ctx, `
		SELECT a.question_id::text, a.raw_value, q.text, q.type, r.language
		FROM answers a
		JOIN questions q ON q.id = a.question_id
		JOIN responses r ON r.id = a.response_id
		WHERE a.response_id = $1
		ORDER BY q.order_index ASC`, responseID)
	if err != nil {
		return err
	}
	var answers []ai.AnswerToAnalyse
	var language string
	for rows.Next() {
		var ans ai.AnswerToAnalyse
		if err := rows.Scan(&ans.QuestionID, &ans.Value, &ans.QuestionText, &ans.QuestionType, &language); err != nil {
			rows.Close()
			return err
		}
		answers = append(answers, ans)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(answers) == 0 {
		return nil
	}

	provider, err := a.newProvider(ctx)
	if err != nil {
		return err
	}
	result, err := provider.AnalyseAnswers(ctx, ai.AnswerAnalysisRequest{Answers: answers, Language: language})
	if err != nil {
		return fmt.Errorf("analysis: analyse answers: %w", err)
	}

	for _, res := range result.Answers {
		if _, err := a.db.Exec(ctx, `
			UPDATE answers
			SET sentiment_label = $1, sentiment_score = $2, topic_tags = $3
			WHERE response_id = $4 AND question_id = $5`,
			res.SentimentLabel, res.SentimentScore, res.Tags, responseID, res.QuestionID,
		); err != nil {
			return err
		}
	}
	return nil
}

// analysisQuestion es la vista mínima de una pregunta que necesita
// AnalyseSurvey para armar el prompt de agregación.
type analysisQuestion struct {
	id   string
	text string
	kind string
}

// AnalyseSurvey corre la agregación batch de una encuesta: por cada
// pregunta, hace exactamente una llamada al proveedor para el resumen y la
// detección de outliers (distribución de sentimiento y topic clusters se
// calculan localmente a partir de los answers ya analizados). Antes de
// agregar, se asegura de que toda respuesta enviada tenga su análisis
// incremental (por si llegó antes de que #15 estuviera activo, o el paso
// incremental falló).
//
// Idempotente: se puede volver a correr sobre una encuesta ya analizada y
// sobrescribe analysis_results y answers.is_outlier sin error. Una encuesta
// sin respuestas (o sin preguntas) termina de inmediato, sin llamadas a IA.
//
// Si algo falla después de marcar 'analysing' (proveedor caído, JSON
// inválido, timeout, panic), el defer de abajo marca la encuesta como
// 'failed' en vez de dejarla colgada para siempre — antes de este fix
// cualquier error a mitad de camino dejaba el status en 'analysing' sin
// salida, y la única señal era una línea de log que nadie veía. 'failed' es
// un estado terminal visible en la UI, y RetryAnalysis puede volver a
// encolar el job desde ahí.
func (a *AnalysisService) AnalyseSurvey(ctx context.Context, surveyID string) (err error) {
	if !isUUID(surveyID) {
		return ErrSurveyNotFound
	}

	tag, err := a.db.Exec(ctx,
		`UPDATE surveys SET status = 'analysing', updated_at = NOW() WHERE id = $1`, surveyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSurveyNotFound
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("analysis: panic: %v", r)
		}
		if err != nil {
			// ctx puede venir ya cancelado/expirado (timeout del job) — usamos
			// uno propio para que marcar 'failed' no se pierda por lo mismo que
			// causó la falla.
			failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, updErr := a.db.Exec(failCtx,
				`UPDATE surveys SET status = 'failed', updated_at = NOW() WHERE id = $1 AND status = 'analysing'`,
				surveyID,
			); updErr != nil {
				log.Printf("analysis: survey %s: failed to mark status as failed after error %v: %v", surveyID, err, updErr)
			}
		}
	}()

	if err = a.ensureResponsesAnalysed(ctx, surveyID); err != nil {
		return err
	}

	questions, err := a.loadQuestions(ctx, surveyID)
	if err != nil {
		return err
	}

	var provider ai.AnalysisProvider
	for _, q := range questions {
		if err = a.aggregateQuestion(ctx, surveyID, q, &provider); err != nil {
			return fmt.Errorf("analysis: question %s: %w", q.id, err)
		}
	}

	if _, err = a.db.Exec(ctx,
		`UPDATE surveys SET status = 'complete', updated_at = NOW() WHERE id = $1`, surveyID); err != nil {
		return err
	}
	return nil
}

// ensureResponsesAnalysed corre el análisis incremental sobre cualquier
// respuesta enviada que todavía no tenga sentimiento (al menos un answer con
// sentiment_label NULL), antes de agregar por pregunta.
func (a *AnalysisService) ensureResponsesAnalysed(ctx context.Context, surveyID string) error {
	rows, err := a.db.Query(ctx, `
		SELECT DISTINCT r.id::text
		FROM responses r
		JOIN answers ans ON ans.response_id = r.id
		WHERE r.survey_id = $1 AND r.status = 'submitted' AND ans.sentiment_label IS NULL`, surveyID)
	if err != nil {
		return err
	}
	var pending []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range pending {
		if err := a.AnalyseResponse(ctx, id); err != nil {
			return fmt.Errorf("ensure response %s analysed: %w", id, err)
		}
	}
	return nil
}

func (a *AnalysisService) loadQuestions(ctx context.Context, surveyID string) ([]analysisQuestion, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id::text, text, type FROM questions WHERE survey_id = $1 ORDER BY order_index ASC`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var questions []analysisQuestion
	for rows.Next() {
		var q analysisQuestion
		if err := rows.Scan(&q.id, &q.text, &q.kind); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// aggregateQuestion resume, agrupa temas y detecta outliers para una pregunta
// (una sola llamada al proveedor si hay al menos una respuesta: las tres cosas
// viajan en el mismo JSON), calcula localmente la distribución de sentimiento y
// persiste todo (analysis_results + answers.is_outlier) de forma idempotente.
// Temas y outliers solo se piden en preguntas abiertas — ver isOpenEnded.
// provider se pasa por referencia y se inicializa perezosamente la primera
// vez que hace falta, para no requerir un proveedor configurado cuando
// ninguna pregunta tiene respuestas todavía.
func (a *AnalysisService) aggregateQuestion(ctx context.Context, surveyID string, q analysisQuestion, provider *ai.AnalysisProvider) error {
	rows, err := a.db.Query(ctx, `
		SELECT r.id::text, a.raw_value, a.sentiment_label, a.topic_tags, r.language
		FROM answers a
		JOIN responses r ON r.id = a.response_id
		WHERE r.survey_id = $1 AND a.question_id = $2 AND r.status = 'submitted'`,
		surveyID, q.id)
	if err != nil {
		return err
	}
	var answers []ai.AggregationAnswer
	var labels []string
	var tagLists [][]string
	var language string
	for rows.Next() {
		var responseID, value string
		var label *string
		var tags []string
		if err := rows.Scan(&responseID, &value, &label, &tags, &language); err != nil {
			rows.Close()
			return err
		}
		answers = append(answers, ai.AggregationAnswer{ResponseID: responseID, Value: value})
		if label != nil {
			labels = append(labels, *label)
		}
		if len(tags) > 0 {
			tagLists = append(tagLists, tags)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	dist := computeSentimentDistribution(labels)

	// Temas y outliers solo tienen sentido en preguntas abiertas: en una escala
	// o una opción múltiple no hay contenido que se desvíe de la mayoría (una
	// calificación de 1 no es una respuesta atípica) ni temas que agrupar. Para
	// esas, el dashboard muestra la distribución de valores, que se calcula al
	// leer (ver AnalysisService.loadAnswerDistribution) sin gastar una llamada.
	openEnded := isOpenEnded(q.kind)

	var summary string
	var clusters []models.TopicCluster
	var outlierIDs []string
	if len(answers) > 0 {
		if *provider == nil {
			p, err := a.newProvider(ctx)
			if err != nil {
				return err
			}
			*provider = p
		}
		result, err := (*provider).AggregateQuestion(ctx, ai.QuestionAggregationRequest{
			QuestionText:   q.text,
			QuestionType:   q.kind,
			Answers:        answers,
			Language:       language,
			DetectOutliers: openEnded,
			ClusterTopics:  openEnded,
		})
		if err != nil {
			return fmt.Errorf("aggregate: %w", err)
		}
		summary = result.SummaryText
		// Solo se confía en temas y outliers cuando se pidieron: un proveedor
		// puede devolverlos igual (el modelo es libre de agregar campos), y en
		// una pregunta estructurada eso reintroduce justo lo que se quiso
		// evitar — una calificación baja marcada como respuesta atípica.
		if openEnded {
			outlierIDs = result.OutlierResponseIDs
			clusters = toTopicClusters(result.TopicClusters)
		}
	}

	// Si el proveedor ignoró el campo topic_clusters, se cae al conteo local de
	// answers.topic_tags: agrupa peor (por igualdad exacta de string), pero es
	// mejor que dejar la pregunta sin temas. Solo en abiertas — en las
	// estructuradas los tags no son temas y la lista debe quedar vacía.
	if openEnded && len(clusters) == 0 {
		clusters = computeTopicClusters(tagLists)
	}

	distJSON, err := json.Marshal(dist)
	if err != nil {
		return err
	}
	clustersJSON, err := json.Marshal(clusters)
	if err != nil {
		return err
	}
	if _, err := a.db.Exec(ctx, `
		INSERT INTO analysis_results (survey_id, question_id, summary_text, sentiment_distribution, topic_clusters, analysed_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (survey_id, question_id) DO UPDATE SET
			summary_text = EXCLUDED.summary_text,
			sentiment_distribution = EXCLUDED.sentiment_distribution,
			topic_clusters = EXCLUDED.topic_clusters,
			analysed_at = EXCLUDED.analysed_at`,
		surveyID, q.id, summary, distJSON, clustersJSON,
	); err != nil {
		return err
	}

	if _, err := a.db.Exec(ctx, `UPDATE answers SET is_outlier = false WHERE question_id = $1`, q.id); err != nil {
		return err
	}
	for _, responseID := range outlierIDs {
		if _, err := a.db.Exec(ctx,
			`UPDATE answers SET is_outlier = true WHERE question_id = $1 AND response_id = $2`,
			q.id, responseID,
		); err != nil {
			return err
		}
	}
	return nil
}

// computeSentimentDistribution calcula el porcentaje (0.0-1.0) de labels en
// cada categoría. Cualquier valor que no sea "positive"/"negative" cuenta
// como neutral (incluye respuestas sin sentimiento asignado por error del
// proveedor). Devuelve ceros si no hay labels.
func computeSentimentDistribution(labels []string) models.SentimentDistribution {
	if len(labels) == 0 {
		return models.SentimentDistribution{}
	}
	var positive, negative int
	for _, label := range labels {
		switch label {
		case "positive":
			positive++
		case "negative":
			negative++
		}
	}
	total := float64(len(labels))
	neutral := len(labels) - positive - negative
	return models.SentimentDistribution{
		Positive: float64(positive) / total,
		Neutral:  float64(neutral) / total,
		Negative: float64(negative) / total,
	}
}

// isOpenEnded distingue las preguntas cuyo contenido es texto libre — las
// únicas donde agrupar temas y detectar respuestas atípicas significa algo.
// Las demás (escala, opción, ranking, matriz) se resumen igual, pero su
// "distribución" es el conteo de valores, no un tag cloud.
func isOpenEnded(questionType string) bool {
	return questionType == "open_ended"
}

// toTopicClusters mapea los clusters del proveedor al modelo del dominio,
// descartando los vacíos y acotando la lista a maxTopicClusters. El paquete ai
// no importa models (para que los proveedores no dependan del dominio), así que
// la conversión vive de este lado.
func toTopicClusters(in []ai.TopicCluster) []models.TopicCluster {
	out := make([]models.TopicCluster, 0, len(in))
	for _, c := range in {
		tag := strings.ToLower(strings.TrimSpace(c.Tag))
		if tag == "" {
			continue
		}
		count := c.Count
		if count < 1 {
			count = 1
		}
		out = append(out, models.TopicCluster{Tag: tag, Count: count})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > maxTopicClusters {
		out = out[:maxTopicClusters]
	}
	return out
}

// maxTopicClusters acota cuántos temas se guardan por pregunta — el
// dashboard (#16) solo necesita los más frecuentes.
const maxTopicClusters = 10

// computeTopicClusters es el fallback local cuando el proveedor no devuelve
// topic_clusters: agrupa tags (normalizados a minúsculas) por frecuencia y
// devuelve los maxTopicClusters más mencionados, de mayor a menor. Orden
// estable: a igualdad de frecuencia, se respeta el orden de primera aparición.
//
// Agrupa por igualdad exacta de string, así que junta mal: los tags los inventa
// el modelo por separado en cada respuesta y casi nunca coinciden al carácter.
// Por eso el camino normal es que los agrupe el proveedor, que ve todas las
// respuestas de la pregunta a la vez (ver ai.QuestionAggregationResult).
func computeTopicClusters(tagLists [][]string) []models.TopicCluster {
	counts := make(map[string]int)
	var order []string
	for _, tags := range tagLists {
		for _, tag := range tags {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag == "" {
				continue
			}
			if _, seen := counts[tag]; !seen {
				order = append(order, tag)
			}
			counts[tag]++
		}
	}
	clusters := make([]models.TopicCluster, 0, len(order))
	for _, tag := range order {
		clusters = append(clusters, models.TopicCluster{Tag: tag, Count: counts[tag]})
	}
	sort.SliceStable(clusters, func(i, j int) bool { return clusters[i].Count > clusters[j].Count })
	if len(clusters) > maxTopicClusters {
		clusters = clusters[:maxTopicClusters]
	}
	return clusters
}

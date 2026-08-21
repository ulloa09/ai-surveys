package services

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

// Lado de LECTURA del Analysis Engine: las consultas que alimentan el
// dashboard de resultados (#16). No encolan trabajo ni llaman al proveedor de
// IA — solo leen lo que AnalyseResponse/AnalyseSurvey ya dejaron escrito en
// answers y analysis_results.
//
// El scoping por equipo y los 403/404 NO viven aquí: los resuelve
// SurveyService.authorizeSurveyAccess antes de que el handler llegue a estas
// funciones, que asumen un surveyID ya autorizado. Como una encuesta
// pertenece a un solo equipo (a diferencia del modelo multi-asignación), no
// hace falta re-filtrar por equipo acá: quien pasó authorizeSurveyAccess ve
// todas las cifras de la encuesta, sin desglose parcial.

// StatsForSurveys agrega las respuestas de varias encuestas de una sola vez
// (dos consultas en total, no dos por encuesta) para que el listado del
// dashboard no caiga en un N+1. Devuelve una entrada por cada id recibido, en
// el mismo orden: una encuesta sin respuestas devuelve stats en cero, no se omite.
func (a *AnalysisService) StatsForSurveys(ctx context.Context, surveyIDs []string) ([]models.SurveyStats, error) {
	if len(surveyIDs) == 0 {
		return []models.SurveyStats{}, nil
	}
	for _, id := range surveyIDs {
		if !isUUID(id) {
			return nil, ErrSurveyNotFound
		}
	}

	byID := make(map[string]*models.SurveyStats, len(surveyIDs))
	stats := make([]models.SurveyStats, len(surveyIDs))
	for i, id := range surveyIDs {
		stats[i] = models.SurveyStats{SurveyID: id, LanguageDistribution: []models.LanguageCount{}}
		byID[id] = &stats[i]
	}

	// EXTRACT(EPOCH ...) se castea a float8 dentro del AVG para no depender
	// del tipo que devuelve según la versión de PostgreSQL.
	rows, err := a.db.Query(ctx, `
		SELECT s.id::text, s.response_cap,
		       COUNT(r.id),
		       COUNT(r.id) FILTER (WHERE r.status = 'submitted'),
		       AVG(EXTRACT(EPOCH FROM (r.submitted_at - r.started_at))::float8)
		           FILTER (WHERE r.status = 'submitted' AND r.submitted_at IS NOT NULL)
		FROM surveys s
		LEFT JOIN responses r ON r.survey_id = s.id
		WHERE s.id = ANY($1::uuid[])
		GROUP BY s.id`, surveyIDs)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var surveyID string
		var expected *int
		var responseCount, completedCount int
		var avgDuration *float64
		if err := rows.Scan(&surveyID, &expected, &responseCount, &completedCount, &avgDuration); err != nil {
			rows.Close()
			return nil, err
		}
		entry, ok := byID[surveyID]
		if !ok {
			continue
		}
		entry.ResponseCount = responseCount
		entry.CompletedCount = completedCount
		if responseCount > 0 {
			entry.CompletionRate = float64(completedCount) / float64(responseCount)
		}
		entry.AvgDurationSeconds = avgDuration
		entry.ExpectedResponses = expected
		if expected != nil {
			entry.MissingResponses = missingOf(*expected, completedCount)
			entry.CoverageRate = coverageOf(*expected, completedCount)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	langRows, err := a.db.Query(ctx, `
		SELECT r.survey_id::text, r.language, COUNT(*)
		FROM responses r
		WHERE r.survey_id = ANY($1::uuid[])
		GROUP BY r.survey_id, r.language
		ORDER BY COUNT(*) DESC, r.language ASC`, surveyIDs)
	if err != nil {
		return nil, err
	}
	defer langRows.Close()
	for langRows.Next() {
		var surveyID string
		var count models.LanguageCount
		if err := langRows.Scan(&surveyID, &count.Language, &count.Count); err != nil {
			return nil, err
		}
		if entry, ok := byID[surveyID]; ok {
			entry.LanguageDistribution = append(entry.LanguageDistribution, count)
		}
	}
	return stats, langRows.Err()
}

// missingOf calcula cuántas respuestas faltan para llegar al tamaño esperado,
// sin bajar de cero (una encuesta puede recibir más respuestas de las
// esperadas — reintentos en encuestas anónimas, gente extra, etc.).
func missingOf(expected, completed int) *int {
	missing := expected - completed
	if missing < 0 {
		missing = 0
	}
	return &missing
}

// coverageOf calcula completed/expected. expected ya se validó > 0 al
// guardarse (constraint de la columna), así que no hace falta guardia extra.
func coverageOf(expected, completed int) *float64 {
	rate := float64(completed) / float64(expected)
	return &rate
}

// SurveyStats agrega las respuestas de una sola encuesta. Lo usa el detalle
// de encuesta, que muestra estas cifras aunque la encuesta todavía no se haya
// analizado.
func (a *AnalysisService) SurveyStats(ctx context.Context, surveyID string) (*models.SurveyStats, error) {
	stats, err := a.StatsForSurveys(ctx, []string{surveyID})
	if err != nil {
		return nil, err
	}
	return &stats[0], nil
}

// SurveyAnalysis arma la vista completa de resultados de una encuesta: stats
// a nivel encuesta más una sección por pregunta, en orden de presentación.
//
// Las preguntas se traen con LEFT JOIN contra analysis_results para que la
// vista sea legible mientras el job todavía corre ('analysing'): las
// preguntas que aún no tienen resultado aparecen con los agregados en cero.
// El caller ya verificó que el status permite ver el análisis.
func (a *AnalysisService) SurveyAnalysis(ctx context.Context, surveyID string) (*models.SurveyAnalysis, error) {
	if !isUUID(surveyID) {
		return nil, ErrSurveyNotFound
	}

	var status string
	err := a.db.QueryRow(ctx, `SELECT status FROM surveys WHERE id = $1`, surveyID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSurveyNotFound
	}
	if err != nil {
		return nil, err
	}

	stats, err := a.SurveyStats(ctx, surveyID)
	if err != nil {
		return nil, err
	}

	questions, err := a.loadQuestionAnalysis(ctx, surveyID)
	if err != nil {
		return nil, err
	}

	answerCounts, err := a.countAnswersByQuestion(ctx, surveyID)
	if err != nil {
		return nil, err
	}
	outliers, err := a.loadOutliersByQuestion(ctx, surveyID)
	if err != nil {
		return nil, err
	}
	distributions, err := a.loadAnswerDistribution(ctx, surveyID)
	if err != nil {
		return nil, err
	}

	var analysedAt *time.Time
	for i := range questions {
		q := &questions[i]
		q.AnswerCount = answerCounts[q.QuestionID]
		if found, ok := outliers[q.QuestionID]; ok {
			q.Outliers = found
		}
		if found, ok := distributions[q.QuestionID]; ok {
			q.AnswerDistribution = found
		}
		if q.AnalysedAt != nil && (analysedAt == nil || q.AnalysedAt.After(*analysedAt)) {
			analysedAt = q.AnalysedAt
		}
	}

	return &models.SurveyAnalysis{
		SurveyID:   surveyID,
		Status:     status,
		AnalysedAt: analysedAt,
		Stats:      *stats,
		Questions:  questions,
	}, nil
}

// SurveyResponses arma el visor de respuestas individuales (a diferencia de
// SurveyAnalysis, que son resúmenes agregados por IA): una entrada por
// participante que envió, con su respuesta cruda a cada pregunta. Existe
// desde que hay respuestas enviadas, sin depender de que el Analysis Engine
// haya corrido — por eso no valida survey.Status como GetSurveyAnalysis.
//
// Solo incluye respuestas enviadas (status = 'submitted'): una en curso no
// tiene nada estable que mostrar.
func (a *AnalysisService) SurveyResponses(ctx context.Context, surveyID string) (*models.SurveyResponses, error) {
	if !isUUID(surveyID) {
		return nil, ErrSurveyNotFound
	}

	var exists bool
	if err := a.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM surveys WHERE id = $1)`, surveyID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrSurveyNotFound
	}

	qRows, err := a.db.Query(ctx,
		`SELECT id::text, text, type, order_index, options FROM questions WHERE survey_id = $1 ORDER BY order_index ASC`, surveyID)
	if err != nil {
		return nil, err
	}
	questions := []models.ResponseQuestion{}
	// questionsByID guarda el tipo/options de cada pregunta (no expuestos en
	// ResponseQuestion) para poder llamar DisplayAnswer al armar cada
	// RespondentAnswer más abajo, sin una segunda consulta.
	questionsByID := make(map[string]models.Question, 8)
	for qRows.Next() {
		var q models.ResponseQuestion
		var full models.Question
		if err := qRows.Scan(&q.QuestionID, &q.Text, &q.Type, &q.OrderIndex, &full.Options); err != nil {
			qRows.Close()
			return nil, err
		}
		full.Type = q.Type
		questionsByID[q.QuestionID] = full
		questions = append(questions, q)
	}
	qRows.Close()
	if err := qRows.Err(); err != nil {
		return nil, err
	}

	rows, err := a.db.Query(ctx, `
		SELECT r.id::text, r.submitted_at, r.language, r.registered_email, u.email
		FROM responses r
		LEFT JOIN users u ON u.id = r.user_id
		WHERE r.survey_id = $1 AND r.status = 'submitted'
		ORDER BY r.submitted_at ASC NULLS LAST, r.id ASC`, surveyID)
	if err != nil {
		return nil, err
	}

	responses := []models.RespondentResponse{}
	order := make(map[string]int, len(responses))
	anonSeq := 0
	for rows.Next() {
		var resp models.RespondentResponse
		var registeredEmail, userEmail *string
		if err := rows.Scan(&resp.ResponseID, &resp.SubmittedAt, &resp.Language, &registeredEmail, &userEmail); err != nil {
			rows.Close()
			return nil, err
		}
		resp.Answers = []models.RespondentAnswer{}
		switch {
		case userEmail != nil && *userEmail != "":
			resp.Label = userEmail
		case registeredEmail != nil && *registeredEmail != "":
			resp.Label = registeredEmail
		default:
			anonSeq++
			n := anonSeq
			resp.AnonNumber = &n
		}
		order[resp.ResponseID] = len(responses)
		responses = append(responses, resp)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ansRows, err := a.db.Query(ctx, `
		SELECT a.response_id::text, a.question_id::text, a.raw_value, a.sentiment_label, a.is_outlier
		FROM answers a
		JOIN responses r ON r.id = a.response_id
		WHERE r.survey_id = $1 AND r.status = 'submitted'`, surveyID)
	if err != nil {
		return nil, err
	}
	defer ansRows.Close()
	for ansRows.Next() {
		var responseID string
		var ans models.RespondentAnswer
		if err := ansRows.Scan(&responseID, &ans.QuestionID, &ans.RawValue, &ans.SentimentLabel, &ans.IsOutlier); err != nil {
			return nil, err
		}
		if i, ok := order[responseID]; ok {
			if q, found := questionsByID[ans.QuestionID]; found {
				ans.DisplayValue = DisplayAnswer(q, ans.RawValue, responses[i].Language)
			} else {
				ans.DisplayValue = ans.RawValue
			}
			responses[i].Answers = append(responses[i].Answers, ans)
		}
	}
	if err := ansRows.Err(); err != nil {
		return nil, err
	}

	return &models.SurveyResponses{SurveyID: surveyID, Questions: questions, Responses: responses}, nil
}

// loadQuestionAnalysis trae cada pregunta con su fila de analysis_results (si
// existe). summary_text, los dos JSONB y analysed_at se escanean como
// punteros/[]byte porque el LEFT JOIN los deja en NULL para las preguntas que
// el job todavía no agregó.
func (a *AnalysisService) loadQuestionAnalysis(ctx context.Context, surveyID string) ([]models.QuestionAnalysis, error) {
	rows, err := a.db.Query(ctx, `
		SELECT q.id::text, q.text, q.type, q.order_index,
		       ar.summary_text, ar.sentiment_distribution, ar.topic_clusters, ar.analysed_at
		FROM questions q
		LEFT JOIN analysis_results ar ON ar.question_id = q.id AND ar.survey_id = q.survey_id
		WHERE q.survey_id = $1
		ORDER BY q.order_index ASC`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	questions := []models.QuestionAnalysis{}
	for rows.Next() {
		q := models.QuestionAnalysis{
			TopicClusters:      []models.TopicCluster{},
			AnswerDistribution: []models.OptionCount{},
			Outliers:           []models.OutlierAnswer{},
		}
		var summary *string
		var distJSON, clustersJSON []byte
		if err := rows.Scan(&q.QuestionID, &q.Text, &q.Type, &q.OrderIndex,
			&summary, &distJSON, &clustersJSON, &q.AnalysedAt); err != nil {
			return nil, err
		}
		if summary != nil {
			q.SummaryText = *summary
		}
		if len(distJSON) > 0 {
			if err := json.Unmarshal(distJSON, &q.SentimentDistribution); err != nil {
				return nil, err
			}
		}
		if len(clustersJSON) > 0 {
			if err := json.Unmarshal(clustersJSON, &q.TopicClusters); err != nil {
				return nil, err
			}
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}

// countAnswersByQuestion cuenta cuántas respuestas enviadas contestaron cada
// pregunta — el denominador que da contexto a los porcentajes de sentimiento.
func (a *AnalysisService) countAnswersByQuestion(ctx context.Context, surveyID string) (map[string]int, error) {
	rows, err := a.db.Query(ctx, `
		SELECT a.question_id::text, COUNT(*)
		FROM answers a
		JOIN responses r ON r.id = a.response_id
		WHERE r.survey_id = $1 AND r.status = 'submitted'
		GROUP BY a.question_id`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var questionID string
		var count int
		if err := rows.Scan(&questionID, &count); err != nil {
			return nil, err
		}
		counts[questionID] = count
	}
	return counts, rows.Err()
}

// loadOutliersByQuestion trae solo las respuestas marcadas como atípicas por
// el proveedor (answers.is_outlier), agrupadas por pregunta. Son pocas por
// definición, así que se cargan con su texto original para mostrarlas.
//
// El Analysis Engine solo marca outliers en preguntas abiertas, donde raw_value
// ya es el texto que escribió el participante. Aun así se pasa por DisplayAnswer
// para no mostrar valores internos ("muy_lento") si quedan filas marcadas de una
// corrida anterior, cuando también se pedían outliers en preguntas estructuradas.
func (a *AnalysisService) loadOutliersByQuestion(ctx context.Context, surveyID string) (map[string][]models.OutlierAnswer, error) {
	rows, err := a.db.Query(ctx, `
		SELECT a.question_id::text, r.id::text, a.raw_value, a.sentiment_label, q.type, q.options, r.language
		FROM answers a
		JOIN questions q ON q.id = a.question_id
		JOIN responses r ON r.id = a.response_id
		WHERE r.survey_id = $1 AND r.status = 'submitted' AND a.is_outlier
		ORDER BY r.submitted_at ASC NULLS LAST, r.id ASC`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	outliers := make(map[string][]models.OutlierAnswer)
	for rows.Next() {
		var questionID, language string
		var q models.Question
		var outlier models.OutlierAnswer
		if err := rows.Scan(&questionID, &outlier.ResponseID, &outlier.RawValue, &outlier.SentimentLabel,
			&q.Type, &q.Options, &language); err != nil {
			return nil, err
		}
		outlier.RawValue = DisplayAnswer(q, outlier.RawValue, language)
		outliers[questionID] = append(outliers[questionID], outlier)
	}
	return outliers, rows.Err()
}

// loadAnswerDistribution cuenta cuántas respuestas eligieron cada valor en las
// preguntas estructuradas — el agregado que reemplaza al tag cloud cuando no hay
// texto libre que agrupar. Se calcula al leer, a partir de answers: no se guarda
// en analysis_results porque no cuesta una llamada de IA ni depende de que el
// engine haya corrido, así que el detalle lo muestra igual mientras la encuesta
// sigue abierta.
//
// Las escalas se ordenan por valor (1, 2, 3...) y el resto por frecuencia: en una
// escala importa la forma de la distribución, en una opción múltiple cuál ganó.
func (a *AnalysisService) loadAnswerDistribution(ctx context.Context, surveyID string) (map[string][]models.OptionCount, error) {
	rows, err := a.db.Query(ctx, `
		SELECT a.question_id::text, q.type, q.options, a.raw_value, COUNT(*), MIN(r.language)
		FROM answers a
		JOIN questions q ON q.id = a.question_id
		JOIN responses r ON r.id = a.response_id
		WHERE r.survey_id = $1 AND r.status = 'submitted' AND q.type <> 'open_ended'
		GROUP BY a.question_id, q.type, q.options, a.raw_value`, surveyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	kinds := make(map[string]string)
	dist := make(map[string][]models.OptionCount)
	for rows.Next() {
		var questionID, language string
		var q models.Question
		var opt models.OptionCount
		if err := rows.Scan(&questionID, &q.Type, &q.Options, &opt.Value, &opt.Count, &language); err != nil {
			return nil, err
		}
		opt.Label = DisplayAnswer(q, opt.Value, language)
		kinds[questionID] = q.Type
		dist[questionID] = append(dist[questionID], opt)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for questionID, opts := range dist {
		sortDistribution(kinds[questionID], opts)
	}
	return dist, nil
}

// sortDistribution ordena los valores de una pregunta estructurada con el mismo
// criterio que el dashboard: las escalas por valor (1, 2, 3...) porque ahí lo
// que importa es la FORMA de la distribución, y el resto por frecuencia porque
// lo que importa es cuál ganó.
func sortDistribution(questionType string, opts []models.OptionCount) {
	if questionType == "linear_scale" {
		sort.SliceStable(opts, func(i, j int) bool {
			a, errA := strconv.Atoi(opts[i].Value)
			b, errB := strconv.Atoi(opts[j].Value)
			if errA != nil || errB != nil {
				return opts[i].Value < opts[j].Value
			}
			return a < b
		})
		return
	}
	sort.SliceStable(opts, func(i, j int) bool {
		if opts[i].Count != opts[j].Count {
			return opts[i].Count > opts[j].Count
		}
		// Desempate estable por etiqueta: sin esto, dos opciones con el mismo
		// conteo salen en el orden en que las devolvió el mapa de agregación —
		// distinto en cada corrida, y dos exports de la misma data no
		// coincidirían.
		return opts[i].Label < opts[j].Label
	})
}

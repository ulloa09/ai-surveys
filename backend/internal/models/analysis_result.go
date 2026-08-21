package models

import "time"

// AnalysisResult corresponde a una fila de analysis_results: el resultado de
// agregación del Analysis Engine (#15) para una pregunta de una encuesta.
// QuestionID es puntero porque la columna admite NULL (reservado para un
// futuro resumen a nivel de encuesta completa, fuera del alcance de #15).
type AnalysisResult struct {
	ID                    string                `json:"id"`
	SurveyID              string                `json:"survey_id"`
	QuestionID            *string               `json:"question_id"`
	SummaryText           string                `json:"summary_text"`
	SentimentDistribution SentimentDistribution `json:"sentiment_distribution"`
	TopicClusters         []TopicCluster        `json:"topic_clusters"`
	AnalysedAt            time.Time             `json:"analysed_at"`
}

// SentimentDistribution es el porcentaje (0.0-1.0) de respuestas a una
// pregunta que cayeron en cada categoría de sentimiento. Se calcula
// localmente a partir de answers.sentiment_label, sin llamada a IA.
type SentimentDistribution struct {
	Positive float64 `json:"positive"`
	Neutral  float64 `json:"neutral"`
	Negative float64 `json:"negative"`
}

// TopicCluster es un tema recurrente entre las respuestas a una pregunta, con
// la cantidad de respuestas que lo mencionan. Lo agrupa el proveedor dentro de
// la misma llamada que produce el resumen (ver ai.QuestionAggregationResult):
// agruparlo localmente por igualdad exacta de tags dejaba un tag cloud
// degenerado, con casi todos los temas en count 1.
//
// Solo se llena en preguntas abiertas. En las estructuradas el agregado
// equivalente es AnswerDistribution.
type TopicCluster struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// OptionCount es la cantidad de respuestas que eligieron un valor concreto en
// una pregunta estructurada (linear_scale, single_choice, true_false...).
// Value es el valor guardado en answers.raw_value; Label es cómo se le
// presentó al participante (ver DisplayAnswer) — el dashboard muestra Label,
// no el valor interno.
type OptionCount struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// SurveyAnalysis es la vista de lectura que consume el dashboard (#16): las
// estadísticas de toda la encuesta más una sección por pregunta, en el orden
// en que se le presentan al participante.
//
// AnalysedAt es el análisis más reciente entre todas las preguntas, o nil si
// el job todavía no escribió ningún resultado (status = 'analysing').
type SurveyAnalysis struct {
	SurveyID   string             `json:"survey_id"`
	Status     string             `json:"status"`
	AnalysedAt *time.Time         `json:"analysed_at"`
	Stats      SurveyStats        `json:"stats"`
	Questions  []QuestionAnalysis `json:"questions"`
}

// QuestionAnalysis es la sección de una pregunta en la vista de análisis:
// el resultado agregado del Analysis Engine (resumen, sentimiento, temas)
// más los datos que el dashboard necesita para contextualizarlo.
//
// Una pregunta sin fila en analysis_results (el job aún no llegó a ella)
// aparece igual, con AnalysedAt nil y los agregados en cero — la vista es
// legible mientras la encuesta está en 'analysing'.
// TopicClusters/Outliers solo se llenan en preguntas abiertas, y
// AnswerDistribution solo en las estructuradas — son los dos agregados
// equivalentes, y el dashboard muestra el que corresponda al tipo.
type QuestionAnalysis struct {
	QuestionID            string                `json:"question_id"`
	Text                  string                `json:"text"`
	Type                  string                `json:"type"`
	OrderIndex            int                   `json:"order_index"`
	AnswerCount           int                   `json:"answer_count"`
	SummaryText           string                `json:"summary_text"`
	SentimentDistribution SentimentDistribution `json:"sentiment_distribution"`
	TopicClusters         []TopicCluster        `json:"topic_clusters"`
	AnswerDistribution    []OptionCount         `json:"answer_distribution"`
	Outliers              []OutlierAnswer       `json:"outliers"`
	AnalysedAt            *time.Time            `json:"analysed_at"`
}

// OutlierAnswer es una respuesta que el proveedor marcó como atípica para su
// pregunta (answers.is_outlier). El dashboard las lista aparte de los
// agregados, con su texto original.
type OutlierAnswer struct {
	ResponseID     string  `json:"response_id"`
	RawValue       string  `json:"raw_value"`
	SentimentLabel *string `json:"sentiment_label"`
}

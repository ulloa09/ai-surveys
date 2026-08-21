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

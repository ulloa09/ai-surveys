package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AnalysisProvider es la interfaz que usa el Analysis Engine (#15) para
// sentimiento/tags por respuesta y resumen/outliers por pregunta. Se separa
// de AIProvider (StreamTurn/Extract, usado por el Engine conversacional)
// para no forzar a cada implementación existente (incluyendo los fakes de
// test) a cargar con métodos que no le corresponden. ClaudeProvider y
// OpenAIProvider implementan ambas interfaces.
type AnalysisProvider interface {
	// AnalyseAnswers analiza sentimiento y tags de TODAS las respuestas de un
	// participante en una sola llamada (una llamada por respuesta, no por
	// pregunta).
	AnalyseAnswers(ctx context.Context, req AnswerAnalysisRequest) (AnswerAnalysisResult, error)

	// AggregateQuestion resume y detecta outliers entre TODAS las respuestas
	// a una misma pregunta en una sola llamada (una llamada por pregunta, no
	// por respuesta × pregunta — requisito explícito de #15).
	AggregateQuestion(ctx context.Context, req QuestionAggregationRequest) (QuestionAggregationResult, error)
}

// AnswerToAnalyse es la respuesta de una pregunta puntual dentro de un mismo
// AnalyseAnswers request.
type AnswerToAnalyse struct {
	QuestionID   string
	QuestionText string
	QuestionType string
	Value        string
}

// AnswerAnalysisRequest agrupa todas las respuestas de un participante que
// necesitan sentimiento/tags.
type AnswerAnalysisRequest struct {
	Answers  []AnswerToAnalyse
	Language string
}

// AnalysedAnswer es el resultado de sentimiento/tags para una pregunta.
// Los tags json van acá (y no solo en el struct anónimo de cada provider)
// porque OpenAIProvider.AnalyseAnswers deserializa la respuesta del modelo
// directamente en []AnalysedAnswer — sin ellos, question_id/sentiment_label
// nunca calzan con QuestionID/SentimentLabel y todo llega en blanco.
type AnalysedAnswer struct {
	QuestionID     string   `json:"question_id"`
	SentimentLabel string   `json:"sentiment_label"` // "positive" | "neutral" | "negative"
	SentimentScore float64  `json:"sentiment_score"`
	Tags           []string `json:"tags"` // solo relevante para preguntas abiertas
}

// AnswerAnalysisResult contiene el análisis de cada respuesta del request.
type AnswerAnalysisResult struct {
	Answers []AnalysedAnswer
}

// AggregationAnswer es la respuesta de un participante a la pregunta que se
// está agregando.
type AggregationAnswer struct {
	ResponseID string
	Value      string
}

// QuestionAggregationRequest agrupa todas las respuestas a una misma
// pregunta, a través de todos los participantes de la encuesta.
//
// DetectOutliers y ClusterTopics los decide el caller según el tipo de
// pregunta, y solo tienen sentido en preguntas abiertas: en una escala o una
// opción múltiple no hay "contenido que se desvíe de la mayoría" (una
// calificación de 1 no es una respuesta atípica, es una calificación baja) ni
// temas que agrupar — para esas, el dashboard muestra la distribución de
// valores, que se calcula localmente sin IA.
type QuestionAggregationRequest struct {
	QuestionText   string
	QuestionType   string
	Answers        []AggregationAnswer
	Language       string
	DetectOutliers bool
	ClusterTopics  bool
}

// TopicCluster es un tema recurrente entre las respuestas a una pregunta, con
// la cantidad de respuestas que lo mencionan. Lo agrupa el proveedor dentro de
// la misma llamada de agregación (ver QuestionAggregationResult).
type TopicCluster struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// QuestionAggregationResult es el resumen, los topic clusters y los outliers
// detectados para una pregunta.
//
// Los clusters los agrupa el proveedor y no se cuentan localmente: los tags de
// answers.topic_tags los inventa el modelo por separado en cada respuesta, así
// que agruparlos por igualdad exacta de string casi nunca junta dos ("temas"
// con count 1 en vez de un tag cloud). El proveedor, en cambio, ve todas las
// respuestas de la pregunta en esta misma llamada y puede fusionar variantes
// equivalentes ("laboratorios" y "laboratorios prácticos") en un solo cluster.
// No cuesta una llamada extra — viaja en el mismo JSON que el resumen, así que
// se mantiene el presupuesto de una llamada por pregunta que exige #15.
//
// La distribución de sentimiento sí se sigue calculando localmente a partir de
// answers.sentiment_label, que ya es un enum cerrado y agrupa sin ambigüedad.
type QuestionAggregationResult struct {
	SummaryText        string
	TopicClusters      []TopicCluster
	OutlierResponseIDs []string
}

// buildAggregationPrompt arma el prompt de agregación por pregunta. Lo comparten
// ClaudeProvider y OpenAIProvider: el prompt es idéntico para ambos, y tenerlo
// duplicado ya había dejado a los dos proveedores pidiendo cosas distintas.
//
// Las secciones de temas y outliers solo se piden cuando el caller las habilita
// (preguntas abiertas). Pedirlas en una escala o una opción múltiple hace que el
// modelo invente outliers a partir de calificaciones bajas.
func buildAggregationPrompt(req QuestionAggregationRequest) string {
	var sb strings.Builder
	sb.WriteString("You are analyzing all responses collected for a single survey question.\n")
	sb.WriteString(fmt.Sprintf("Question (type: %s): %s\n\n", req.QuestionType, req.QuestionText))
	sb.WriteString("Write a concise summary (~150 words) of the responses.\n")

	// Las claves del JSON se van acumulando según lo que se pida, para que el
	// ejemplo que ve el modelo tenga exactamente la forma esperada.
	fields := []string{"\"summary\": \"...\""}

	if req.ClusterTopics {
		sb.WriteString("Also group the responses into topic clusters: recurring themes across the responses. ")
		sb.WriteString("Merge equivalent phrasings into a single cluster (e.g. \"labs\" and \"hands-on labs\" are one topic). ")
		sb.WriteString("For each cluster give a short tag (1-3 words) and count how many responses mention it. ")
		sb.WriteString("Return at most 10 clusters, most mentioned first, and only themes mentioned by at least one response.\n")
		fields = append(fields, "\"topic_clusters\": [{\"tag\": \"...\", \"count\": 1}]")
	}
	if req.DetectOutliers {
		sb.WriteString("Also identify which response_ids are outliers — responses whose content deviates ")
		sb.WriteString("significantly from the majority (off-topic, or an unusually extreme position). ")
		sb.WriteString("If no response stands out, return an empty list.\n")
		fields = append(fields, "\"outlier_response_ids\": [\"...\"]")
	}
	if req.Language != "" {
		sb.WriteString(fmt.Sprintf("Write the summary and any tags in %s.\n", req.Language))
	}

	sb.WriteString("Respond ONLY with a JSON object: {" + strings.Join(fields, ", ") + "}\n\n")
	sb.WriteString("Responses:\n")
	for _, a := range req.Answers {
		sb.WriteString(fmt.Sprintf("- response_id:%s value:%s\n", a.ResponseID, a.Value))
	}
	return sb.String()
}

// parseAggregationResponse deserializa la respuesta del modelo al resultado de
// agregación. Los campos ausentes (temas/outliers no pedidos) quedan en su cero
// natural, así que no hace falta distinguir "no pedido" de "vacío".
func parseAggregationResponse(content string) (QuestionAggregationResult, error) {
	var result struct {
		Summary            string         `json:"summary"`
		TopicClusters      []TopicCluster `json:"topic_clusters"`
		OutlierResponseIDs []string       `json:"outlier_response_ids"`
	}
	if err := json.Unmarshal([]byte(sanitizeJSONResponse(content)), &result); err != nil {
		return QuestionAggregationResult{}, fmt.Errorf("parse question aggregation JSON: %w", err)
	}
	return QuestionAggregationResult{
		SummaryText:        result.Summary,
		TopicClusters:      result.TopicClusters,
		OutlierResponseIDs: result.OutlierResponseIDs,
	}, nil
}

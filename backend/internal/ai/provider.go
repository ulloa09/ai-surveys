package ai

import "context"

// AIProvider es la interfaz que todo proveedor de IA debe implementar.
// Usa el strategy pattern para que cambiar de Claude a OpenAI o cualquier
// otro proveedor no requiera tocar el resto del código.
type AIProvider interface {
	// StreamTurn envía el turno actual al proveedor y devuelve un canal de
	// chunks. El canal se cierra cuando la respuesta termina o hay un error.
	StreamTurn(ctx context.Context, req TurnRequest) (<-chan TurnChunk, error)

	// Extract corre una llamada no-streaming después de que la conversación
	// termina para extraer respuestas estructuradas por pregunta.
	Extract(ctx context.Context, req ExtractionRequest) (ExtractionResult, error)
}

// TurnRequest contiene todo el contexto necesario para que el proveedor
// genere el siguiente turno de la conversación.
type TurnRequest struct {
	// SystemPrompt ya viene ensamblado por BuildSystemPrompt — incluye el
	// prompt del admin, la instrucción de idioma y la lista de preguntas.
	SystemPrompt string
	// Transcript es el historial completo de turnos anteriores. Se pasa como
	// mensajes alternados user/assistant para que el proveedor tenga contexto.
	Transcript []Turn
	// UserMessage es el mensaje del participante en este turno.
	UserMessage string
}

// TurnChunk es un fragmento de la respuesta en streaming.
// Error es no-nil si el stream falló; en ese caso Token puede estar vacío.
type TurnChunk struct {
	Token string
	Error error
}

// Turn representa un intercambio en la conversación (un mensaje y su rol).
type Turn struct {
	Role    string // "user" | "assistant"
	Content string
}

// ExtractionRequest contiene el contexto para la llamada de extracción
// post-conversación.
type ExtractionRequest struct {
	Questions  []QuestionSummary
	Transcript []Turn
	Language   string
}

// QuestionSummary es la vista mínima de una pregunta que necesita el extractor.
type QuestionSummary struct {
	ID   string
	Text string
	Type string
}

// ExtractionResult contiene las respuestas estructuradas extraídas.
type ExtractionResult struct {
	Answers []ExtractedAnswer
}

// ExtractedAnswer es la respuesta extraída para una pregunta específica.
type ExtractedAnswer struct {
	QuestionID string
	Value      string
	Sentiment  string   // "positive" | "neutral" | "negative"
	Tags       []string // temas mencionados
}

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ClaudeProvider implementa AIProvider usando el SDK oficial de Anthropic.
// El modelo y la API key se leen de platform_settings en tiempo de ejecución
// para que el super-admin los pueda cambiar sin reiniciar el servidor.
type ClaudeProvider struct {
	client *anthropic.Client
	model  string
}

// NewClaudeProvider crea un provider con las credenciales indicadas.
// apiKey y model vienen de SettingsService.GetAIConfig().
func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeProvider{client: &client, model: model}
}

// StreamTurn envía el turno actual a Claude usando streaming del SDK oficial.
// Los mensajes del participante llegan como role=user — nunca se interpola
// contenido del estudiante en el system prompt (hardening de inyección).
// UserMessage puede venir vacío cuando el transcript ya termina con el mensaje
// del usuario (p. ej. followups); en ese caso no se agrega un turno extra.
func (p *ClaudeProvider) StreamTurn(ctx context.Context, req TurnRequest) (<-chan TurnChunk, error) {
	messages := buildClaudeMessages(req)

	stream := p.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		},
		Messages: messages,
	})

	ch := make(chan TurnChunk, 32)
	go func() {
		defer close(ch)

		for stream.Next() {
			event := stream.Current()
			// AsAny() es el método correcto en el SDK oficial de Anthropic
			switch eventVariant := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch deltaVariant := eventVariant.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					select {
					case ch <- TurnChunk{Token: deltaVariant.Text}:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- TurnChunk{Error: fmt.Errorf("claude: stream error: %w", err)}
		}
	}()

	return ch, nil
}

// buildClaudeMessages arma la lista de mensajes cumpliendo las restricciones
// de la API de Anthropic: el primer mensaje debe ser de rol user y los roles
// deben alternar. Turnos consecutivos del mismo rol (p. ej. la pregunta del
// sistema seguida del followup de la IA) se fusionan en un solo mensaje.
func buildClaudeMessages(req TurnRequest) []anthropic.MessageParam {
	type flat struct {
		role    string
		content string
	}
	flats := make([]flat, 0, len(req.Transcript)+1)
	for _, t := range req.Transcript {
		role := "assistant"
		if t.Role == "user" {
			role = "user"
		}
		flats = append(flats, flat{role: role, content: t.Content})
	}
	if strings.TrimSpace(req.UserMessage) != "" {
		flats = append(flats, flat{role: "user", content: req.UserMessage})
	}

	// La API exige que el primer mensaje sea del usuario.
	if len(flats) == 0 || flats[0].role != "user" {
		flats = append([]flat{{role: "user", content: "(inicio de la sesión)"}}, flats...)
	}

	// Fusionar mensajes consecutivos del mismo rol.
	merged := make([]flat, 0, len(flats))
	for _, f := range flats {
		if n := len(merged); n > 0 && merged[n-1].role == f.role {
			merged[n-1].content += "\n" + f.content
			continue
		}
		merged = append(merged, f)
	}

	messages := make([]anthropic.MessageParam, 0, len(merged))
	for _, f := range merged {
		if f.role == "user" {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(f.content)))
		} else {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(f.content)))
		}
	}
	return messages
}

// Extract corre una llamada no-streaming para extraer respuestas estructuradas
// de la conversación una vez que termina. Sirve para análisis posterior.
func (p *ClaudeProvider) Extract(ctx context.Context, req ExtractionRequest) (ExtractionResult, error) {
	var sb strings.Builder
	sb.WriteString("Extract structured answers from the following conversation.\n")
	sb.WriteString("For each question provide: value (the answer), sentiment (positive/neutral/negative), and tags (key topics).\n")
	sb.WriteString("Respond ONLY with a JSON object: {\"answers\": [{\"question_id\": \"...\", \"value\": \"...\", \"sentiment\": \"...\", \"tags\": [...]}]}\n\n")
	sb.WriteString("Questions:\n")
	for _, q := range req.Questions {
		sb.WriteString(fmt.Sprintf("- id:%s type:%s text:%s\n", q.ID, q.Type, q.Text))
	}
	sb.WriteString("\nConversation:\n")
	for _, t := range req.Transcript {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t.Role, t.Content))
	}

	msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 2048,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(sb.String())),
		},
	})
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("claude: extraction request: %w", err)
	}

	if len(msg.Content) == 0 {
		return ExtractionResult{}, fmt.Errorf("claude: empty extraction response")
	}

	text := msg.Content[0].Text

	var parsed struct {
		Answers []struct {
			QuestionID string   `json:"question_id"`
			Value      string   `json:"value"`
			Sentiment  string   `json:"sentiment"`
			Tags       []string `json:"tags"`
		} `json:"answers"`
	}
	if err := json.Unmarshal([]byte(sanitizeJSONResponse(text)), &parsed); err != nil {
		return ExtractionResult{}, fmt.Errorf("claude: parse extraction JSON: %w", err)
	}

	answers := make([]ExtractedAnswer, 0, len(parsed.Answers))
	for _, a := range parsed.Answers {
		answers = append(answers, ExtractedAnswer{
			QuestionID: a.QuestionID,
			Value:      a.Value,
			Sentiment:  a.Sentiment,
			Tags:       a.Tags,
		})
	}

	return ExtractionResult{Answers: answers}, nil
}

// Probe envía una petición mínima a Claude para verificar que el API key
// y el modelo son válidos. Lo usa el endpoint de test connection.
func (p *ClaudeProvider) Probe(ctx context.Context) error {
	_, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("ping")),
		},
	})
	if err != nil {
		log.Printf("claude probe error: %v", err)
		return fmt.Errorf("claude probe failed: %w", err)
	}
	return nil
}

package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// openAIClient tiene timeouts de conexión y de primera respuesta, pero sin
// timeout total — las respuestas en streaming pueden tardar más de un minuto
// y cortarlas a la mitad rompería la conversación. La cancelación del stream
// completo la controla el context del request.
var openAIClient = &http.Client{
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	},
}

// OpenAIProvider implementa AIProvider usando la API de OpenAI.
// El modelo se configura desde platform_settings (p. ej. gpt-4o, gpt-5-mini).
type OpenAIProvider struct {
	apiKey string
	model  string
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{apiKey: apiKey, model: model}
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamTurn envía el turno al endpoint de OpenAI con streaming SSE.
// Los mensajes del participante llegan como role=user — nunca se mezclan
// con el system prompt (hardening de inyección idéntico al de Claude).
// UserMessage puede venir vacío cuando el transcript ya termina con el
// mensaje del usuario (followups); en ese caso no se agrega turno extra.
func (p *OpenAIProvider) StreamTurn(ctx context.Context, req TurnRequest) (<-chan TurnChunk, error) {
	messages := make([]openAIMessage, 0, len(req.Transcript)+2)
	messages = append(messages, openAIMessage{Role: "system", Content: req.SystemPrompt})
	for _, t := range req.Transcript {
		messages = append(messages, openAIMessage{Role: t.Role, Content: t.Content})
	}
	if strings.TrimSpace(req.UserMessage) != "" {
		messages = append(messages, openAIMessage{Role: "user", Content: req.UserMessage})
	}

	body := map[string]any{
		"model":    p.model,
		"messages": messages,
		"stream":   true,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := openAIClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("openai: api error %d: %s", resp.StatusCode, string(b))
	}

	ch := make(chan TurnChunk, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}
			if len(event.Choices) > 0 && event.Choices[0].Delta.Content != "" {
				select {
				case ch <- TurnChunk{Token: event.Choices[0].Delta.Content}:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- TurnChunk{Error: fmt.Errorf("openai: stream error: %w", err)}
		}
	}()

	return ch, nil
}

// Extract extrae respuestas estructuradas de la conversación usando OpenAI.
func (p *OpenAIProvider) Extract(ctx context.Context, req ExtractionRequest) (ExtractionResult, error) {
	var sb strings.Builder
	sb.WriteString("Extract structured answers from the following conversation.\n")
	sb.WriteString("For each question, provide: value, sentiment (positive/neutral/negative), and tags.\n")
	sb.WriteString("Respond ONLY with a JSON object with an 'answers' array.\n\n")
	sb.WriteString("Questions:\n")
	for _, q := range req.Questions {
		sb.WriteString(fmt.Sprintf("- %s (id: %s, type: %s)\n", q.Text, q.ID, q.Type))
	}
	sb.WriteString("\nConversation:\n")
	for _, t := range req.Transcript {
		sb.WriteString(fmt.Sprintf("%s: %s\n", t.Role, t.Content))
	}

	body := map[string]any{
		"model": p.model,
		"messages": []openAIMessage{
			{Role: "user", Content: sb.String()},
		},
		// Fuerza una respuesta JSON válida en vez de confiar solo en el
		// prompt — sin esto el modelo ocasionalmente envuelve la respuesta en
		// una cerca de markdown o le agrega una frase, y json.Unmarshal falla
		// (ver sanitizeJSONResponse para la red de seguridad complementaria).
		"response_format": map[string]string{"type": "json_object"},
	}
	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return ExtractionResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := openAIClient.Do(httpReq)
	if err != nil {
		return ExtractionResult{}, fmt.Errorf("openai: extraction request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ExtractionResult{}, fmt.Errorf("openai: extraction error %d: %s", resp.StatusCode, string(b))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return ExtractionResult{}, fmt.Errorf("openai: decode extraction: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return ExtractionResult{}, fmt.Errorf("openai: empty extraction response")
	}

	var result struct {
		Answers []ExtractedAnswer `json:"answers"`
	}
	if err := json.Unmarshal([]byte(sanitizeJSONResponse(apiResp.Choices[0].Message.Content)), &result); err != nil {
		return ExtractionResult{}, fmt.Errorf("openai: parse extraction JSON: %w", err)
	}
	return ExtractionResult{Answers: result.Answers}, nil
}

// Probe verifica que el API key y el modelo son válidos.
func (p *OpenAIProvider) Probe(ctx context.Context) error {
	body := map[string]any{
		"model":                 p.model,
		"messages":              []openAIMessage{{Role: "user", Content: "ping"}},
		"max_completion_tokens": 16,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := openAIClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("model %q not found", p.model)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("provider error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// AnalyseAnswers extrae sentimiento y tags de todas las respuestas de un
// participante en una sola llamada (una llamada por respuesta, no por
// pregunta — ver AnalysisProvider).
func (p *OpenAIProvider) AnalyseAnswers(ctx context.Context, req AnswerAnalysisRequest) (AnswerAnalysisResult, error) {
	var sb strings.Builder
	sb.WriteString("Analyze the sentiment and topics of each answer below.\n")
	sb.WriteString("For each answer provide: sentiment_label (positive/neutral/negative), sentiment_score ")
	sb.WriteString("(0.0-1.0, how strongly that sentiment is expressed), and tags (2-5 short topic phrases; ")
	sb.WriteString("only meaningful for open-ended answers, empty array otherwise).\n")
	if req.Language != "" {
		sb.WriteString(fmt.Sprintf("Write tags in %s.\n", req.Language))
	}
	sb.WriteString("Respond ONLY with a JSON object with an 'answers' array: ")
	sb.WriteString("{\"answers\": [{\"question_id\", \"sentiment_label\", \"sentiment_score\", \"tags\"}]}\n\n")
	sb.WriteString("Answers:\n")
	for _, a := range req.Answers {
		sb.WriteString(fmt.Sprintf("- question_id:%s type:%s question:%s answer:%s\n",
			a.QuestionID, a.QuestionType, a.QuestionText, a.Value))
	}

	body := map[string]any{
		"model": p.model,
		"messages": []openAIMessage{
			{Role: "user", Content: sb.String()},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return AnswerAnalysisResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := openAIClient.Do(httpReq)
	if err != nil {
		return AnswerAnalysisResult{}, fmt.Errorf("openai: answer analysis request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return AnswerAnalysisResult{}, fmt.Errorf("openai: answer analysis error %d: %s", resp.StatusCode, string(b))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return AnswerAnalysisResult{}, fmt.Errorf("openai: decode answer analysis: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return AnswerAnalysisResult{}, fmt.Errorf("openai: empty answer analysis response")
	}

	var result struct {
		Answers []AnalysedAnswer `json:"answers"`
	}
	if err := json.Unmarshal([]byte(sanitizeJSONResponse(apiResp.Choices[0].Message.Content)), &result); err != nil {
		return AnswerAnalysisResult{}, fmt.Errorf("openai: parse answer analysis JSON: %w", err)
	}
	return AnswerAnalysisResult{Answers: result.Answers}, nil
}

// AggregateQuestion resume todas las respuestas a una misma pregunta, agrupa
// sus temas y marca cuáles son outliers, en una sola llamada (una llamada por
// pregunta — requisito explícito de #15).
func (p *OpenAIProvider) AggregateQuestion(ctx context.Context, req QuestionAggregationRequest) (QuestionAggregationResult, error) {
	body := map[string]any{
		"model": p.model,
		"messages": []openAIMessage{
			{Role: "user", Content: buildAggregationPrompt(req)},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	bodyBytes, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return QuestionAggregationResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := openAIClient.Do(httpReq)
	if err != nil {
		return QuestionAggregationResult{}, fmt.Errorf("openai: question aggregation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return QuestionAggregationResult{}, fmt.Errorf("openai: question aggregation error %d: %s", resp.StatusCode, string(b))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return QuestionAggregationResult{}, fmt.Errorf("openai: decode question aggregation: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return QuestionAggregationResult{}, fmt.Errorf("openai: empty question aggregation response")
	}

	result, err := parseAggregationResponse(apiResp.Choices[0].Message.Content)
	if err != nil {
		return QuestionAggregationResult{}, fmt.Errorf("openai: %w", err)
	}
	return result, nil
}

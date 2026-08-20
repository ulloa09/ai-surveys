package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	ErrResponseNotFound      = errors.New("response not found")
	ErrResponseNotInProgress = errors.New("response is not in progress")
	ErrRequiredQuestionsOpen = errors.New("required questions still unanswered")
	ErrAlreadySubmitted      = errors.New("response already submitted")
	ErrInvalidAnswer         = errors.New("invalid answer")
	ErrAIProviderUnavailable = errors.New("ai provider unavailable")
	// ErrLowQualityAnswer indica que la respuesta parece basura y el frontend
	// debe pedir al usuario que la reformule (followup de recuperación, una vez).
	ErrLowQualityAnswer = errors.New("low quality answer")
	// ErrResponseBlocked indica que el respondiente insistió con respuestas
	// basura y la sesión se cerró para no gastar más tokens.
	ErrResponseBlocked = errors.New("response blocked")
	// ErrEmptyResponse indica un intento de enviar una respuesta sin ninguna
	// pregunta contestada en una encuesta basada en preguntas (no prompt_only).
	// Una sesión que terminó por tiempo/turnos sin aportar nada no debe quedar
	// 'submitted' — se vería como una respuesta incompleta y vacía en el visor.
	ErrEmptyResponse = errors.New("response has no answers")
)

type MissingRequiredQuestionsError struct {
	Missing []string
}

func (e *MissingRequiredQuestionsError) Error() string {
	return fmt.Sprintf("%s: %d required question(s) unanswered", ErrRequiredQuestionsOpen, len(e.Missing))
}

func (e *MissingRequiredQuestionsError) Unwrap() error {
	return ErrRequiredQuestionsOpen
}

// EngineService orquesta el flujo de conversación para cada respuesta en progreso.
// Es el núcleo del sistema: recibe mensajes del respondiente, decide qué hacer
// con ellos (registrar respuesta, hacer followup, evaluar terminación) y llama
// al proveedor de IA cuando corresponde.
//
// El estado vive en la base de datos, no en el frontend: cada turno el engine
// reconstruye qué preguntas están respondidas, cuál es la activa y cuántos
// intercambios lleva la sesión. Las señales del frontend ([FOLLOWUP_NEEDED],
// [FOLLOWUP_DONE], [FOLLOWUP_CLOSE], [WELCOME]) solo indican el tipo de turno;
// el contexto real siempre se lee de la DB.
type EngineService struct {
	db              *pgxpool.Pool
	settingsSvc     *SettingsService
	questionSvc     *QuestionService
	providerFactory func(providerName, apiKey, model string) (ai.AIProvider, error)
}

type EngineOption func(*EngineService)

func WithProviderFactory(factory func(providerName, apiKey, model string) (ai.AIProvider, error)) EngineOption {
	return func(e *EngineService) {
		if factory != nil {
			e.providerFactory = factory
		}
	}
}

func NewEngineService(db *pgxpool.Pool, settingsSvc *SettingsService, questionSvc *QuestionService, opts ...EngineOption) *EngineService {
	e := &EngineService{
		db:              db,
		settingsSvc:     settingsSvc,
		questionSvc:     questionSvc,
		providerFactory: ai.NewProvider,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// TurnResult es lo que devuelve ProcessTurn al handler SSE.
// TokenCh es el canal de chunks de la respuesta en streaming.
// FinalStatus entrega exactamente un valor: el estado de la respuesta una vez
// que el stream terminó y la terminación fue evaluada. El handler debe leerlo
// DESPUÉS de drenar TokenCh para reportar el estado real al cliente.
type TurnResult struct {
	TokenCh     <-chan ai.TurnChunk
	FinalStatus <-chan string
}

// finishedTurnResult construye un TurnResult sin llamada a la IA:
// canal de tokens vacío y estado final ya conocido.
func finishedTurnResult(status string) *TurnResult {
	emptyCh := make(chan ai.TurnChunk)
	close(emptyCh)
	statusCh := make(chan string, 1)
	statusCh <- status
	close(statusCh)
	return &TurnResult{TokenCh: emptyCh, FinalStatus: statusCh}
}

// ProcessTurn ejecuta un turno completo de la conversación:
// interpreta la señal del frontend, construye el system prompt con el estado
// real de la DB, llama a la IA en streaming, persiste el intercambio y evalúa
// las condiciones de terminación.
func (e *EngineService) ProcessTurn(ctx context.Context, responseID, userMessage string) (*TurnResult, error) {
	response, survey, err := e.getResponseWithSurvey(ctx, responseID)
	if err != nil {
		return nil, err
	}

	signal, actualMessage := parseSignal(userMessage)

	// FOLLOWUP_CLOSE y SESSION_CLOSE se permiten en pending_submission — son
	// mensajes de cierre que no modifican el estado de la respuesta.
	if response.Status != "in_progress" && response.Status != "not_started" {
		isCloseSignal := signal == signalFollowupClose || signal == signalSessionClose
		if !(isCloseSignal && response.Status == "pending_submission") {
			return nil, ErrResponseNotInProgress
		}
	}
	if response.Status == "not_started" {
		if _, err := e.db.Exec(ctx,
			`UPDATE responses SET status = 'in_progress', updated_at = NOW() WHERE id = $1`, responseID,
		); err != nil {
			return nil, err
		}
		response.Status = "in_progress"
	}

	questions, err := e.questionSvc.List(ctx, survey.ID)
	if err != nil {
		return nil, fmt.Errorf("engine: load questions: %w", err)
	}

	coverage, err := e.getAnsweredQuestions(ctx, responseID)
	if err != nil {
		return nil, err
	}

	// Cuando el usuario responde al followup de una pregunta open_ended,
	// concatenamos su respuesta original con la del followup y avanzamos.
	// La pregunta en followup es la última respondida según la DB — no el
	// current_question_index, que puede quedar obsoleto tras un reload.
	if signal == signalFollowupDone {
		followedQ, qErr := e.getLastAnsweredQuestion(ctx, responseID)
		if qErr == nil && followedQ != nil && followedQ.Type == "open_ended" && followupEnabled(survey, followedQ) {
			var existingAnswer string
			_ = e.db.QueryRow(ctx,
				`SELECT raw_value FROM answers WHERE response_id = $1 AND question_id = $2`,
				responseID, followedQ.ID,
			).Scan(&existingAnswer)
			if existingAnswer != "" && strings.TrimSpace(actualMessage) != "" {
				combined := existingAnswer + " | " + strings.TrimSpace(actualMessage)
				if _, recErr := e.recordAnswer(ctx, responseID, followedQ.ID, combined); recErr != nil {
					log.Printf("engine: concat followup: %v", recErr)
				} else {
					_ = e.advanceQuestion(ctx, responseID, followedQ.OrderIndex)
					coverage, _ = e.getAnsweredQuestions(ctx, responseID)
				}
			}
		}
	}

	// Las señales de cierre (FOLLOWUP_CLOSE / SESSION_CLOSE) SÍ deben llegar a la
	// IA aunque la sesión ya esté terminada: son precisamente el mensaje de cierre
	// que el participante debe ver. El resto de señales se cortocircuitan.
	isCloseSignal := signal == signalFollowupClose || signal == signalSessionClose

	// Evaluar terminación ANTES de llamar a la IA. Si la sesión ya terminó,
	// no gastamos tokens: el frontend muestra el botón "Terminar encuesta".
	if !isCloseSignal {
		if terminated, _ := e.checkTermination(ctx, responseID, survey, questions, coverage); terminated {
			return finishedTurnResult("pending_submission"), nil
		}
	}

	exchanges, err := e.getExchangeCount(ctx, responseID)
	if err != nil {
		return nil, err
	}

	// Presupuesto de turnos agotado pero faltan preguntas requeridas: la sesión
	// sigue viva para que el usuario las complete, pero ya no se generan
	// followups nuevos (ahorro de tokens). El frontend avanza a la siguiente
	// pregunta al recibir el stream vacío con status in_progress.
	if signal == signalFollowupNeeded && e.turnBudgetExhausted(survey, exchanges) {
		return finishedTurnResult(response.Status), nil
	}

	// Modo A ('form', "preguntas fijas"): el cuestionario es exactamente el que
	// escribió el admin y nada más. Aunque el frontend pidiera una repregunta, no
	// se genera — se responde igual que con el presupuesto agotado (stream vacío)
	// y el frontend avanza a la siguiente pregunta. El gate va acá y no solo en la
	// UI porque las preguntas guardadas de una encuesta que ya estuvo en otro modo
	// pueden conservar ai_followup = true.
	if signal == signalFollowupNeeded && !followupsAllowed(survey) {
		return finishedTurnResult(response.Status), nil
	}

	var followupQ *models.Question
	if signal == signalFollowupNeeded {
		followupQ, err = e.getLastAnsweredQuestion(ctx, responseID)
		if err != nil {
			log.Printf("engine: last answered question: %v", err)
		}
	}

	systemPrompt := BuildEnginePrompt(PromptContext{
		Survey:           survey,
		Questions:        questions,
		Coverage:         coverage,
		Language:         response.Language,
		Exchanges:        exchanges,
		Signal:           signal,
		FollowupQuestion: followupQ,
	})

	transcript, err := e.getTranscript(ctx, responseID)
	if err != nil {
		return nil, err
	}

	cfg, err := e.settingsSvc.GetAIConfig(ctx)
	if err != nil || cfg.APIKey == "" {
		if err != nil {
			log.Printf("engine: ai config: %v", err)
		}
		return nil, ErrAIProviderUnavailable
	}
	provider, err := e.providerFactory(cfg.Provider, cfg.APIKey, cfg.Model)
	if err != nil {
		log.Printf("engine: provider factory: %v", err)
		return nil, ErrAIProviderUnavailable
	}

	ch, err := provider.StreamTurn(ctx, ai.TurnRequest{
		SystemPrompt: systemPrompt,
		Transcript:   transcript,
		UserMessage:  providerUserMessage(signal, actualMessage, len(transcript)),
	})
	if err != nil {
		log.Printf("engine: start stream: %v", err)
		return nil, ErrAIProviderUnavailable
	}

	outCh := make(chan ai.TurnChunk, 32)
	statusCh := make(chan string, 1)

	go func() {
		defer close(outCh)
		finalStatus := response.Status
		defer func() {
			statusCh <- finalStatus
			close(statusCh)
		}()

		var fullResponse strings.Builder
		for chunk := range ch {
			if chunk.Error == nil {
				fullResponse.WriteString(chunk.Token)
				outCh <- chunk
				continue
			}
			// Nunca filtrar el error crudo del proveedor al cliente.
			log.Printf("engine: stream chunk: %v", chunk.Error)
			outCh <- ai.TurnChunk{Error: ErrAIProviderUnavailable}
			return
		}
		msg := fullResponse.String()
		if strings.TrimSpace(msg) == "" {
			log.Printf("engine: empty provider response")
			outCh <- ai.TurnChunk{Error: ErrAIProviderUnavailable}
			return
		}
		// Si el cliente se desconectó a mitad del stream (ctx cancelado), no
		// persistimos el intercambio: el frontend reintentará el POST y una
		// segunda escritura duplicaría el turno en el transcript. Los answers
		// ya son idempotentes por su UNIQUE (response_id, question_id).
		if ctx.Err() != nil {
			log.Printf("engine: client disconnected mid-stream, skipping persist for %s", responseID)
			return
		}

		var persistErr error
		switch {
		case (signal == signalMessage || signal == signalFollowupDone) && strings.TrimSpace(actualMessage) != "":
			// Mensaje real del usuario + respuesta de la IA: un intercambio completo.
			persistErr = e.appendExchange(ctx, responseID, actualMessage, msg)
		default:
			// WELCOME, FOLLOWUP_NEEDED y FOLLOWUP_CLOSE no llevan mensaje real
			// del usuario: solo se guarda el turno de la IA (bienvenida,
			// pregunta de followup o cierre empático).
			persistErr = e.appendTurn(ctx, responseID, "assistant", msg)
		}
		if persistErr != nil {
			log.Printf("engine: persist turn: %v", persistErr)
			outCh <- ai.TurnChunk{Error: ErrAIProviderUnavailable}
			return
		}

		updatedCoverage, _ := e.getAnsweredQuestions(ctx, responseID)
		if terminated, newSt := e.checkTermination(ctx, responseID, survey, questions, updatedCoverage); terminated {
			finalStatus = newSt
		}
	}()

	return &TurnResult{TokenCh: outCh, FinalStatus: statusCh}, nil
}

// providerUserMessage decide qué se manda como mensaje de usuario al proveedor.
// El contenido del respondiente ya vive en el transcript como turnos user; las
// señales de control nunca llegan al modelo como texto.
func providerUserMessage(signal turnSignal, actualMessage string, transcriptLen int) string {
	switch signal {
	case signalMessage, signalFollowupDone:
		return actualMessage
	case signalWelcome:
		return "Hola."
	default:
		// FOLLOWUP_NEEDED / FOLLOWUP_CLOSE: el transcript ya termina con el
		// último mensaje del usuario. No agregamos nada — salvo que el
		// transcript esté vacío, en cuyo caso el proveedor necesita al menos
		// un mensaje de usuario.
		if transcriptLen == 0 {
			return "Hola."
		}
		return ""
	}
}

// turnBudgetExhausted indica si ya no quedan intercambios en el presupuesto
// de la encuesta (solo aplica cuando el turn_limit participa en la terminación).
func (e *EngineService) turnBudgetExhausted(survey *models.Survey, exchanges int) bool {
	if survey.TurnLimit == nil {
		return false
	}
	if survey.TerminationMode != "turn_limit" && survey.TerminationMode != "combination" {
		return false
	}
	return exchanges >= *survey.TurnLimit
}

// RecordAnswerDirect registra una respuesta estructurada enviada directamente
// desde el frontend (slider, radio, checkbox, etc.). Valida el valor, persiste
// el par pregunta/respuesta en el transcript, avanza el índice si la pregunta
// no tiene followup y evalúa las condiciones de terminación.
// Si la encuesta tiene allow_revisit y la respuesta está submitted, la reactiva.
func (e *EngineService) RecordAnswerDirect(ctx context.Context, responseID, questionID, value string) error {
	response, survey, err := e.getResponseWithSurvey(ctx, responseID)
	if err != nil {
		return ErrResponseNotFound
	}
	if survey.Status != "open" {
		return ErrSurveyNotOpen
	}
	if response.Status == "submitted" && survey.AllowRevisit {
		// Reactivar la respuesta para permitir edición
		if _, err := e.db.Exec(ctx,
			`UPDATE responses SET status = 'in_progress', submitted_at = NULL, updated_at = NOW() WHERE id = $1`,
			responseID); err != nil {
			return err
		}
		response.Status = "in_progress"
	}
	// pending_submission también acepta respuestas: la terminación por cobertura
	// se dispara cuando las requeridas están completas, pero el usuario aún
	// puede contestar las opcionales antes de enviar.
	if response.Status != "in_progress" && response.Status != "not_started" && response.Status != "pending_submission" {
		return ErrResponseNotInProgress
	}

	question, err := e.getQuestionForAnswer(ctx, survey.ID, questionID)
	if err != nil {
		return err
	}
	normalized, err := ValidateStructuredAnswer(value, *question)
	if err != nil {
		return err
	}

	inserted, err := e.recordAnswer(ctx, responseID, questionID, normalized)
	if err != nil {
		return err
	}
	// El par pregunta/respuesta queda en el transcript para que el historial
	// del chat y el contexto de la IA reflejen lo que el usuario realmente vio.
	// Solo en el primer registro — las ediciones no duplican turnos.
	if inserted {
		if err := e.appendQuestionAnswerTurns(ctx, responseID, question.Text, DisplayAnswer(*question, normalized, response.Language)); err != nil {
			log.Printf("engine: append question/answer turns: %v", err)
		}
	}

	// Sin repregunta, el índice avanza acá mismo. En modo 'form' NUNCA hay
	// repregunta, así que avanza siempre: si nos guiáramos solo por
	// question.AIFollowup, una pregunta que quedó con el toggle prendido de cuando
	// la encuesta estaba en otro modo dejaría el índice esperando un FOLLOWUP_DONE
	// que nadie va a mandar.
	if !followupEnabled(survey, question) {
		_, _ = e.db.Exec(ctx, `
			UPDATE responses
			SET current_question_index = current_question_index + 1, updated_at = NOW()
			WHERE id = $1 AND current_question_index = (SELECT order_index FROM questions WHERE id = $2)`,
			responseID, questionID)
	}

	questions, err := e.questionSvc.List(ctx, survey.ID)
	if err == nil {
		if coverage, err := e.getAnsweredQuestions(ctx, responseID); err == nil {
			e.checkTermination(ctx, responseID, survey, questions, coverage)
		}
	}

	return nil
}

// followupsAllowed indica si el MODO de la encuesta admite repreguntas de la IA.
//
// El modo 'form' (Mode A, "preguntas fijas") es justamente eso: el respondiente
// contesta el cuestionario que escribió el admin y nada más. Los otros dos modos
// sí conversan — 'conversational' (Mode C, híbrido) profundiza sobre las
// preguntas fijas, y 'prompt_only' (Mode B) es conversación libre.
//
// El modo manda sobre el toggle ai_followup de cada pregunta: ese toggle solo
// decide en qué preguntas profundizar DENTRO de un modo que ya conversa.
func followupsAllowed(survey *models.Survey) bool {
	return survey.Mode != "form"
}

// followupEnabled combina el modo con el toggle de la pregunta: es la condición
// real de "acá va una repregunta de la IA".
func followupEnabled(survey *models.Survey, q *models.Question) bool {
	return followupsAllowed(survey) && q.AIFollowup
}

// RecordOpenEndedAnswer registra la respuesta inicial de una pregunta open_ended
// desde el frontend, sin avanzar el índice (si tiene ai_followup, el índice
// avanza cuando llega FOLLOWUP_DONE).
//
// Detección de respuestas basura (conservadora): si el texto parece inválido
// devuelve ErrLowQualityAnswer para que el frontend pida reformular UNA vez;
// si el usuario insiste con basura en la misma pregunta, la respuesta se marca
// abandoned (closed_reason = 'low_quality_answers') y devuelve ErrResponseBlocked.
func (e *EngineService) RecordOpenEndedAnswer(ctx context.Context, responseID, questionID, value string) error {
	response, survey, err := e.getResponseWithSurvey(ctx, responseID)
	if err != nil {
		return ErrResponseNotFound
	}
	if response.Status == "submitted" && survey.AllowRevisit {
		if _, err := e.db.Exec(ctx,
			`UPDATE responses SET status = 'in_progress', submitted_at = NULL, updated_at = NOW() WHERE id = $1`,
			responseID); err != nil {
			return err
		}
		response.Status = "in_progress"
	} else if response.Status != "in_progress" && response.Status != "not_started" && response.Status != "pending_submission" {
		return ErrResponseNotInProgress
	}

	question, err := e.getQuestionForAnswer(ctx, survey.ID, questionID)
	if err != nil {
		return err
	}
	if question.Type != "open_ended" {
		return fmt.Errorf("%w: question is not open_ended", ErrInvalidAnswer)
	}
	value = strings.TrimSpace(value)

	if IsLowQualityAnswer(value) {
		return e.registerLowQualityAttempt(ctx, responseID, questionID)
	}
	// Respuesta válida: reiniciar el contador de intentos basura.
	_, _ = e.db.Exec(ctx,
		`UPDATE responses SET low_quality_attempts = 0, low_quality_question_id = NULL WHERE id = $1 AND low_quality_attempts > 0`,
		responseID)

	inserted, err := e.recordAnswer(ctx, responseID, questionID, value)
	if err != nil {
		return err
	}
	if inserted {
		if err := e.appendQuestionAnswerTurns(ctx, responseID, question.Text, value); err != nil {
			log.Printf("engine: append question/answer turns: %v", err)
		}
	}
	return nil
}

// registerLowQualityAttempt lleva la cuenta de respuestas basura.
// Primer intento en una pregunta → followup de recuperación (ErrLowQualityAnswer).
// Segundo intento en la misma pregunta → sesión bloqueada (ErrResponseBlocked).
func (e *EngineService) registerLowQualityAttempt(ctx context.Context, responseID, questionID string) error {
	var attempts int
	var lastQ *string
	if err := e.db.QueryRow(ctx,
		`SELECT low_quality_attempts, low_quality_question_id::text FROM responses WHERE id = $1`,
		responseID,
	).Scan(&attempts, &lastQ); err != nil {
		return err
	}

	if lastQ != nil && *lastQ == questionID && attempts >= 1 {
		if _, err := e.db.Exec(ctx, `
			UPDATE responses
			SET status = 'abandoned', closed_reason = 'low_quality_answers', updated_at = NOW()
			WHERE id = $1`, responseID); err != nil {
			return err
		}
		log.Printf("engine: response %s blocked for repeated low quality answers", responseID)
		return ErrResponseBlocked
	}

	if _, err := e.db.Exec(ctx, `
		UPDATE responses
		SET low_quality_attempts = 1, low_quality_question_id = $2, updated_at = NOW()
		WHERE id = $1`, responseID, questionID); err != nil {
		return err
	}
	return ErrLowQualityAnswer
}

// GetAnswersWithValues devuelve los answers con su questionId y valor para
// reconstruir el historial del chat al reanudar con allow_revisit.
func (e *EngineService) GetAnswersWithValues(ctx context.Context, responseID string) ([]map[string]string, error) {
	rows, err := e.db.Query(ctx,
		`SELECT question_id::text, raw_value FROM answers WHERE response_id = $1 ORDER BY created_at ASC`,
		responseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var answers []map[string]string
	for rows.Next() {
		var qID, value string
		if err := rows.Scan(&qID, &value); err != nil {
			return nil, err
		}
		answers = append(answers, map[string]string{"questionId": qID, "value": value})
	}
	return answers, rows.Err()
}

// GetAnsweredQuestionIDs devuelve los IDs de las preguntas que ya tienen
// una respuesta registrada para esta respuesta. El frontend lo usa para
// sincronizar su estado de progreso con la DB.
func (e *EngineService) GetAnsweredQuestionIDs(ctx context.Context, responseID string) ([]string, error) {
	rows, err := e.db.Query(ctx,
		`SELECT question_id::text FROM answers WHERE response_id = $1 ORDER BY created_at ASC`,
		responseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Submit transiciona la respuesta a 'submitted' solo si todas las preguntas
// requeridas tienen respuesta, sin depender del modo ni del motivo de terminación.
func (e *EngineService) Submit(ctx context.Context, responseID string) error {
	response, survey, err := e.getResponseWithSurvey(ctx, responseID)
	if err != nil {
		return err
	}
	if response.Status == "submitted" || response.Status == "complete" {
		return ErrAlreadySubmitted
	}
	if response.Status != "in_progress" && response.Status != "pending_submission" {
		return ErrResponseNotInProgress
	}

	questions, err := e.questionSvc.List(ctx, survey.ID)
	if err != nil {
		return err
	}
	coverage, err := e.getAnsweredQuestions(ctx, responseID)
	if err != nil {
		return err
	}
	// Mode B (prompt_only) no tiene preguntas fijas por diseño (issue #06): la
	// conversación es libre y no hay cobertura estructurada que exigir. Si el
	// survey conserva preguntas huérfanas (p. ej. tras cambiar de modo), no deben
	// bloquear el envío — el frontend nunca las muestra en este modo.
	if survey.Mode != "prompt_only" {
		if missing := MissingRequiredQuestions(questions, coverage); len(missing) > 0 {
			return &MissingRequiredQuestionsError{Missing: missing}
		}
		// Salvaguarda: una encuesta basada en preguntas nunca debe enviarse sin
		// ninguna respuesta. La terminación por tiempo/turnos ya no ofrece este
		// envío (ver evaluateTermination/ExpireResponse), pero el endpoint es
		// público, así que lo rechazamos también acá para no persistir una
		// respuesta 'submitted' vacía que aparecería como incompleta en el visor.
		if len(coverage) == 0 {
			return ErrEmptyResponse
		}
	}

	_, err = e.db.Exec(ctx,
		`UPDATE responses SET status = 'submitted', submitted_at = NOW(), updated_at = NOW() WHERE id = $1`,
		responseID)
	if err != nil {
		return err
	}
	// Emite el evento survey.submitted (por ahora solo un log entry) que el
	// Analysis Engine (#15) consumirá para disparar el análisis incremental de
	// sentimiento/tags por respuesta.
	log.Printf("survey.submitted response_id=%s survey_id=%s", responseID, survey.ID)
	return nil
}

func MissingRequiredQuestions(questions []models.Question, coverage map[string]bool) []string {
	var missing []string
	for _, q := range questions {
		if q.Required && !coverage[q.ID] {
			missing = append(missing, q.ID)
		}
	}
	return missing
}

// checkTermination evalúa si la respuesta debe terminar y actualiza el status.
func (e *EngineService) checkTermination(ctx context.Context, responseID string, survey *models.Survey, questions []models.Question, coverage map[string]bool) (bool, string) {
	if !e.evaluateTermination(ctx, responseID, survey, questions, coverage) {
		return false, "in_progress"
	}
	_, _ = e.db.Exec(ctx,
		`UPDATE responses SET status = 'pending_submission', updated_at = NOW() WHERE id = $1 AND status IN ('in_progress', 'not_started')`,
		responseID)
	return true, "pending_submission"
}

// evaluateTermination decide si se cumple la condición de terminación.
//
// Regla transversal: nunca se transiciona a pending_submission con preguntas
// requeridas sin responder — un submit que el backend va a rechazar no debe
// ofrecerse al usuario. Si el turn_limit se alcanza con requeridas pendientes,
// la sesión sigue viva (sin followups nuevos) hasta cubrirlas.
//
//   - Modo A (form): termina cuando todas las requeridas están cubiertas.
//   - Modo B (prompt_only): no tiene preguntas fijas; solo termina por
//     turn_limit (o decisión explícita del usuario con el botón Terminar).
//   - Modo C (conversational): respeta el termination_mode configurado.
//
// El límite de turnos cuenta INTERCAMBIOS (mensajes del usuario en el
// transcript), no mensajes totales — así "12 turnos" significa 12 aportes
// del respondiente, como lo define el issue #06.
func (e *EngineService) evaluateTermination(ctx context.Context, responseID string, survey *models.Survey, questions []models.Question, coverage map[string]bool) bool {
	// Mode B (prompt_only) no tiene preguntas fijas: solo termina por turn_limit
	// (o por la acción explícita del usuario con el botón Terminar). La cobertura
	// estructurada no participa — cualquier pregunta huérfana en la DB se ignora.
	if survey.Mode == "prompt_only" {
		if survey.TurnLimit == nil {
			return false
		}
		if survey.TerminationMode != "turn_limit" && survey.TerminationMode != "combination" {
			return false
		}
		exchanges, err := e.getExchangeCount(ctx, responseID)
		if err != nil {
			return false
		}
		return exchanges >= *survey.TurnLimit
	}

	requiredCovered := allRequiredCovered(questions, coverage)

	// Una sesión es "enviable" solo si cubre las requeridas Y tiene al menos una
	// respuesta. El segundo requisito importa cuando TODAS las preguntas son
	// opcionales: ahí requiredCovered es verdadero de forma vacía, así que sin él
	// una sesión sin contestar nada terminaría igual (issue "empezar de nuevo":
	// tras reiniciar, el tiempo/turnos se agotaban y la respuesta quedaba
	// 'submitted' sin ningún dato). Es el mismo criterio que ya aplicaba
	// coverageDone; aquí se extiende a la terminación por turnos/tiempo.
	submittable := requiredCovered && len(coverage) > 0
	coverageDone := len(questions) > 0 && submittable

	if survey.Mode == "form" {
		return coverageDone
	}

	exchanges, err := e.getExchangeCount(ctx, responseID)
	if err != nil {
		return false
	}
	limitReached := survey.TurnLimit != nil && exchanges >= *survey.TurnLimit

	switch survey.TerminationMode {
	case "turn_limit":
		return limitReached && submittable
	case "question_coverage":
		return coverageDone
	case "time_estimate":
		// El tiempo lo controla el cliente vía POST /expire.
		return false
	case "combination":
		return coverageDone || (limitReached && submittable)
	}
	return false
}

// allRequiredCovered retorna true si todas las preguntas requeridas tienen
// al menos una respuesta registrada en el mapa de cobertura.
func allRequiredCovered(questions []models.Question, coverage map[string]bool) bool {
	for _, q := range questions {
		if q.Required && !coverage[q.ID] {
			return false
		}
	}
	return true
}

// Turn representa un turno de la conversación para exponerlo al frontend.
type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GetTurns devuelve el historial de turnos de una respuesta en orden de
// inserción. Lo usa el frontend para reconstruir el chat al reanudar una sesión.
func (e *EngineService) GetTurns(ctx context.Context, responseID string) ([]Turn, error) {
	var status string
	if err := e.db.QueryRow(ctx, `SELECT status FROM responses WHERE id = $1`, responseID).Scan(&status); err != nil {
		return nil, ErrResponseNotFound
	}
	rows, err := e.db.Query(ctx,
		`SELECT role, content FROM turns WHERE response_id = $1 ORDER BY seq ASC`,
		responseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []Turn
	for rows.Next() {
		var t Turn
		if err := rows.Scan(&t.Role, &t.Content); err != nil {
			return nil, err
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

// getAnsweredQuestions devuelve un mapa de question_id → true para todas las
// preguntas que ya tienen una respuesta registrada. Se usa para calcular cobertura
// y para encontrar la pregunta activa.
func (e *EngineService) getAnsweredQuestions(ctx context.Context, responseID string) (map[string]bool, error) {
	rows, err := e.db.Query(ctx, `SELECT question_id::text FROM answers WHERE response_id = $1`, responseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	coverage := make(map[string]bool)
	for rows.Next() {
		var qID string
		if err := rows.Scan(&qID); err != nil {
			return nil, err
		}
		coverage[qID] = true
	}
	return coverage, rows.Err()
}

// getExchangeCount cuenta los intercambios reales de la sesión: cada mensaje
// de rol user en el transcript es un aporte del respondiente (respuesta a una
// pregunta, followup o mensaje libre en Modo B).
func (e *EngineService) getExchangeCount(ctx context.Context, responseID string) (int, error) {
	var count int
	err := e.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM turns WHERE response_id = $1 AND role = 'user'`,
		responseID).Scan(&count)
	return count, err
}

// getLastAnsweredQuestion devuelve la pregunta cuya respuesta se registró más
// recientemente. Es la pregunta sobre la que se pide el followup — el engine
// la determina desde la DB en vez de confiar en texto embebido por el frontend.
func (e *EngineService) getLastAnsweredQuestion(ctx context.Context, responseID string) (*models.Question, error) {
	row := e.db.QueryRow(ctx,
		`SELECT `+questionColumns+` FROM questions WHERE id = (
			SELECT question_id FROM answers WHERE response_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1
		)`, responseID)
	question, err := scanQuestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return question, nil
}

func (e *EngineService) getQuestionForAnswer(ctx context.Context, surveyID, questionID string) (*models.Question, error) {
	if !isUUID(questionID) {
		return nil, fmt.Errorf("%w: invalid question_id", ErrInvalidAnswer)
	}
	row := e.db.QueryRow(ctx,
		`SELECT `+questionColumns+` FROM questions WHERE id = $1 AND survey_id = $2`,
		questionID, surveyID,
	)
	question, err := scanQuestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: question does not belong to response survey", ErrInvalidAnswer)
	}
	if err != nil {
		return nil, err
	}
	return question, nil
}

// recordAnswer inserta o actualiza una respuesta en la tabla answers.
// Devuelve inserted=true si la fila es nueva (primera respuesta a esa pregunta)
// para que el caller sepa si debe persistir el intercambio en el transcript.
func (e *EngineService) recordAnswer(ctx context.Context, responseID, questionID, value string) (bool, error) {
	var inserted bool
	err := e.db.QueryRow(ctx, `
		INSERT INTO answers (response_id, question_id, raw_value)
		VALUES ($1, $2, $3)
		ON CONFLICT (response_id, question_id) DO UPDATE SET raw_value = EXCLUDED.raw_value
		RETURNING (xmax = 0)`,
		responseID, questionID, value).Scan(&inserted)
	return inserted, err
}

// advanceQuestion incrementa el índice de pregunta activa de la respuesta.
func (e *EngineService) advanceQuestion(ctx context.Context, responseID string, currentOrderIndex int) error {
	_, err := e.db.Exec(ctx,
		`UPDATE responses SET current_question_index = $1, updated_at = NOW() WHERE id = $2`,
		currentOrderIndex+1, responseID)
	return err
}

// appendTurn guarda un turno en la tabla turns e incrementa turn_count.
func (e *EngineService) appendTurn(ctx context.Context, responseID, role, content string) error {
	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO turns (response_id, role, content) VALUES ($1, $2, $3)`,
		responseID, role, content); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE responses SET turn_count = turn_count + 1, updated_at = NOW() WHERE id = $1`,
		responseID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// appendExchange guarda un intercambio completo (mensaje del usuario +
// respuesta de la IA) en una sola transacción, en ese orden.
func (e *EngineService) appendExchange(ctx context.Context, responseID, userContent, assistantContent string) error {
	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO turns (response_id, role, content) VALUES ($1, 'user', $2), ($1, 'assistant', $3)`,
		responseID, userContent, assistantContent); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE responses SET turn_count = turn_count + 2, updated_at = NOW() WHERE id = $1`,
		responseID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// appendQuestionAnswerTurns registra en el transcript la pregunta que el
// sistema mostró (como turno assistant) y la respuesta del usuario (como turno
// user). Así el historial y el contexto de la IA reflejan la conversación real
// aunque la pregunta se haya renderizado como formulario y no como mensaje.
func (e *EngineService) appendQuestionAnswerTurns(ctx context.Context, responseID, questionText, answerDisplay string) error {
	tx, err := e.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO turns (response_id, role, content) VALUES ($1, 'assistant', $2), ($1, 'user', $3)`,
		responseID, questionText, answerDisplay); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE responses SET turn_count = turn_count + 2, updated_at = NOW() WHERE id = $1`,
		responseID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// getTranscript devuelve todos los turnos de una respuesta en orden de
// inserción. Se pasa al proveedor de IA como historial de la conversación.
func (e *EngineService) getTranscript(ctx context.Context, responseID string) ([]ai.Turn, error) {
	rows, err := e.db.Query(ctx,
		`SELECT role, content FROM turns WHERE response_id = $1 ORDER BY seq ASC`,
		responseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []ai.Turn
	for rows.Next() {
		var t ai.Turn
		if err := rows.Scan(&t.Role, &t.Content); err != nil {
			return nil, err
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

// getResponseWithSurvey carga la respuesta y su encuesta en una sola operación.
func (e *EngineService) getResponseWithSurvey(ctx context.Context, responseID string) (*models.Response, *models.Survey, error) {
	if !isUUID(responseID) {
		return nil, nil, ErrResponseNotFound
	}
	row := e.db.QueryRow(ctx, `
		SELECT r.id::text, r.survey_id::text, r.user_id::text, r.fingerprint_hash,
		       r.status, r.language, r.started_at, r.submitted_at, r.updated_at,
		       r.current_question_index, r.turn_count,
		       r.registered_name, r.registered_email,
		       s.id::text, s.title, s.description, s.owner_id::text, s.owner_id::text,
		       s.team_id::text, s.status, s.mode, s.system_prompt, s.anonymity_level,
		       s.available_languages, s.default_language,
		       s.allow_revisit, s.optional_registration, s.termination_mode,
		       s.turn_limit, s.time_estimate_minutes, s.created_at, s.updated_at,
		       s.opens_at, s.closes_at, s.response_cap, s.public_token::text, s.qr_png_url, s.qr_svg_url
		FROM responses r
		JOIN surveys s ON s.id = r.survey_id
		WHERE r.id = $1`, responseID)
	var response models.Response
	var survey models.Survey
	err := row.Scan(
		&response.ID, &response.SurveyID, &response.UserID, &response.FingerprintHash,
		&response.Status, &response.Language, &response.StartedAt, &response.SubmittedAt, &response.UpdatedAt,
		&response.CurrentQuestionIndex, &response.TurnCount,
		&response.RegisteredName, &response.RegisteredEmail,
		&survey.ID, &survey.Title, &survey.Description, &survey.OwnerID, &survey.OwnerName,
		&survey.TeamID, &survey.Status, &survey.Mode, &survey.SystemPrompt, &survey.AnonymityLevel,
		&survey.AvailableLanguages, &survey.DefaultLanguage,
		&survey.AllowRevisit, &survey.OptionalRegistration, &survey.TerminationMode,
		&survey.TurnLimit, &survey.TimeEstimateMinutes, &survey.CreatedAt, &survey.UpdatedAt,
		&survey.OpensAt, &survey.ClosesAt, &survey.ResponseCap, &survey.PublicToken, &survey.QRPngURL, &survey.QRSVGURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrResponseNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &response, &survey, nil
}

// ExpireResponse se invoca cuando el temporizador de time_estimate llega a
// cero en el cliente. Si todas las requeridas están cubiertas, transiciona a
// pending_submission (el usuario decide cuándo enviar con el botón). Si faltan
// requeridas, la sesión sigue en in_progress para que pueda completarlas —
// nunca se ofrece un submit que el backend va a rechazar.
// Devuelve el status resultante. Es idempotente.
func (e *EngineService) ExpireResponse(ctx context.Context, responseID string) (string, error) {
	response, survey, err := e.getResponseWithSurvey(ctx, responseID)
	if err != nil {
		return "", ErrResponseNotFound
	}
	if response.Status != "in_progress" && response.Status != "not_started" {
		return response.Status, nil
	}

	questions, err := e.questionSvc.List(ctx, survey.ID)
	if err != nil {
		return "", err
	}
	coverage, err := e.getAnsweredQuestions(ctx, responseID)
	if err != nil {
		return "", err
	}
	if !allRequiredCovered(questions, coverage) {
		return "in_progress", nil
	}
	// Nunca finalizamos una sesión vacía por expiración del tiempo: si el
	// respondiente no aportó nada, la sesión sigue viva en vez de ofrecer un
	// envío que dejaría una respuesta 'submitted' sin datos. En prompt_only la
	// aportación son los turnos de la conversación; en los demás modos, al menos
	// una pregunta contestada. Mismo criterio que evaluateTermination.
	if survey.Mode == "prompt_only" {
		exchanges, err := e.getExchangeCount(ctx, responseID)
		if err != nil {
			return "", err
		}
		if exchanges == 0 {
			return "in_progress", nil
		}
	} else if len(coverage) == 0 {
		return "in_progress", nil
	}

	if _, err := e.db.Exec(ctx,
		`UPDATE responses SET status = 'pending_submission', updated_at = NOW() WHERE id = $1`,
		responseID); err != nil {
		return "", err
	}
	return "pending_submission", nil
}

// GetResponseStatus devuelve el status actual de una respuesta.
// Lo usa el frontend para detectar si el engine marcó la respuesta
// como pending_submission por límite de turnos u otro trigger.
func (e *EngineService) GetResponseStatus(ctx context.Context, responseID string) (string, error) {
	var status string
	err := e.db.QueryRow(ctx,
		`SELECT status FROM responses WHERE id = $1`, responseID,
	).Scan(&status)
	if err != nil {
		return "", ErrResponseNotFound
	}
	return status, nil
}

// GetResponseByUser busca la respuesta más reciente de un usuario para una
// encuesta específica. Lo usa el landing con allow_revisit para redirigir en
// vez de bloquear.
func (e *EngineService) GetResponseByUser(ctx context.Context, surveyID, userID string) (*models.Response, error) {
	row := e.db.QueryRow(ctx, `
		SELECT r.id::text, r.survey_id::text, r.user_id::text, r.fingerprint_hash,
		       r.status, r.language, r.started_at, r.submitted_at, r.updated_at,
		       r.current_question_index, r.turn_count, r.registered_name, r.registered_email
		FROM responses r
		WHERE r.survey_id = $1 AND r.user_id = $2
		ORDER BY r.started_at DESC
		LIMIT 1`, surveyID, userID)
	var response models.Response
	err := row.Scan(
		&response.ID, &response.SurveyID, &response.UserID, &response.FingerprintHash,
		&response.Status, &response.Language, &response.StartedAt, &response.SubmittedAt,
		&response.UpdatedAt, &response.CurrentQuestionIndex, &response.TurnCount,
		&response.RegisteredName, &response.RegisteredEmail,
	)
	if err != nil {
		return nil, ErrResponseNotFound
	}
	return &response, nil
}

// GetMyResponse devuelve la respuesta reanudable más reciente de un usuario
// para una encuesta específica: la que está en progreso o pendiente de
// envío. Se usa en el landing del respondiente autenticado para ofrecer
// "Continuar donde lo dejaste" desde cualquier dispositivo (issue #14). Si
// el usuario tiene dos respuestas en progreso (caso borde) se devuelve la
// más reciente. Devuelve (nil, nil) cuando no hay ninguna reanudable — no es
// un error, simplemente no hay nada que ofrecer.
func (e *EngineService) GetMyResponse(ctx context.Context, surveyID, userID string) (*models.Response, error) {
	if !isUUID(surveyID) || !isUUID(userID) {
		return nil, nil
	}
	row := e.db.QueryRow(ctx, `
		SELECT r.id::text, r.survey_id::text, r.user_id::text, r.fingerprint_hash,
		       r.status, r.language, r.started_at, r.submitted_at, r.updated_at,
		       r.current_question_index, r.turn_count, r.registered_name, r.registered_email
		FROM responses r
		WHERE r.survey_id = $1 AND r.user_id = $2
		  AND r.status IN ('in_progress', 'pending_submission')
		ORDER BY r.started_at DESC
		LIMIT 1`, surveyID, userID)
	var response models.Response
	err := row.Scan(
		&response.ID, &response.SurveyID, &response.UserID, &response.FingerprintHash,
		&response.Status, &response.Language, &response.StartedAt, &response.SubmittedAt,
		&response.UpdatedAt, &response.CurrentQuestionIndex, &response.TurnCount,
		&response.RegisteredName, &response.RegisteredEmail,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// ResponseDetail es la vista consolidada de una respuesta: el registro, el
// transcript completo, los ids de preguntas ya contestadas, el índice de la
// pregunta activa y el status. La consumen tanto la reanudación (#14) como la
// función de revisita (#13).
type ResponseDetail struct {
	Response             *models.Response `json:"response"`
	Turns                []Turn           `json:"turns"`
	AnsweredQuestionIDs  []string         `json:"answered_question_ids"`
	CurrentQuestionIndex int              `json:"current_question_index"`
	Status               string           `json:"status"`
}

// GetResponseDetail arma la vista consolidada de una respuesta. Devuelve también
// la encuesta asociada para que el handler pueda aplicar el control de acceso por
// dueño / equipo. Devuelve ErrResponseNotFound si la respuesta no existe.
func (e *EngineService) GetResponseDetail(ctx context.Context, responseID string) (*ResponseDetail, *models.Survey, error) {
	response, survey, err := e.getResponseWithSurvey(ctx, responseID)
	if err != nil {
		return nil, nil, err
	}
	turns, err := e.GetTurns(ctx, responseID)
	if err != nil {
		return nil, nil, err
	}
	coverage, err := e.getAnsweredQuestions(ctx, responseID)
	if err != nil {
		return nil, nil, err
	}
	answered := make([]string, 0, len(coverage))
	for id := range coverage {
		answered = append(answered, id)
	}
	detail := &ResponseDetail{
		Response:             response,
		Turns:                turns,
		AnsweredQuestionIDs:  answered,
		CurrentQuestionIndex: response.CurrentQuestionIndex,
		Status:               response.Status,
	}
	return detail, survey, nil
}

// GetResponseSurvey carga solo la respuesta y su encuesta, sin el transcript.
// Lo usa el handler de abandono para verificar propiedad antes de mutar.
func (e *EngineService) GetResponseSurvey(ctx context.Context, responseID string) (*models.Response, *models.Survey, error) {
	return e.getResponseWithSurvey(ctx, responseID)
}

// AbandonResponse marca una respuesta como abandonada. Lo invoca el flujo
// "Comenzar de nuevo": el usuario descarta su sesión previa para arrancar una
// limpia. Solo afecta sesiones aún vivas (in_progress / pending_submission /
// not_started); es idempotente sobre cualquier otro estado.
func (e *EngineService) AbandonResponse(ctx context.Context, responseID string) error {
	if !isUUID(responseID) {
		return ErrResponseNotFound
	}
	_, err := e.db.Exec(ctx,
		`UPDATE responses
		 SET status = 'abandoned', updated_at = NOW()
		 WHERE id = $1 AND status IN ('in_progress', 'pending_submission', 'not_started')`,
		responseID)
	return err
}

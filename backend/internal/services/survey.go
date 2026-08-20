package services

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/qr"
)

var (
	ErrSurveyNotFound      = errors.New("survey not found")
	ErrSurveyForbidden     = errors.New("access denied to survey")
	ErrAnonymityLocked     = errors.New("anonymity level cannot change after the first response")
	ErrSurveyHasResponses  = errors.New("survey has responses and cannot be deleted")
	ErrInvalidLanguage     = errors.New("invalid language configuration")
	ErrInvalidSurveyConfig = errors.New("invalid survey configuration")
	// ErrInvalidStatusTransition se devuelve cuando se intenta una transición de
	// ciclo de vida no permitida (p. ej. cerrar una encuesta en draft).
	ErrInvalidStatusTransition = errors.New("invalid survey status transition")
)

// SupportedLanguages son los únicos idiomas configurables por ahora.
var SupportedLanguages = map[string]string{"es": "Spanish", "en": "English"}

// ValidAnonymityLevels contiene los valores que acepta la columna anonymity_level.
var ValidAnonymityLevels = map[string]bool{"none": true, "partial": true, "full": true}

// ValidModes contiene los modos de encuesta soportados.
// conversational = Mode C (híbrido, recomendado), form = Mode A (preguntas fijas), prompt_only = Mode B.
var ValidModes = map[string]bool{"conversational": true, "form": true, "prompt_only": true}

// ValidTerminationModes contiene los modos de terminación soportados.
var ValidTerminationModes = map[string]bool{
	"turn_limit":        true,
	"question_coverage": true,
	"time_estimate":     true,
	"combination":       true,
}

// surveyColumns define las columnas que se seleccionan en cada consulta.
// Centralizar esto garantiza que List y Get escaneen siempre el mismo orden.
const surveyColumns = `
	s.id::text, s.title, s.description, s.owner_id::text, u.display_name,
	s.team_id::text, t.name,
	s.status, s.mode, s.system_prompt, s.available_languages, s.default_language,
	s.anonymity_level, s.allow_revisit, s.optional_registration,
	s.termination_mode, s.turn_limit, s.time_estimate_minutes,
	s.opens_at, s.closes_at, s.response_cap, s.public_token::text, s.qr_png_url, s.qr_svg_url,
	s.created_at, s.updated_at`

// QuestionCopier es lo único que SurveyService necesita de QuestionService.
// Permite copiar las preguntas de una encuesta a otra dentro de una transacción.
type QuestionCopier interface {
	CopyQuestions(ctx context.Context, tx pgx.Tx, srcSurveyID, dstSurveyID string) error
}

// SurveyService maneja el CRUD y la duplicación de encuestas.
type SurveyService struct {
	db        *pgxpool.Pool
	questions QuestionCopier
	// baseURL es el origen del frontend (FRONTEND_ORIGIN). Se usa para construir
	// el link público /s/<token> que se codifica en el QR.
	baseURL string
}

func NewSurveyService(db *pgxpool.Pool, questions QuestionCopier, baseURL string) *SurveyService {
	return &SurveyService{db: db, questions: questions, baseURL: strings.TrimRight(baseURL, "/")}
}

// PublicURL arma el link público de la encuesta a partir de su token inmutable.
func (s *SurveyService) PublicURL(publicToken string) string {
	return fmt.Sprintf("%s/s/%s", s.baseURL, publicToken)
}

// CreateSurveyInput agrupa los campos que el cliente puede enviar al crear una encuesta.
type CreateSurveyInput struct {
	Title                string
	Description          *string
	TeamID               string
	AnonymityLevel       string
	AllowRevisit         bool
	OptionalRegistration bool
	Mode                 string
	SystemPrompt         *string
	TerminationMode      string
	TurnLimit            *int
	TimeEstimateMinutes  *int
	AvailableLanguages   *[]string
	DefaultLanguage      string
}

// UpdateSurveyInput usa punteros para distinguir un campo no enviado (nil) de
// uno enviado con valor. Así el PATCH solo modifica los campos presentes.
type UpdateSurveyInput struct {
	Title                *string
	Description          *string
	AnonymityLevel       *string
	AllowRevisit         *bool
	OptionalRegistration *bool
	Mode                 *string
	SystemPrompt         *string
	TerminationMode      *string
	TurnLimit            *int
	TimeEstimateMinutes  *int
	AvailableLanguages   *[]string
	DefaultLanguage      *string
	// Programación (#08). Doble puntero para distinguir tres casos en un PATCH:
	// ausente (nil), establecer un valor, o limpiarlo a NULL (puntero a nil).
	OpensAt     **time.Time
	ClosesAt    **time.Time
	ResponseCap **int
}

// Create inserta una encuesta nueva en estado draft. Solo admin/super_admin
// llegan aquí (ver RequireRole("admin") en main.go); ambos administran
// cualquier equipo vía su rol global, así que no hace falta verificar
// membresía por equipo antes de crear.
func (s *SurveyService) Create(ctx context.Context, user *models.User, in CreateSurveyInput) (*models.Survey, error) {
	if user.Role != "super_admin" && user.Role != "admin" {
		return nil, ErrSurveyForbidden
	}

	availableLanguages, defaultLanguage, err := normalizeLanguageConfig(in.AvailableLanguages, in.DefaultLanguage)
	if err != nil {
		return nil, err
	}

	var newID string
	err = s.db.QueryRow(ctx, `
		INSERT INTO surveys (title, description, owner_id, team_id, anonymity_level, allow_revisit, optional_registration,
		                      mode, system_prompt, termination_mode, turn_limit, time_estimate_minutes,
		                      available_languages, default_language)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id::text`,
		in.Title, in.Description, user.ID, in.TeamID, in.AnonymityLevel, in.AllowRevisit, in.OptionalRegistration,
		in.Mode, in.SystemPrompt, in.TerminationMode, in.TurnLimit, in.TimeEstimateMinutes,
		availableLanguages, defaultLanguage,
	).Scan(&newID)
	if err != nil {
		return nil, err
	}

	return s.loadByID(ctx, newID)
}

// List devuelve las encuestas visibles para el usuario: todas si es
// super_admin o admin (Coordinador administra de forma transversal), solo
// las de sus equipos si es profesor. Las archivadas quedan ocultas por
// defecto — includeArchived las vuelve a mostrar ("show archived").
func (s *SurveyService) List(ctx context.Context, user *models.User, includeArchived bool) ([]models.Survey, error) {
	query := `SELECT ` + surveyColumns + `
		FROM surveys s
		JOIN users u ON u.id = s.owner_id
		JOIN teams t ON t.id = s.team_id`
	args := []any{}

	if user.Role != "super_admin" && user.Role != "admin" {
		query += ` JOIN team_members tm ON tm.team_id = s.team_id AND tm.user_id = $1`
		args = append(args, user.ID)
	}
	if !includeArchived {
		query += ` WHERE s.status != 'archived'`
	}
	query += ` ORDER BY s.created_at DESC`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var surveys []models.Survey
	for rows.Next() {
		survey, err := scanSurvey(rows)
		if err != nil {
			return nil, err
		}
		surveys = append(surveys, *survey)
	}
	return surveys, rows.Err()
}

// Get devuelve una encuesta si el usuario tiene acceso de lectura.
func (s *SurveyService) Get(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSurveyAccess(ctx, user, survey.TeamID, false); err != nil {
		return nil, err
	}
	return survey, nil
}

// Update aplica un PATCH parcial. Cambiar anonymity_level después de la
// primera respuesta devuelve ErrAnonymityLocked.
func (s *SurveyService) Update(ctx context.Context, user *models.User, id string, in UpdateSurveyInput) (*models.Survey, error) {
	survey, err := s.loadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSurveyAccess(ctx, user, survey.TeamID, true); err != nil {
		return nil, err
	}

	if in.AnonymityLevel != nil && *in.AnonymityLevel != survey.AnonymityLevel {
		hasResponses, err := s.hasResponses(ctx, id)
		if err != nil {
			return nil, err
		}
		if hasResponses {
			return nil, ErrAnonymityLocked
		}
	}

	title := survey.Title
	if in.Title != nil {
		title = *in.Title
	}
	description := survey.Description
	if in.Description != nil {
		description = in.Description
	}
	anonymityLevel := survey.AnonymityLevel
	if in.AnonymityLevel != nil {
		anonymityLevel = *in.AnonymityLevel
	}
	allowRevisit := survey.AllowRevisit
	if in.AllowRevisit != nil {
		allowRevisit = *in.AllowRevisit
	}
	optionalRegistration := survey.OptionalRegistration
	if in.OptionalRegistration != nil {
		optionalRegistration = *in.OptionalRegistration
	}
	mode := survey.Mode
	if in.Mode != nil {
		mode = *in.Mode
	}
	systemPrompt := survey.SystemPrompt
	if in.SystemPrompt != nil {
		systemPrompt = in.SystemPrompt
	}
	terminationMode := survey.TerminationMode
	if in.TerminationMode != nil {
		terminationMode = *in.TerminationMode
	}
	turnLimit := survey.TurnLimit
	if in.TurnLimit != nil {
		turnLimit = in.TurnLimit
	}
	timeEstimateMinutes := survey.TimeEstimateMinutes
	if in.TimeEstimateMinutes != nil {
		timeEstimateMinutes = in.TimeEstimateMinutes
	}

	availableLanguages := survey.AvailableLanguages
	defaultLanguage := survey.DefaultLanguage
	if in.AvailableLanguages != nil || in.DefaultLanguage != nil {
		newDefault := defaultLanguage
		if in.DefaultLanguage != nil {
			newDefault = *in.DefaultLanguage
		}
		newAvailable := in.AvailableLanguages
		if newAvailable == nil {
			newAvailable = &availableLanguages
		}
		normalized, normalizedDefault, err := normalizeLanguageConfig(newAvailable, newDefault)
		if err != nil {
			return nil, err
		}
		availableLanguages, defaultLanguage = normalized, normalizedDefault
	}

	opensAt := survey.OpensAt
	if in.OpensAt != nil {
		opensAt = *in.OpensAt
	}
	closesAt := survey.ClosesAt
	if in.ClosesAt != nil {
		closesAt = *in.ClosesAt
	}
	responseCap := survey.ResponseCap
	if in.ResponseCap != nil {
		responseCap = *in.ResponseCap
	}

	_, err = s.db.Exec(ctx, `
		UPDATE surveys
		SET title = $1, description = $2, anonymity_level = $3,
		    allow_revisit = $4, optional_registration = $5,
		    mode = $6, system_prompt = $7, termination_mode = $8,
		    turn_limit = $9, time_estimate_minutes = $10,
		    available_languages = $11, default_language = $12,
		    opens_at = $13, closes_at = $14, response_cap = $15, updated_at = NOW()
		WHERE id = $16`,
		title, description, anonymityLevel, allowRevisit, optionalRegistration,
		mode, systemPrompt, terminationMode, turnLimit, timeEstimateMinutes,
		availableLanguages, defaultLanguage,
		opensAt, closesAt, responseCap, id,
	)
	if err != nil {
		return nil, err
	}

	return s.loadByID(ctx, id)
}

// Delete borra una encuesta en draft. Se rechaza con ErrSurveyHasResponses
// si ya tiene respuestas.
func (s *SurveyService) Delete(ctx context.Context, user *models.User, id string) error {
	survey, err := s.loadByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.authorizeSurveyAccess(ctx, user, survey.TeamID, true); err != nil {
		return err
	}

	hasResponses, err := s.hasResponses(ctx, id)
	if err != nil {
		return err
	}
	if hasResponses {
		return ErrSurveyHasResponses
	}

	_, err = s.db.Exec(ctx, `DELETE FROM surveys WHERE id = $1`, id)
	return err
}

// Duplicate crea una copia en draft con " (copia)" en el título, sin
// respuestas, y deep-copia sus preguntas (#05) dentro de la misma
// transacción — si la copia de preguntas falla, la encuesta tampoco se crea.
func (s *SurveyService) Duplicate(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSurveyAccess(ctx, user, survey.TeamID, true); err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var newID string
	err = tx.QueryRow(ctx, `
		INSERT INTO surveys (title, description, owner_id, team_id, anonymity_level, allow_revisit, optional_registration,
		                      mode, system_prompt, termination_mode, turn_limit, time_estimate_minutes,
		                      available_languages, default_language)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id::text`,
		survey.Title+" (copia)", survey.Description, user.ID, survey.TeamID,
		survey.AnonymityLevel, survey.AllowRevisit, survey.OptionalRegistration,
		survey.Mode, survey.SystemPrompt, survey.TerminationMode, survey.TurnLimit, survey.TimeEstimateMinutes,
		survey.AvailableLanguages, survey.DefaultLanguage,
	).Scan(&newID)
	if err != nil {
		return nil, err
	}

	if s.questions != nil {
		if err := s.questions.CopyQuestions(ctx, tx, id, newID); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.loadByID(ctx, newID)
}

// CheckWriteAccess verifica que el usuario pueda editar la encuesta, sin
// devolverla — lo usan endpoints de otros slices (preguntas, etc.) que
// necesitan el mismo gate sin cargar la encuesta completa.
func (s *SurveyService) CheckWriteAccess(ctx context.Context, user *models.User, surveyID string) error {
	survey, err := s.loadByID(ctx, surveyID)
	if err != nil {
		return err
	}
	return s.authorizeSurveyAccess(ctx, user, survey.TeamID, true)
}

// Activate abre una encuesta en draft (draft → open), generando su QR si
// todavía no lo tiene. Exige al menos una pregunta salvo en Mode B
// (prompt_only), donde la IA conduce la conversación sin cuestionario fijo.
func (s *SurveyService) Activate(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadAuthorizedForWrite(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if survey.Status != "draft" {
		return nil, ErrInvalidStatusTransition
	}
	if survey.Mode != "prompt_only" {
		var questionCount int
		if err := s.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM questions WHERE survey_id = $1`, survey.ID,
		).Scan(&questionCount); err != nil {
			return nil, err
		}
		if questionCount == 0 {
			return nil, fmt.Errorf("%w: add at least one question before activating this survey", ErrInvalidSurveyConfig)
		}
	}
	return s.openSurvey(ctx, survey)
}

// Reopen reabre una encuesta cerrada (closed → open). El token y el QR ya
// existen desde la primera activación, por lo que no se regeneran.
func (s *SurveyService) Reopen(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadAuthorizedForWrite(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if survey.Status != "closed" {
		return nil, ErrInvalidStatusTransition
	}
	return s.openSurvey(ctx, survey)
}

// Close cierra una encuesta activa (open → closed). El link público pasa a
// mostrar "encuesta cerrada" y deja de aceptar respuestas.
func (s *SurveyService) Close(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadAuthorizedForWrite(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if survey.Status != "open" {
		return nil, ErrInvalidStatusTransition
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE surveys SET status = 'closed', updated_at = NOW() WHERE id = $1`, id); err != nil {
		return nil, err
	}
	return s.loadByID(ctx, id)
}

// Archive archiva una encuesta (draft/closed → archived). El router restringe
// este endpoint a super_admin. No se permite archivar una encuesta abierta:
// primero hay que cerrarla.
func (s *SurveyService) Archive(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadAuthorizedForWrite(ctx, user, id)
	if err != nil {
		return nil, err
	}
	if survey.Status == "open" || survey.Status == "archived" {
		return nil, ErrInvalidStatusTransition
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE surveys SET status = 'archived', updated_at = NOW() WHERE id = $1`, id); err != nil {
		return nil, err
	}
	return s.loadByID(ctx, id)
}

// loadAuthorizedForWrite carga la encuesta y verifica permiso de escritura.
// Centraliza el patrón repetido por las transiciones de ciclo de vida.
func (s *SurveyService) loadAuthorizedForWrite(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSurveyAccess(ctx, user, survey.TeamID, true); err != nil {
		return nil, err
	}
	return survey, nil
}

// openSurvey ejecuta la transición a 'open', generando el QR si la encuesta
// todavía no lo tiene (primera activación). Lo comparten Activate, Reopen y
// el scheduler.
func (s *SurveyService) openSurvey(ctx context.Context, survey *models.Survey) (*models.Survey, error) {
	if survey.QRPngURL == nil {
		png, svg, err := qr.Generate(s.PublicURL(survey.PublicToken))
		if err != nil {
			return nil, err
		}
		if _, err := s.db.Exec(ctx,
			`UPDATE surveys SET qr_png_url = $2, qr_svg_url = $3, updated_at = NOW() WHERE id = $1`,
			survey.ID, png, svg,
		); err != nil {
			return nil, err
		}
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE surveys SET status = 'open', updated_at = NOW() WHERE id = $1`, survey.ID); err != nil {
		return nil, err
	}
	return s.loadByID(ctx, survey.ID)
}

// RunScheduledTransitions aplica las transiciones automáticas basadas en
// tiempo y en tope de respuestas. Lo invoca el scheduler en segundo plano
// cada minuto (ver main.go).
//
// Orden importante: primero abrir, luego cerrar. Una encuesta cuya ventana
// (opens_at..closes_at) ya pasó por completo se abre y se cierra en la misma
// pasada, quedando correctamente en 'closed'.
func (s *SurveyService) RunScheduledTransitions(ctx context.Context) error {
	rows, err := s.db.Query(ctx,
		`SELECT id::text FROM surveys
		 WHERE status IN ('draft', 'closed') AND opens_at IS NOT NULL AND opens_at <= NOW()`)
	if err != nil {
		return err
	}
	var toOpen []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		toOpen = append(toOpen, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range toOpen {
		survey, err := s.loadByID(ctx, id)
		if err != nil {
			return err
		}
		if _, err := s.openSurvey(ctx, survey); err != nil {
			return err
		}
	}

	if _, err := s.db.Exec(ctx,
		`UPDATE surveys SET status = 'closed', updated_at = NOW()
		 WHERE status = 'open' AND closes_at IS NOT NULL AND closes_at <= NOW()`); err != nil {
		return err
	}

	// Auto-cerrar las encuestas abiertas que alcanzaron su tope de respuestas.
	if _, err := s.db.Exec(ctx,
		`UPDATE surveys s SET status = 'closed', updated_at = NOW()
		 WHERE s.status = 'open' AND s.response_cap IS NOT NULL
		   AND (SELECT COUNT(*) FROM responses r WHERE r.survey_id = s.id) >= s.response_cap`); err != nil {
		return err
	}

	return nil
}

func (s *SurveyService) loadByID(ctx context.Context, id string) (*models.Survey, error) {
	query := `SELECT ` + surveyColumns + `
		FROM surveys s
		JOIN users u ON u.id = s.owner_id
		JOIN teams t ON t.id = s.team_id
		WHERE s.id = $1`

	survey, err := scanSurvey(s.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSurveyNotFound
	}
	if err != nil {
		return nil, err
	}
	return survey, nil
}

// hasResponses indica si la encuesta tiene al menos una respuesta registrada.
func (s *SurveyService) hasResponses(ctx context.Context, surveyID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM responses WHERE survey_id = $1)`,
		surveyID,
	).Scan(&exists)
	return exists, err
}

// authorizeSurveyAccess verifica si el usuario tiene acceso a la encuesta.
// super_admin y admin (Coordinador) siempre pasan — administran cualquier
// equipo vía su rol global (ver Rbac.go), sin necesidad de ser miembros.
// profesor solo puede leer, y solo si pertenece al equipo de la encuesta.
func (s *SurveyService) authorizeSurveyAccess(ctx context.Context, user *models.User, teamID string, write bool) error {
	if user.Role == "super_admin" || user.Role == "admin" {
		return nil
	}
	if write {
		return ErrSurveyForbidden
	}

	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)`,
		teamID, user.ID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrSurveyForbidden
	}
	return nil
}

// normalizeLanguageConfig valida y normaliza available_languages y
// default_language: descarta idiomas no soportados y duplicados, y exige
// que default_language esté incluido en available_languages. Si el cliente
// no manda nada, el default es español únicamente.
func normalizeLanguageConfig(availableInput *[]string, defaultLanguage string) ([]string, string, error) {
	available := []string{"es"}
	if availableInput != nil {
		available = *availableInput
	}

	if len(available) == 0 {
		return nil, "", fmt.Errorf("%w: available_languages cannot be empty", ErrInvalidLanguage)
	}

	seen := map[string]bool{}
	normalized := make([]string, 0, len(available))
	for _, language := range available {
		language = strings.TrimSpace(language)
		if _, ok := SupportedLanguages[language]; !ok {
			return nil, "", fmt.Errorf("%w: supported languages are es and en", ErrInvalidLanguage)
		}
		if seen[language] {
			continue
		}
		seen[language] = true
		normalized = append(normalized, language)
	}
	if len(normalized) == 0 {
		return nil, "", fmt.Errorf("%w: available_languages cannot be empty", ErrInvalidLanguage)
	}

	if defaultLanguage == "" {
		defaultLanguage = normalized[0]
	}
	defaultLanguage = strings.TrimSpace(defaultLanguage)
	if _, ok := SupportedLanguages[defaultLanguage]; !ok {
		return nil, "", fmt.Errorf("%w: supported languages are es and en", ErrInvalidLanguage)
	}
	if !slices.Contains(normalized, defaultLanguage) {
		return nil, "", fmt.Errorf("%w: default_language must be included in available_languages", ErrInvalidLanguage)
	}
	return normalized, defaultLanguage, nil
}

// ValidateSurveyLanguage verifica que un idioma esté soportado y disponible
// para una encuesta en particular. Lo usan los slices posteriores (#10, #12)
// al validar el idioma elegido por un respondiente.
func ValidateSurveyLanguage(available []string, language string) error {
	language = strings.TrimSpace(language)
	if _, ok := SupportedLanguages[language]; !ok {
		return fmt.Errorf("%w: supported languages are es and en", ErrInvalidLanguage)
	}
	if !slices.Contains(available, language) {
		return fmt.Errorf("%w: language is not available for this survey", ErrInvalidLanguage)
	}
	return nil
}

// isUUID valida el formato de un id sin llegar a Postgres — evita que un
// valor malformado provoque un error de casteo (500) en vez de un 400.
func isUUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

// scanSurvey lee una fila con el orden definido en surveyColumns.
// Acepta tanto pgx.Row como pgx.Rows porque ambos implementan la interfaz Scan.
func scanSurvey(row pgx.Row) (*models.Survey, error) {
	var s models.Survey
	err := row.Scan(
		&s.ID, &s.Title, &s.Description, &s.OwnerID, &s.OwnerName,
		&s.TeamID, &s.TeamName,
		&s.Status, &s.Mode, &s.SystemPrompt, &s.AvailableLanguages, &s.DefaultLanguage,
		&s.AnonymityLevel, &s.AllowRevisit, &s.OptionalRegistration,
		&s.TerminationMode, &s.TurnLimit, &s.TimeEstimateMinutes,
		&s.OpensAt, &s.ClosesAt, &s.ResponseCap, &s.PublicToken, &s.QRPngURL, &s.QRSVGURL,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

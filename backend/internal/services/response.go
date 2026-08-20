package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	ErrResponseSurveyNotFound  = errors.New("survey not found")
	ErrSurveyNotOpen           = errors.New("survey is not open")
	ErrInvalidResponseLanguage = errors.New("invalid response language")
	// ErrResponseCapReached se devuelve cuando la encuesta ya alcanzó su tope
	// de respuestas y no puede aceptar más.
	ErrResponseCapReached = errors.New("response cap reached")
	// ErrResumeTokenNotFound se devuelve cuando un token de continuación no
	// existe, expiró, o su encuesta ya no está abierta. El cliente debe tratar
	// esto como "empezar de cero".
	ErrResumeTokenNotFound = errors.New("resume token not found")
	// ErrAuthRequired se devuelve cuando nadie llega autenticado a un endpoint
	// de respondiente que sí exige sesión: las encuestas 'partial' y 'none'.
	// Las 'full' (anónimas) se abren y contestan sin cuenta — su link es
	// público de verdad.
	ErrAuthRequired = errors.New("authentication required")
	// ErrNotTeamMember se devuelve cuando el usuario autenticado no pertenece
	// al equipo dueño de esta encuesta: el creador controla su propio roster
	// (team_members) y solo sus miembros pueden abrir/contestar el link.
	ErrNotTeamMember = errors.New("user is not a member of this survey's team")
	// ErrAlreadyResponded se devuelve cuando el usuario ya tiene una respuesta
	// submitted o in_progress para esta encuesta y allow_revisit está desactivado.
	ErrAlreadyResponded = errors.New("user already responded to this survey")
)

// PublicSurvey es la vista pública de una encuesta para los respondientes,
// resuelta a partir de su public_token. Incluye los campos necesarios para
// que el frontend configure el timer, el modo de terminación y el flujo de
// conversación correcto. ResponseCap se oculta del JSON con json:"-" porque
// es información interna de control de acceso, no de configuración pública.
type PublicSurvey struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Description          *string  `json:"description"`
	Status               string   `json:"status"`
	Mode                 string   `json:"mode"`
	AnonymityLevel       string   `json:"anonymity_level"`
	AvailableLanguages   []string `json:"available_languages"`
	DefaultLanguage      string   `json:"default_language"`
	TimeEstimateMinutes  *int     `json:"time_estimate_minutes"`
	TerminationMode      string   `json:"termination_mode"`
	TurnLimit            *int     `json:"turn_limit"`
	OptionalRegistration bool     `json:"optional_registration"`
	AllowRevisit         bool     `json:"allow_revisit"`
	PublicToken          string   `json:"public_token"`
	TeamID               string   `json:"-"`
	ResponseCap          *int     `json:"-"`
}

type CreateResponseInput struct {
	PublicToken     string
	Language        string
	RegisteredName  *string
	RegisteredEmail *string
	UserID          *string // nil para encuestas anónimas
	// DeviceHash es el HMAC del device_id del navegador (ver internal/fingerprint).
	// Solo lo usan —y solo lo guardan— las encuestas 'partial', para detectar
	// duplicados sin guardar identidad. nil si el cliente no mandó cookie.
	DeviceHash *string
}

// CreateResponseResult agrupa la respuesta creada y, cuando aplica, el token de
// continuación (resume token) emitido para ella. El token solo se emite para
// encuestas que no son completamente anónimas (anonymity_level != 'full'),
// porque las truly_anonymous arrancan una sesión nueva en cada visita y no
// guardan nada que permita reanudar (issue #09). ResumeToken es nil en ese caso.
type CreateResponseResult struct {
	Response    *models.Response `json:"response"`
	ResumeToken *string          `json:"resume_token"`
}

type ResponseService struct {
	db *pgxpool.Pool
}

func NewResponseService(db *pgxpool.Pool) *ResponseService {
	return &ResponseService{db: db}
}

func (s *ResponseService) GetPublicSurvey(ctx context.Context, token string) (*PublicSurvey, error) {
	if !isUUID(token) {
		return nil, ErrResponseSurveyNotFound
	}

	row := s.db.QueryRow(ctx, `
		SELECT id::text, title, description, status, mode, anonymity_level,
		       available_languages, default_language, time_estimate_minutes,
		       termination_mode, turn_limit,
		       optional_registration, allow_revisit, public_token::text, team_id::text, response_cap
		FROM surveys
		WHERE public_token = $1`, token)

	var survey PublicSurvey
	err := row.Scan(
		&survey.ID, &survey.Title, &survey.Description, &survey.Status, &survey.Mode,
		&survey.AnonymityLevel, &survey.AvailableLanguages, &survey.DefaultLanguage,
		&survey.TimeEstimateMinutes, &survey.TerminationMode, &survey.TurnLimit,
		&survey.OptionalRegistration, &survey.AllowRevisit, &survey.PublicToken, &survey.TeamID, &survey.ResponseCap,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResponseSurveyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &survey, nil
}

// IsTeamMember confirma si userID pertenece al equipo dueño de la encuesta —
// el chequeo de acceso central para "solo el roster del equipo puede
// abrir/contestar esta encuesta". Lo usa Create() antes de aceptar una
// respuesta, y el handler del landing page antes de mostrar el contenido de
// la encuesta a quien la visita.
func (s *ResponseService) IsTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)`,
		teamID, userID,
	).Scan(&exists)
	return exists, err
}

func (s *ResponseService) Create(ctx context.Context, in CreateResponseInput) (*CreateResponseResult, error) {
	survey, err := s.GetPublicSurvey(ctx, in.PublicToken)
	if err != nil {
		return nil, err
	}
	if survey.Status != "open" {
		return nil, ErrSurveyNotOpen
	}

	// El gate de acceso depende del anonymity_level:
	//
	//   - 'full' (anónima): link verdaderamente público. No se exige cuenta ni
	//     membresía — cualquiera con el link contesta. Es el precio explícito de
	//     lo que promete el landing ("no guardamos ningún dato que permita
	//     identificarte"): sin identidad no hay roster contra quién verificar, ni
	//     forma de detectar envíos duplicados. El cap de la encuesta
	//     (response_cap) es el único freno disponible aquí.
	//   - 'partial' / 'none': sesión obligatoria y membresía del equipo dueño de
	//     la encuesta — el creador define el roster agregando alumnos al equipo
	//     (team_members), no cualquiera con el link.
	//
	// Independiente de esto, anonymity_level sigue gobernando qué identidad queda
	// GUARDADA en la respuesta (ver INSERT más abajo).
	if survey.AnonymityLevel != "full" {
		if in.UserID == nil || *in.UserID == "" {
			return nil, ErrAuthRequired
		}
		isMember, err := s.IsTeamMember(ctx, survey.TeamID, *in.UserID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, ErrNotTeamMember
		}
	}

	language, err := ResolveResponseLanguage(*survey, in.Language)
	if err != nil {
		return nil, err
	}

	// Transacción con bloqueo de la fila de la encuesta para serializar las
	// inserciones concurrentes: así el chequeo del cap y el cierre automático
	// son consistentes aunque dos respuestas lleguen al mismo tiempo.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var surveyStatus string
	var responseCap *int
	err = tx.QueryRow(ctx,
		`SELECT status, response_cap FROM surveys WHERE id = $1 FOR UPDATE`, survey.ID,
	).Scan(&surveyStatus, &responseCap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResponseSurveyNotFound
	}
	if err != nil {
		return nil, err
	}
	// Revalidamos el estado dentro del lock: pudo cerrarse entre el
	// GetPublicSurvey y el bloqueo.
	if surveyStatus != "open" {
		return nil, ErrSurveyNotOpen
	}

	// Duplicados: contra qué se compara depende del anonymity_level, porque cada
	// nivel guarda una cosa distinta. Bloqueamos tanto las respuestas ya enviadas
	// como las in_progress.
	//
	//   - 'none': por user_id, que es lo único que esta encuesta persiste.
	//   - 'partial': por el hash del dispositivo. NO se puede usar user_id aquí,
	//     justamente porque no se guarda.
	//   - 'full': no se comparan duplicados. No hay nada guardado con qué hacerlo,
	//     y esa es la promesa explícita del nivel (issue #09).
	//
	// El lock FOR UPDATE de la encuesta (arriba) serializa las inserciones,
	// así que entre este SELECT y el INSERT no se cuela otra.
	var dupColumn, dupValue string
	switch {
	case survey.AnonymityLevel == "none" && in.UserID != nil && *in.UserID != "":
		dupColumn, dupValue = "user_id", *in.UserID
	case survey.AnonymityLevel == "partial" && in.DeviceHash != nil && *in.DeviceHash != "":
		dupColumn, dupValue = "fingerprint_hash", *in.DeviceHash
	}
	if dupColumn != "" {
		var existing int
		// dupColumn es un literal de este switch, nunca entrada del usuario.
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM responses
			WHERE survey_id = $1 AND `+dupColumn+` = $2
			  AND status IN ('submitted', 'in_progress')`,
			survey.ID, dupValue,
		).Scan(&existing); err != nil {
			return nil, err
		}
		if existing > 0 {
			return nil, ErrAlreadyResponded
		}
	}

	if responseCap != nil {
		var count int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM responses WHERE survey_id = $1`, survey.ID,
		).Scan(&count); err != nil {
			return nil, err
		}
		if count >= *responseCap {
			return nil, ErrResponseCapReached
		}
	}

	// La identidad solo se PERSISTE cuando anonymity_level = 'none'. En
	// 'partial' se exige login para entrar (roster del equipo) pero la respuesta
	// guardada no lleva el user_id real; en 'full' no hay siquiera sesión que
	// guardar. En ambos casos la promesa del landing ("tus respuestas no se
	// vinculan a tu identidad") se mantiene cierta.
	var storedUserID *string
	if survey.AnonymityLevel == "none" {
		storedUserID = in.UserID
	}

	// El hash del dispositivo solo se guarda en 'partial', que es el nivel que lo
	// necesita para detectar duplicados sin identidad. 'none' ya se deduplica por
	// user_id, y 'full' no guarda NADA derivado del respondiente.
	var storedFingerprint *string
	if survey.AnonymityLevel == "partial" {
		storedFingerprint = in.DeviceHash
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO responses (survey_id, user_id, fingerprint_hash, language, registered_name, registered_email)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, survey_id::text, user_id::text, fingerprint_hash, status, language,
		          started_at, submitted_at, current_question_index, turn_count,
		          registered_name, registered_email`,
		survey.ID, storedUserID, storedFingerprint, language, in.RegisteredName, in.RegisteredEmail,
	)
	response, err := scanResponse(row)
	if err != nil {
		return nil, err
	}

	// Token de continuación (resume token): se emite para encuestas que no son
	// completamente anónimas. El token es el id de la fila en survey_access_tokens;
	// el cliente lo guarda en localStorage y lo presenta para reanudar la sesión
	// en el mismo dispositivo (issue #09). Las truly_anonymous ('full') no reciben
	// token: cada visita es una sesión nueva.
	var resumeToken *string
	if survey.AnonymityLevel != "full" {
		var token string
		if err := tx.QueryRow(ctx, `
			INSERT INTO survey_access_tokens (response_id, survey_id, expires_at)
			VALUES ($1, $2, NOW() + INTERVAL '1 year')
			RETURNING id::text`,
			response.ID, survey.ID,
		).Scan(&token); err != nil {
			return nil, err
		}
		resumeToken = &token
	}

	// Si esta respuesta alcanzó el cap, cerramos la encuesta de inmediato.
	if responseCap != nil {
		var count int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM responses WHERE survey_id = $1`, survey.ID,
		).Scan(&count); err != nil {
			return nil, err
		}
		if count >= *responseCap {
			if _, err := tx.Exec(ctx,
				`UPDATE surveys SET status = 'closed', updated_at = NOW() WHERE id = $1`, survey.ID,
			); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &CreateResponseResult{Response: response, ResumeToken: resumeToken}, nil
}

// GetByResumeToken reanuda una respuesta a partir de un token de continuación.
//
// El token solo es válido si: existe, no ha expirado, pertenece a la encuesta
// del publicToken indicado, la encuesta sigue abierta, y la respuesta sigue en
// progreso. Cualquier otro caso devuelve ErrResumeTokenNotFound para que el
// cliente arranque una sesión nueva. Atar el token al publicToken evita que un
// token de una encuesta sirva para reanudar otra.
func (s *ResponseService) GetByResumeToken(ctx context.Context, publicToken, resumeToken string) (*models.Response, error) {
	if !isUUID(publicToken) || !isUUID(resumeToken) {
		return nil, ErrResumeTokenNotFound
	}

	row := s.db.QueryRow(ctx, `
		SELECT r.id::text, r.survey_id::text, r.user_id::text, r.fingerprint_hash,
			r.status, r.language, r.started_at, r.submitted_at,
			r.current_question_index, r.turn_count,
			r.registered_name, r.registered_email
		FROM survey_access_tokens sat
		JOIN responses r ON r.id = sat.response_id
		JOIN surveys s ON s.id = sat.survey_id
		WHERE sat.id = $1
		  AND s.public_token = $2
		  AND sat.expires_at > NOW()
		  AND s.status = 'open'
		  AND r.status = 'in_progress'`,
		resumeToken, publicToken,
	)

	response, err := scanResponse(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResumeTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func ResolveResponseLanguage(survey PublicSurvey, selectedLanguage string) (string, error) {
	language := selectedLanguage
	if language == "" {
		language = survey.DefaultLanguage
	}
	if err := ValidateSurveyLanguage(survey.AvailableLanguages, language); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidResponseLanguage, err)
	}
	return language, nil
}

func scanResponse(row pgx.Row) (*models.Response, error) {
	var response models.Response
	err := row.Scan(
		&response.ID, &response.SurveyID, &response.UserID, &response.FingerprintHash,
		&response.Status, &response.Language, &response.StartedAt, &response.SubmittedAt,
		&response.CurrentQuestionIndex, &response.TurnCount,
		&response.RegisteredName, &response.RegisteredEmail,
	)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// GetResponseWithSurvey carga la respuesta y su encuesta en una sola
// operación. Lo usa el handler de streaming para ensamblar el contexto antes
// de llamar al proveedor de IA.
func (s *ResponseService) GetResponseWithSurvey(ctx context.Context, responseID string) (*models.Response, *models.Survey, error) {
	if !isUUID(responseID) {
		return nil, nil, ErrResponseSurveyNotFound
	}

	row := s.db.QueryRow(ctx, `
		SELECT r.id::text, r.survey_id::text, r.user_id::text, r.fingerprint_hash,
		       r.status, r.language, r.started_at, r.submitted_at,
		       r.current_question_index, r.turn_count,
		       r.registered_name, r.registered_email,
		       s.id::text, s.title, s.description, s.owner_id::text, s.owner_id::text,
		       s.team_id::text, s.status, s.mode, s.system_prompt,
		       s.available_languages, s.default_language,
		       s.anonymity_level, s.allow_revisit, s.optional_registration,
		       s.termination_mode, s.turn_limit, s.time_estimate_minutes,
		       s.opens_at, s.closes_at, s.response_cap, s.public_token::text, s.qr_png_url, s.qr_svg_url,
		       s.created_at, s.updated_at
		FROM responses r
		JOIN surveys s ON s.id = r.survey_id
		WHERE r.id = $1`, responseID)

	var response models.Response
	var survey models.Survey
	err := row.Scan(
		&response.ID, &response.SurveyID, &response.UserID, &response.FingerprintHash,
		&response.Status, &response.Language, &response.StartedAt, &response.SubmittedAt,
		&response.CurrentQuestionIndex, &response.TurnCount,
		&response.RegisteredName, &response.RegisteredEmail,
		&survey.ID, &survey.Title, &survey.Description, &survey.OwnerID, &survey.OwnerName,
		&survey.TeamID, &survey.Status, &survey.Mode, &survey.SystemPrompt,
		&survey.AvailableLanguages, &survey.DefaultLanguage,
		&survey.AnonymityLevel, &survey.AllowRevisit, &survey.OptionalRegistration,
		&survey.TerminationMode, &survey.TurnLimit, &survey.TimeEstimateMinutes,
		&survey.OpensAt, &survey.ClosesAt, &survey.ResponseCap, &survey.PublicToken, &survey.QRPngURL, &survey.QRSVGURL,
		&survey.CreatedAt, &survey.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrResponseSurveyNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	return &response, &survey, nil
}

// GetTranscript devuelve todos los turnos de una respuesta en orden
// cronológico. Se pasa al proveedor de IA como historial de la conversación.
func (s *ResponseService) GetTranscript(ctx context.Context, responseID string) ([]ai.Turn, error) {
	rows, err := s.db.Query(ctx, `
		SELECT role, content
		FROM turns
		WHERE response_id = $1
		ORDER BY created_at ASC`, responseID)
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

// AppendTurn guarda un turno nuevo en la tabla turns y actualiza el contador
// turn_count de la respuesta. Lo llama el handler SSE después de cada
// intercambio usuario/asistente.
func (s *ResponseService) AppendTurn(ctx context.Context, responseID, role, content string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO turns (response_id, role, content)
		VALUES ($1, $2, $3)`, responseID, role, content); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE responses
		SET turn_count = turn_count + 1
		WHERE id = $1`, responseID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

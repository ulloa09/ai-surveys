package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	// ErrQuestionNotFound se devuelve cuando se busca una pregunta por id
	// dentro de una encuesta y no existe.
	ErrQuestionNotFound = errors.New("question not found")

	// ErrQuestionsLocked se devuelve al intentar crear, editar, borrar o
	// reordenar preguntas de una encuesta que ya tiene al menos una
	// respuesta registrada. A partir de ese punto las preguntas son
	// inmutables: cambiarlas invalidaría los datos ya recolectados.
	ErrQuestionsLocked = errors.New("survey has responses and questions cannot be modified")

	// ErrInvalidQuestion agrupa errores de validación de entrada (tipo no
	// soportado, texto vacío, etc.). Se devuelve como 400 Bad Request.
	ErrInvalidQuestion = errors.New("invalid question")
)

// ValidQuestionTypes son los siete tipos que acepta el enum question_type
// en la base de datos. Se usa tanto al crear como al actualizar para
// rechazar tipos desconocidos antes de llegar a la base de datos.
var ValidQuestionTypes = map[string]bool{
	"open_ended":    true,
	"single_choice": true,
	"multi_choice":  true,
	"true_false":    true,
	"linear_scale":  true,
	"ranking":       true,
	"matrix":        true,
}

// defaultAIFollowup calcula el valor por defecto del toggle ai_followup
// cuando el cliente no lo especifica: true solo para preguntas abiertas,
// false para cualquier tipo estructurado. El seguimiento automático de la
// IA tiene sentido sobre texto libre, pero aporta poco sobre una opción
// múltiple o un sí/no.
func defaultAIFollowup(questionType string) bool {
	return questionType == "open_ended"
}

// questionColumns centraliza el orden de columnas que se seleccionan y se
// escanean. Mantenerlo en un solo lugar evita que un SELECT y su Scan
// correspondiente se desincronicen si alguien agrega una columna nueva.
const questionColumns = `
	id::text, survey_id::text, type, text, required, ai_followup,
	options, order_index, created_at`

// QuestionService contiene las operaciones de lectura y escritura sobre la
// tabla questions. No conoce nada sobre equipos, roles ni permisos: esa
// responsabilidad vive en el middleware RequireSurveyAccess. Aquí solo se
// resuelven dos cosas: la forma de los datos y la regla de inmutabilidad
// una vez que existen respuestas.
type QuestionService struct {
	db *pgxpool.Pool
}

func NewQuestionService(db *pgxpool.Pool) *QuestionService {
	return &QuestionService{db: db}
}

// CreateQuestionInput agrupa los campos que llegan del cliente al crear una
// pregunta. Required y AIFollowup son punteros para poder distinguir entre
// "el cliente no mandó este campo" (nil, se aplica el default) y "el
// cliente lo mandó explícitamente en false".
type CreateQuestionInput struct {
	Type       string
	Text       string
	Required   *bool
	AIFollowup *bool
	Options    []byte
}

// UpdateQuestionInput es la versión para PATCH: todos los campos son
// punteros, así que solo los que vienen distintos de nil se incluyen en el
// UPDATE. Si el cliente no manda "text" en el body, el texto actual no se
// toca.
type UpdateQuestionInput struct {
	Type       *string
	Text       *string
	Required   *bool
	AIFollowup *bool
	Options    []byte
}

// List devuelve todas las preguntas de una encuesta en el orden definido
// por order_index. No valida permisos: se asume que el middleware ya
// confirmó que el usuario puede leer esta encuesta.
func (s *QuestionService) List(ctx context.Context, surveyID string) ([]models.Question, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+questionColumns+` FROM questions WHERE survey_id = $1 ORDER BY order_index ASC`,
		surveyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, *q)
	}
	return questions, rows.Err()
}

// Create agrega una pregunta al final de la encuesta. El nuevo order_index
// es el máximo actual más uno, o cero si es la primera pregunta. Antes de
// insertar valida el tipo y el texto, y verifica que la encuesta todavía no
// tenga respuestas.
func (s *QuestionService) Create(ctx context.Context, surveyID string, in CreateQuestionInput) (*models.Question, error) {
	if err := s.assertNotLocked(ctx, surveyID); err != nil {
		return nil, err
	}

	if !ValidQuestionTypes[in.Type] {
		return nil, fmt.Errorf("%w: type must be one of the 7 supported types", ErrInvalidQuestion)
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil, fmt.Errorf("%w: text is required", ErrInvalidQuestion)
	}

	if err := validateOptions(in.Type, in.Options); err != nil {
		return nil, err
	}

	required := true
	if in.Required != nil {
		required = *in.Required
	}

	aiFollowup := defaultAIFollowup(in.Type)
	if in.AIFollowup != nil {
		aiFollowup = *in.AIFollowup
	}

	var options any
	if len(in.Options) > 0 {
		options = in.Options
	}

	// El order_index se calcula como MAX(order_index)+1, pero leer el máximo y
	// luego insertar en dos pasos no es atómico: bajo creación concurrente,
	// varias goroutines leen el mismo máximo y terminan con order_index
	// duplicados (orden inconsistente). Serializamos por encuesta con un
	// advisory lock de transacción, que se libera solo al hacer commit/rollback.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, surveyID); err != nil {
		return nil, err
	}

	var nextOrder int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(order_index) + 1, 0) FROM questions WHERE survey_id = $1`,
		surveyID,
	).Scan(&nextOrder); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO questions (survey_id, type, text, required, ai_followup, options, order_index)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+questionColumns,
		surveyID, in.Type, in.Text, required, aiFollowup, options, nextOrder,
	)
	question, err := scanQuestion(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return question, nil
}

// Update aplica un PATCH parcial sobre una pregunta existente. Solo los
// campos no nulos de in se incluyen en el UPDATE. Devuelve
// ErrQuestionsLocked si la encuesta ya tiene respuestas, y
// ErrQuestionNotFound si la pregunta no existe dentro de esa encuesta.
func (s *QuestionService) Update(ctx context.Context, surveyID, questionID string, in UpdateQuestionInput) (*models.Question, error) {
	if err := s.assertNotLocked(ctx, surveyID); err != nil {
		return nil, err
	}

	if in.Type != nil && !ValidQuestionTypes[*in.Type] {
		return nil, fmt.Errorf("%w: type must be one of the 7 supported types", ErrInvalidQuestion)
	}
	if in.Text != nil && strings.TrimSpace(*in.Text) == "" {
		return nil, fmt.Errorf("%w: text cannot be empty", ErrInvalidQuestion)
	}

	if in.Options != nil {
		questionType := ""
		if in.Type != nil {
			questionType = *in.Type
		}
		if err := validateOptions(questionType, in.Options); err != nil {
			return nil, err
		}
	}

	var sets []string
	var args []any
	add := func(col string, val any) {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}

	if in.Type != nil {
		add("type", *in.Type)
	}
	if in.Text != nil {
		add("text", *in.Text)
	}
	if in.Required != nil {
		add("required", *in.Required)
	}
	if in.AIFollowup != nil {
		add("ai_followup", *in.AIFollowup)
	}
	if in.Options != nil {
		add("options", []byte(in.Options))
	}

	// Si el body no traía ningún campo reconocido, no hay nada que
	// actualizar: devolvemos la pregunta tal cual está.
	if len(sets) == 0 {
		return s.getOne(ctx, surveyID, questionID)
	}

	args = append(args, surveyID, questionID)
	query := fmt.Sprintf(
		`UPDATE questions SET %s WHERE survey_id = $%d AND id = $%d RETURNING %s`,
		strings.Join(sets, ", "), len(args)-1, len(args), questionColumns,
	)

	row := s.db.QueryRow(ctx, query, args...)
	q, err := scanQuestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQuestionNotFound
	}
	return q, err
}

// Delete elimina una pregunta de una encuesta. Devuelve ErrQuestionsLocked
// si la encuesta ya tiene respuestas, y ErrQuestionNotFound si la pregunta
// no existía.
func (s *QuestionService) Delete(ctx context.Context, surveyID, questionID string) error {
	if err := s.assertNotLocked(ctx, surveyID); err != nil {
		return err
	}

	tag, err := s.db.Exec(ctx,
		`DELETE FROM questions WHERE survey_id = $1 AND id = $2`,
		surveyID, questionID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrQuestionNotFound
	}
	return nil
}

// Reorder recibe la lista completa de ids de preguntas en el orden deseado
// y actualiza order_index de cada una según su posición: el primer id queda
// en 0, el segundo en 1, y así sucesivamente. Se ejecuta dentro de una
// transacción para que, si algún id no existe, ninguna fila quede
// actualizada a medias.
func (s *QuestionService) Reorder(ctx context.Context, surveyID string, orderedIDs []string) error {
	if err := s.assertNotLocked(ctx, surveyID); err != nil {
		return err
	}

	if len(orderedIDs) == 0 {
		return fmt.Errorf("%w: order list cannot be empty", ErrInvalidQuestion)
	}

	// Cada id se compara contra una columna uuid. Validar el formato aquí evita
	// que un id malformado provoque un error de casteo en Postgres (500).
	for _, qID := range orderedIDs {
		if !isUUID(qID) {
			return fmt.Errorf("%w: invalid question id %q", ErrInvalidQuestion, qID)
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, qID := range orderedIDs {
		tag, err := tx.Exec(ctx,
			`UPDATE questions SET order_index = $1 WHERE survey_id = $2 AND id = $3`,
			i, surveyID, qID,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: question %s not found in this survey", ErrQuestionNotFound, qID)
		}
	}

	return tx.Commit(ctx)
}

// CopyQuestions copia todas las preguntas de srcSurveyID hacia dstSurveyID,
// preservando tipo, texto, required, ai_followup, options y order_index. La
// usa SurveyService.Duplicate del slice 04 para que la encuesta duplicada
// tenga las mismas preguntas que el original.
func (s *QuestionService) CopyQuestions(ctx context.Context, tx pgx.Tx, srcSurveyID, dstSurveyID string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO questions (survey_id, type, text, required, ai_followup, options, order_index)
		 SELECT $1, type, text, required, ai_followup, options, order_index
		 FROM questions WHERE survey_id = $2`,
		dstSurveyID, srcSurveyID,
	)
	return err
}

// getOne trae una sola pregunta de una encuesta. Se usa cuando Update no
// tiene nada que cambiar pero igual necesita devolver el estado actual.
func (s *QuestionService) getOne(ctx context.Context, surveyID, questionID string) (*models.Question, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+questionColumns+` FROM questions WHERE survey_id = $1 AND id = $2`,
		surveyID, questionID,
	)
	q, err := scanQuestion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQuestionNotFound
	}
	return q, err
}

// assertNotLocked revisa si la encuesta ya tiene al menos una respuesta. Si
// la tiene, cualquier intento de modificar sus preguntas debe rechazarse:
// devuelve ErrQuestionsLocked, que el handler traduce a 409 Conflict.
func (s *QuestionService) assertNotLocked(ctx context.Context, surveyID string) error {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM responses WHERE survey_id = $1)`,
		surveyID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrQuestionsLocked
	}
	return nil
}

// scanQuestion lee una fila siguiendo el orden definido en questionColumns.
// Funciona tanto con pgx.Row (QueryRow) como con pgx.Rows (Query), ya que
// ambos exponen el método Scan con la misma firma.
func scanQuestion(row pgx.Row) (*models.Question, error) {
	var q models.Question
	err := row.Scan(
		&q.ID, &q.SurveyID, &q.Type, &q.Text, &q.Required, &q.AIFollowup,
		&q.Options, &q.OrderIndex, &q.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

package services

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	ErrSurveyNotFound     = errors.New("survey not found")
	ErrSurveyForbidden    = errors.New("access denied to survey")
	ErrAnonymityLocked    = errors.New("anonymity level cannot change after the first response")
	ErrSurveyHasResponses = errors.New("survey has responses and cannot be deleted")
)

// ValidAnonymityLevels contiene los valores que acepta la columna anonymity_level.
var ValidAnonymityLevels = map[string]bool{"none": true, "partial": true, "full": true}

// surveyColumns define las columnas que se seleccionan en cada consulta.
// Centralizar esto garantiza que List y Get escaneen siempre el mismo orden.
const surveyColumns = `
	s.id::text, s.title, s.description, s.owner_id::text, u.display_name,
	s.team_id::text, t.name,
	s.status, s.anonymity_level, s.allow_revisit, s.optional_registration,
	s.created_at, s.updated_at`

// returningColumns es el bloque RETURNING que usan Create, Update y Duplicate.
// No incluye owner_name/team_name (vienen de un JOIN) — se rellenan aparte.
const returningColumns = `
	RETURNING id::text, title, description, owner_id::text, team_id::text,
	          status, anonymity_level, allow_revisit, optional_registration,
	          created_at, updated_at`

// SurveyService maneja el CRUD y la duplicación de encuestas.
type SurveyService struct {
	db *pgxpool.Pool
}

func NewSurveyService(db *pgxpool.Pool) *SurveyService {
	return &SurveyService{db: db}
}

// CreateSurveyInput agrupa los campos que el cliente puede enviar al crear una encuesta.
type CreateSurveyInput struct {
	Title                string
	Description          *string
	TeamID               string
	AnonymityLevel       string
	AllowRevisit         bool
	OptionalRegistration bool
}

// UpdateSurveyInput usa punteros para distinguir un campo no enviado (nil) de
// uno enviado con valor. Así el PATCH solo modifica los campos presentes.
type UpdateSurveyInput struct {
	Title                *string
	Description          *string
	AnonymityLevel       *string
	AllowRevisit         *bool
	OptionalRegistration *bool
}

// Create inserta una encuesta nueva en estado draft. Solo admin/super_admin
// llegan aquí (ver RequireRole("admin") en main.go); ambos administran
// cualquier equipo vía su rol global, así que no hace falta verificar
// membresía por equipo antes de crear.
func (s *SurveyService) Create(ctx context.Context, user *models.User, in CreateSurveyInput) (*models.Survey, error) {
	if user.Role != "super_admin" && user.Role != "admin" {
		return nil, ErrSurveyForbidden
	}

	var survey models.Survey
	err := s.db.QueryRow(ctx, `
		INSERT INTO surveys (title, description, owner_id, team_id, anonymity_level, allow_revisit, optional_registration)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		`+returningColumns,
		in.Title, in.Description, user.ID, in.TeamID, in.AnonymityLevel, in.AllowRevisit, in.OptionalRegistration,
	).Scan(&survey.ID, &survey.Title, &survey.Description, &survey.OwnerID, &survey.TeamID,
		&survey.Status, &survey.AnonymityLevel, &survey.AllowRevisit, &survey.OptionalRegistration,
		&survey.CreatedAt, &survey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return s.loadByID(ctx, survey.ID)
}

// List devuelve las encuestas visibles para el usuario: todas si es
// super_admin o admin (Coordinador administra de forma transversal),
// solo las de sus equipos si es profesor.
func (s *SurveyService) List(ctx context.Context, user *models.User) ([]models.Survey, error) {
	query := `SELECT ` + surveyColumns + `
		FROM surveys s
		JOIN users u ON u.id = s.owner_id
		JOIN teams t ON t.id = s.team_id`
	args := []any{}

	if user.Role != "super_admin" && user.Role != "admin" {
		query += ` JOIN team_members tm ON tm.team_id = s.team_id AND tm.user_id = $1`
		args = append(args, user.ID)
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

	_, err = s.db.Exec(ctx, `
		UPDATE surveys
		SET title = $1, description = $2, anonymity_level = $3,
		    allow_revisit = $4, optional_registration = $5, updated_at = NOW()
		WHERE id = $6`,
		title, description, anonymityLevel, allowRevisit, optionalRegistration, id,
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
// respuestas. Las preguntas todavía no existen como concepto (#05) — cuando
// existan, Duplicate también las copiará.
func (s *SurveyService) Duplicate(ctx context.Context, user *models.User, id string) (*models.Survey, error) {
	survey, err := s.loadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeSurveyAccess(ctx, user, survey.TeamID, true); err != nil {
		return nil, err
	}

	var newID string
	err = s.db.QueryRow(ctx, `
		INSERT INTO surveys (title, description, owner_id, team_id, anonymity_level, allow_revisit, optional_registration)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text`,
		survey.Title+" (copia)", survey.Description, user.ID, survey.TeamID,
		survey.AnonymityLevel, survey.AllowRevisit, survey.OptionalRegistration,
	).Scan(&newID)
	if err != nil {
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

// scanSurvey lee una fila con el orden definido en surveyColumns.
// Acepta tanto pgx.Row como pgx.Rows porque ambos implementan la interfaz Scan.
func scanSurvey(row pgx.Row) (*models.Survey, error) {
	var s models.Survey
	err := row.Scan(
		&s.ID, &s.Title, &s.Description, &s.OwnerID, &s.OwnerName,
		&s.TeamID, &s.TeamName,
		&s.Status, &s.AnonymityLevel, &s.AllowRevisit, &s.OptionalRegistration,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

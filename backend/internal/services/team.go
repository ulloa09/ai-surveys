package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	ErrTeamNotFound      = errors.New("team not found")
	ErrAlreadyMember     = errors.New("user is already a member of this team")
	ErrMemberNotFound    = errors.New("member not found in team")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidMemberRole = errors.New("only profesor or alumno accounts can be added to a team")
)

// TeamService maneja toda la lógica de equipos.
// Recibe el pool directamente porque las queries son simples —
// no necesita una interfaz por ahora.
type TeamService struct {
	db *pgxpool.Pool
}

func NewTeamService(db *pgxpool.Pool) *TeamService {
	return &TeamService{db: db}
}

// Create crea un equipo nuevo. El creador (siempre admin/super_admin, ver
// RequireAdminRole) no se agrega como team_member — administra el equipo vía
// su rol global, igual que cualquier otro admin/super_admin (ver Rbac.go y
// authorizeTeam). Solo profesor/alumno son miembros reales de un equipo.
func (s *TeamService) Create(ctx context.Context, name, createdByID string) (*models.Team, error) {
	var team models.Team
	err := s.db.QueryRow(ctx,
		`INSERT INTO teams (name, created_by)
		VALUES ($1, $2)
		RETURNING id::text, name, created_by::text, created_at`,
		name, createdByID,
	).Scan(&team.ID, &team.Name, &team.CreatedBy, &team.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &team, nil
}

// List devuelve todos los equipos (uso exclusivo de super_admin).
// Si name no esta vacio filtra por nombre usando ILIKE para busqueda parcial
// sin distinguir mayusculas: "inge" encuentra "Ingenieria" y "INGENIERIA".
func (s *TeamService) List(ctx context.Context, name, order, year, month string) ([]models.Team, error) {
	query := `SELECT id::text, name, created_by::text, created_at FROM teams WHERE 1=1`
	var args []any

	if name != "" {
		args = append(args, "%"+name+"%")
		query += fmt.Sprintf(` AND name ILIKE $%d`, len(args))
	}
	if year != "" {
		args = append(args, year)
		query += fmt.Sprintf(` AND EXTRACT(YEAR FROM created_at) = $%d`, len(args))
	}
	if month != "" {
		args = append(args, month)
		query += fmt.Sprintf(` AND EXTRACT(MONTH FROM created_at) = $%d`, len(args))
	}

	if order == "asc" {
		query += ` ORDER BY created_at ASC`
	} else {
		query += ` ORDER BY created_at DESC`
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// ListForUser devuelve solo los equipos donde el usuario es miembro.
// Si name no esta vacio filtra ademas por nombre de equipo.
func (s *TeamService) ListForUser(ctx context.Context, userID, name, order, year, month string) ([]models.Team, error) {
	query := `SELECT t.id::text, t.name, t.created_by::text, t.created_at
		FROM teams t
		JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1`
	args := []any{userID}

	if name != "" {
		args = append(args, "%"+name+"%")
		query += fmt.Sprintf(` AND t.name ILIKE $%d`, len(args))
	}
	if year != "" {
		args = append(args, year)
		query += fmt.Sprintf(` AND EXTRACT(YEAR FROM t.created_at) = $%d`, len(args))
	}
	if month != "" {
		args = append(args, month)
		query += fmt.Sprintf(` AND EXTRACT(MONTH FROM t.created_at) = $%d`, len(args))
	}

	if order == "asc" {
		query += ` ORDER BY t.created_at ASC`
	} else {
		query += ` ORDER BY t.created_at DESC`
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

// GetWithMembers devuelve el equipo y sus miembros.
// Si member no esta vacio filtra los miembros por email o nombre usando ILIKE,
// util para encontrar a una persona especifica dentro de un equipo grande.
func (s *TeamService) GetWithMembers(ctx context.Context, teamID, member string) (*models.TeamWithMembers, error) {
	var team models.Team
	err := s.db.QueryRow(ctx,
		`SELECT id::text, name, created_by::text, created_at
		FROM teams WHERE id = $1`,
		teamID,
	).Scan(&team.ID, &team.Name, &team.CreatedBy, &team.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTeamNotFound
	}
	if err != nil {
		return nil, err
	}

	query := `SELECT tm.team_id::text, tm.user_id::text, u.email, u.display_name, tm.role
		FROM team_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.team_id = $1`
	args := []any{teamID}

	if member != "" {
		args = append(args, "%"+member+"%")
		query += ` AND (u.email ILIKE $2 OR u.display_name ILIKE $2)`
	}

	query += ` ORDER BY tm.role, u.display_name`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []models.TeamMember
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Email, &m.DisplayName, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.TeamWithMembers{Team: team, Members: members}, nil
}

// AddMember busca al usuario por email y lo agrega al equipo. El rol dentro
// del equipo no se elige a mano — se toma directamente del rol de cuenta del
// usuario (profesor/alumno), así nunca queda desalineado con lo que esa
// persona puede hacer en el resto de la plataforma. Cuentas admin/super_admin
// no pueden agregarse como miembro (ErrInvalidMemberRole): ya administran
// cualquier equipo vía su rol global.
// Devuelve ErrAlreadyMember si ya pertenece al equipo.
func (s *TeamService) AddMember(ctx context.Context, teamID, email string) (*models.TeamMember, error) {
	// Buscar el usuario por email, junto con su rol de cuenta
	var userID, displayName, role string
	err := s.db.QueryRow(ctx,
		`SELECT id::text, display_name, role::text FROM users WHERE email = $1`,
		email,
	).Scan(&userID, &displayName, &role)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if role != "profesor" && role != "alumno" {
		return nil, ErrInvalidMemberRole
	}

	// Insertarlo en team_members
	_, err = s.db.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)`,
		teamID, userID, role,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 = unique_violation — ya es miembro
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyMember
		}
		return nil, err
	}

	return &models.TeamMember{
		TeamID:      teamID,
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		Role:        role,
	}, nil
}

// RemoveMember elimina a un usuario del equipo.
func (s *TeamService) RemoveMember(ctx context.Context, teamID, userID string) error {
	var memberRole string
	err := s.db.QueryRow(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&memberRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	)
	return err
}

// MoveMember mueve a un usuario del equipo origen al equipo destino,
// preservando su rol. Es atómico — si el INSERT en el equipo destino falla
// (ej. ya es miembro ahí), el DELETE del equipo origen se revierte, así el
// usuario nunca queda sin equipo.
func (s *TeamService) MoveMember(ctx context.Context, fromTeamID, toTeamID, userID string) (*models.TeamMember, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var memberRole string
	err = tx.QueryRow(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`,
		fromTeamID, userID,
	).Scan(&memberRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`,
		fromTeamID, userID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3)`,
		toTeamID, userID, memberRole,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyMember
		}
		return nil, err
	}

	var email, displayName string
	if err := tx.QueryRow(ctx,
		`SELECT email, display_name FROM users WHERE id = $1`,
		userID,
	).Scan(&email, &displayName); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &models.TeamMember{
		TeamID:      toTeamID,
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		Role:        memberRole,
	}, nil
}

// GetMemberRole devuelve el rol de un usuario en un equipo específico.
// Devuelve ErrMemberNotFound si el usuario no pertenece al equipo.
// Lo usa el middleware de RBAC para checar permisos.
func (s *TeamService) GetMemberRole(ctx context.Context, teamID, userID string) (string, error) {
	var role string
	err := s.db.QueryRow(ctx,
		`SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrMemberNotFound
	}
	return role, err
}

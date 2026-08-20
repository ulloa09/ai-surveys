package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	ErrInvalidRole         = errors.New("invalid role")
	ErrCannotModifySelf    = errors.New("cannot perform this action on your own account")
	ErrLastSuperAdmin      = errors.New("cannot remove the last super admin")
	ErrUserHasOwnedSurveys = errors.New("cannot delete a user who owns surveys")
	ErrUserHasCreatedTeams = errors.New("cannot delete a user who created teams")
)

var validUserRoles = map[string]bool{"super_admin": true, "admin": true, "profesor": true, "alumno": true}

// UserDB is the minimal pgxpool.Pool subset UserService needs.
// Using an interface allows unit tests to inject a fake without a real database.
type UserDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// UserService maneja la gestion de usuarios de la plataforma (CRUD, rol, password).
// Uso exclusivo de super_admin — la autorizacion se aplica en el middleware de rutas.
type UserService struct {
	db UserDB
}

func NewUserService(db UserDB) *UserService {
	return &UserService{db: db}
}

// List devuelve los usuarios de la plataforma, opcionalmente filtrados por
// texto libre (email o nombre) y/o rol exacto.
func (s *UserService) List(ctx context.Context, search, role string) ([]models.User, error) {
	query := `SELECT id::text, email, display_name, role, created_at FROM users WHERE 1=1`
	var args []any

	if search != "" {
		args = append(args, "%"+search+"%")
		query += fmt.Sprintf(` AND (email ILIKE $%d OR display_name ILIKE $%d)`, len(args), len(args))
	}
	if role != "" {
		args = append(args, role)
		query += fmt.Sprintf(` AND role = $%d`, len(args))
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// Get devuelve un usuario por ID. Devuelve ErrUserNotFound si no existe.
func (s *UserService) Get(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(ctx,
		`SELECT id::text, email, display_name, role, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Create crea un usuario nuevo con el rol indicado explicitamente por el super_admin
// que lo crea (a diferencia de AuthService.Register, que deriva el rol del allowlist).
func (s *UserService) Create(ctx context.Context, email, password, displayName, role string) (*models.User, error) {
	if !validUserRoles[role] {
		return nil, ErrInvalidRole
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("email is required")
	}
	if displayName == "" {
		return nil, errors.New("display name is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var user models.User
	err = s.db.QueryRow(ctx,
		`INSERT INTO users (email, display_name, role, password_hash)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id::text, email, display_name, role, created_at`,
		email, displayName, role, string(hash),
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	return &user, nil
}

// Update actualiza email y nombre de un usuario. El rol y la contraseña se
// cambian con acciones separadas (ChangeRole, ResetPassword).
func (s *UserService) Update(ctx context.Context, id, email, displayName string) (*models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("email is required")
	}
	if displayName == "" {
		return nil, errors.New("display name is required")
	}

	var user models.User
	err := s.db.QueryRow(ctx,
		`UPDATE users SET email = $1, display_name = $2 WHERE id = $3
		 RETURNING id::text, email, display_name, role, created_at`,
		email, displayName, id,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	return &user, nil
}

// ChangeRole cambia el rol de un usuario. No permite que el super_admin que
// hace la peticion se cambie su propio rol, ni degradar al ultimo super_admin
// de la plataforma (para no dejarla sin nadie que la administre).
func (s *UserService) ChangeRole(ctx context.Context, id, newRole, callerID string) (*models.User, error) {
	if !validUserRoles[newRole] {
		return nil, ErrInvalidRole
	}
	if id == callerID {
		return nil, ErrCannotModifySelf
	}

	if newRole != "super_admin" {
		var currentRole string
		err := s.db.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, id).Scan(&currentRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		if err != nil {
			return nil, err
		}

		if currentRole == "super_admin" {
			var superAdminCount int
			if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'super_admin'`).Scan(&superAdminCount); err != nil {
				return nil, err
			}
			if superAdminCount <= 1 {
				return nil, ErrLastSuperAdmin
			}
		}
	}

	var user models.User
	err := s.db.QueryRow(ctx,
		`UPDATE users SET role = $1 WHERE id = $2
		 RETURNING id::text, email, display_name, role, created_at`,
		newRole, id,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ResetPassword establece una nueva contraseña para el usuario e invalida
// todas sus sesiones activas, forzando un nuevo login con la contraseña nueva.
func (s *UserService) ResetPassword(ctx context.Context, id, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	tag, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE id = $2`, string(hash), id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	_, err = s.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, id)
	return err
}

// Delete elimina un usuario de la plataforma.
// No permite que el super_admin que hace la peticion se elimine a si mismo,
// ni eliminar al ultimo super_admin. Tampoco permite eliminar usuarios que
// son dueños de encuestas o que crearon equipos — hacerlo dispararia un
// borrado en cascada de esos recursos (o fallaria a nivel de base de datos),
// asi que se bloquea con un error explicito para que primero se reasignen.
func (s *UserService) Delete(ctx context.Context, id, callerID string) error {
	if id == callerID {
		return ErrCannotModifySelf
	}

	var role string
	err := s.db.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, id).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	if role == "super_admin" {
		var superAdminCount int
		if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'super_admin'`).Scan(&superAdminCount); err != nil {
			return err
		}
		if superAdminCount <= 1 {
			return ErrLastSuperAdmin
		}
	}

	var surveyCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM surveys WHERE owner_id = $1`, id).Scan(&surveyCount); err != nil {
		return err
	}
	if surveyCount > 0 {
		return ErrUserHasOwnedSurveys
	}

	var teamCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM teams WHERE created_by = $1`, id).Scan(&teamCount); err != nil {
		return err
	}
	if teamCount > 0 {
		return ErrUserHasCreatedTeams
	}

	tag, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/ulloa09/ai-surveys/backend/internal/config"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrSessionNotFound    = errors.New("session not found or expired")
)

// AuthDB is the minimal pgxpool.Pool subset the service needs.
// Using an interface allows unit tests to inject a mock without a real database.
type AuthDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// AuthService handles registration, login, session validation, and logout.
type AuthService struct {
	db               AuthDB
	superAdminEmails map[string]struct{}
	sessionDuration  time.Duration
	appEnv           string
}

// NewAuthService builds an AuthService from the pool and auth config.
func NewAuthService(db AuthDB, cfg config.AuthConfig) *AuthService {
	emails := make(map[string]struct{}, len(cfg.SuperAdminEmails))
	for _, e := range cfg.SuperAdminEmails {
		emails[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}
	return &AuthService{
		db:               db,
		superAdminEmails: emails,
		sessionDuration:  time.Duration(cfg.SessionDurationH) * time.Hour,
		appEnv:           cfg.AppEnv,
	}
}

// Register hashes the password and inserts a new user.
// The role is super_admin if the email is in the SUPER_ADMIN_EMAILS allowlist,
// otherwise admin. displayName defaults to the email prefix when empty.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*models.User, error) {
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, errors.New("email is required")
	}

	if displayName == "" {
		if idx := strings.Index(email, "@"); idx > 0 {
			displayName = email[:idx]
		} else {
			displayName = email
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := "admin"
	if _, ok := s.superAdminEmails[email]; ok {
		role = "super_admin"
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

// Login validates credentials and creates a new session on success.
// Returns the user and opaque session token. Returns ErrInvalidCredentials
// for both unknown email and wrong password to prevent user enumeration.
func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	var user models.User
	err := s.db.QueryRow(ctx,
		`SELECT id::text, email, display_name, role, password_hash, created_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.PasswordHash, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return nil, "", err
	}

	expiresAt := time.Now().Add(s.sessionDuration)
	_, err = s.db.Exec(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, user.ID, expiresAt,
	)
	if err != nil {
		return nil, "", err
	}

	return &user, token, nil
}

// ValidateSession looks up the token in the sessions table and returns the
// associated user. Returns ErrSessionNotFound if the token is missing or expired.
func (s *AuthService) ValidateSession(ctx context.Context, token string) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(ctx,
		`SELECT u.id::text, u.email, u.display_name, u.role, u.created_at
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.id = $1 AND s.expires_at > NOW()`,
		token,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Logout deletes the session from the database, invalidating the token immediately.
func (s *AuthService) Logout(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, token)
	return err
}

// generateToken produces a cryptographically random 32-byte hex string.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

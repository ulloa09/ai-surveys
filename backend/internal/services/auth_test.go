package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/ulloa09/ai-surveys/backend/internal/config"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// --- mock pgx.Row ---

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r fakeRow) Scan(dest ...any) error { return r.scanFn(dest...) }

// --- mock AuthDB ---

type fakeDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRowFn(ctx, sql, args...)
}

func (f *fakeDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

// --- helper ---

func newSvc(db services.AuthDB, superAdmins ...string) *services.AuthService {
	return services.NewAuthService(db, config.AuthConfig{
		SuperAdminEmails: superAdmins,
		SessionDurationH: 24,
		AppEnv:           "development",
	})
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	return string(h)
}

// --- Register ---

func TestRegister_AssignsAdminRole(t *testing.T) {
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "uuid-1"
				*dest[1].(*string) = "test@example.com"
				*dest[2].(*string) = "test"
				*dest[3].(*string) = "admin"
				*dest[4].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := newSvc(db)
	user, err := svc.Register(context.Background(), "test@example.com", "password123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role admin, got %q", user.Role)
	}
}

func TestRegister_AssignsSuperAdminRole(t *testing.T) {
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "uuid-2"
				*dest[1].(*string) = "super@example.com"
				*dest[2].(*string) = "super"
				*dest[3].(*string) = "super_admin"
				*dest[4].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := newSvc(db, "super@example.com")
	user, err := svc.Register(context.Background(), "super@example.com", "password123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != "super_admin" {
		t.Errorf("expected role super_admin, got %q", user.Role)
	}
}

func TestRegister_EmailTaken(t *testing.T) {
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				return &pgconn.PgError{Code: "23505"}
			}}
		},
	}
	svc := newSvc(db)
	_, err := svc.Register(context.Background(), "dup@example.com", "password123", "")
	if !errors.Is(err, services.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc := newSvc(&fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error { return nil }}
		},
	})
	_, err := svc.Register(context.Background(), "a@b.com", "short", "")
	if err == nil {
		t.Error("expected error for short password, got nil")
	}
}

func TestRegister_DefaultDisplayNameFromEmail(t *testing.T) {
	var capturedName string
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			capturedName = args[1].(string) // second arg is display_name
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "id"
				*dest[1].(*string) = "user@example.com"
				*dest[2].(*string) = capturedName
				*dest[3].(*string) = "admin"
				*dest[4].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := newSvc(db)
	_, _ = svc.Register(context.Background(), "user@example.com", "password123", "")
	if capturedName != "user" {
		t.Errorf("expected display name 'user', got %q", capturedName)
	}
}

// --- Login ---

func TestLogin_ValidCredentials(t *testing.T) {
	hash := mustHash(t, "password123")
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "user-id"
				*dest[1].(*string) = "test@example.com"
				*dest[2].(*string) = "Test"
				*dest[3].(*string) = "admin"
				*dest[4].(*string) = hash
				*dest[5].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := newSvc(db)
	user, token, err := svc.Login(context.Background(), "test@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if token == "" {
		t.Error("expected non-empty session token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	hash := mustHash(t, "correctpassword")
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "user-id"
				*dest[1].(*string) = "test@example.com"
				*dest[2].(*string) = "Test"
				*dest[3].(*string) = "admin"
				*dest[4].(*string) = hash
				*dest[5].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := newSvc(db)
	_, _, err := svc.Login(context.Background(), "test@example.com", "wrongpassword")
	if !errors.Is(err, services.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	svc := newSvc(db)
	_, _, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	if !errors.Is(err, services.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for unknown email, got %v", err)
	}
}

// --- ValidateSession ---

func TestValidateSession_Valid(t *testing.T) {
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "user-id"
				*dest[1].(*string) = "test@example.com"
				*dest[2].(*string) = "Test"
				*dest[3].(*string) = "admin"
				*dest[4].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := newSvc(db)
	user, err := svc.ValidateSession(context.Background(), "valid-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("unexpected email: %q", user.Email)
	}
}

func TestValidateSession_NotFound(t *testing.T) {
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
		},
	}
	svc := newSvc(db)
	_, err := svc.ValidateSession(context.Background(), "missing-token")
	if !errors.Is(err, services.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// --- Logout ---

func TestLogout_DeletesSession(t *testing.T) {
	var deletedToken string
	db := &fakeDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			return fakeRow{scanFn: func(dest ...any) error { return nil }}
		},
		execFn: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
			if len(args) > 0 {
				deletedToken = args[0].(string)
			}
			return pgconn.CommandTag{}, nil
		},
	}
	svc := newSvc(db)
	if err := svc.Logout(context.Background(), "my-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedToken != "my-token" {
		t.Errorf("expected 'my-token' to be deleted, got %q", deletedToken)
	}
}

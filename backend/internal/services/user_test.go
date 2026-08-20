package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// --- mock UserDB ---

type fakeUserDB struct {
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (f *fakeUserDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return f.queryRowFn(ctx, sql, args...)
}

func (f *fakeUserDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if f.execFn != nil {
		return f.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeUserDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented in this fake")
}

// --- Create ---

func TestUserCreate_InvalidRole(t *testing.T) {
	svc := services.NewUserService(&fakeUserDB{})
	_, err := svc.Create(context.Background(), "a@b.com", "password123", "A", "superuser")
	if !errors.Is(err, services.ErrInvalidRole) {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestUserCreate_EmailTaken(t *testing.T) {
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return fakeRow{scanFn: func(_ ...any) error {
				return &pgconn.PgError{Code: "23505"}
			}}
		},
	}
	svc := services.NewUserService(db)
	_, err := svc.Create(context.Background(), "dup@b.com", "password123", "A", "alumno")
	if !errors.Is(err, services.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestUserCreate_ExplicitRolePersisted(t *testing.T) {
	var capturedRole string
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, _ string, args ...any) pgx.Row {
			capturedRole = args[2].(string)
			return fakeRow{scanFn: func(dest ...any) error {
				*dest[0].(*string) = "id-1"
				*dest[1].(*string) = "a@b.com"
				*dest[2].(*string) = "A"
				*dest[3].(*string) = capturedRole
				*dest[4].(*time.Time) = time.Now()
				return nil
			}}
		},
	}
	svc := services.NewUserService(db)
	user, err := svc.Create(context.Background(), "a@b.com", "password123", "A", "alumno")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != "alumno" {
		t.Errorf("expected role alumno, got %q", user.Role)
	}
}

// --- ChangeRole ---

func TestChangeRole_InvalidRole(t *testing.T) {
	svc := services.NewUserService(&fakeUserDB{})
	_, err := svc.ChangeRole(context.Background(), "target-id", "superuser", "caller-id")
	if !errors.Is(err, services.ErrInvalidRole) {
		t.Errorf("expected ErrInvalidRole, got %v", err)
	}
}

func TestChangeRole_CannotModifySelf(t *testing.T) {
	svc := services.NewUserService(&fakeUserDB{})
	_, err := svc.ChangeRole(context.Background(), "same-id", "admin", "same-id")
	if !errors.Is(err, services.ErrCannotModifySelf) {
		t.Errorf("expected ErrCannotModifySelf, got %v", err)
	}
}

func TestChangeRole_BlocksLastSuperAdminDemotion(t *testing.T) {
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT role FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "super_admin"
					return nil
				}}
			case strings.Contains(sql, "COUNT(*)"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 1
					return nil
				}}
			}
			t.Fatalf("unexpected query: %s", sql)
			return nil
		},
	}
	svc := services.NewUserService(db)
	_, err := svc.ChangeRole(context.Background(), "target-id", "admin", "caller-id")
	if !errors.Is(err, services.ErrLastSuperAdmin) {
		t.Errorf("expected ErrLastSuperAdmin, got %v", err)
	}
}

func TestChangeRole_AllowsDemotionWhenMultipleSuperAdmins(t *testing.T) {
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, sql string, args ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT role FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "super_admin"
					return nil
				}}
			case strings.Contains(sql, "COUNT(*)"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 2
					return nil
				}}
			case strings.Contains(sql, "UPDATE users SET role"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "target-id"
					*dest[1].(*string) = "t@b.com"
					*dest[2].(*string) = "T"
					*dest[3].(*string) = args[0].(string)
					*dest[4].(*time.Time) = time.Now()
					return nil
				}}
			}
			t.Fatalf("unexpected query: %s", sql)
			return nil
		},
	}
	svc := services.NewUserService(db)
	user, err := svc.ChangeRole(context.Background(), "target-id", "admin", "caller-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("expected role admin, got %q", user.Role)
	}
}

// --- ResetPassword ---

func TestResetPassword_ShortPassword(t *testing.T) {
	svc := services.NewUserService(&fakeUserDB{})
	err := svc.ResetPassword(context.Background(), "id", "short")
	if err == nil {
		t.Error("expected error for short password, got nil")
	}
}

func TestResetPassword_NotFound(t *testing.T) {
	db := &fakeUserDB{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	}
	svc := services.NewUserService(db)
	err := svc.ResetPassword(context.Background(), "missing-id", "password123")
	if !errors.Is(err, services.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestResetPassword_InvalidatesSessions(t *testing.T) {
	var deletedSessionsFor string
	db := &fakeUserDB{
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "DELETE FROM sessions") {
				deletedSessionsFor = args[0].(string)
			}
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
	}
	svc := services.NewUserService(db)
	if err := svc.ResetPassword(context.Background(), "user-id", "password123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedSessionsFor != "user-id" {
		t.Errorf("expected sessions to be invalidated for user-id, got %q", deletedSessionsFor)
	}
}

// --- Delete ---

func TestDelete_CannotDeleteSelf(t *testing.T) {
	svc := services.NewUserService(&fakeUserDB{})
	err := svc.Delete(context.Background(), "same-id", "same-id")
	if !errors.Is(err, services.ErrCannotModifySelf) {
		t.Errorf("expected ErrCannotModifySelf, got %v", err)
	}
}

func TestDelete_BlocksLastSuperAdmin(t *testing.T) {
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT role FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "super_admin"
					return nil
				}}
			case strings.Contains(sql, "COUNT(*)"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 1
					return nil
				}}
			}
			t.Fatalf("unexpected query: %s", sql)
			return nil
		},
	}
	svc := services.NewUserService(db)
	err := svc.Delete(context.Background(), "target-id", "caller-id")
	if !errors.Is(err, services.ErrLastSuperAdmin) {
		t.Errorf("expected ErrLastSuperAdmin, got %v", err)
	}
}

func TestDelete_BlocksUserWithOwnedSurveys(t *testing.T) {
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT role FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "admin"
					return nil
				}}
			case strings.Contains(sql, "FROM surveys"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 3
					return nil
				}}
			}
			t.Fatalf("unexpected query: %s", sql)
			return nil
		},
	}
	svc := services.NewUserService(db)
	err := svc.Delete(context.Background(), "target-id", "caller-id")
	if !errors.Is(err, services.ErrUserHasOwnedSurveys) {
		t.Errorf("expected ErrUserHasOwnedSurveys, got %v", err)
	}
}

func TestDelete_BlocksUserWhoCreatedTeams(t *testing.T) {
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT role FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "admin"
					return nil
				}}
			case strings.Contains(sql, "FROM surveys"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				}}
			case strings.Contains(sql, "FROM teams"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 1
					return nil
				}}
			}
			t.Fatalf("unexpected query: %s", sql)
			return nil
		},
	}
	svc := services.NewUserService(db)
	err := svc.Delete(context.Background(), "target-id", "caller-id")
	if !errors.Is(err, services.ErrUserHasCreatedTeams) {
		t.Errorf("expected ErrUserHasCreatedTeams, got %v", err)
	}
}

func TestDelete_Success(t *testing.T) {
	var deletedID string
	db := &fakeUserDB{
		queryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch {
			case strings.Contains(sql, "SELECT role FROM users"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "alumno"
					return nil
				}}
			case strings.Contains(sql, "FROM surveys"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				}}
			case strings.Contains(sql, "FROM teams"):
				return fakeRow{scanFn: func(dest ...any) error {
					*dest[0].(*int) = 0
					return nil
				}}
			}
			t.Fatalf("unexpected query: %s", sql)
			return nil
		},
		execFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "DELETE FROM users") {
				deletedID = args[0].(string)
			}
			return pgconn.NewCommandTag("DELETE 1"), nil
		},
	}
	svc := services.NewUserService(db)
	if err := svc.Delete(context.Background(), "target-id", "caller-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "target-id" {
		t.Errorf("expected target-id to be deleted, got %q", deletedID)
	}
}

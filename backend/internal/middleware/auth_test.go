package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

// --- mock SessionValidator ---

type fakeValidator struct {
	user *models.User
	err  error
}

func (f *fakeValidator) ValidateSession(_ context.Context, _ string) (*models.User, error) {
	return f.user, f.err
}

// --- tests ---

func TestAuthenticate_ValidCookie(t *testing.T) {
	user := &models.User{ID: "u1", Email: "a@b.com", Role: "admin"}
	svc := &fakeValidator{user: user}

	var contextUser *models.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextUser = middleware.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if contextUser == nil || contextUser.Email != "a@b.com" {
		t.Errorf("expected user in context, got %v", contextUser)
	}
}

func TestAuthenticate_NoCookie(t *testing.T) {
	svc := &fakeValidator{}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	handler := middleware.Authenticate(svc)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Error("next handler should not have been called")
	}
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	svc := &fakeValidator{err: errors.New("session not found")}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	handler := middleware.Authenticate(svc)(next)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "bad-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Error("next handler should not have been called")
	}
}

package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

// serveAsUser runs h through Authenticate with a fake validator so
// UserFromContext resolves to user — userContextKey is unexported, so this is
// the only way to inject an authenticated caller from outside the package.
// Also attaches a chi {teamID} URL param, since RequireTeamMember reads it.
func serveAsUser(h http.Handler, user *models.User) *httptest.ResponseRecorder {
	handler := middleware.Authenticate(&fakeValidator{user: user})(h)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok"})

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("teamID", "team-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

// --- RequireRole ---

func TestRequireRole_SuperAdminAlwaysBypasses(t *testing.T) {
	user := &models.User{ID: "u1", Role: "super_admin"}
	rec := serveAsUser(middleware.RequireRole("profesor")(okHandler()), user)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_AllowedRolePasses(t *testing.T) {
	user := &models.User{ID: "u1", Role: "profesor"}
	rec := serveAsUser(middleware.RequireRole("admin", "profesor")(okHandler()), user)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_DisallowedRoleForbidden(t *testing.T) {
	user := &models.User{ID: "u1", Role: "alumno"}
	rec := serveAsUser(middleware.RequireRole("admin", "profesor")(okHandler()), user)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for alumno, got %d", rec.Code)
	}
}

func TestRequireRole_NoUserUnauthorized(t *testing.T) {
	handler := middleware.RequireRole("admin")(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// --- RequireTeamMember: admin bypass (Fase 3) ---

type fakeTeamRoleChecker struct {
	role string
	err  error
}

func (f *fakeTeamRoleChecker) GetMemberRole(_ context.Context, _, _ string) (string, error) {
	return f.role, f.err
}

func TestRequireTeamMember_AdminBypassesWithoutMembership(t *testing.T) {
	// El checker devolveria "not a member" — el bypass de admin no debe
	// siquiera necesitar que la consulta de membresia tenga exito.
	svc := &fakeTeamRoleChecker{err: errors.New("not a member")}
	user := &models.User{ID: "u1", Role: "admin"}
	rec := serveAsUser(middleware.RequireTeamMember(svc)(okHandler()), user)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (admin bypass), got %d", rec.Code)
	}
}

func TestRequireTeamMember_PlainRoleStillNeedsMembership(t *testing.T) {
	// Rol "profesor" — no bypasea, si no es miembro del equipo recibe 403.
	svc := &fakeTeamRoleChecker{err: errors.New("not a member")}
	user := &models.User{ID: "u1", Role: "profesor"}
	rec := serveAsUser(middleware.RequireTeamMember(svc)(okHandler()), user)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member profesor, got %d", rec.Code)
	}
}

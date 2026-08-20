package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// --- mock UserServicer ---

type fakeUserSvc struct {
	listFn        func(ctx context.Context, search, role string) ([]models.User, error)
	getFn         func(ctx context.Context, id string) (*models.User, error)
	createFn      func(ctx context.Context, email, password, displayName, role string) (*models.User, error)
	updateFn      func(ctx context.Context, id, email, displayName string) (*models.User, error)
	changeRoleFn  func(ctx context.Context, id, newRole, callerID string) (*models.User, error)
	resetPasswdFn func(ctx context.Context, id, newPassword string) error
	deleteFn      func(ctx context.Context, id, callerID string) error
}

func (f *fakeUserSvc) List(ctx context.Context, search, role string) ([]models.User, error) {
	return f.listFn(ctx, search, role)
}
func (f *fakeUserSvc) Get(ctx context.Context, id string) (*models.User, error) {
	return f.getFn(ctx, id)
}
func (f *fakeUserSvc) Create(ctx context.Context, email, password, displayName, role string) (*models.User, error) {
	return f.createFn(ctx, email, password, displayName, role)
}
func (f *fakeUserSvc) Update(ctx context.Context, id, email, displayName string) (*models.User, error) {
	return f.updateFn(ctx, id, email, displayName)
}
func (f *fakeUserSvc) ChangeRole(ctx context.Context, id, newRole, callerID string) (*models.User, error) {
	return f.changeRoleFn(ctx, id, newRole, callerID)
}
func (f *fakeUserSvc) ResetPassword(ctx context.Context, id, newPassword string) error {
	return f.resetPasswdFn(ctx, id, newPassword)
}
func (f *fakeUserSvc) Delete(ctx context.Context, id, callerID string) error {
	return f.deleteFn(ctx, id, callerID)
}

var _ handlers.UserServicer = (*fakeUserSvc)(nil)

// withChiParam attaches a chi URL param to the request, since these handlers
// use chi.URLParam and are tested without a full router mux.
func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// serveWithCaller runs h through middleware.Authenticate with a fake session
// validator so appmw.UserFromContext(r.Context()) resolves to caller, mirroring
// the pattern used by TestMeHandler_ReturnsUser in auth_test.go — userContextKey
// is unexported, so this is the only way to inject a caller from this package.
func serveWithCaller(h http.HandlerFunc, caller *models.User, r *http.Request, rec *httptest.ResponseRecorder) {
	r.AddCookie(&http.Cookie{Name: "session", Value: "tok"})
	handler := middleware.Authenticate(&fakeSessionValidator{user: caller})(h)
	handler.ServeHTTP(rec, r)
}

// --- CreateUser ---

func TestCreateUserHandler_Created(t *testing.T) {
	svc := &fakeUserSvc{
		createFn: func(_ context.Context, email, _, _, role string) (*models.User, error) {
			return &models.User{ID: "1", Email: email, Role: role}, nil
		},
	}
	body := `{"email":"a@b.com","password":"password123","display_name":"A","role":"alumno"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.CreateUser(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUserHandler_MissingFields(t *testing.T) {
	svc := &fakeUserSvc{}
	body := `{"email":"","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.CreateUser(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCreateUserHandler_InvalidRole(t *testing.T) {
	svc := &fakeUserSvc{
		createFn: func(_ context.Context, _, _, _, _ string) (*models.User, error) {
			return nil, services.ErrInvalidRole
		},
	}
	body := `{"email":"a@b.com","password":"password123","display_name":"A","role":"superuser"}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handlers.CreateUser(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// --- ChangeUserRole ---

func TestChangeUserRoleHandler_CannotModifySelf(t *testing.T) {
	svc := &fakeUserSvc{
		changeRoleFn: func(_ context.Context, _, _, _ string) (*models.User, error) {
			return nil, services.ErrCannotModifySelf
		},
	}
	caller := &models.User{ID: "11111111-1111-1111-1111-111111111111", Role: "super_admin"}
	body := `{"role":"admin"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/11111111-1111-1111-1111-111111111111/role", strings.NewReader(body))
	req = withChiParam(req, "userID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()

	serveWithCaller(handlers.ChangeUserRole(svc), caller, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUserRoleHandler_LastSuperAdmin(t *testing.T) {
	svc := &fakeUserSvc{
		changeRoleFn: func(_ context.Context, _, _, _ string) (*models.User, error) {
			return nil, services.ErrLastSuperAdmin
		},
	}
	caller := &models.User{ID: "33333333-3333-3333-3333-333333333333", Role: "super_admin"}
	body := `{"role":"admin"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/22222222-2222-2222-2222-222222222222/role", strings.NewReader(body))
	req = withChiParam(req, "userID", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()

	serveWithCaller(handlers.ChangeUserRole(svc), caller, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUserRoleHandler_Success(t *testing.T) {
	svc := &fakeUserSvc{
		changeRoleFn: func(_ context.Context, id, newRole, _ string) (*models.User, error) {
			return &models.User{ID: id, Role: newRole}, nil
		},
	}
	caller := &models.User{ID: "33333333-3333-3333-3333-333333333333", Role: "super_admin"}
	body := `{"role":"admin"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/22222222-2222-2222-2222-222222222222/role", strings.NewReader(body))
	req = withChiParam(req, "userID", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()

	serveWithCaller(handlers.ChangeUserRole(svc), caller, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- DeleteUser ---

func TestDeleteUserHandler_CannotDeleteSelf(t *testing.T) {
	svc := &fakeUserSvc{
		deleteFn: func(_ context.Context, _, _ string) error {
			return services.ErrCannotModifySelf
		},
	}
	caller := &models.User{ID: "11111111-1111-1111-1111-111111111111", Role: "super_admin"}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/11111111-1111-1111-1111-111111111111", nil)
	req = withChiParam(req, "userID", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()

	serveWithCaller(handlers.DeleteUser(svc), caller, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteUserHandler_OwnedSurveysConflict(t *testing.T) {
	svc := &fakeUserSvc{
		deleteFn: func(_ context.Context, _, _ string) error {
			return services.ErrUserHasOwnedSurveys
		},
	}
	caller := &models.User{ID: "33333333-3333-3333-3333-333333333333", Role: "super_admin"}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/22222222-2222-2222-2222-222222222222", nil)
	req = withChiParam(req, "userID", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()

	serveWithCaller(handlers.DeleteUser(svc), caller, req, rec)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteUserHandler_Success(t *testing.T) {
	svc := &fakeUserSvc{
		deleteFn: func(_ context.Context, _, _ string) error {
			return nil
		},
	}
	caller := &models.User{ID: "33333333-3333-3333-3333-333333333333", Role: "super_admin"}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/22222222-2222-2222-2222-222222222222", nil)
	req = withChiParam(req, "userID", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()

	serveWithCaller(handlers.DeleteUser(svc), caller, req, rec)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- ResetUserPassword ---

func TestResetUserPasswordHandler_Success(t *testing.T) {
	svc := &fakeUserSvc{
		resetPasswdFn: func(_ context.Context, _, _ string) error {
			return nil
		},
	}
	body := `{"new_password":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/22222222-2222-2222-2222-222222222222/password", strings.NewReader(body))
	req = withChiParam(req, "userID", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()

	handlers.ResetUserPassword(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetUserPasswordHandler_NotFound(t *testing.T) {
	svc := &fakeUserSvc{
		resetPasswdFn: func(_ context.Context, _, _ string) error {
			return services.ErrUserNotFound
		},
	}
	body := `{"new_password":"newpassword123"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/44444444-4444-4444-4444-444444444444/password", strings.NewReader(body))
	req = withChiParam(req, "userID", "44444444-4444-4444-4444-444444444444")
	rec := httptest.NewRecorder()

	handlers.ResetUserPassword(svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

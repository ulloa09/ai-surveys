package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// --- fakes -----------------------------------------------------------------

type fakeDetailSvc struct {
	detail     *services.ResponseDetail
	survey     *models.Survey
	response   *models.Response
	detailErr  error
	surveyErr  error
	abandonFn  func(ctx context.Context, responseID string) error
	myResponse *models.Response
	myResErr   error
}

func (f *fakeDetailSvc) GetResponseDetail(_ context.Context, _ string) (*services.ResponseDetail, *models.Survey, error) {
	if f.detailErr != nil {
		return nil, nil, f.detailErr
	}
	return f.detail, f.survey, nil
}

func (f *fakeDetailSvc) GetResponseSurvey(_ context.Context, _ string) (*models.Response, *models.Survey, error) {
	if f.surveyErr != nil {
		return nil, nil, f.surveyErr
	}
	return f.response, f.survey, nil
}

func (f *fakeDetailSvc) AbandonResponse(ctx context.Context, responseID string) error {
	if f.abandonFn != nil {
		return f.abandonFn(ctx, responseID)
	}
	return nil
}

func (f *fakeDetailSvc) GetMyResponse(_ context.Context, _, _ string) (*models.Response, error) {
	return f.myResponse, f.myResErr
}

type fakeTeamRole struct {
	role string
	err  error
}

func (f *fakeTeamRole) GetMemberRole(_ context.Context, _, _ string) (string, error) {
	return f.role, f.err
}

var (
	_ handlers.ResponseDetailServicer = (*fakeDetailSvc)(nil)
	_ handlers.MyResponseServicer     = (*fakeDetailSvc)(nil)
	_ handlers.TeamRoleReader         = (*fakeTeamRole)(nil)
)

// serveWithUser envuelve el handler con la autenticación real para inyectar al
// usuario en el context tal como lo hace el router en producción.
func serveWithUser(handler http.HandlerFunc, user *models.User) *httptest.ResponseRecorder {
	wrapped := middleware.Authenticate(&fakeSessionValidator{user: user})(handler)
	req := httptest.NewRequest(http.MethodGet, "/api/responses/x", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "tok"})
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	return rec
}

func strptr(s string) *string { return &s }

// --- GetResponseDetail: control de acceso ----------------------------------

func TestGetResponseDetail_OwnerAllowed(t *testing.T) {
	svc := &fakeDetailSvc{
		detail: &services.ResponseDetail{Status: "in_progress"},
		survey: &models.Survey{OwnerID: "owner-x"},
	}
	svc.detail.Response = &models.Response{ID: "r1", UserID: strptr("u1")}

	rec := serveWithUser(handlers.GetResponseDetail(svc, &fakeTeamRole{err: services.ErrResponseNotFound}),
		&models.User{ID: "u1", Role: "respondent"})

	if rec.Code != http.StatusOK {
		t.Fatalf("owner should get 200, got %d", rec.Code)
	}
}

func TestGetResponseDetail_StrangerForbidden(t *testing.T) {
	svc := &fakeDetailSvc{
		detail: &services.ResponseDetail{Status: "in_progress", Response: &models.Response{ID: "r1", UserID: strptr("u1")}},
		survey: &models.Survey{OwnerID: "owner-x", TeamID: "team-1"},
	}
	// El usuario no es dueño y no pertenece al equipo de la encuesta.
	rec := serveWithUser(handlers.GetResponseDetail(svc, &fakeTeamRole{err: errors.New("not a member")}),
		&models.User{ID: "stranger", Role: "respondent"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger should get 403, got %d", rec.Code)
	}
}

func TestGetResponseDetail_SuperAdminAllowed(t *testing.T) {
	svc := &fakeDetailSvc{
		detail: &services.ResponseDetail{Status: "in_progress", Response: &models.Response{ID: "r1", UserID: strptr("u1")}},
		survey: &models.Survey{OwnerID: "owner-x"},
	}
	rec := serveWithUser(handlers.GetResponseDetail(svc, &fakeTeamRole{err: errors.New("n/a")}),
		&models.User{ID: "admin", Role: "super_admin"})

	if rec.Code != http.StatusOK {
		t.Fatalf("super_admin should get 200, got %d", rec.Code)
	}
}

func TestGetResponseDetail_TeamProfesorAllowed(t *testing.T) {
	svc := &fakeDetailSvc{
		detail: &services.ResponseDetail{Status: "in_progress", Response: &models.Response{ID: "r1", UserID: strptr("u1")}},
		survey: &models.Survey{OwnerID: "owner-x", TeamID: "team-1"},
	}
	rec := serveWithUser(handlers.GetResponseDetail(svc, &fakeTeamRole{role: "profesor"}),
		&models.User{ID: "profesor-y", Role: "profesor"})

	if rec.Code != http.StatusOK {
		t.Fatalf("team profesor should get 200, got %d", rec.Code)
	}
}

func TestGetResponseDetail_TeamAlumnoForbidden(t *testing.T) {
	svc := &fakeDetailSvc{
		detail: &services.ResponseDetail{Status: "in_progress", Response: &models.Response{ID: "r1", UserID: strptr("u1")}},
		survey: &models.Survey{OwnerID: "owner-x", TeamID: "team-1"},
	}
	rec := serveWithUser(handlers.GetResponseDetail(svc, &fakeTeamRole{role: "alumno"}),
		&models.User{ID: "alumno-z", Role: "alumno"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("team alumno should get 403, got %d", rec.Code)
	}
}

func TestGetResponseDetail_NotFound(t *testing.T) {
	svc := &fakeDetailSvc{detailErr: services.ErrResponseNotFound}
	rec := serveWithUser(handlers.GetResponseDetail(svc, &fakeTeamRole{}),
		&models.User{ID: "u1", Role: "super_admin"})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing response should get 404, got %d", rec.Code)
	}
}

// --- AbandonResponse -------------------------------------------------------

func TestAbandonResponse_OwnerAllowed(t *testing.T) {
	called := false
	svc := &fakeDetailSvc{
		response:  &models.Response{ID: "r1", UserID: strptr("u1"), Status: "in_progress"},
		survey:    &models.Survey{OwnerID: "owner-x"},
		abandonFn: func(context.Context, string) error { called = true; return nil },
	}
	rec := serveWithUser(handlers.AbandonResponse(svc, &fakeTeamRole{err: errors.New("n/a")}),
		&models.User{ID: "u1", Role: "respondent"})

	if rec.Code != http.StatusOK {
		t.Fatalf("owner abandon should get 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("AbandonResponse was not invoked")
	}
}

func TestAbandonResponse_StrangerForbidden(t *testing.T) {
	called := false
	svc := &fakeDetailSvc{
		response:  &models.Response{ID: "r1", UserID: strptr("u1"), Status: "in_progress"},
		survey:    &models.Survey{OwnerID: "owner-x", TeamID: "team-1"},
		abandonFn: func(context.Context, string) error { called = true; return nil },
	}
	rec := serveWithUser(handlers.AbandonResponse(svc, &fakeTeamRole{err: errors.New("not a member")}),
		&models.User{ID: "stranger", Role: "respondent"})

	if rec.Code != http.StatusForbidden {
		t.Fatalf("stranger abandon should get 403, got %d", rec.Code)
	}
	if called {
		t.Fatal("AbandonResponse must not run for an unauthorized user")
	}
}

// --- GetMyResponse ---------------------------------------------------------

func TestGetMyResponse_ReturnsResponse(t *testing.T) {
	svc := &fakeDetailSvc{myResponse: &models.Response{ID: "r1", Status: "in_progress"}}
	rec := serveWithUser(handlers.GetMyResponse(svc), &models.User{ID: "u1", Role: "respondent"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Response *models.Response `json:"response"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Response == nil || body.Response.ID != "r1" {
		t.Fatalf("expected response r1, got %+v", body.Response)
	}
}

func TestGetMyResponse_NullWhenNone(t *testing.T) {
	svc := &fakeDetailSvc{myResponse: nil}
	rec := serveWithUser(handlers.GetMyResponse(svc), &models.User{ID: "u1", Role: "respondent"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Response *models.Response `json:"response"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Response != nil {
		t.Fatalf("expected null response, got %+v", body.Response)
	}
}

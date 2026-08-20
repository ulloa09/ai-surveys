package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/fingerprint"
	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

type fakeResponseSvc struct {
	getPublicSurveyFn  func(ctx context.Context, token string) (*services.PublicSurvey, error)
	createFn           func(ctx context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error)
	getByResumeTokenFn func(ctx context.Context, publicToken, resumeToken string) (*models.Response, error)
	isTeamMemberFn     func(ctx context.Context, teamID, userID string) (bool, error)
}

func (f *fakeResponseSvc) GetPublicSurvey(ctx context.Context, token string) (*services.PublicSurvey, error) {
	return f.getPublicSurveyFn(ctx, token)
}

func (f *fakeResponseSvc) Create(ctx context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
	return f.createFn(ctx, in)
}

func (f *fakeResponseSvc) GetByResumeToken(ctx context.Context, publicToken, resumeToken string) (*models.Response, error) {
	return f.getByResumeTokenFn(ctx, publicToken, resumeToken)
}

// IsTeamMember por defecto (isTeamMemberFn == nil) responde "sí es miembro" —
// así los tests existentes, centrados en otro comportamiento, no necesitan
// configurar este stub para seguir pasando por el chequeo de acceso.
func (f *fakeResponseSvc) IsTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	if f.isTeamMemberFn != nil {
		return f.isTeamMemberFn(ctx, teamID, userID)
	}
	return true, nil
}

var _ handlers.ResponseServicer = (*fakeResponseSvc)(nil)

// testSalt es la sal del HMAC de dispositivo en los tests. Su valor da igual;
// lo que importa es que el handler la use y no filtre el device_id en claro.
const testSalt = "test-fingerprint-salt"

// respondentUser es el usuario autenticado "de prueba" para los tests de
// GetPublicSurvey/CreateResponse — ambos ahora requieren sesión.
func respondentUser() *models.User {
	return &models.User{ID: "alumno-1", Email: "alumno@test.com", DisplayName: "Alumno", Role: "alumno"}
}

// okResult envuelve una respuesta en el CreateResponseResult que el servicio
// real devuelve, para no repetir el wrapping en cada test.
func okResult(r *models.Response) *services.CreateResponseResult {
	return &services.CreateResponseResult{Response: r}
}

func servePublic(handler http.Handler, req *http.Request, params map[string]string) *httptest.ResponseRecorder {
	if len(params) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestCreateResponse_SendsSelectedLanguageToService(t *testing.T) {
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			if in.Language != "en" {
				t.Fatalf("expected selected language en, got %q", in.Language)
			}
			return okResult(&models.Response{
				ID:                 "r1",
				SurveyID: "s1",
				Status:             "in_progress",
				Language:           in.Language,
				StartedAt:          time.Now(),
			}), nil
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"language":"en"}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"language":"en"`) {
		t.Fatalf("expected response language en, got %s", rec.Body.String())
	}
}

func TestCreateResponse_DefaultLanguageCanBeAppliedByService(t *testing.T) {
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			if in.Language != "" {
				t.Fatalf("expected empty language when respondent omitted it, got %q", in.Language)
			}
			return okResult(&models.Response{
				ID:                 "r1",
				SurveyID: "s1",
				Status:             "in_progress",
				Language:           "es",
				StartedAt:          time.Now(),
			}), nil
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"language":"es"`) {
		t.Fatalf("expected response language es, got %s", rec.Body.String())
	}
}

func TestCreateResponse_InvalidLanguageReturnsBadRequest(t *testing.T) {
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, _ services.CreateResponseInput) (*services.CreateResponseResult, error) {
			return nil, services.ErrInvalidResponseLanguage
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"language":"fr"}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetPublicSurvey_NotFoundReturnsNotFound(t *testing.T) {
	svc := &fakeResponseSvc{
		getPublicSurveyFn: func(_ context.Context, token string) (*services.PublicSurvey, error) {
			if token != "undefined" {
				t.Fatalf("expected token undefined, got %q", token)
			}
			return nil, services.ErrResponseSurveyNotFound
		},
	}

	rec := serveAuthed(
		handlers.GetPublicSurvey(svc),
		jsonReq(http.MethodGet, "/api/public/surveys/undefined", ""),
		respondentUser(),
		map[string]string{"token": "undefined"},
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestResumeResponse_ValidTokenReturnsResponse(t *testing.T) {
	svc := &fakeResponseSvc{
		getByResumeTokenFn: func(_ context.Context, publicToken, resumeToken string) (*models.Response, error) {
			if publicToken != "surv" || resumeToken != "tok" {
				t.Fatalf("unexpected args: public=%q resume=%q", publicToken, resumeToken)
			}
			return &models.Response{ID: "r1", SurveyID: "s1", Status: "in_progress", Language: "es", StartedAt: time.Now()}, nil
		},
	}

	rec := servePublic(
		handlers.ResumeResponse(svc),
		jsonReq(http.MethodGet, "/api/public/surveys/surv/resume?token=tok", ""),
		map[string]string{"token": "surv"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"r1"`) {
		t.Fatalf("expected resumed response id, got %s", rec.Body.String())
	}
}

func TestResumeResponse_UnknownTokenReturnsNotFound(t *testing.T) {
	svc := &fakeResponseSvc{
		getByResumeTokenFn: func(_ context.Context, _, _ string) (*models.Response, error) {
			return nil, services.ErrResumeTokenNotFound
		},
	}

	rec := servePublic(
		handlers.ResumeResponse(svc),
		jsonReq(http.MethodGet, "/api/public/surveys/surv/resume?token=bad", ""),
		map[string]string{"token": "surv"},
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- Auth & team-membership gating ---

// Una encuesta que NO es anónima sigue exigiendo login. El token se resuelve
// primero (es la única forma de conocer su anonymity_level) y recién ahí se
// rechaza a quien llega sin sesión.
func TestGetPublicSurvey_UnauthenticatedNonAnonymousReturnsUnauthorized(t *testing.T) {
	svc := &fakeResponseSvc{
		getPublicSurveyFn: func(context.Context, string) (*services.PublicSurvey, error) {
			return &services.PublicSurvey{ID: "s1", TeamID: "a1", Status: "open", AnonymityLevel: "partial"}, nil
		},
		isTeamMemberFn: func(context.Context, string, string) (bool, error) {
			t.Fatal("no se debe verificar el roster sin sesión")
			return false, nil
		},
	}

	rec := servePublic(
		handlers.GetPublicSurvey(svc),
		jsonReq(http.MethodGet, "/api/public/surveys/tok", ""),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// Una encuesta anónima ('full') es un link público de verdad: se abre sin cuenta
// y sin roster que verificar — no hay identidad con la cual hacerlo.
func TestGetPublicSurvey_AnonymousSurveyNeedsNoSession(t *testing.T) {
	svc := &fakeResponseSvc{
		getPublicSurveyFn: func(context.Context, string) (*services.PublicSurvey, error) {
			return &services.PublicSurvey{ID: "s1", TeamID: "a1", Status: "open", AnonymityLevel: "full"}, nil
		},
		isTeamMemberFn: func(context.Context, string, string) (bool, error) {
			t.Fatal("una encuesta anónima no verifica membresía de equipo")
			return false, nil
		},
	}

	rec := servePublic(
		handlers.GetPublicSurvey(svc),
		jsonReq(http.MethodGet, "/api/public/surveys/tok", ""),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetPublicSurvey_NonMemberReturnsForbidden(t *testing.T) {
	svc := &fakeResponseSvc{
		getPublicSurveyFn: func(context.Context, string) (*services.PublicSurvey, error) {
			return &services.PublicSurvey{ID: "s1", TeamID: "a1", Status: "open"}, nil
		},
		isTeamMemberFn: func(context.Context, string, string) (bool, error) { return false, nil },
	}

	rec := serveAuthed(
		handlers.GetPublicSurvey(svc),
		jsonReq(http.MethodGet, "/api/public/surveys/tok", ""),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Sin sesión el handler ya no corta por su cuenta: delega en el servicio, que es
// quien conoce el anonymity_level de la encuesta. Le pasa UserID=nil y, si esa
// encuesta sí exigía login, traduce su ErrAuthRequired a un 401.
func TestCreateResponse_UnauthenticatedDelegatesAndMapsAuthRequired(t *testing.T) {
	var called bool
	var gotUserID *string
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			called, gotUserID = true, in.UserID
			return nil, services.ErrAuthRequired
		},
	}

	rec := servePublic(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{}`),
		map[string]string{"token": "tok"},
	)
	if !called {
		t.Fatal("el handler debe delegar en el servicio: es quien sabe si la encuesta admite respondientes anónimos")
	}
	if gotUserID != nil {
		t.Fatalf("user_id = %v, want nil sin sesión", gotUserID)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// En una encuesta anónima el servicio acepta la respuesta sin sesión, y el
// handler la crea igual: 201 con user_id nil.
func TestCreateResponse_AnonymousSurveyAcceptsUnauthenticated(t *testing.T) {
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			if in.UserID != nil {
				t.Fatalf("user_id = %v, want nil en una encuesta anónima", *in.UserID)
			}
			return okResult(&models.Response{
				ID: "r1", SurveyID: "s1", Status: "in_progress",
				Language: "es", StartedAt: time.Now(),
			}), nil
		},
	}

	rec := servePublic(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"language":"es"}`),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// El user_id SIEMPRE sale de la sesión (context), nunca del body — la petición
// ni siquiera acepta un campo user_id (decodeJSON rechaza campos desconocidos),
// así que no hay forma de que el cliente le diga al backend de quién es la
// respuesta: siempre es de quien está autenticado.
func TestCreateResponse_UserIDAlwaysComesFromSession(t *testing.T) {
	var gotUserID *string
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			gotUserID = in.UserID
			return okResult(&models.Response{ID: "r1", SurveyID: "s1", Status: "in_progress", Language: "es", StartedAt: time.Now()}), nil
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"language":"es"}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if gotUserID == nil || *gotUserID != respondentUser().ID {
		t.Fatalf("user_id = %v, want the session user's id (%q)", gotUserID, respondentUser().ID)
	}

	// Confirmación estructural: un body que intenta colar user_id ni siquiera
	// decodifica — el tipo del handler no tiene ese campo.
	rec2 := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"user_id":"someone-else"}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown user_id field in the body, got %d", rec2.Code)
	}
}

func TestCreateResponse_NonMemberPropagatesForbidden(t *testing.T) {
	svc := &fakeResponseSvc{
		createFn: func(context.Context, services.CreateResponseInput) (*services.CreateResponseResult, error) {
			return nil, services.ErrNotTeamMember
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Huella de dispositivo (prevención de duplicados, #09) ---

// El handler traduce la cookie device_id a su HMAC antes de pasarla al servicio:
// el UUID en claro nunca cruza esa frontera, así que nada lo puede persistir.
func TestCreateResponse_HashesDeviceCookieBeforeReachingService(t *testing.T) {
	const deviceID = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"

	var got *string
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			got = in.DeviceHash
			return okResult(&models.Response{
				ID: "r1", SurveyID: "s1", Status: "in_progress",
				Language: "es", StartedAt: time.Now(),
			}), nil
		},
	}

	req := jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"language":"es"}`)
	req.AddCookie(&http.Cookie{Name: "device_id", Value: deviceID})

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		req,
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if got == nil {
		t.Fatal("DeviceHash = nil, want el HMAC de la cookie device_id")
	}
	if *got == deviceID {
		t.Fatal("el device_id llegó en claro al servicio: debe llegar solo su HMAC")
	}
	if want := fingerprint.Hash(deviceID, testSalt); *got != want {
		t.Fatalf("DeviceHash = %q, want %q", *got, want)
	}
}

// Sin cookie no hay hash. La encuesta se contesta igual (no se bloquea a nadie
// por no tener cookies); esa respuesta simplemente no participa del chequeo de
// duplicados.
func TestCreateResponse_NoDeviceCookieSendsNilHash(t *testing.T) {
	var called bool
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			called = true
			if in.DeviceHash != nil {
				t.Fatalf("DeviceHash = %q, want nil sin cookie device_id", *in.DeviceHash)
			}
			return okResult(&models.Response{
				ID: "r1", SurveyID: "s1", Status: "in_progress",
				Language: "es", StartedAt: time.Now(),
			}), nil
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/tok/responses", `{"language":"es"}`),
		respondentUser(),
		map[string]string{"token": "tok"},
	)
	if !called {
		t.Fatal("el servicio no fue llamado: la falta de cookie no debe bloquear la respuesta")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateResponse_InvalidTokenReturnsNotFound(t *testing.T) {
	svc := &fakeResponseSvc{
		createFn: func(_ context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error) {
			if in.PublicToken != "undefined" {
				t.Fatalf("expected token undefined, got %q", in.PublicToken)
			}
			return nil, services.ErrResponseSurveyNotFound
		},
	}

	rec := serveAuthed(
		handlers.CreateResponse(svc, testSalt),
		jsonReq(http.MethodPost, "/api/public/surveys/undefined/responses", `{}`),
		respondentUser(),
		map[string]string{"token": "undefined"},
	)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

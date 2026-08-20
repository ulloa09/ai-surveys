package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

type fakeEngineSvc struct {
	processTurnFn func(ctx context.Context, responseID, userMessage string) (*services.TurnResult, error)
	submitFn      func(ctx context.Context, responseID string) error
	expireFn      func(ctx context.Context, responseID string) (string, error)
}

func (f *fakeEngineSvc) ProcessTurn(ctx context.Context, responseID, userMessage string) (*services.TurnResult, error) {
	return f.processTurnFn(ctx, responseID, userMessage)
}

func (f *fakeEngineSvc) Submit(ctx context.Context, responseID string) error {
	return f.submitFn(ctx, responseID)
}

func (f *fakeEngineSvc) ExpireResponse(ctx context.Context, responseID string) (string, error) {
	if f.expireFn == nil {
		return "pending_submission", nil
	}
	return f.expireFn(ctx, responseID)
}

type fakeAnswerSvc struct {
	recordAnswerDirectFn func(ctx context.Context, responseID, questionID, value string) error
}

func (f *fakeAnswerSvc) GetAnsweredQuestionIDs(ctx context.Context, responseID string) ([]string, error) {
	return nil, nil
}

func (f *fakeAnswerSvc) GetAnswersWithValues(ctx context.Context, responseID string) ([]map[string]string, error) {
	return nil, nil
}

func (f *fakeAnswerSvc) RecordAnswerDirect(ctx context.Context, responseID, questionID, value string) error {
	return f.recordAnswerDirectFn(ctx, responseID, questionID, value)
}

var _ handlers.EngineServicer = (*fakeEngineSvc)(nil)
var _ handlers.AnswerServicer = (*fakeAnswerSvc)(nil)

const testResponseID = "44444444-4444-4444-4444-444444444444"

// closedStatusCh construye el canal de estado final que espera el handler SSE.
func closedStatusCh(status string) <-chan string {
	ch := make(chan string, 1)
	ch <- status
	close(ch)
	return ch
}

func TestSubmitResponse_RequiredQuestionsMissingReturns422WithIDs(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			ch := make(chan ai.TurnChunk)
			close(ch)
			return &services.TurnResult{TokenCh: ch, FinalStatus: closedStatusCh("in_progress")}, nil
		},
		submitFn: func(_ context.Context, responseID string) error {
			if responseID != testResponseID {
				t.Fatalf("expected response id %q, got %q", testResponseID, responseID)
			}
			return &services.MissingRequiredQuestionsError{Missing: []string{"q-open"}}
		},
	}

	rec := servePublic(
		handlers.SubmitResponse(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/submit", `{}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"missing_required_questions":["q-open"]`) {
		t.Fatalf("expected missing question id in response, got %s", rec.Body.String())
	}
}

func TestSubmitResponse_AlreadySubmittedReturnsConflict(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			ch := make(chan ai.TurnChunk)
			close(ch)
			return &services.TurnResult{TokenCh: ch, FinalStatus: closedStatusCh("in_progress")}, nil
		},
		submitFn: func(context.Context, string) error {
			return services.ErrAlreadySubmitted
		},
	}

	rec := servePublic(
		handlers.SubmitResponse(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/submit", `{}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExpireResponse_OKReturns200(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			ch := make(chan ai.TurnChunk)
			close(ch)
			return &services.TurnResult{TokenCh: ch, FinalStatus: closedStatusCh("in_progress")}, nil
		},
		submitFn: func(context.Context, string) error { return nil },
		expireFn: func(_ context.Context, responseID string) (string, error) {
			if responseID != testResponseID {
				t.Fatalf("expected response id %q, got %q", testResponseID, responseID)
			}
			return "pending_submission", nil
		},
	}

	rec := servePublic(
		handlers.ExpireResponse(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/expire", `{}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExpireResponse_NotFoundReturns404(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			ch := make(chan ai.TurnChunk)
			close(ch)
			return &services.TurnResult{TokenCh: ch, FinalStatus: closedStatusCh("in_progress")}, nil
		},
		submitFn: func(context.Context, string) error { return nil },
		expireFn: func(context.Context, string) (string, error) { return "", services.ErrResponseNotFound },
	}

	rec := servePublic(
		handlers.ExpireResponse(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/expire", `{}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRecordAnswer_InvalidStructuredAnswerReturns422(t *testing.T) {
	svc := &fakeAnswerSvc{
		recordAnswerDirectFn: func(_ context.Context, responseID, questionID, value string) error {
			if responseID != testResponseID || questionID != testQuestionID || value != "999" {
				t.Fatalf("unexpected args: response=%q question=%q value=%q", responseID, questionID, value)
			}
			return services.ErrInvalidAnswer
		},
	}

	rec := servePublic(
		handlers.RecordAnswer(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/answers", `{"question_id":"`+testQuestionID+`","value":"999"}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRecordAnswer_MalformedBodyReturns400(t *testing.T) {
	svc := &fakeAnswerSvc{
		recordAnswerDirectFn: func(context.Context, string, string, string) error {
			t.Fatal("service should not be called for malformed input")
			return nil
		},
	}

	rec := servePublic(
		handlers.RecordAnswer(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/answers", `{"question_id":"`+testQuestionID+`","value":999}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessTurn_ResponseNotFoundReturns404(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			return nil, services.ErrResponseNotFound
		},
		submitFn: func(context.Context, string) error { return nil },
	}

	rec := servePublic(
		handlers.ProcessTurn(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessTurn_SubmittedResponseReturnsConflict(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			return nil, services.ErrResponseNotInProgress
		},
		submitFn: func(context.Context, string) error { return nil },
	}

	rec := servePublic(
		handlers.ProcessTurn(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProcessTurn_AIProviderUnavailableReturns503(t *testing.T) {
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			return nil, services.ErrAIProviderUnavailable
		},
		submitFn: func(context.Context, string) error { return nil },
	}

	rec := servePublic(
		handlers.ProcessTurn(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AI provider unavailable") {
		t.Fatalf("expected generic provider error, got %s", rec.Body.String())
	}
}

func TestProcessTurn_InternalErrorDoesNotLeakRawProviderBody(t *testing.T) {
	raw := "openai 401 {\"error\":\"bad key sk-secret\"}"
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			return nil, fmt.Errorf("%s", raw)
		},
		submitFn: func(context.Context, string) error { return nil },
	}

	rec := servePublic(
		handlers.ProcessTurn(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-secret") || strings.Contains(rec.Body.String(), "openai 401") {
		t.Fatalf("raw provider body leaked: %s", rec.Body.String())
	}
}

func TestProcessTurn_StreamErrorReturnsGenericSSEError(t *testing.T) {
	ch := make(chan ai.TurnChunk, 1)
	ch <- ai.TurnChunk{Error: fmt.Errorf("claude 500 body with sk-secret")}
	close(ch)
	svc := &fakeEngineSvc{
		processTurnFn: func(context.Context, string, string) (*services.TurnResult, error) {
			return &services.TurnResult{TokenCh: ch, FinalStatus: closedStatusCh("in_progress")}, nil
		},
		submitFn: func(context.Context, string) error { return nil },
	}

	rec := servePublic(
		handlers.ProcessTurn(svc),
		jsonReq(http.MethodPost, "/api/responses/"+testResponseID+"/turns", `{"message":"hola"}`),
		map[string]string{"id": testResponseID},
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected SSE 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AI provider unavailable") {
		t.Fatalf("expected generic SSE error, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sk-secret") || strings.Contains(rec.Body.String(), "claude 500") {
		t.Fatalf("raw stream error leaked: %s", rec.Body.String())
	}
}

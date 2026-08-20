package services

import (
	"context"
	"errors"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

func TestSubmitCompleteness_RequiredOpenEndedMissingFails(t *testing.T) {
	questions := []models.Question{
		{ID: "q-open", Type: "open_ended", Required: true},
	}

	missing := MissingRequiredQuestions(questions, map[string]bool{})

	if len(missing) != 1 || missing[0] != "q-open" {
		t.Fatalf("expected q-open missing, got %#v", missing)
	}
}

func TestSubmitCompleteness_RequiredStructuredMissingFails(t *testing.T) {
	questions := []models.Question{
		{ID: "q-scale", Type: "linear_scale", Required: true},
	}

	missing := MissingRequiredQuestions(questions, map[string]bool{})

	if len(missing) != 1 || missing[0] != "q-scale" {
		t.Fatalf("expected q-scale missing, got %#v", missing)
	}
}

func TestSubmitCompleteness_AllRequiredAnsweredPasses(t *testing.T) {
	questions := []models.Question{
		{ID: "q-open", Type: "open_ended", Required: true},
		{ID: "q-scale", Type: "linear_scale", Required: true},
	}
	coverage := map[string]bool{"q-open": true, "q-scale": true}

	missing := MissingRequiredQuestions(questions, coverage)

	if len(missing) != 0 {
		t.Fatalf("expected no missing questions, got %#v", missing)
	}
}

func TestSubmitCompleteness_OptionalMissingPasses(t *testing.T) {
	questions := []models.Question{
		{ID: "q-required", Type: "open_ended", Required: true},
		{ID: "q-optional", Type: "linear_scale", Required: false},
	}
	coverage := map[string]bool{"q-required": true}

	missing := MissingRequiredQuestions(questions, coverage)

	if len(missing) != 0 {
		t.Fatalf("expected optional missing to pass, got %#v", missing)
	}
}

func TestMissingRequiredQuestionsErrorWrapsDomainError(t *testing.T) {
	err := &MissingRequiredQuestionsError{Missing: []string{"q1"}}

	if !errors.Is(err, ErrRequiredQuestionsOpen) {
		t.Fatalf("expected ErrRequiredQuestionsOpen wrapping, got %v", err)
	}
}

func TestValidateStructuredAnswer_LinearScaleOutOfRangeFails(t *testing.T) {
	q := models.Question{Type: "linear_scale", Options: []byte(`{"min":1,"max":5}`)}

	_, err := ValidateStructuredAnswer("999", q)

	if !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("expected ErrInvalidAnswer, got %v", err)
	}
}

func TestValidateStructuredAnswer_LinearScaleWithinRangePasses(t *testing.T) {
	q := models.Question{Type: "linear_scale", Options: []byte(`{"min":1,"max":5}`)}

	value, err := ValidateStructuredAnswer("5", q)

	if err != nil {
		t.Fatalf("expected valid scale answer, got %v", err)
	}
	if value != "5" {
		t.Fatalf("expected normalized value 5, got %q", value)
	}
}

func TestValidateStructuredAnswer_SingleChoiceOutsideOptionsFails(t *testing.T) {
	q := models.Question{Type: "single_choice", Options: []byte(`{"choices":[{"label":"A","value":"a"}]}`)}

	_, err := ValidateStructuredAnswer("z", q)

	if !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("expected ErrInvalidAnswer, got %v", err)
	}
}

func TestValidateStructuredAnswer_MultiChoiceOneInvalidOptionFails(t *testing.T) {
	q := models.Question{Type: "multi_choice", Options: []byte(`{"choices":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}`)}

	_, err := ValidateStructuredAnswer("a, z", q)

	if !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("expected ErrInvalidAnswer, got %v", err)
	}
}

func TestValidateStructuredAnswer_OpenEndedViaStructuredEndpointFails(t *testing.T) {
	q := models.Question{Type: "open_ended"}

	_, err := ValidateStructuredAnswer("text", q)

	if !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("expected ErrInvalidAnswer, got %v", err)
	}
}

func TestGetQuestionForAnswer_InvalidQuestionIDFailsBeforeDB(t *testing.T) {
	svc := NewEngineService(nil, nil, nil)

	_, err := svc.getQuestionForAnswer(context.Background(), "survey-id", "not-a-uuid")

	if !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("expected ErrInvalidAnswer, got %v", err)
	}
}

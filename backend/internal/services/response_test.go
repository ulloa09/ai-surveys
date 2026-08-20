package services

import (
	"context"
	"errors"
	"testing"
)

func TestResolveResponseLanguage_UsesSelectedLanguage(t *testing.T) {
	language, err := ResolveResponseLanguage(PublicSurvey{
		AvailableLanguages: []string{"es", "en"},
		DefaultLanguage:    "es",
	}, "en")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if language != "en" {
		t.Fatalf("expected en, got %q", language)
	}
}

func TestResolveResponseLanguage_FallsBackToSurveyDefault(t *testing.T) {
	language, err := ResolveResponseLanguage(PublicSurvey{
		AvailableLanguages: []string{"es", "en"},
		DefaultLanguage:    "en",
	}, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if language != "en" {
		t.Fatalf("expected en default, got %q", language)
	}
}

func TestResolveResponseLanguage_RejectsUnavailableLanguage(t *testing.T) {
	_, err := ResolveResponseLanguage(PublicSurvey{
		AvailableLanguages: []string{"es"},
		DefaultLanguage:    "es",
	}, "en")
	if !errors.Is(err, ErrInvalidResponseLanguage) {
		t.Fatalf("expected ErrInvalidResponseLanguage, got %v", err)
	}
}

func TestGetPublicSurvey_InvalidTokenReturnsNotFound(t *testing.T) {
	svc := NewResponseService(nil)
	_, err := svc.GetPublicSurvey(context.Background(), "undefined")
	if !errors.Is(err, ErrResponseSurveyNotFound) {
		t.Fatalf("expected ErrResponseSurveyNotFound, got %v", err)
	}
}

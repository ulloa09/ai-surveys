package services

import (
	"errors"
	"testing"
)

func languages(languages ...string) *[]string {
	return &languages
}

func TestNormalizeLanguageConfig_DefaultsToSpanishOnly(t *testing.T) {
	available, def, err := normalizeLanguageConfig(nil, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(available) != 1 || available[0] != "es" {
		t.Fatalf("expected [es], got %#v", available)
	}
	if def != "es" {
		t.Fatalf("expected default es, got %q", def)
	}
}

func TestNormalizeLanguageConfig_AllowsEnglishWhenEnabled(t *testing.T) {
	available, def, err := normalizeLanguageConfig(languages("es", "en"), "es")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(available) != 2 || available[0] != "es" || available[1] != "en" {
		t.Fatalf("expected [es en], got %#v", available)
	}
	if def != "es" {
		t.Fatalf("expected default es, got %q", def)
	}
}

func TestNormalizeLanguageConfig_AllowsEnglishDefaultWhenEnabled(t *testing.T) {
	_, def, err := normalizeLanguageConfig(languages("es", "en"), "en")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if def != "en" {
		t.Fatalf("expected default en, got %q", def)
	}
}

func TestNormalizeLanguageConfig_RejectsEmptyLanguages(t *testing.T) {
	_, _, err := normalizeLanguageConfig(languages(), "es")
	if !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("expected ErrInvalidLanguage, got %v", err)
	}
}

func TestNormalizeLanguageConfig_RejectsDefaultOutsideAvailableLanguages(t *testing.T) {
	_, _, err := normalizeLanguageConfig(languages("es"), "en")
	if !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("expected ErrInvalidLanguage, got %v", err)
	}
}

func TestValidateSurveyLanguage_RejectsUnavailableLanguage(t *testing.T) {
	err := ValidateSurveyLanguage([]string{"es"}, "en")
	if !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("expected ErrInvalidLanguage, got %v", err)
	}
}

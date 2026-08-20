package ai

import (
	"strings"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

func TestBuildSystemPromptIncludesSelectedLanguageInstruction(t *testing.T) {
	prompt := "Eres un entrevistador universitario."
	got := BuildSystemPrompt(TurnContext{
		Survey:   models.Survey{SystemPrompt: &prompt},
		Response: models.Response{Language: "en"},
	})

	if !strings.Contains(got, prompt) {
		t.Fatalf("expected admin prompt in system prompt, got %q", got)
	}
	want := "Respondent selected language: English (en). Respond in this language."
	if !strings.Contains(got, want) {
		t.Fatalf("expected language instruction %q, got %q", want, got)
	}
}

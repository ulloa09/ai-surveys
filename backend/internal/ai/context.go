package ai

import (
	"fmt"
	"strings"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

var languageNames = map[string]string{
	"es": "Spanish",
	"en": "English",
}

type TurnContext struct {
	Survey   models.Survey
	Response models.Response
}

// BuildSystemPrompt ensambla el prompt básico (sin preguntas).
// Se mantiene para compatibilidad con el endpoint de streaming directo.
func BuildSystemPrompt(ctx TurnContext) string {
	var parts []string
	if ctx.Survey.SystemPrompt != nil && strings.TrimSpace(*ctx.Survey.SystemPrompt) != "" {
		parts = append(parts, strings.TrimSpace(*ctx.Survey.SystemPrompt))
	}

	languageName := languageNames[ctx.Response.Language]
	if languageName == "" {
		languageName = ctx.Response.Language
	}
	parts = append(parts, fmt.Sprintf(
		"Respondent selected language: %s (%s). Respond in this language.",
		languageName, ctx.Response.Language,
	))

	return strings.Join(parts, "\n\n")
}

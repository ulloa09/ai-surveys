package services

import (
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

// El modo manda sobre el toggle: en 'form' ("preguntas fijas") no hay repregunta
// aunque la pregunta tenga ai_followup = true, que es justo el caso de una
// encuesta que estuvo en modo conversacional y se pasó a form — las preguntas
// guardadas conservan el toggle prendido.
func TestFollowupEnabled_FormModeNeverFollowsUp(t *testing.T) {
	withFollowup := &models.Question{Type: "open_ended", AIFollowup: true}

	for _, tc := range []struct {
		mode string
		want bool
	}{
		{"form", false},
		{"conversational", true},
		{"prompt_only", true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			survey := &models.Survey{Mode: tc.mode}

			if got := followupsAllowed(survey); got != tc.want {
				t.Fatalf("followupsAllowed(mode=%s) = %v, want %v", tc.mode, got, tc.want)
			}
			if got := followupEnabled(survey, withFollowup); got != tc.want {
				t.Fatalf("followupEnabled(mode=%s, ai_followup=true) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

// Fuera de 'form', el toggle de la pregunta sigue mandando: apagarlo en una
// pregunta puntual debe seguir evitando la repregunta.
func TestFollowupEnabled_ToggleStillRulesInConversationalModes(t *testing.T) {
	survey := &models.Survey{Mode: "conversational"}
	noFollowup := &models.Question{Type: "open_ended", AIFollowup: false}

	if followupEnabled(survey, noFollowup) {
		t.Fatal("ai_followup=false debe seguir desactivando la repregunta en modo conversacional")
	}
}

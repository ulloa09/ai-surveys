package services

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

func q(t, options string) models.Question {
	return models.Question{Type: t, Options: json.RawMessage(options)}
}

func TestValidateStructuredAnswer_AcceptsCanonicalFrontendFormats(t *testing.T) {
	tests := []struct {
		name     string
		question models.Question
		value    string
		want     string
	}{
		{
			name:     "multi_choice JSON array",
			question: q("multi_choice", `{"choices":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}`),
			value:    `["a","b"]`,
			want:     `["a","b"]`,
		},
		{
			name:     "multi_choice legacy comma separated",
			question: q("multi_choice", `{"choices":[{"label":"A","value":"a"},{"label":"B","value":"b"}]}`),
			value:    `a, b`,
			want:     `["a","b"]`,
		},
		{
			name:     "ranking positions JSON",
			question: q("ranking", `{"items":[{"label":"X","value":"x"},{"label":"Y","value":"y"}]}`),
			value:    `[{"position":1,"value":"y"},{"position":2,"value":"x"}]`,
			want:     `[{"position":1,"value":"y"},{"position":2,"value":"x"}]`,
		},
		{
			name:     "matrix JSON object",
			question: q("matrix", `{"rows":["Fila 1","Fila 2"],"columns":["Sí","No"]}`),
			value:    `{"Fila 1":"Sí","Fila 2":"No"}`,
			want:     `{"Fila 1":"Sí","Fila 2":"No"}`,
		},
		{
			name:     "single_choice value",
			question: q("single_choice", `{"choices":[{"label":"A","value":"a"}]}`),
			value:    "a",
			want:     "a",
		},
		{
			name:     "linear_scale in range",
			question: q("linear_scale", `{"min":1,"max":5}`),
			value:    "3",
			want:     "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateStructuredAnswer(tt.value, tt.question)
			if err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
			// Comparar JSON semánticamente cuando aplica.
			var a, b any
			if json.Unmarshal([]byte(got), &a) == nil && json.Unmarshal([]byte(tt.want), &b) == nil {
				aj, _ := json.Marshal(a)
				bj, _ := json.Marshal(b)
				if string(aj) != string(bj) {
					t.Fatalf("normalized = %s, want %s", got, tt.want)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("normalized = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateStructuredAnswer_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		question models.Question
		value    string
	}{
		{"multi_choice unknown option", q("multi_choice", `{"choices":[{"label":"A","value":"a"}]}`), `["z"]`},
		{"ranking missing item", q("ranking", `{"items":[{"label":"X","value":"x"},{"label":"Y","value":"y"}]}`), `[{"position":1,"value":"x"}]`},
		{"ranking duplicate item", q("ranking", `{"items":[{"label":"X","value":"x"},{"label":"Y","value":"y"}]}`), `[{"position":1,"value":"x"},{"position":2,"value":"x"}]`},
		{"matrix missing row", q("matrix", `{"rows":["F1","F2"],"columns":["Sí","No"]}`), `{"F1":"Sí"}`},
		{"matrix invalid column", q("matrix", `{"rows":["F1"],"columns":["Sí","No"]}`), `{"F1":"Tal vez"}`},
		{"scale out of range", q("linear_scale", `{"min":1,"max":5}`), "9"},
		{"open_ended through structured endpoint", q("open_ended", ``), "texto"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidateStructuredAnswer(tt.value, tt.question); !errors.Is(err, ErrInvalidAnswer) {
				t.Fatalf("expected ErrInvalidAnswer, got %v", err)
			}
		})
	}
}

func TestDisplayAnswer_BuildsHumanReadableLabels(t *testing.T) {
	tests := []struct {
		name     string
		question models.Question
		value    string
		language string
		want     string
	}{
		{"single choice label", q("single_choice", `{"choices":[{"label":"Muy bueno","value":"a"}]}`), "a", "es", "Muy bueno"},
		{"true in spanish", q("true_false", ``), "true", "es", "Sí"},
		{"true in english", q("true_false", ``), "true", "en", "Yes"},
		{"multi choice labels", q("multi_choice", `{"choices":[{"label":"Uno","value":"1"},{"label":"Dos","value":"2"}]}`), `["1","2"]`, "es", "Uno, Dos"},
		{"ranking arrows", q("ranking", `{"items":[{"label":"X","value":"x"},{"label":"Y","value":"y"}]}`), `[{"position":1,"value":"y"},{"position":2,"value":"x"}]`, "es", "Y → X"},
		{"matrix rows", q("matrix", `{"rows":["F1","F2"],"columns":["Sí","No"]}`), `{"F1":"Sí","F2":"No"}`, "es", "F1: Sí; F2: No"},
		{"scale passthrough", q("linear_scale", `{"min":1,"max":5}`), "4", "es", "4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayAnswer(tt.question, tt.value, tt.language); got != tt.want {
				t.Fatalf("DisplayAnswer = %q, want %q", got, tt.want)
			}
		})
	}
}

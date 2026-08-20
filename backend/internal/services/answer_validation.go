package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

// ValidateStructuredAnswer valida el valor recibido por POST /responses/{id}/answers.
// Ese endpoint solo acepta preguntas estructuradas; open_ended debe entrar por
// /open-answer para mantener separado el flujo conversacional.
//
// El frontend envía formatos canónicos JSON para los tipos compuestos:
//   - multi_choice: `["a","b"]`
//   - ranking:      `[{"position":1,"value":"x"}, ...]`
//   - matrix:       `{"fila":"columna", ...}`
//
// Se aceptan además formatos de texto plano (coma-separado, "fila: col") como
// fallback. El valor devuelto siempre está normalizado al formato canónico.
func ValidateStructuredAnswer(value string, q models.Question) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%w: value is required", ErrInvalidAnswer)
	}

	switch q.Type {
	case "single_choice":
		return validateDirectSingleChoice(value, q.Options)
	case "multi_choice", "multiple_choice":
		return validateDirectMultiChoice(value, q.Options)
	case "linear_scale", "rating_scale":
		return validateDirectScale(value, q.Options)
	case "true_false", "boolean", "yes_no":
		return validateDirectBoolean(value)
	case "ranking":
		return validateDirectRanking(value, q.Options)
	case "matrix":
		return validateDirectMatrix(value, q.Options)
	case "open_ended":
		return "", fmt.Errorf("%w: open_ended answers must use open-answer endpoint", ErrInvalidAnswer)
	default:
		return "", fmt.Errorf("%w: unsupported question type %q", ErrInvalidAnswer, q.Type)
	}
}

func validateDirectSingleChoice(value string, options json.RawMessage) (string, error) {
	choices, err := directChoices(options)
	if err != nil {
		return "", err
	}
	for _, c := range choices {
		if value == c.Value {
			return value, nil
		}
	}
	return "", fmt.Errorf("%w: invalid single_choice option", ErrInvalidAnswer)
}

func validateDirectMultiChoice(value string, options json.RawMessage) (string, error) {
	choices, err := directChoices(options)
	if err != nil {
		return "", err
	}
	valid := make(map[string]bool, len(choices))
	for _, c := range choices {
		valid[c.Value] = true
	}

	var parts []string
	if strings.HasPrefix(value, "[") {
		// Formato canónico del frontend: JSON array de values.
		if err := json.Unmarshal([]byte(value), &parts); err != nil {
			return "", fmt.Errorf("%w: malformed multi_choice value", ErrInvalidAnswer)
		}
	} else {
		parts = splitChoices(value)
	}

	var selected []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !valid[part] {
			return "", fmt.Errorf("%w: invalid multi_choice option %q", ErrInvalidAnswer, part)
		}
		selected = append(selected, part)
	}
	if len(selected) == 0 {
		return "", fmt.Errorf("%w: multi_choice requires at least one option", ErrInvalidAnswer)
	}
	out, _ := json.Marshal(selected)
	return string(out), nil
}

func validateDirectScale(value string, options json.RawMessage) (string, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return "", fmt.Errorf("%w: scale expects an integer", ErrInvalidAnswer)
	}
	var opts struct {
		Min *int `json:"min"`
		Max *int `json:"max"`
	}
	if len(options) == 0 {
		return "", fmt.Errorf("%w: scale options are required", ErrInvalidAnswer)
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return "", fmt.Errorf("%w: malformed scale options", ErrInvalidAnswer)
	}
	if opts.Min == nil || opts.Max == nil {
		return "", fmt.Errorf("%w: scale min/max are required", ErrInvalidAnswer)
	}
	if n < *opts.Min || n > *opts.Max {
		return "", fmt.Errorf("%w: scale value out of range", ErrInvalidAnswer)
	}
	return strconv.Itoa(n), nil
}

func validateDirectBoolean(value string) (string, error) {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "si", "sí":
		return "true", nil
	case "false", "0", "no":
		return "false", nil
	default:
		return "", fmt.Errorf("%w: boolean expects true or false", ErrInvalidAnswer)
	}
}

// rankedItem es el formato canónico de un elemento de ranking.
type rankedItem struct {
	Position int    `json:"position"`
	Value    string `json:"value"`
}

func validateDirectRanking(value string, options json.RawMessage) (string, error) {
	var opts struct {
		Items []struct {
			Value string `json:"value"`
		} `json:"items"`
	}
	if len(options) == 0 {
		return "", fmt.Errorf("%w: ranking options are required", ErrInvalidAnswer)
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return "", fmt.Errorf("%w: malformed ranking options", ErrInvalidAnswer)
	}
	if len(opts.Items) == 0 {
		return "", fmt.Errorf("%w: ranking options are required", ErrInvalidAnswer)
	}
	valid := make(map[string]bool, len(opts.Items))
	for _, item := range opts.Items {
		valid[item.Value] = true
	}

	var ordered []string
	if strings.HasPrefix(value, "[") {
		// Formato canónico del frontend: [{"position":1,"value":"x"}, ...].
		var items []rankedItem
		if err := json.Unmarshal([]byte(value), &items); err == nil && len(items) > 0 && items[0].Value != "" {
			for _, it := range items {
				ordered = append(ordered, it.Value)
			}
		} else {
			// JSON array plano de values: ["x","y"].
			var plain []string
			if err := json.Unmarshal([]byte(value), &plain); err != nil {
				return "", fmt.Errorf("%w: malformed ranking value", ErrInvalidAnswer)
			}
			ordered = plain
		}
	} else {
		for _, part := range splitChoices(value) {
			part = strings.TrimSpace(part)
			if part != "" {
				ordered = append(ordered, part)
			}
		}
	}

	seen := make(map[string]bool, len(ordered))
	for _, v := range ordered {
		if !valid[v] {
			return "", fmt.Errorf("%w: invalid ranking item %q", ErrInvalidAnswer, v)
		}
		if seen[v] {
			return "", fmt.Errorf("%w: duplicate ranking item %q", ErrInvalidAnswer, v)
		}
		seen[v] = true
	}
	if len(seen) != len(opts.Items) {
		return "", fmt.Errorf("%w: ranking must include every item exactly once", ErrInvalidAnswer)
	}

	canonical := make([]rankedItem, len(ordered))
	for i, v := range ordered {
		canonical[i] = rankedItem{Position: i + 1, Value: v}
	}
	out, _ := json.Marshal(canonical)
	return string(out), nil
}

func validateDirectMatrix(value string, options json.RawMessage) (string, error) {
	var opts struct {
		Rows    []string `json:"rows"`
		Columns []string `json:"columns"`
	}
	if len(options) == 0 {
		return "", fmt.Errorf("%w: matrix options are required", ErrInvalidAnswer)
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return "", fmt.Errorf("%w: malformed matrix options", ErrInvalidAnswer)
	}
	if len(opts.Rows) == 0 || len(opts.Columns) == 0 {
		return "", fmt.Errorf("%w: matrix rows and columns are required", ErrInvalidAnswer)
	}

	validRows := make(map[string]bool, len(opts.Rows))
	validCols := make(map[string]bool, len(opts.Columns))
	for _, row := range opts.Rows {
		validRows[row] = true
	}
	for _, col := range opts.Columns {
		validCols[col] = true
	}

	answers := make(map[string]string, len(opts.Rows))
	if strings.HasPrefix(value, "{") {
		// Formato canónico del frontend: {"fila": "columna", ...}.
		if err := json.Unmarshal([]byte(value), &answers); err != nil {
			return "", fmt.Errorf("%w: malformed matrix value", ErrInvalidAnswer)
		}
	} else {
		for _, line := range strings.Split(value, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			row, col, ok := strings.Cut(line, ":")
			if !ok {
				return "", fmt.Errorf("%w: malformed matrix answer", ErrInvalidAnswer)
			}
			answers[strings.TrimSpace(row)] = strings.TrimSpace(col)
		}
	}

	for row, col := range answers {
		if !validRows[row] || !validCols[col] {
			return "", fmt.Errorf("%w: invalid matrix row or column", ErrInvalidAnswer)
		}
	}
	if len(answers) != len(opts.Rows) {
		return "", fmt.Errorf("%w: matrix must answer every row", ErrInvalidAnswer)
	}
	out, _ := json.Marshal(answers)
	return string(out), nil
}

type directChoice struct {
	Value string `json:"value"`
}

func directChoices(options json.RawMessage) ([]directChoice, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("%w: choices are required", ErrInvalidAnswer)
	}
	var opts struct {
		Choices []directChoice `json:"choices"`
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return nil, fmt.Errorf("%w: malformed choices", ErrInvalidAnswer)
	}
	if len(opts.Choices) == 0 {
		return nil, fmt.Errorf("%w: choices are required", ErrInvalidAnswer)
	}
	for _, c := range opts.Choices {
		if strings.TrimSpace(c.Value) == "" {
			return nil, fmt.Errorf("%w: choice values are required", ErrInvalidAnswer)
		}
	}
	return opts.Choices, nil
}

// splitChoices divide un string por comas, " y ", " and ", o saltos de línea.
func splitChoices(s string) []string {
	s = strings.ReplaceAll(s, " y ", ",")
	s = strings.ReplaceAll(s, " and ", ",")
	s = strings.ReplaceAll(s, "\n", ",")
	s = strings.ReplaceAll(s, ";", ",")
	return strings.Split(s, ",")
}

// DisplayAnswer convierte el valor normalizado de una respuesta estructurada
// en el texto legible que ve el usuario en el chat. Se usa para persistir el
// turno user del transcript con el mismo contenido que muestra la burbuja.
func DisplayAnswer(q models.Question, value, language string) string {
	switch q.Type {
	case "single_choice":
		return choiceLabel(q.Options, value)
	case "true_false", "boolean", "yes_no":
		if value == "true" {
			if language == "en" {
				return "Yes"
			}
			return "Sí"
		}
		return "No"
	case "multi_choice", "multiple_choice":
		var vals []string
		if err := json.Unmarshal([]byte(value), &vals); err != nil {
			return value
		}
		labels := make([]string, len(vals))
		for i, v := range vals {
			labels[i] = choiceLabel(q.Options, v)
		}
		return strings.Join(labels, ", ")
	case "ranking":
		var items []rankedItem
		if err := json.Unmarshal([]byte(value), &items); err != nil {
			return value
		}
		labels := make([]string, len(items))
		for i, it := range items {
			labels[i] = rankingLabel(q.Options, it.Value)
		}
		return strings.Join(labels, " → ")
	case "matrix":
		var answers map[string]string
		if err := json.Unmarshal([]byte(value), &answers); err != nil {
			return value
		}
		parts := make([]string, 0, len(answers))
		// Respetar el orden de filas configurado en la pregunta.
		var opts struct {
			Rows []string `json:"rows"`
		}
		_ = json.Unmarshal(q.Options, &opts)
		for _, row := range opts.Rows {
			if col, ok := answers[row]; ok {
				parts = append(parts, row+": "+col)
			}
		}
		return strings.Join(parts, "; ")
	default:
		return value
	}
}

// choiceLabel devuelve el label configurado para un value de choice;
// si no lo encuentra devuelve el value tal cual.
func choiceLabel(options json.RawMessage, value string) string {
	var opts struct {
		Choices []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return value
	}
	for _, c := range opts.Choices {
		if c.Value == value {
			return c.Label
		}
	}
	return value
}

// rankingLabel devuelve el label de un item de ranking.
func rankingLabel(options json.RawMessage, value string) string {
	var opts struct {
		Items []struct {
			Label string `json:"label"`
			Value string `json:"value"`
		} `json:"items"`
	}
	if err := json.Unmarshal(options, &opts); err != nil {
		return value
	}
	for _, item := range opts.Items {
		if item.Value == value {
			return item.Label
		}
	}
	return value
}

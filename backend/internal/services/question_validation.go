package services

import (
	"encoding/json"
	"fmt"
)

// Los structs siguientes definen la forma esperada de options para cada tipo
// de pregunta estructurada. Se usan solo para validar — el service los
// deserializa, revisa que los campos requeridos existan, y descarta el struct.
// El JSON original se guarda intacto en la base de datos como JSONB.

// choiceOption es una opcion dentro de single_choice o multi_choice.
// Tanto label como value son requeridos para que el frontend pueda mostrar
// el texto y guardar el valor seleccionado correctamente.
type choiceOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// choiceOptions es el options de single_choice y multi_choice.
// Se requieren al menos 2 opciones para que la pregunta tenga sentido.
type choiceOptions struct {
	Choices []choiceOption `json:"choices"`
}

// scaleOptions es el options de linear_scale.
// Min y Max son requeridos. MinLabel y MaxLabel son opcionales.
type scaleOptions struct {
	Min      *int    `json:"min"`
	Max      *int    `json:"max"`
	MinLabel *string `json:"min_label"`
	MaxLabel *string `json:"max_label"`
}

// rankingItem es un elemento dentro de ranking.
type rankingItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// rankingOptions es el options de ranking.
// Se requieren al menos 2 items para que el respondente pueda ordenar algo.
type rankingOptions struct {
	Items []rankingItem `json:"items"`
}

// matrixOptions es el options de matrix.
// Rows son las filas que el respondente evalua.
// Columns son las etiquetas de la escala (ej. "Malo", "Regular", "Bueno").
// Se requiere al menos 1 fila y 2 columnas.
type matrixOptions struct {
	Rows    []string `json:"rows"`
	Columns []string `json:"columns"`
}

// validateOptions verifica que el campo options tenga la estructura correcta
// para el tipo de pregunta dado. Se llama tanto en Create como en Update.
// Devuelve ErrInvalidQuestion con un mensaje descriptivo si algo no cumple.
func validateOptions(questionType string, options []byte) error {
	switch questionType {

	case "open_ended", "true_false":
		// Estos tipos no usan options. Si el cliente manda algo igual no es error,
		// simplemente se ignora. No bloqueamos al admin por mandar campos de más.
		return nil

	case "single_choice", "multi_choice":
		if len(options) == 0 {
			return fmt.Errorf("%w: %s requires options with at least 2 choices", ErrInvalidQuestion, questionType)
		}
		var opts choiceOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return fmt.Errorf("%w: options must be {\"choices\":[{\"label\":\"...\",\"value\":\"...\"}]}", ErrInvalidQuestion)
		}
		if len(opts.Choices) < 2 {
			return fmt.Errorf("%w: %s requires at least 2 choices", ErrInvalidQuestion, questionType)
		}
		for i, c := range opts.Choices {
			if c.Label == "" || c.Value == "" {
				return fmt.Errorf("%w: choice %d must have both label and value", ErrInvalidQuestion, i+1)
			}
		}
		return nil

	case "linear_scale":
		if len(options) == 0 {
			return fmt.Errorf("%w: linear_scale requires options with min and max", ErrInvalidQuestion)
		}
		var opts scaleOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return fmt.Errorf("%w: options must be {\"min\":1,\"max\":5}", ErrInvalidQuestion)
		}
		if opts.Min == nil || opts.Max == nil {
			return fmt.Errorf("%w: linear_scale requires both min and max", ErrInvalidQuestion)
		}
		if *opts.Min >= *opts.Max {
			return fmt.Errorf("%w: linear_scale min must be less than max", ErrInvalidQuestion)
		}
		return nil

	case "ranking":
		if len(options) == 0 {
			return fmt.Errorf("%w: ranking requires options with at least 2 items", ErrInvalidQuestion)
		}
		var opts rankingOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return fmt.Errorf("%w: options must be {\"items\":[{\"label\":\"...\",\"value\":\"...\"}]}", ErrInvalidQuestion)
		}
		if len(opts.Items) < 2 {
			return fmt.Errorf("%w: ranking requires at least 2 items", ErrInvalidQuestion)
		}
		for i, item := range opts.Items {
			if item.Label == "" || item.Value == "" {
				return fmt.Errorf("%w: ranking item %d must have both label and value", ErrInvalidQuestion, i+1)
			}
		}
		return nil

	case "matrix":
		if len(options) == 0 {
			return fmt.Errorf("%w: matrix requires options with rows and columns", ErrInvalidQuestion)
		}
		var opts matrixOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return fmt.Errorf("%w: options must be {\"rows\":[\"...\"],\"columns\":[\"...\"]}", ErrInvalidQuestion)
		}
		if len(opts.Rows) < 1 {
			return fmt.Errorf("%w: matrix requires at least 1 row", ErrInvalidQuestion)
		}
		if len(opts.Columns) < 2 {
			return fmt.Errorf("%w: matrix requires at least 2 columns", ErrInvalidQuestion)
		}
		return nil
	}

	// Tipo desconocido — esto no debería llegar aquí porque ValidQuestionTypes
	// ya lo rechazó antes, pero por si acaso.
	return fmt.Errorf("%w: unknown question type %q", ErrInvalidQuestion, questionType)
}

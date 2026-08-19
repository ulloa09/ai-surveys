package models

import (
	"encoding/json"
	"time"
)

// Question representa una pregunta dentro de una encuesta.
// Corresponde a la tabla questions.
//
// Options es JSONB en la base de datos y su forma depende de Type:

//	open_ended, true_false:      no necesita Options (queda null)
//	single_choice, multi_choice: {"choices": [{"label": "...", "value": "..."}]}
//	linear_scale:                {"min": 1, "max": 5, "min_label": "...", "max_label": "..."}
//	ranking:                      {"items": [{"label": "...", "value": "..."}]}
//	matrix:                       {"rows": [...], "columns": [...]}

// Se usa json.RawMessage en vez de un struct para no tener que definir un
// tipo distinto por cada uno de los 7 tipos de pregunta. El backend solo
// pasa este JSON tal cual entre la base de datos y el cliente; quien
// construye y valida su contenido en detalle es el frontend, según el
// selector de tipo.

type Question struct {
	ID         string          `json:"id"`
	SurveyID   string          `json:"survey_id"`
	Type       string          `json:"type"`
	Text       string          `json:"text"`
	Required   bool            `json:"required"`
	AIFollowup bool            `json:"ai_followup"`
	Options    json.RawMessage `json:"options,omitempty"`
	OrderIndex int             `json:"order_index"`
	CreatedAt  time.Time       `json:"created_at"`
}

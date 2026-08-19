package models

import "time"

// Survey representa una encuesta y su configuración de nivel superior.
// Corresponde a la tabla `surveys`. Este slice (#04) solo expone los campos
// de configuración — preguntas, modos de IA y ciclo de vida llegan en
// slices posteriores; las columnas ya existen en el schema (ver
// migrations/001_init.sql) pero todavía no se editan desde la API.
//
// Description es puntero porque la columna admite NULL.
// OwnerName y TeamName no son columnas: se llenan con un JOIN para mostrar
// el dueño y el equipo en los listados sin una query extra por fila.
type Survey struct {
	ID                   string    `json:"id"`
	Title                string    `json:"title"`
	Description          *string   `json:"description"`
	OwnerID              string    `json:"owner_id"`
	OwnerName            string    `json:"owner_name,omitempty"`
	TeamID               string    `json:"team_id"`
	TeamName             string    `json:"team_name,omitempty"`
	Status               string    `json:"status"`
	AnonymityLevel       string    `json:"anonymity_level"`
	AllowRevisit         bool      `json:"allow_revisit"`
	OptionalRegistration bool      `json:"optional_registration"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

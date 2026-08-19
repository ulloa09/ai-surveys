package models

import "time"

// Survey representa una encuesta y su configuración de nivel superior.
// Corresponde a la tabla `surveys`. Ciclo de vida (#08) y asignación a
// múltiples equipos llegan en slices posteriores; esas columnas ya existen
// en el schema (ver migrations/001_init.sql) pero todavía no se editan
// desde la API.
//
// Description, SystemPrompt, TurnLimit y TimeEstimateMinutes son punteros
// porque sus columnas admiten NULL.
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
	Mode                 string    `json:"mode"`
	SystemPrompt         *string   `json:"system_prompt"`
	AnonymityLevel       string    `json:"anonymity_level"`
	AllowRevisit         bool      `json:"allow_revisit"`
	OptionalRegistration bool      `json:"optional_registration"`
	TerminationMode      string    `json:"termination_mode"`
	TurnLimit            *int      `json:"turn_limit"`
	TimeEstimateMinutes  *int      `json:"time_estimate_minutes"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

package models

import "time"

// Team representa un equipo en la plataforma.
// Corresponde directamente a la tabla `teams` en la DB.
type Team struct {
	ID        string    `json:"id"         db:"id"`
	Name      string    `json:"name"       db:"name"`
	CreatedBy string    `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TeamMember representa a un usuario dentro de un equipo con su rol.
// Corresponde a la tabla `team_members` + datos del usuario (JOIN).
type TeamMember struct {
	TeamID      string `json:"team_id"      db:"team_id"`
	UserID      string `json:"user_id"      db:"user_id"`
	Email       string `json:"email"        db:"email"`
	DisplayName string `json:"display_name" db:"display_name"`
	Role        string `json:"role"         db:"role"` // "profesor" | "alumno"
}

// TeamWithMembers es lo que devolvemos en GET /api/admin/teams/{id}
// Une el equipo con su lista de miembros en una sola respuesta.
type TeamWithMembers struct {
	Team
	Members []TeamMember `json:"members"`
}

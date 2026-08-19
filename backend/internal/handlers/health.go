// Package handlers contiene las funciones HTTP de la aplicación.
//
// Cada handler es delgado — solo recibe el request, llama a lo que necesite
// (un service, la DB directamente) y escribe la respuesta.
// La lógica de negocio va en internal/services/, no aquí.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
)

// Pinger es la interfaz mínima que necesita el health handler para verificar la DB.
// *pgxpool.Pool la satisface automáticamente — no hace falta ningún adaptador.
// Usarla en lugar del tipo concreto permite probar el handler sin levantar postgres.
type Pinger interface {
	Ping(context.Context) error
}

// healthResponse define la forma del JSON que devuelve este endpoint.
// Las etiquetas `json:"..."` controlan el nombre del campo en el JSON.
//
//	healthResponse{Status: "ok", Database: "connected"}
//	→ {"status":"ok","database":"connected"}
type healthResponse struct {
	Status   string `json:"status"`   // "ok" | "degraded"
	Database string `json:"database"` // "connected" | "disconnected"
}

// Health devuelve un http.HandlerFunc que verifica el estado del sistema.
//
// ¿Por qué devuelve una función en vez de ser directamente un handler?
// Porque necesitamos "inyectar" el pool. Si fuera un handler directo,
// no tendría forma de recibir el pool sin una variable global.
//
//	// Así lo registramos en main.go:
//	r.Get("/api/health", handlers.Health(pool))
//	//                              ↑ aquí pasamos el pool una vez
//	//                 ↑ chi llama esta función en cada request
//
// Criterios de aceptación:
//   - 200 + { status: "ok", database: "connected" }    → postgres responde
//   - 503 + { status: "degraded", database: "disconnected" } → postgres caído
func Health(db Pinger) http.HandlerFunc {
	// Esta función se ejecuta UNA VEZ al registrar la ruta.
	// "db" queda capturado en el closure — siempre disponible adentro.

	return func(w http.ResponseWriter, r *http.Request) {
		// Esta función se ejecuta en CADA request a GET /api/health.

		status, dbStatus, code := "ok", "connected", http.StatusOK // 200

		// db == nil cubre el caso de arranque fallido antes de conectar a postgres.
		// El || es short-circuit: si db es nil no llama a Ping (evita panic).
		if db == nil || db.Ping(r.Context()) != nil {
			// Postgres no responde — degradamos el status y cambiamos a 503
			status, dbStatus, code = "degraded", "disconnected", http.StatusServiceUnavailable
		}

		// En Go los headers deben escribirse ANTES que el status code,
		// y el status code ANTES que el body. El orden importa.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		// json.NewEncoder(w).Encode(...) serializa el struct a JSON
		// y lo escribe directamente en la respuesta (w).
		json.NewEncoder(w).Encode(healthResponse{
			Status:   status,
			Database: dbStatus,
		})
	}
}

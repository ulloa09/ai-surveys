package handlers

import "net/http"

// NotImplemented es un stub para features de la Fase 3 (RBAC) que todavia no
// se construyen — Analytics, estadisticas, reportes, insights de IA, y la
// vista de "mis encuestas asignadas" para alumnos. El objetivo de esta fase
// es dejar la arquitectura de acceso (rutas + middleware de rol) lista, sin
// implementar la logica real todavia.
//
// scope identifica que futura feature ocupa esta ruta, para que el frontend
// pueda distinguir placeholders sin necesitar rutas nuevas cuando se
// implementen de verdad.
func NotImplemented(scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"error": "not implemented",
			"scope": scope,
		})
	}
}

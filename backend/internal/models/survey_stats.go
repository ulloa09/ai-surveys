package models

// SurveyStats agrega las respuestas de una encuesta. Lo consume el dashboard
// (#16) tanto en el listado como en el detalle.
//
// AvgDurationSeconds es puntero porque no hay duración que promediar
// mientras ninguna respuesta se haya enviado — cero sería una mentira, no un
// dato.
type SurveyStats struct {
	SurveyID string `json:"survey_id"`
	// ResponseCount cuenta todas las respuestas, en cualquier estado. En
	// encuestas totalmente anónimas esto NO es "cuánta gente participó": cada
	// visita crea una respuesta nueva (sin resume, #09), así que abandonos y
	// reintentos del mismo alumno inflan el conteo. Por eso existe
	// ExpectedResponses/CoverageRate abajo — un dato mejor para saber quién
	// falta cuando no se puede identificar al respondiente.
	ResponseCount int `json:"response_count"`
	// CompletedCount cuenta solo las enviadas (status = 'submitted').
	CompletedCount int `json:"completed_count"`
	// CompletionRate es CompletedCount / ResponseCount (0.0-1.0), 0 si no hay
	// respuestas. Mide abandono de sesión (¿quién empezó pero no terminó?),
	// no cobertura de grupo — para eso ver CoverageRate.
	CompletionRate float64 `json:"completion_rate"`
	// AvgDurationSeconds promedia submitted_at - started_at de las respuestas enviadas.
	AvgDurationSeconds *float64 `json:"avg_duration_seconds"`
	// LanguageDistribution cuenta respuestas por idioma elegido, de mayor a menor.
	LanguageDistribution []LanguageCount `json:"language_distribution"`
	// ExpectedResponses es el response_cap que el creador declaró para la
	// encuesta, o nil si no declaró uno. Es lo más cercano a un tamaño de
	// grupo esperado que existe en el schema (una encuesta pertenece a un
	// solo equipo, no a varias asignaciones).
	ExpectedResponses *int `json:"expected_responses"`
	// MissingResponses es max(ExpectedResponses - CompletedCount, 0), o nil
	// si ExpectedResponses no está definido. Es el número que un profesor
	// realmente quiere ver: cuántos de su grupo todavía no han respondido.
	MissingResponses *int `json:"missing_responses"`
	// CoverageRate es CompletedCount / ExpectedResponses (0.0-1.0+), o nil si
	// ExpectedResponses no está definido. En encuestas anónimas, esta es la
	// métrica que reemplaza a CompletionRate como "% real de avance".
	CoverageRate *float64 `json:"coverage_rate"`
}

// LanguageCount es la cantidad de respuestas que eligieron un idioma.
type LanguageCount struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
}

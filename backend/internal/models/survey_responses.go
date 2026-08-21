package models

import "time"

// SurveyResponses es la vista de respuestas individuales de una encuesta: una
// fila por participante que envió, con su respuesta a cada pregunta. A
// diferencia de SurveyAnalysis (resúmenes agregados por IA), esto es la data
// cruda ya guardada en `answers` — existe desde que hay respuestas enviadas,
// sin depender de que el Analysis Engine haya corrido.
type SurveyResponses struct {
	SurveyID  string               `json:"survey_id"`
	Questions []ResponseQuestion   `json:"questions"`
	Responses []RespondentResponse `json:"responses"`
}

// ResponseQuestion es la vista mínima de una pregunta para el visor de
// respuestas: solo lo necesario para rotular columnas/secciones.
type ResponseQuestion struct {
	QuestionID string `json:"question_id"`
	Text       string `json:"text"`
	Type       string `json:"type"`
	OrderIndex int    `json:"order_index"`
}

// RespondentResponse agrupa todo lo que una persona respondió en una
// encuesta. Label es el email a mostrar cuando hay identidad que mostrar
// (user_id de una encuesta anonymity_level='none', o un registered_email que
// el participante compartió voluntariamente); si ambos vienen vacíos, Label
// es nil y AnonNumber lleva el número de orden de envío ("Anónimo #N") — ver
// AnalysisService.SurveyResponses.
type RespondentResponse struct {
	ResponseID  string             `json:"response_id"`
	Label       *string            `json:"label"`
	AnonNumber  *int               `json:"anon_number"`
	Language    string             `json:"language"`
	SubmittedAt *time.Time         `json:"submitted_at"`
	Answers     []RespondentAnswer `json:"answers"`
}

// RespondentAnswer es la respuesta de una persona a una pregunta puntual, con
// el sentimiento/outlier que ya haya calculado el Analysis Engine si corrió
// (nil/false si todavía no). RawValue es el formato canónico que persiste
// answers (JSON para multi_choice/ranking/matrix, ver ValidateStructuredAnswer);
// DisplayValue es el mismo texto legible que ve el participante en el chat
// (services.DisplayAnswer) — lo que el visor de respuestas debe mostrar y
// usar para agrupar en las estadísticas por pregunta.
type RespondentAnswer struct {
	QuestionID     string  `json:"question_id"`
	RawValue       string  `json:"raw_value"`
	DisplayValue   string  `json:"display_value"`
	SentimentLabel *string `json:"sentiment_label"`
	IsOutlier      bool    `json:"is_outlier"`
}

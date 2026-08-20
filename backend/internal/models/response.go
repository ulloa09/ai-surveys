package models

import "time"

type Response struct {
	ID                   string     `json:"id"`
	SurveyID             string     `json:"survey_id"`
	UserID               *string    `json:"user_id"`
	FingerprintHash      *string    `json:"fingerprint_hash"`
	Status               string     `json:"status"`
	Language             string     `json:"language"`
	StartedAt            time.Time  `json:"started_at"`
	SubmittedAt          *time.Time `json:"submitted_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	CurrentQuestionIndex int        `json:"current_question_index"`
	TurnCount            int        `json:"turn_count"`
	RegisteredName       *string    `json:"registered_name"`
	RegisteredEmail      *string    `json:"registered_email"`
}

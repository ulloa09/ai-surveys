package handlers_test

// Tests de integración del visor de respuestas individuales (#16):
//   - una respuesta con user_id (anonymity_level='none') se etiqueta con el
//     email de esa cuenta
//   - una respuesta sin user_id pero con registered_email (compartido
//     voluntariamente) se etiqueta con ese email
//   - una respuesta sin ninguno de los dos se numera "anónima" en orden de
//     envío (submitted_at ASC)
//
// Requiere Postgres local (se salta si no está disponible).

import (
	"context"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/models"
)

func TestIntegrationSurveyResponses_LabelsIdentityAnonymityAndOrder(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := context.Background()

	owner, teamID := f.createOwnerTeam(t)

	var surveyID string
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO surveys (title, owner_id, team_id, status, mode, anonymity_level, available_languages, default_language, termination_mode, turn_limit)
		VALUES ('Visor de respuestas', $1, $2, 'open', 'form', 'partial', ARRAY['es'], 'es', 'turn_limit', 12)
		RETURNING id::text`, owner.ID, teamID,
	).Scan(&surveyID); err != nil {
		t.Fatalf("insert survey: %v", err)
	}

	q1 := f.createQuestion(t, surveyID, "open_ended", "¿Cómo te sentiste?", true, "")
	q2 := f.createQuestion(t, surveyID, "single_choice", "¿Recomendarías el curso?",
		true, `{"choices":[{"value":"y","label":"Sí"},{"value":"n","label":"No"}]}`)

	var studentID string
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name, role, password_hash)
		VALUES ('alumno@test.com', 'Alumno Uno', 'alumno', 'hash')
		RETURNING id::text`,
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

	insertSubmitted := func(userID *string, registeredEmail *string, offsetSeconds int) string {
		var responseID string
		if err := f.pool.QueryRow(ctx, `
			INSERT INTO responses (survey_id, user_id, language, status, submitted_at, registered_email)
			VALUES ($1, $2, 'es', 'submitted', NOW() + make_interval(secs => $3), $4)
			RETURNING id::text`,
			surveyID, userID, offsetSeconds, registeredEmail,
		).Scan(&responseID); err != nil {
			t.Fatalf("insert response: %v", err)
		}
		f.createAnswer(t, responseID, q1, "respuesta de "+responseID)
		f.createAnswer(t, responseID, q2, "y")
		return responseID
	}

	voluntary := "voluntario@test.com"
	// Orden de envío: identificado (t+0), voluntario (t+1), anónimo (t+2).
	identifiedID := insertSubmitted(&studentID, nil, 0)
	voluntaryID := insertSubmitted(nil, &voluntary, 1)
	anonID := insertSubmitted(nil, nil, 2)

	analysisSvc := f.analysisSvcWith(&fakeAnalysisProvider{})
	result, err := analysisSvc.SurveyResponses(ctx, surveyID)
	if err != nil {
		t.Fatalf("survey responses: %v", err)
	}

	if len(result.Questions) != 2 || result.Questions[0].QuestionID != q1 || result.Questions[1].QuestionID != q2 {
		t.Fatalf("questions = %+v, want q1 luego q2 en orden", result.Questions)
	}
	if len(result.Responses) != 3 {
		t.Fatalf("responses = %d, want 3", len(result.Responses))
	}

	byID := make(map[string]models.RespondentResponse, 3)
	for _, r := range result.Responses {
		byID[r.ResponseID] = r
	}

	identified := byID[identifiedID]
	if identified.Label == nil || *identified.Label != "alumno@test.com" {
		t.Fatalf("identified.Label = %v, want alumno@test.com", identified.Label)
	}
	if identified.AnonNumber != nil {
		t.Fatalf("identified.AnonNumber = %v, want nil (tiene identidad)", *identified.AnonNumber)
	}
	if len(identified.Answers) != 2 || identified.Answers[0].RawValue != "respuesta de "+identifiedID {
		t.Fatalf("identified.Answers = %+v", identified.Answers)
	}
	var choiceAnswer models.RespondentAnswer
	for _, ans := range identified.Answers {
		if ans.QuestionID == q2 {
			choiceAnswer = ans
		}
	}
	if choiceAnswer.RawValue != "y" {
		t.Fatalf("choiceAnswer.RawValue = %q, want the canonical option value", choiceAnswer.RawValue)
	}
	if choiceAnswer.DisplayValue != "Sí" {
		t.Fatalf("choiceAnswer.DisplayValue = %q, want the resolved option label 'Sí'", choiceAnswer.DisplayValue)
	}

	voluntaryResp := byID[voluntaryID]
	if voluntaryResp.Label == nil || *voluntaryResp.Label != voluntary {
		t.Fatalf("voluntaryResp.Label = %v, want %s (compartido voluntariamente)", voluntaryResp.Label, voluntary)
	}

	anon := byID[anonID]
	if anon.Label != nil {
		t.Fatalf("anon.Label = %v, want nil", *anon.Label)
	}
	if anon.AnonNumber == nil || *anon.AnonNumber != 1 {
		t.Fatalf("anon.AnonNumber = %v, want 1 (única respuesta sin identidad)", anon.AnonNumber)
	}

	// El orden de salida sigue submitted_at ASC.
	if result.Responses[0].ResponseID != identifiedID ||
		result.Responses[1].ResponseID != voluntaryID ||
		result.Responses[2].ResponseID != anonID {
		t.Fatalf("orden de respuestas incorrecto: %+v", result.Responses)
	}
}

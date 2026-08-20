package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/fingerprint"
	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/models"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

type ResponseServicer interface {
	GetPublicSurvey(ctx context.Context, token string) (*services.PublicSurvey, error)
	Create(ctx context.Context, in services.CreateResponseInput) (*services.CreateResponseResult, error)
	GetByResumeToken(ctx context.Context, publicToken, resumeToken string) (*models.Response, error)
	IsTeamMember(ctx context.Context, teamID, userID string) (bool, error)
}

// GetPublicSurvey maneja GET /api/public/surveys/{token}. El requisito de
// sesión depende del anonymity_level, por eso el gate va DESPUÉS de resolver
// el token (la ruta usa appmw.OptionalAuthenticate, que no rechaza a nadie):
//
//   - 'full' (anónima): link verdaderamente público. Sin cuenta y sin roster —
//     cualquiera con el link puede abrirla, que es justo lo que promete el
//     landing page ("no guardamos ningún dato que permita identificarte").
//   - 'partial' / 'none': sigue exigiendo sesión y membresía del equipo dueño
//     de la encuesta — cada equipo controla su propio roster (team_members).
func GetPublicSurvey(svc ResponseServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		survey, err := svc.GetPublicSurvey(r.Context(), token)
		if err != nil {
			writeResponseError(w, err)
			return
		}
		if survey.Status == "draft" || survey.Status == "archived" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "survey not found"})
			return
		}

		if survey.AnonymityLevel != "full" {
			user := appmw.UserFromContext(r.Context())
			if user == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
				return
			}

			isMember, err := svc.IsTeamMember(r.Context(), survey.TeamID, user.ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			if !isMember {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member of this survey's team"})
				return
			}
		}

		writeJSON(w, http.StatusOK, survey)
	}
}

// CreateResponse maneja POST /api/public/surveys/{token}/responses. El user_id
// SIEMPRE se toma de la sesión, nunca del body — un valor puesto por el cliente
// no es un dato de identidad confiable, y aceptarlo permitiría a cualquiera
// hacerse pasar por otro alumno con solo conocer su id.
//
// Puede llegar sin sesión, y eso no es un error por sí solo: las encuestas
// 'full' (anónimas) se contestan sin cuenta. Quién puede enviar la respuesta lo
// decide ResponseService.Create, que ya resuelve el anonymity_level del token.
//
// fingerprintSalt es la sal secreta del HMAC con que se deriva el identificador
// de dispositivo a partir de la cookie device_id (ver internal/fingerprint). El
// device_id en claro nunca sale de aquí: al servicio solo le llega el hash, y
// solo las encuestas 'partial' lo guardan.
func CreateResponse(svc ResponseServicer, fingerprintSalt string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Language        string  `json:"language"`
			RegisteredName  *string `json:"registered_name"`
			RegisteredEmail *string `json:"registered_email"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		var userID *string
		if user := appmw.UserFromContext(r.Context()); user != nil {
			userID = &user.ID
		}

		// La cookie la emite el frontend (SvelteKit es quien habla con el
		// navegador; el backend solo ve la petición servidor-a-servidor). Si no
		// viene, DeviceHash queda nil: la encuesta sigue contestándose, pero esa
		// respuesta no participa de la detección de duplicados.
		var deviceHash *string
		if cookie, err := r.Cookie("device_id"); err == nil {
			if h := fingerprint.Hash(cookie.Value, fingerprintSalt); h != "" {
				deviceHash = &h
			}
		}

		result, err := svc.Create(r.Context(), services.CreateResponseInput{
			PublicToken:     chi.URLParam(r, "token"),
			Language:        body.Language,
			RegisteredName:  body.RegisteredName,
			RegisteredEmail: body.RegisteredEmail,
			UserID:          userID,
			DeviceHash:      deviceHash,
		})
		if err != nil {
			writeResponseError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
}

// ResumeResponse reanuda una sesión a partir del token de continuación enviado
// como query param `?token=`. Devuelve la respuesta en progreso si el token es
// válido; en cualquier otro caso responde 404 para que el cliente arranque una
// sesión nueva (criterio de aceptación #09).
func ResumeResponse(svc ResponseServicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		publicToken := chi.URLParam(r, "token")
		resumeToken := r.URL.Query().Get("token")

		response, err := svc.GetByResumeToken(r.Context(), publicToken, resumeToken)
		if err != nil {
			writeResponseError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func writeResponseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrResponseSurveyNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "survey not found"})
	case errors.Is(err, services.ErrSurveyNotOpen):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "survey is not open"})
	case errors.Is(err, services.ErrResponseCapReached):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "survey has reached its response cap"})
	case errors.Is(err, services.ErrInvalidResponseLanguage):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "language is not available for this survey"})
	case errors.Is(err, services.ErrResumeTokenNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resume token not found"})
	case errors.Is(err, services.ErrAlreadyResponded):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "already responded to this survey"})
	case errors.Is(err, services.ErrAuthRequired):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	case errors.Is(err, services.ErrNotTeamMember):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not a member of this survey's team"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

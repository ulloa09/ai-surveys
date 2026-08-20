package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// StreamTurn maneja GET /api/responses/{id}/stream?message=<texto>.
// Carga el contexto completo de la respuesta, lo manda al proveedor de IA
// activo y hace streaming del resultado como SSE al cliente.
// El contenido del participante llega SOLO como mensaje de usuario — nunca
// se interpola en el system prompt (hardening de inyección).
//
// Requiere sesión: es el endpoint que usan los admins para probar/inspeccionar
// una conversación puntual. El acceso se limita a admin/super_admin o a un
// miembro del equipo dueño de la encuesta.
func StreamTurn(settingsSvc *services.SettingsService, responseSvc *services.ResponseService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		responseID := chi.URLParam(r, "id")
		userMessage := r.URL.Query().Get("message")
		if userMessage == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
			return
		}

		ctx := r.Context()

		// Cargar respuesta + encuesta
		response, survey, err := responseSvc.GetResponseWithSurvey(ctx, responseID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "response not found"})
			return
		}

		if user.Role != "super_admin" && user.Role != "admin" {
			isMember, err := responseSvc.IsTeamMember(ctx, survey.TeamID, user.ID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
				return
			}
			if !isMember {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
				return
			}
		}

		// Cargar transcript anterior
		transcript, err := responseSvc.GetTranscript(ctx, responseID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load transcript"})
			return
		}

		// Ensamblar system prompt — BuildSystemPrompt vive en internal/ai/context.go
		systemPrompt := ai.BuildSystemPrompt(ai.TurnContext{
			Survey:   *survey,
			Response: *response,
		})

		// Cargar configuración del proveedor activo
		cfg, err := settingsSvc.GetAIConfig(ctx)
		if err != nil || cfg.APIKey == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai provider not configured"})
			return
		}

		// Crear proveedor usando la factory — soporta claude y openai
		provider, err := ai.NewProvider(cfg.Provider, cfg.APIKey, cfg.Model)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "unknown ai provider"})
			return
		}

		// Guardar turno del usuario antes de hacer streaming
		if err := responseSvc.AppendTurn(ctx, responseID, "user", userMessage); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save turn"})
			return
		}

		// Iniciar stream — el mensaje del usuario va como UserMessage, nunca en SystemPrompt
		ch, err := provider.StreamTurn(ctx, ai.TurnRequest{
			SystemPrompt: systemPrompt,
			Transcript:   transcript,
			UserMessage:  userMessage,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not start ai stream"})
			return
		}

		// Configurar cabeceras SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
			return
		}

		// Hacer streaming de tokens y acumular la respuesta completa del asistente
		var fullResponse string
		for chunk := range ch {
			if chunk.Error != nil {
				fmt.Fprintf(w, "data: [ERROR]\n\n")
				flusher.Flush()
				return
			}
			fullResponse += chunk.Token
			fmt.Fprintf(w, "data: %s\n\n", chunk.Token)
			flusher.Flush()
		}

		// Guardar respuesta completa del asistente
		if fullResponse != "" {
			_ = responseSvc.AppendTurn(ctx, responseID, "assistant", fullResponse)
		}

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// DevTestAI maneja POST /api/dev/test-ai.
// Solo disponible cuando APP_ENV=development. Envía un prompt libre y hace
// streaming de la respuesta para verificar la configuración del proveedor
// sin necesitar una encuesta real.
func DevTestAI(settingsSvc *services.SettingsService, appEnv string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if appEnv != "development" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		var body struct {
			Prompt string `json:"prompt"`
		}
		if err := decodeJSON(r, &body); err != nil || body.Prompt == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
			return
		}

		ctx := r.Context()
		cfg, err := settingsSvc.GetAIConfig(ctx)
		if err != nil || cfg.APIKey == "" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai provider not configured"})
			return
		}

		provider, err := ai.NewProvider(cfg.Provider, cfg.APIKey, cfg.Model)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		ch, err := provider.StreamTurn(ctx, ai.TurnRequest{
			SystemPrompt: "You are a helpful assistant.",
			UserMessage:  body.Prompt,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, _ := w.(http.Flusher)
		for chunk := range ch {
			if chunk.Error != nil {
				fmt.Fprintf(w, "data: [ERROR]\n\n")
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", chunk.Token)
			if flusher != nil {
				flusher.Flush()
			}
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}
}

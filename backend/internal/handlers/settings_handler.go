package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ulloa09/ai-surveys/backend/internal/ai"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
)

// GetSettings maneja GET /api/admin/settings.
// Devuelve la configuración actual con las API keys enmascaradas.
// Solo accesible para super_admin (aplicado en el router).
func GetSettings(svc *services.SettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		claudeKeyMasked, _ := svc.GetMasked(ctx, services.SettingClaudeKey)
		claudeModel, _ := svc.GetMasked(ctx, services.SettingClaudeModel)
		openaiKeyMasked, _ := svc.GetMasked(ctx, services.SettingOpenAIKey)
		openaiModel, _ := svc.GetMasked(ctx, services.SettingOpenAIModel)
		provider, _ := svc.GetMasked(ctx, services.SettingActiveProvider)

		if provider == "" {
			provider = "claude"
		}
		if claudeModel == "" {
			claudeModel = "claude-sonnet-4-5"
		}
		if openaiModel == "" {
			openaiModel = "gpt-4o"
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"active_provider":   provider,
			"claude_key_masked": claudeKeyMasked,
			"claude_model":      claudeModel,
			"openai_key_masked": openaiKeyMasked,
			"openai_model":      openaiModel,
		})
	}
}

// UpdateSettings maneja PATCH /api/admin/settings.
// Actualiza uno o más valores de configuración. Solo super_admin.
func UpdateSettings(svc *services.SettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ActiveProvider *string `json:"active_provider"`
			ClaudeKey      *string `json:"claude_key"`
			ClaudeModel    *string `json:"claude_model"`
			OpenAIKey      *string `json:"openai_key"`
			OpenAIModel    *string `json:"openai_model"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		ctx := r.Context()

		if body.ActiveProvider != nil {
			if *body.ActiveProvider != "claude" && *body.ActiveProvider != "openai" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "active_provider must be claude or openai"})
				return
			}
			if err := svc.Set(ctx, services.SettingActiveProvider, *body.ActiveProvider); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save provider"})
				return
			}
		}
		if body.ClaudeKey != nil {
			if err := svc.Set(ctx, services.SettingClaudeKey, *body.ClaudeKey); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save claude key"})
				return
			}
		}
		if body.ClaudeModel != nil {
			if err := svc.Set(ctx, services.SettingClaudeModel, *body.ClaudeModel); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save claude model"})
				return
			}
		}
		if body.OpenAIKey != nil {
			if err := svc.Set(ctx, services.SettingOpenAIKey, *body.OpenAIKey); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save openai key"})
				return
			}
		}
		if body.OpenAIModel != nil {
			if err := svc.Set(ctx, services.SettingOpenAIModel, *body.OpenAIModel); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save openai model"})
				return
			}
		}

		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// TestConnection maneja POST /api/admin/settings/test.
// Envía una petición mínima al proveedor activo para verificar que el API key
// y el modelo son válidos. Responde en máximo 5 segundos.
func TestConnection(svc *services.SettingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		cfg, err := svc.GetAIConfig(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not load ai config"})
			return
		}
		if cfg.APIKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no api key configured"})
			return
		}

		provider, err := ai.NewProvider(cfg.Provider, cfg.APIKey, cfg.Model)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		// Ambos providers (Claude y OpenAI) implementan Prober.
		type prober interface {
			Probe(ctx context.Context) error
		}
		p, ok := provider.(prober)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "provider does not support probe"})
			return
		}

		if err := p.Probe(ctx); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    true,
			"model": cfg.Model,
		})
	}
}

package ai

import (
	"context"
	"fmt"
)

// NewProvider crea el proveedor correcto según el nombre.
// Lo usa el handler de streaming para instanciar el proveedor activo
// sin saber qué implementación específica se está usando.
func NewProvider(providerName, apiKey, model string) (AIProvider, error) {
	switch providerName {
	case "claude", "":
		return NewClaudeProvider(apiKey, model), nil
	case "openai":
		return NewOpenAIProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown ai provider: %q", providerName)
	}
}

// Prober es la interfaz que exponen Claude y OpenAI para el test de conexión.
type Prober interface {
	Probe(ctx context.Context) error
}

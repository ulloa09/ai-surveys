package ai

import "strings"

// sanitizeJSONResponse limpia la respuesta cruda de un modelo antes de
// json.Unmarshal. Aunque el prompt pide "responde SOLO con JSON", los
// modelos frecuentemente envuelven la respuesta en una cerca de markdown
// (```json ... ```) o le agregan una frase antes/después — cualquiera de
// las dos cosas rompe json.Unmarshal y, en AnalyseSurvey, eso dejaba la
// encuesta atascada en 'analysing' para siempre (ver AnalysisService).
// Esto es una red de seguridad barata: quita la cerca de markdown si existe
// y recorta al primer '{' .. último '}', que es donde vive el objeto JSON
// en cualquiera de esas variantes.
func sanitizeJSONResponse(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start == -1 || end == -1 || end < start {
		return text
	}
	return text[start : end+1]
}

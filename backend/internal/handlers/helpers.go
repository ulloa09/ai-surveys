package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// jsonDecoder devuelve un decoder configurado para el body del request.
// DisallowUnknownFields hace que campos inesperados en el JSON devuelvan error —
// ayuda a detectar typos en los nombres de campos del body.
func jsonDecoder(r *http.Request) *json.Decoder {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	return d
}

func pathUUIDParam(w http.ResponseWriter, r *http.Request, name, label string) (string, bool) {
	value := chi.URLParam(r, name)
	if !isUUID(value) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid " + label})
		return "", false
	}
	return value, true
}

func isUUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

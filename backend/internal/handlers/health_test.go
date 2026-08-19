package handlers_test

// Los tests de Go usan el sufijo _test.go — el compilador los excluye del binario final.
// El paquete handlers_test (caja negra) solo accede a lo que está exportado,
// igual que cualquier otro paquete consumidor. Así probamos la interfaz pública,
// no los detalles de implementación.

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
)

// fakePinger implementa handlers.Pinger sin necesitar postgres.
// err == nil simula DB disponible; cualquier error simula DB caída.
type fakePinger struct{ err error }

func (f fakePinger) Ping(_ context.Context) error { return f.err }

// TestHealth_OK verifica el camino feliz: la DB responde y el handler devuelve 200.
func TestHealth_OK(t *testing.T) {
	h := handlers.Health(fakePinger{err: nil})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("código HTTP: esperaba 200, obtuve %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("el body no es JSON válido: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status: esperaba %q, obtuve %q", "ok", body["status"])
	}
	if body["database"] != "connected" {
		t.Errorf("database: esperaba %q, obtuve %q", "connected", body["database"])
	}
}

// TestHealth_Degradado verifica el camino triste: la DB no responde → 503.
//
// httptest.NewRecorder() simula un http.ResponseWriter en memoria —
// no necesitamos un servidor real para probar un handler.
func TestHealth_Degradado(t *testing.T) {
	h := handlers.Health(fakePinger{err: errors.New("connection refused")})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("código HTTP: esperaba 503, obtuve %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("el body no es JSON válido: %v", err)
	}
	if body["status"] != "degraded" {
		t.Errorf("status: esperaba %q, obtuve %q", "degraded", body["status"])
	}
	if body["database"] != "disconnected" {
		t.Errorf("database: esperaba %q, obtuve %q", "disconnected", body["database"])
	}
}

// TestHealth_ContentType verifica que la respuesta siempre declare JSON.
// Un cliente que no recibe Content-Type: application/json puede fallar al parsear.
//
// mime.ParseMediaType separa el tipo base de los parámetros opcionales
// (ej. "application/json; charset=utf-8" → mediatype="application/json"),
// así el test no falla si el framework agrega charset u otros parámetros.
func TestHealth_ContentType(t *testing.T) {
	h := handlers.Health(fakePinger{err: nil})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	mediatype, _, err := mime.ParseMediaType(rec.Header().Get("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type no es un media type válido: %v", err)
	}
	if mediatype != "application/json" {
		t.Errorf("Content-Type: esperaba %q, obtuve %q", "application/json", mediatype)
	}
}

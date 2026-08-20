package qr

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerate_ProducesPNGAndSVGDataURIs(t *testing.T) {
	png, svg, err := Generate("https://example.com/s/11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("Generate devolvió error: %v", err)
	}

	const pngPrefix = "data:image/png;base64,"
	const svgPrefix = "data:image/svg+xml;base64,"

	if !strings.HasPrefix(png, pngPrefix) {
		t.Errorf("PNG no tiene el prefijo data URI esperado: %q", png[:min(40, len(png))])
	}
	if !strings.HasPrefix(svg, svgPrefix) {
		t.Errorf("SVG no tiene el prefijo data URI esperado: %q", svg[:min(40, len(svg))])
	}

	// El payload base64 debe decodificar sin error en ambos casos.
	if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(png, pngPrefix)); err != nil {
		t.Errorf("payload PNG no es base64 válido: %v", err)
	}
	svgBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(svg, svgPrefix))
	if err != nil {
		t.Fatalf("payload SVG no es base64 válido: %v", err)
	}
	if !strings.Contains(string(svgBytes), "<svg") {
		t.Errorf("el SVG decodificado no contiene un elemento <svg>: %q", string(svgBytes[:min(60, len(svgBytes))]))
	}
}

func TestGenerate_DeterministicForSameContent(t *testing.T) {
	const url = "https://example.com/s/abc"
	png1, svg1, err := Generate(url)
	if err != nil {
		t.Fatalf("primera generación falló: %v", err)
	}
	png2, svg2, err := Generate(url)
	if err != nil {
		t.Fatalf("segunda generación falló: %v", err)
	}
	if png1 != png2 || svg1 != svg2 {
		t.Error("Generate debería ser determinista para el mismo contenido")
	}
}

// Package qr genera códigos QR para los links públicos de las encuestas.
//
// Devuelve los códigos como data URIs en base64 (PNG y SVG) para guardarlos
// directamente en las columnas qr_png_url / qr_svg_url y mostrarlos en el admin
// sin necesidad de un servidor de archivos estáticos.
package qr

import (
	"encoding/base64"
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// pngSize es el lado en píxeles del PNG generado. 512 es suficiente para
// escanear desde un proyector o impreso en tamaño carta.
const pngSize = 512

// Generate produce un código QR para content (la URL pública de la encuesta)
// en dos formatos, ambos como data URIs base64 listos para <img src> o descarga.
func Generate(content string) (pngDataURI string, svgDataURI string, err error) {
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", "", fmt.Errorf("crear qr: %w", err)
	}

	png, err := code.PNG(pngSize)
	if err != nil {
		return "", "", fmt.Errorf("codificar png: %w", err)
	}
	pngDataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)

	svg := bitmapToSVG(code.Bitmap())
	svgDataURI = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))

	return pngDataURI, svgDataURI, nil
}

// bitmapToSVG dibuja la matriz de módulos del QR como un SVG vectorial.
// El bitmap ya incluye la zona de silencio (borde) que añade go-qrcode.
// Cada módulo oscuro se emite como un cuadro de 1x1 dentro de un único path,
// lo que mantiene el SVG compacto y nítido a cualquier escala.
func bitmapToSVG(bitmap [][]bool) string {
	n := len(bitmap)

	var b strings.Builder
	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`,
		n, n)
	b.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/>`)
	b.WriteString(`<path fill="#000000" d="`)
	for y := 0; y < n; y++ {
		row := bitmap[y]
		for x := 0; x < n; x++ {
			if row[x] {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

package services

import "testing"

func TestIsLowQualityAnswer_DetectsObviousJunk(t *testing.T) {
	junk := []string{
		"",        // vacío
		"   ",     // whitespace
		"a",       // 1 char
		".",       // puntuación sola
		"xx",      // 2 chars
		"sdjfdfa", // teclazo con proporción de vocales imposible
		"asdfgh",  // fila de teclado
		"qwerty",  // fila de teclado
		"zxcv",    // fila de teclado
		"aaaaa",   // repetición sin contenido
		"ababab",  // dos caracteres alternados
		"12345",   // fila numérica
	}
	for _, s := range junk {
		if !IsLowQualityAnswer(s) {
			t.Errorf("expected %q to be flagged as low quality", s)
		}
	}
}

func TestIsLowQualityAnswer_AcceptsRealAnswers(t *testing.T) {
	valid := []string{
		"bien",                          // palabra corta real
		"nada",                          // respuesta corta válida
		"me gustó el curso",             // frase normal
		"no sé",                         // multi-palabra siempre válida
		"el profesor explica muy bien",  // respuesta real
		"x y z",                         // multi-palabra (conservador: se acepta)
		"regular",                       // palabra real
		"aburrido",                      // palabra real
		"la organización del semestre",  // frase
		"good",                          // inglés corto
		"it was fine, nothing special",  // inglés frase
	}
	for _, s := range valid {
		if IsLowQualityAnswer(s) {
			t.Errorf("expected %q to be accepted", s)
		}
	}
}

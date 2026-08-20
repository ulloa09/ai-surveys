package services

import "strings"

// IsLowQualityAnswer detecta respuestas claramente inválidas para preguntas
// abiertas: vacías, demasiado cortas o teclazos aleatorios evidentes.
//
// Es deliberadamente conservador: ante la duda la respuesta se acepta.
// Una respuesta de varias palabras nunca se marca como basura — solo se
// evalúan mensajes de una sola palabra, donde el patrón de teclado o la
// falta de vocales son señales confiables.
func IsLowQualityAnswer(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return true
	}
	if len([]rune(t)) <= 2 {
		return true
	}
	words := strings.Fields(t)
	if len(words) > 1 {
		return false
	}

	w := []rune(words[0])
	if len(w) < 4 {
		return false
	}

	// Filas de teclado: "qwerty", "asdf", "zxcv", "1234"...
	for _, row := range []string{"qwertyuiop", "asdfghjkl", "zxcvbnm", "1234567890"} {
		if strings.Contains(row, words[0]) {
			return true
		}
	}

	if len(w) < 5 {
		return false
	}

	uniq := make(map[rune]bool, len(w))
	vowels := 0
	for _, r := range w {
		uniq[r] = true
		if strings.ContainsRune("aeiouáéíóúü", r) {
			vowels++
		}
	}
	// "aaaaa", "ababab" — repetición sin contenido.
	if len(uniq) <= 2 {
		return true
	}
	// "sdfghjkl", "sdjfdfa" — sin vocales o proporción imposible en una palabra real.
	if vowels == 0 {
		return true
	}
	if float64(vowels)/float64(len(w)) < 0.15 {
		return true
	}
	return false
}

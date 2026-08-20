// Package fingerprint deriva el identificador de dispositivo que usan las
// encuestas pseudónimas (anonymity_level = 'partial') para detectar envíos
// duplicados sin guardar la identidad del respondiente.
//
// La entrada NO es una huella derivada de headers (user-agent +
// accept-language), como proponía el issue #09: esa combinación tiene
// entropía bajísima y colisiona masivamente — todos los alumnos con el mismo
// modelo de teléfono y el mismo idioma producirían el MISMO hash, y el segundo
// en contestar quedaría bloqueado como "duplicado" sin tener cuenta con la cual
// reclamar. En vez de eso, el cliente guarda un UUID aleatorio por navegador
// (cookie device_id) y aquí solo se guarda su HMAC.
//
// Propiedades:
//
//   - Determinista: el mismo device_id y la misma sal dan siempre el mismo hash,
//     que es lo que permite reconocer al dispositivo en una segunda visita.
//   - No reversible: es un HMAC-SHA256 con una sal secreta del servidor. Aun con
//     acceso a la base de datos no se puede volver del hash al device_id (y el
//     device_id, al ser aleatorio, tampoco diría quién es la persona: solo vive
//     en la cookie de su navegador). No existe ninguna ruta que haga el mapeo
//     inverso.
package fingerprint

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Hash devuelve el HMAC-SHA256 en hexadecimal de deviceID bajo salt.
// Devuelve "" si deviceID viene vacío — sin dispositivo no hay nada que
// identificar, y un hash de la cadena vacía sería un valor constante que
// agruparía por error a todos los respondientes sin cookie.
func Hash(deviceID, salt string) string {
	if deviceID == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(deviceID))
	return hex.EncodeToString(mac.Sum(nil))
}

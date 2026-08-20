package fingerprint_test

import (
	"testing"

	"github.com/ulloa09/ai-surveys/backend/internal/fingerprint"
)

// Determinista: es la propiedad que hace posible reconocer el mismo dispositivo
// en una segunda visita (criterio de aceptación #09).
func TestHash_IsDeterministic(t *testing.T) {
	const device, salt = "9f8a1c4e-0000-4444-8888-abcdefabcdef", "salt"

	first := fingerprint.Hash(device, salt)
	second := fingerprint.Hash(device, salt)

	if first != second {
		t.Fatalf("mismo device_id y sal dieron hashes distintos: %q vs %q", first, second)
	}
	if first == "" {
		t.Fatal("hash vacío para un device_id válido")
	}
}

// Dos dispositivos distintos NO pueden colisionar: es justamente la falla de la
// huella por headers (UA + idioma) que este diseño evita — dos alumnos con el
// mismo teléfono y el mismo idioma deben poder contestar los dos.
func TestHash_DifferentDevicesDoNotCollide(t *testing.T) {
	const salt = "salt"

	if a, b := fingerprint.Hash("device-a", salt), fingerprint.Hash("device-b", salt); a == b {
		t.Fatalf("dos device_id distintos produjeron el mismo hash: %q", a)
	}
}

// La sal es secreta y es lo que hace no reversible al hash: sin ella, el mismo
// device_id da otro valor, así que nadie puede recomputar la tabla desde afuera.
func TestHash_SaltChangesTheResult(t *testing.T) {
	const device = "same-device"

	if a, b := fingerprint.Hash(device, "salt-a"), fingerprint.Hash(device, "salt-b"); a == b {
		t.Fatalf("la sal no afectó el hash: %q", a)
	}
}

// Sin cookie no hay dispositivo que identificar. Devolver "" (y no el hash de la
// cadena vacía) evita que todos los respondientes sin cookie caigan en el mismo
// valor y se bloqueen entre sí como si fueran duplicados.
func TestHash_EmptyDeviceReturnsEmpty(t *testing.T) {
	if got := fingerprint.Hash("", "salt"); got != "" {
		t.Fatalf("Hash(\"\", salt) = %q, want \"\"", got)
	}
}

// El hash no filtra su entrada: no es el device_id ni lo contiene.
func TestHash_DoesNotLeakDeviceID(t *testing.T) {
	const device = "9f8a1c4e-0000-4444-8888-abcdefabcdef"

	got := fingerprint.Hash(device, "salt")
	if got == device {
		t.Fatal("el hash es el device_id en claro")
	}
	if len(got) != 64 { // HMAC-SHA256 en hex
		t.Fatalf("largo del hash = %d, want 64", len(got))
	}
}

package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Claves usadas en platform_settings. Las claves de tipo *_key se guardan
// cifradas con AES-GCM; el resto se guarda en texto plano.
const (
	SettingActiveProvider = "ai.provider"   // "claude" | "openai"
	SettingClaudeKey      = "ai.claude.key" // cifrada
	SettingClaudeModel    = "ai.claude.model"
	SettingOpenAIKey      = "ai.openai.key" // cifrada
	SettingOpenAIModel    = "ai.openai.model"
)

var ErrSettingNotFound = errors.New("setting not found")

// SettingsService lee y escribe la tabla platform_settings.
// Los valores de API key se cifran con AES-256-GCM antes de guardarse.
type SettingsService struct {
	db            *pgxpool.Pool
	encryptionKey []byte // 32 bytes para AES-256
}

func NewSettingsService(db *pgxpool.Pool, encryptionKey []byte) (*SettingsService, error) {
	if len(encryptionKey) != 32 {
		return nil, fmt.Errorf("settings: encryption key must be 32 bytes, got %d", len(encryptionKey))
	}
	return &SettingsService{db: db, encryptionKey: encryptionKey}, nil
}

// Get devuelve el valor de texto plano de una clave.
// Si la clave es una API key (*_key), la descifra antes de devolverla.
func (s *SettingsService) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRow(ctx,
		`SELECT value FROM platform_settings WHERE key = $1`, key,
	).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrSettingNotFound
	}
	if err != nil {
		return "", err
	}

	if isEncryptedKey(key) {
		return s.decrypt(value)
	}
	return value, nil
}

// Set guarda un valor. Si la clave es una API key, la cifra antes de guardar.
func (s *SettingsService) Set(ctx context.Context, key, value string) error {
	toStore := value
	if isEncryptedKey(key) {
		var err error
		toStore, err = s.encrypt(value)
		if err != nil {
			return fmt.Errorf("settings: encrypt %s: %w", key, err)
		}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO platform_settings (key, value, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, toStore,
	)
	return err
}

// GetMasked devuelve el valor enmascarado para mostrarlo en la UI.
// Las API keys aparecen como "sk-...•••••••••••••••[últimos 4 chars]".
func (s *SettingsService) GetMasked(ctx context.Context, key string) (string, error) {
	plain, err := s.Get(ctx, key)
	if errors.Is(err, ErrSettingNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !isEncryptedKey(key) {
		return plain, nil
	}
	return maskAPIKey(plain), nil
}

// AIConfig agrupa la configuración activa del proveedor de IA.
type AIConfig struct {
	Provider string
	APIKey   string
	Model    string
}

// GetAIConfig devuelve la configuración del proveedor activo.
// Si no hay configuración guarda, devuelve valores por defecto seguros
// (provider = "claude", model = "claude-sonnet-4-6", key vacía).
func (s *SettingsService) GetAIConfig(ctx context.Context) (*AIConfig, error) {
	provider, err := s.Get(ctx, SettingActiveProvider)
	if errors.Is(err, ErrSettingNotFound) {
		provider = "claude"
	} else if err != nil {
		return nil, err
	}

	var keySettingKey, modelSettingKey, defaultModel string
	switch provider {
	case "openai":
		keySettingKey = SettingOpenAIKey
		modelSettingKey = SettingOpenAIModel
		defaultModel = "gpt-4o"
	default:
		keySettingKey = SettingClaudeKey
		modelSettingKey = SettingClaudeModel
		defaultModel = "claude-sonnet-4-5"
	}

	apiKey, err := s.Get(ctx, keySettingKey)
	if errors.Is(err, ErrSettingNotFound) {
		apiKey = ""
	} else if err != nil {
		return nil, err
	}

	model, err := s.Get(ctx, modelSettingKey)
	if errors.Is(err, ErrSettingNotFound) {
		model = defaultModel
	} else if err != nil {
		return nil, err
	}

	return &AIConfig{Provider: provider, APIKey: apiKey, Model: model}, nil
}

// isEncryptedKey devuelve true si la clave corresponde a un API key
// que debe almacenarse cifrado.
func isEncryptedKey(key string) bool {
	return key == SettingClaudeKey || key == SettingOpenAIKey
}

// encrypt cifra el texto con AES-256-GCM y lo codifica en base64.
func (s *SettingsService) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt descifra un valor cifrado por encrypt.
func (s *SettingsService) decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("settings: decode base64: %w", err)
	}
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("settings: ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("settings: decrypt: %w", err)
	}
	return string(plaintext), nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "••••••••"
	}
	prefix := key[:4]
	suffix := key[len(key)-4:]
	return fmt.Sprintf("%s%s%s", prefix, "••••••••••••", suffix)
}

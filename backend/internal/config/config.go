// Package config centraliza la lectura de variables de entorno.
//
// En lugar de llamar os.Getenv("DB_HOST") en distintos archivos,
// cargamos todo aquí una vez y pasamos el struct a quien lo necesite.
// Esto hace que sea fácil ver qué vars necesita la app y testear con valores distintos.
package config

import (
	"os"
	"strconv"
	"strings"
)

// Config agrupa toda la configuración de la aplicación.
// Se crea una sola vez en main.go y se pasa hacia abajo.
type Config struct {
	Port           string
	DB             DBConfig
	Auth           AuthConfig
	FrontendOrigin string
}

// DBConfig tiene los datos de conexión a PostgreSQL.
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// AuthConfig holds authentication-related configuration.
type AuthConfig struct {
	SuperAdminEmails []string // SUPER_ADMIN_EMAILS, comma-separated
	SessionDurationH int      // SESSION_DURATION_HOURS
	AppEnv           string   // APP_ENV: "development" | "production"
}

// DSN (Data Source Name) arma el string de conexión que pgx espera.
// Ejemplo resultado: "host=localhost port=5432 user=postgres ..."
//
// Es un método en DBConfig (no una función suelta) para que
// siempre que tengas un DBConfig, puedas pedirle su DSN.
func (c DBConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + c.Port +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.Name +
		" sslmode=disable"
}

// Load lee las variables de entorno y devuelve un Config listo.
// Los valores por defecto sirven para desarrollo local sin .env.
func Load() *Config {
	return &Config{
		Port: env("PORT", "8080"),
		DB: DBConfig{
			Host:     env("DB_HOST", "localhost"),
			Port:     env("DB_PORT", "5432"),
			User:     env("DB_USER", "postgres"),
			Password: env("DB_PASSWORD", "postgres"),
			Name:     env("DB_NAME", "appdb"),
		},
		Auth: AuthConfig{
			SuperAdminEmails: parseCSV(env("SUPER_ADMIN_EMAILS", "")),
			SessionDurationH: envInt("SESSION_DURATION_HOURS", 24),
			AppEnv:           env("APP_ENV", "development"),
		},
		FrontendOrigin: env("FRONTEND_ORIGIN", "http://localhost:5173"),
	}
}

// env lee una variable de entorno y devuelve el fallback si no existe.
// Es privada (minúscula) — solo se usa dentro de este package.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

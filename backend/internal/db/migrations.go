package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Migrate aplica todas las migraciones pendientes al arrancar el backend.
//
// Goose lleva su propio registro en la tabla "goose_db_version" — sabe
// exactamente qué archivos SQL ya corrieron y cuáles son nuevos.
// Si no hay nada pendiente, no hace nada. Es seguro llamarlo siempre.
//
// Los archivos SQL deben estar en backend/migrations/ con el formato:
//
//	001_init.sql, 002_add_users.sql, etc.
func Migrate(pool *pgxpool.Pool) error {
	// goose necesita un *sql.DB estándar de Go, no un pgxpool.
	// stdlib.OpenDBFromPool hace la conversión sin abrir una conexión nueva.
	sqlDB := stdlib.OpenDBFromPool(pool)

	_ = goose.SetDialect("postgres") // le decimos que estamos en PostgreSQL

	// "up" significa: aplica todas las migraciones pendientes.
	// La carpeta "migrations" es relativa al directorio donde corre el binario.
	return goose.RunContext(context.Background(), "up", sqlDB, "migrations")
}

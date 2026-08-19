// Package db maneja la conexión a PostgreSQL y las migraciones.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ulloa09/ai-surveys/backend/internal/config"
)

// Connect abre un pool de conexiones a PostgreSQL.
//
// Un "pool" reutiliza conexiones en lugar de abrir una nueva por cada query.
// Recibe config.DBConfig (sin *) porque solo necesitamos leer los valores,
// no modificarlos — Go hace una copia pequeña del struct.
//
// Devuelve (*pgxpool.Pool, error) — patrón estándar de Go:
// si hay error, pool será nil; si no hay error, err será nil.
func Connect(cfg config.DBConfig) (*pgxpool.Pool, error) {
	// pgxpool.New crea el pool pero no abre conexiones todavía
	pool, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		return nil, err
	}

	// Ping verifica que realmente podemos hablar con la base de datos.
	// Sin esto, Connect podría "exitosamente" devolver un pool que no funciona.
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close() // limpiamos antes de devolver el error
		return nil, err
	}

	return pool, nil
}

// Ping verifica si la conexión sigue activa.
// Lo llama el health handler en cada request para reportar el estado real de la DB.
//
// Recibe *pgxpool.Pool (puntero) — todos los que llamen Ping
// están hablando con el mismo pool, no con copias.
func Ping(pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("pool is nil")
	}
	return pool.Ping(context.Background())
}

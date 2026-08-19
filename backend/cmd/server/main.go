package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/ulloa09/ai-surveys/backend/internal/config"
	"github.com/ulloa09/ai-surveys/backend/internal/db"
	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	pool, err := db.Connect(cfg.DB)
	if err != nil {
		log.Fatalf("no se pudo conectar a la base de datos: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		log.Fatalf("error en migraciones: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.FrontendOrigin))

	r.Get("/api/health", handlers.Health(pool))

	log.Printf("backend corriendo en :%s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}

func corsMiddleware(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

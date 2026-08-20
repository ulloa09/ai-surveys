package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/ulloa09/ai-surveys/backend/internal/config"
	"github.com/ulloa09/ai-surveys/backend/internal/db"
	"github.com/ulloa09/ai-surveys/backend/internal/handlers"
	appmw "github.com/ulloa09/ai-surveys/backend/internal/middleware"
	"github.com/ulloa09/ai-surveys/backend/internal/services"
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

	authSvc := services.NewAuthService(pool, cfg.Auth)
	teamSvc := services.NewTeamService(pool)
	questionSvc := services.NewQuestionService(pool)
	surveySvc := services.NewSurveyService(pool, questionSvc, cfg.FrontendOrigin)
	responseSvc := services.NewResponseService(pool)
	settingsSvc, err := services.NewSettingsService(pool, []byte(cfg.EncryptionKey))
	if err != nil {
		log.Fatalf("settings service: %v", err)
	}

	startScheduler(surveySvc)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.FrontendOrigin))

	// ── Rutas públicas ────────────────────────────────────────────────────
	r.Get("/api/health", handlers.Health(pool))
	r.Post("/api/auth/register", handlers.Register(authSvc))
	r.Post("/api/auth/login", handlers.Login(authSvc, cfg.Auth.AppEnv, cfg.Auth.SessionDurationH*3600))
	r.Post("/api/auth/logout", handlers.Logout(authSvc))

	// Encuestas de respondientes. El requisito de sesión depende del
	// anonymity_level de la encuesta, que solo se conoce después de resolver el
	// token — por eso estas rutas usan OptionalAuthenticate (adjunta el usuario
	// si hay sesión válida, pero no rechaza a nadie) y son los handlers quienes
	// aplican el gate:
	//
	//   - 'full' (anónima): link público de verdad. Sin cuenta y sin roster.
	//   - 'partial' / 'none': exigen sesión y membresía del equipo dueño de la
	//     encuesta — ver GetPublicSurvey y CreateResponse.
	r.With(appmw.OptionalAuthenticate(authSvc)).Get("/api/public/surveys/{token}", handlers.GetPublicSurvey(responseSvc))
	r.With(appmw.OptionalAuthenticate(authSvc)).Post("/api/public/surveys/{token}/responses", handlers.CreateResponse(responseSvc, cfg.FingerprintSalt))
	r.Get("/api/public/surveys/{token}/resume", handlers.ResumeResponse(responseSvc))

	// ── Rutas protegidas ─────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(appmw.Authenticate(authSvc))

		r.Get("/api/admin/me", handlers.Me())

		// Settings — solo super_admin
		r.With(appmw.RequireSuperAdmin()).Get("/api/admin/settings", handlers.GetSettings(settingsSvc))
		r.With(appmw.RequireSuperAdmin()).Patch("/api/admin/settings", handlers.UpdateSettings(settingsSvc))
		r.With(appmw.RequireSuperAdmin()).Post("/api/admin/settings/test", handlers.TestConnection(settingsSvc))

		// SSE streaming (admin)
		r.Get("/api/responses/{id}/stream", handlers.StreamTurn(settingsSvc, responseSvc))

		// Dev test
		r.Post("/api/dev/test-ai", handlers.DevTestAI(settingsSvc, cfg.Auth.AppEnv))

		// Teams — reutilizado como concepto de grupo/clase (Fase 3, RBAC).
		// Crear equipos y ver el listado global quedan en RequireAdminRole /
		// bypass de RequireTeamMember (admin y super_admin, ver Rbac.go).
		r.With(appmw.RequireAdminRole()).Post("/api/admin/teams", handlers.CreateTeam(teamSvc))
		r.Get("/api/admin/teams", handlers.ListTeams(teamSvc))

		r.Route("/api/admin/teams/{teamID}", func(r chi.Router) {
			r.Use(appmw.RequireTeamMember(teamSvc))
			r.Get("/", handlers.GetTeam(teamSvc))
			r.With(appmw.RequireAdminRole()).Post("/members", handlers.AddMember(teamSvc))
			r.With(appmw.RequireAdminRole()).Delete("/members/{userID}", handlers.RemoveMember(teamSvc))
			r.With(appmw.RequireAdminRole()).Post("/members/{userID}/move", handlers.MoveMember(teamSvc))

			// Placeholder (Fase 3): estadisticas/insights de IA del equipo — a
			// futuro, acceso de Admin y Profesor limitado solo a sus propios
			// equipos; Alumno queda excluido explicitamente aunque sea miembro.
			r.With(appmw.RequireRole("admin", "profesor")).Get("/analytics", handlers.NotImplemented("team_analytics"))
		})

		// Surveys
		r.Route("/api/surveys", func(r chi.Router) {
			r.With(appmw.RequireRole("admin")).Post("/", handlers.CreateSurvey(surveySvc))
			r.With(appmw.RequireRole("admin", "profesor")).Get("/", handlers.ListSurveys(surveySvc))
			r.With(appmw.RequireRole("admin", "profesor")).Get("/{id}", handlers.GetSurvey(surveySvc))
			r.With(appmw.RequireRole("admin")).Patch("/{id}", handlers.UpdateSurvey(surveySvc))
			r.With(appmw.RequireRole("admin")).Delete("/{id}", handlers.DeleteSurvey(surveySvc))
			r.With(appmw.RequireRole("admin")).Post("/{id}/duplicate", handlers.DuplicateSurvey(surveySvc))

			r.With(appmw.RequireRole("admin")).Post("/{id}/activate", handlers.ActivateSurvey(surveySvc))
			r.With(appmw.RequireRole("admin")).Post("/{id}/close", handlers.CloseSurvey(surveySvc))
			r.With(appmw.RequireRole("admin")).Post("/{id}/reopen", handlers.ReopenSurvey(surveySvc))
			r.With(appmw.RequireSuperAdmin()).Post("/{id}/archive", handlers.ArchiveSurvey(surveySvc))

			r.With(appmw.RequireRole("admin", "profesor")).Get("/{id}/questions", handlers.ListQuestions(questionSvc, surveySvc))
			r.With(appmw.RequireRole("admin")).Post("/{id}/questions", handlers.CreateQuestion(questionSvc, surveySvc))
			r.With(appmw.RequireRole("admin")).Put("/{id}/questions/order", handlers.ReorderQuestions(questionSvc, surveySvc))
			r.With(appmw.RequireRole("admin")).Patch("/{id}/questions/{qid}", handlers.UpdateQuestion(questionSvc, surveySvc))
			r.With(appmw.RequireRole("admin")).Delete("/{id}/questions/{qid}", handlers.DeleteQuestion(questionSvc, surveySvc))
		})
	})

	log.Printf("backend corriendo en :%s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}

// startScheduler arranca el job que aplica transiciones automáticas de
// ciclo de vida (apertura/cierre programados, cierre por tope de respuestas)
// una vez al arrancar y luego cada minuto, en su propia goroutine.
func startScheduler(svc *services.SurveyService) {
	runOnce := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("scheduler: panic recuperado: %v", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := svc.RunScheduledTransitions(ctx); err != nil {
			log.Printf("scheduler: error aplicando transiciones: %v", err)
		}
	}

	go func() {
		runOnce()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			runOnce()
		}
	}()
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

package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/ramonskie/prunarr/internal/api/handlers"
	mw "github.com/ramonskie/prunarr/internal/api/middleware"
	"github.com/ramonskie/prunarr/internal/services"
	"github.com/ramonskie/prunarr/internal/storage"
)

// RouterDependencies holds dependencies for the router
type RouterDependencies struct {
	AuthService *services.AuthService
	SyncEngine  *services.SyncEngine
	JobsFile    *storage.JobsFile
	SPAHandler  http.Handler // Optional: handler for serving the SPA frontend
}

// NewRouter creates and configures the HTTP router
func NewRouter(deps *RouterDependencies) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(mw.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler(deps.AuthService)
	mediaHandler := handlers.NewMediaHandler(deps.SyncEngine)
	syncHandler := handlers.NewSyncHandler(deps.SyncEngine)
	jobsHandler := handlers.NewJobsHandler(deps.JobsFile)
	configHandler := handlers.NewConfigHandler(deps.SyncEngine)
	rulesHandler := handlers.NewRulesHandler()

	// Public routes
	r.Get("/health", healthHandler.Handle)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public API routes
		r.Post("/auth/login", authHandler.Login)

		// Protected API routes
		r.Group(func(r chi.Router) {
			r.Use(mw.Auth)

			// Media routes - specific endpoints before parameterized {id}
			r.Route("/media", func(r chi.Router) {
				r.Get("/movies", mediaHandler.ListMovies)
				r.Get("/shows", mediaHandler.ListShows)
				r.Get("/leaving-soon", mediaHandler.ListLeavingSoon)
				r.Get("/unmatched", mediaHandler.ListUnmatched)

				// Parameterized routes must come last
				r.Get("/{id}", mediaHandler.GetMediaItem)
				r.Post("/{id}/exclude", mediaHandler.AddExclusion)
				r.Delete("/{id}/exclude", mediaHandler.RemoveExclusion)
				r.Delete("/{id}", mediaHandler.DeleteMedia)
			})

			// Sync routes
			r.Post("/sync/full", syncHandler.TriggerFullSync)
			r.Post("/sync/incremental", syncHandler.TriggerIncrementalSync)
			r.Get("/sync/status", syncHandler.GetSyncStatus)

			// Deletion routes
			r.Post("/deletions/execute", syncHandler.ExecuteDeletions)

			// Jobs routes
			r.Get("/jobs", jobsHandler.ListJobs)
			r.Get("/jobs/latest", jobsHandler.GetLatestJob)
			r.Get("/jobs/{id}", jobsHandler.GetJob)

			// Config routes
			r.Get("/config", configHandler.GetConfig)
			r.Put("/config", configHandler.UpdateConfig)

			// Rules routes
			r.Get("/rules", rulesHandler.ListRules)
			r.Post("/rules", rulesHandler.CreateRule)
			r.Put("/rules/{name}", rulesHandler.UpdateRule)
			r.Delete("/rules/{name}", rulesHandler.DeleteRule)
			r.Patch("/rules/{name}/toggle", rulesHandler.ToggleRule)
		})
	})

	// Mount SPA handler for frontend (if provided)
	if deps.SPAHandler != nil {
		r.Handle("/*", deps.SPAHandler)
	}

	return r
}

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

	// Public routes
	r.Get("/health", healthHandler.Handle)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public API routes
		r.Post("/auth/login", authHandler.Login)

		// Protected API routes
		r.Group(func(r chi.Router) {
			r.Use(mw.Auth)

			// Media routes
			r.Get("/media/movies", mediaHandler.ListMovies)
			r.Get("/media/shows", mediaHandler.ListShows)
			r.Get("/media/leaving-soon", mediaHandler.ListLeavingSoon)
			r.Get("/media/{id}", mediaHandler.GetMediaItem)
			r.Post("/media/{id}/exclude", mediaHandler.AddExclusion)
			r.Delete("/media/{id}/exclude", mediaHandler.RemoveExclusion)
			r.Delete("/media/{id}", mediaHandler.DeleteMedia)

			// Sync routes
			r.Post("/sync/full", syncHandler.TriggerFullSync)
			r.Post("/sync/incremental", syncHandler.TriggerIncrementalSync)
			r.Get("/sync/status", syncHandler.GetSyncStatus)

			// Jobs routes
			r.Get("/jobs", jobsHandler.ListJobs)
			r.Get("/jobs/latest", jobsHandler.GetLatestJob)
			r.Get("/jobs/{id}", jobsHandler.GetJob)
		})
	})

	// Mount SPA handler for frontend (if provided)
	if deps.SPAHandler != nil {
		r.Handle("/*", deps.SPAHandler)
	}

	return r
}

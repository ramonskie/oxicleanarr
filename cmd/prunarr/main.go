package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ramonskie/prunarr/internal/api"
	"github.com/ramonskie/prunarr/internal/api/handlers"
	"github.com/ramonskie/prunarr/internal/cache"
	"github.com/ramonskie/prunarr/internal/config"
	"github.com/ramonskie/prunarr/internal/services"
	"github.com/ramonskie/prunarr/internal/storage"
	"github.com/ramonskie/prunarr/internal/utils"
	"github.com/rs/zerolog/log"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	logFormat := getEnv("LOG_FORMAT", "json")
	utils.InitLogger(logLevel, logFormat)

	log.Info().Msg("Starting Prunarr...")

	// Load configuration (priority: flag > env var > default)
	configPathValue := *configPath
	if configPathValue == "" {
		configPathValue = getEnv("CONFIG_PATH", "")
	}
	cfg, err := config.Load(configPathValue)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().
		Bool("dry_run", cfg.App.DryRun).
		Int("leaving_soon_days", cfg.App.LeavingSoonDays).
		Str("config_path", configPathValue).
		Msg("Configuration loaded")

	// Initialize JWT
	jwtSecret := getEnv("JWT_SECRET", "")
	jwtExpiry, _ := time.ParseDuration(getEnv("JWT_EXPIRATION", "24h"))
	utils.InitJWT(jwtSecret, jwtExpiry)

	// Initialize storage
	dataPath := getEnv("DATA_PATH", "./data")
	exclusionsFile, err := storage.NewExclusionsFile(dataPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize exclusions storage")
	}
	log.Info().Int("exclusions", len(exclusionsFile.GetAll())).Msg("Exclusions loaded")

	jobsFile, err := storage.NewJobsFile(dataPath, 100)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize jobs storage")
	}
	log.Info().Int("jobs", len(jobsFile.GetAll())).Msg("Jobs loaded")

	// Initialize cache
	appCache := cache.New()
	log.Info().Msg("Cache initialized")

	// Initialize services
	authService := services.NewAuthService(cfg)
	log.Info().Msg("Authentication service initialized")

	// Initialize rules engine
	rulesEngine := services.NewRulesEngine(cfg, exclusionsFile)
	rulesEngine.UseGlobalConfig() // Enable hot-reload support
	log.Info().Msg("Rules engine initialized")

	// Initialize sync engine
	syncEngine := services.NewSyncEngine(cfg, appCache, jobsFile, exclusionsFile, rulesEngine)
	log.Info().Msg("Sync engine initialized")

	// Start sync engine scheduler
	if err := syncEngine.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start sync engine")
	}
	log.Info().Msg("Sync engine started")

	// Initialize SPA handler for serving frontend
	distPath := getEnv("FRONTEND_DIST_PATH", "./web/dist")
	spaHandler, err := handlers.NewSPAHandler(distPath)
	if err != nil {
		log.Warn().Err(err).Msg("Frontend not available, running in API-only mode")
	}

	// Create router with dependencies
	router := api.NewRouter(&api.RouterDependencies{
		AuthService: authService,
		SyncEngine:  syncEngine,
		JobsFile:    jobsFile,
		SPAHandler:  spaHandler,
	})
	log.Info().Msg("Router initialized")

	// Start config watcher for hot-reload
	if err := config.StartWatcher(func() {
		log.Info().Msg("Configuration reloaded, clearing cache and reapplying retention rules")
		appCache.Clear()
		syncEngine.ReapplyRetentionRules()
	}); err != nil {
		log.Warn().Err(err).Msg("Failed to start config watcher, hot-reload disabled")
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info().
			Str("address", addr).
			Msg("HTTP server starting")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	log.Info().Msg("Prunarr started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Stop sync engine
	syncEngine.Stop()
	log.Info().Msg("Sync engine stopped")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped")
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"stockmarket/internal/api"
	"stockmarket/internal/config"
	"stockmarket/internal/db"
	"stockmarket/internal/web"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create template renderer
	templates, err := web.NewTemplates(database)
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Create API server
	apiServer := api.NewServer(database, cfg)

	// Setup routes
	mux := http.NewServeMux()
	
	// API routes
	apiServer.SetupRoutes(mux)

	// Page routes (Go templates + HTMX)
	mux.HandleFunc("/", templates.Dashboard)
	mux.HandleFunc("/analysis", templates.Analysis)
	mux.HandleFunc("/analysis/", templates.Analysis)
	mux.HandleFunc("/recommendations", templates.Recommendations)
	mux.HandleFunc("/alerts", templates.Alerts)
	mux.HandleFunc("/settings", templates.Settings)

	// Partial routes for HTMX
	mux.HandleFunc("/partials/watchlist", templates.PartialWatchlist)
	mux.HandleFunc("/partials/recommendations", templates.PartialRecommendations)
	mux.HandleFunc("/partials/recommendations-list", templates.PartialRecommendationsList)
	mux.HandleFunc("/partials/analysis-history", templates.PartialAnalysisHistory)
	mux.HandleFunc("/partials/analysis-detail/", templates.PartialAnalysisDetail)
	mux.HandleFunc("/partials/alerts-list", templates.PartialAlertsList)
	mux.HandleFunc("/partials/quick-analyze", templates.PartialQuickAnalyze)
	mux.HandleFunc("/partials/watchlist-alert-buttons", templates.PartialWatchlistAlertButtons)

	// Add CORS middleware
	handler := corsMiddleware(mux)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		httpServer.Close()
	}()

	log.Printf("Starting server on port %s", cfg.Port)
	log.Printf("Environment: %s", cfg.Environment)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, HX-Request, HX-Target, HX-Trigger")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

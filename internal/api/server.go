package api

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"stockmarket/internal/config"
	"stockmarket/internal/db"
	"stockmarket/internal/notify"
)

const (
	// HTTP Headers
	HEADER_CONTENT_TYPE = "Content-Type"

	// Content Types
	CONTENT_TYPE_HTML = "text/html"
	CONTENT_TYPE_JSON = "application/json"

	// HTTP Status Codes
	METHOD_NOT_ALLOWED = "Method not allowed"

	// Validation Errors
	INVALID_JSON      = "Invalid JSON"
	INVALID_FORM_DATA = "Invalid form data"

	// Errors
	ALL_FIELDS_REQUIRED           = "All fields are required"
	FAILED_TO_DECRYPT_API_KEY     = "Failed to decrypt API key"
	FAILED_TO_ENCRYPT_API_KEY     = "Failed to encrypt API key"
	FAILED_TO_GET_ANALYZE         = "Failed to get analyze"
	FAILED_TO_GET_CONFIG          = "Failed to get config"
	FAILED_TO_GET_HISTORICAL_DATA = "Failed to get historical data"
	FAILED_TO_GET_QUOTE           = "Failed to get quote"
	FAILED_TO_UPDATE_CONFIG       = "Failed to update config"
	INVALID_ALERT_ID              = "Invalid alert ID"
	INVALID_POLLING_INTERVAL      = "Invalid polling interval"
	INVALID_PRICE                 = "Invalid price"
	SYMBOL_REQUIRED               = "Symbol is required"
)

// Server holds the API server dependencies
type Server struct {
	db            *db.DB
	config        *config.Config
	notifyService *notify.Service
	clients       map[*websocket.Conn]bool
	clientsMu     sync.RWMutex
	upgrader      websocket.Upgrader
}

// NewServer creates a new API server
func NewServer(database *db.DB, cfg *config.Config) *Server {
	// Initialize notification service with notifiers
	notifyService := notify.NewService()
	notifyService.RegisterNotifier(notify.NewEmailNotifier(map[string]string{}))
	notifyService.RegisterNotifier(notify.NewDiscordNotifier())
	notifyService.RegisterNotifier(notify.NewSMSNotifier(map[string]string{}))

	return &Server{
		db:            database,
		config:        cfg,
		notifyService: notifyService,
		clients:       make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
	}
}

// SetupRoutes sets up all API routes
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	// Health check
	mux.HandleFunc("/api/health", s.handleHealth)

	// Configuration (JSON API)
	mux.HandleFunc("/api/config", s.handleConfig)

	// Configuration (HTMX form handlers)
	mux.HandleFunc("/api/config/market", s.handleConfigMarket)
	mux.HandleFunc("/api/config/ai", s.handleConfigAI)
	mux.HandleFunc("/api/config/strategy", s.handleConfigStrategy)
	mux.HandleFunc("/api/config/watchlist", s.handleConfigWatchlist)
	mux.HandleFunc("/api/config/watchlist/", s.handleConfigWatchlistSymbol)
	mux.HandleFunc("/api/config/polling", s.handleConfigPolling)
	mux.HandleFunc("/api/config/notifications", s.handleConfigNotifications)

	// Market data
	mux.HandleFunc("/api/quote/", s.handleQuote)
	mux.HandleFunc("/api/historical/", s.handleHistorical)

	// Analysis (JSON API)
	mux.HandleFunc("/api/analyze/", s.handleAnalyze)
	mux.HandleFunc("/api/analyses", s.handleAnalyses)
	mux.HandleFunc("/api/analyses/", s.handleAnalysesForSymbol)

	// Analysis (HTMX)
	mux.HandleFunc("/api/analyze", s.handleAnalyzeHTMX)

	// Alerts (JSON API)
	mux.HandleFunc("/api/alerts", s.handleAlertsHTMX)       // Changed to HTMX handler
	mux.HandleFunc("/api/alerts/", s.handleAlertDeleteHTMX) // Changed to HTMX handler

	// Notification channels
	mux.HandleFunc("/api/notification-channels", s.handleNotificationChannels)
	mux.HandleFunc("/api/notification-channels/", s.handleNotificationChannelDelete)

	// WebSocket for real-time updates
	mux.HandleFunc("/api/ws", s.handleWebSocket)

	// Risk and frequency profiles
	mux.HandleFunc("/api/profiles", s.handleProfiles)
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

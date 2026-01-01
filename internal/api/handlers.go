package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"stockmarket/internal/ai"
	"stockmarket/internal/config"
	"stockmarket/internal/db"
	"stockmarket/internal/market"
	"stockmarket/internal/models"
	"stockmarket/internal/notify"
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
	mux.HandleFunc("/api/alerts", s.handleAlertsHTMX) // Changed to HTMX handler
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

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// handleHealth returns server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleConfig handles configuration CRUD
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := s.db.GetOrCreateConfig()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Decrypt API keys for response (masked)
		if cfg.MarketDataAPIKey != "" {
			key, _ := config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
			if len(key) > 4 {
				cfg.MarketDataAPIKey = key[:4] + "****" + key[len(key)-4:]
			}
		}
		if cfg.AIProviderAPIKey != "" {
			key, _ := config.Decrypt(cfg.AIProviderAPIKey, s.config.EncryptionKey)
			if len(key) > 4 {
				cfg.AIProviderAPIKey = key[:4] + "****" + key[len(key)-4:]
			}
		}

		respondJSON(w, http.StatusOK, cfg)

	case http.MethodPut:
		var input struct {
			MarketDataProvider string   `json:"market_data_provider"`
			MarketDataAPIKey   string   `json:"market_data_api_key"`
			AIProvider         string   `json:"ai_provider"`
			AIProviderAPIKey   string   `json:"ai_provider_api_key"`
			AIModel            string   `json:"ai_model"`
			RiskTolerance      string   `json:"risk_tolerance"`
			TradeFrequency     string   `json:"trade_frequency"`
			TrackedSymbols     []string `json:"tracked_symbols"`
		}

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		cfg, err := s.db.GetOrCreateConfig()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Update fields
		if input.MarketDataProvider != "" {
			cfg.MarketDataProvider = input.MarketDataProvider
		}
		if input.MarketDataAPIKey != "" && !strings.Contains(input.MarketDataAPIKey, "****") {
			encrypted, _ := config.Encrypt(input.MarketDataAPIKey, s.config.EncryptionKey)
			cfg.MarketDataAPIKey = encrypted
		}
		if input.AIProvider != "" {
			cfg.AIProvider = input.AIProvider
		}
		if input.AIProviderAPIKey != "" && !strings.Contains(input.AIProviderAPIKey, "****") {
			encrypted, _ := config.Encrypt(input.AIProviderAPIKey, s.config.EncryptionKey)
			cfg.AIProviderAPIKey = encrypted
		}
		if input.AIModel != "" {
			cfg.AIModel = input.AIModel
		}
		if input.RiskTolerance != "" {
			cfg.RiskTolerance = input.RiskTolerance
		}
		if input.TradeFrequency != "" {
			cfg.TradeFrequency = input.TradeFrequency
		}
		if input.TrackedSymbols != nil {
			// Normalize symbols to uppercase
			for i := range input.TrackedSymbols {
				input.TrackedSymbols[i] = strings.ToUpper(strings.TrimSpace(input.TrackedSymbols[i]))
			}
			cfg.TrackedSymbols = input.TrackedSymbols
		}

		if err := s.db.UpdateConfig(cfg); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleQuote fetches a stock quote
func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/quote/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, "Symbol required")
		return
	}
	symbol = strings.ToUpper(symbol)

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Decrypt API key
	apiKey := ""
	if cfg.MarketDataAPIKey != "" {
		apiKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}

	provider, err := market.NewProvider(cfg.MarketDataProvider, apiKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	quote, err := provider.GetQuote(ctx, symbol)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, quote)
}

// handleHistorical fetches historical data
func (s *Server) handleHistorical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/historical/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, "Symbol required")
		return
	}
	symbol = strings.ToUpper(symbol)

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1m" // Default to 1 month
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	apiKey := ""
	if cfg.MarketDataAPIKey != "" {
		apiKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}

	provider, err := market.NewProvider(cfg.MarketDataProvider, apiKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	candles, err := provider.GetHistoricalData(ctx, symbol, period)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, candles)
}

// handleAnalyze triggers AI analysis for a symbol
func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/analyze/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, "Symbol required")
		return
	}
	symbol = strings.ToUpper(symbol)

	var input struct {
		UserContext string `json:"user_context"`
	}
	json.NewDecoder(r.Body).Decode(&input)

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get market data
	marketAPIKey := ""
	if cfg.MarketDataAPIKey != "" {
		marketAPIKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}

	provider, err := market.NewProvider(cfg.MarketDataProvider, marketAPIKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Market provider error: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	quote, err := provider.GetQuote(ctx, symbol)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to get quote: "+err.Error())
		return
	}

	historical, err := provider.GetHistoricalData(ctx, symbol, "1m")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Failed to get historical data: "+err.Error())
		return
	}

	// Get AI analyzer
	aiAPIKey := ""
	if cfg.AIProviderAPIKey != "" {
		aiAPIKey, _ = config.Decrypt(cfg.AIProviderAPIKey, s.config.EncryptionKey)
	}

	analyzer, err := ai.NewAnalyzer(cfg.AIProvider, aiAPIKey, cfg.AIModel)
	if err != nil {
		respondError(w, http.StatusBadRequest, "AI provider error: "+err.Error())
		return
	}

	// Perform analysis
	analysisReq := models.AnalysisRequest{
		Symbol:         symbol,
		CurrentPrice:   quote.Price,
		HistoricalData: historical,
		RiskProfile:    cfg.RiskTolerance,
		TradeFrequency: cfg.TradeFrequency,
		UserContext:    input.UserContext,
	}

	analysis, err := analyzer.Analyze(ctx, analysisReq)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Analysis failed: "+err.Error())
		return
	}

	// Save analysis
	if err := s.db.SaveAnalysis(analysis); err != nil {
		log.Printf("Failed to save analysis: %v", err)
	}

	// Send notifications if action is BUY or SELL with high confidence
	if (analysis.Action == "BUY" || analysis.Action == "SELL") && analysis.Confidence >= 0.7 {
		notification := models.Notification{
			Type:    strings.ToLower(analysis.Action) + "_signal",
			Title:   fmt.Sprintf("%s Signal: %s", analysis.Action, symbol),
			Message: analysis.Reasoning,
			Symbol:  symbol,
		}
		go s.notifyService.SendToChannels(notification, cfg.NotificationChannels)
	}

	respondJSON(w, http.StatusOK, analysis)
}


// handleAnalyses returns recent analysis results
func (s *Server) handleAnalyses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	analyses, err := s.db.GetRecentAnalyses(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, analyses)
}

// handleAnalysesForSymbol returns analyses for a specific symbol
func (s *Server) handleAnalysesForSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/analyses/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, "Symbol required")
		return
	}
	symbol = strings.ToUpper(symbol)

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	analyses, err := s.db.GetAnalysesForSymbol(symbol, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, analyses)
}

// handleAlerts handles price alerts CRUD
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		alerts, err := s.db.GetActiveAlerts()
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, alerts)

	case http.MethodPost:
		var alert models.PriceAlert
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		alert.Symbol = strings.ToUpper(strings.TrimSpace(alert.Symbol))
		if alert.Symbol == "" || alert.Price <= 0 {
			respondError(w, http.StatusBadRequest, "Symbol and price required")
			return
		}
		if alert.Condition != "above" && alert.Condition != "below" {
			respondError(w, http.StatusBadRequest, "Condition must be 'above' or 'below'")
			return
		}

		if err := s.db.SavePriceAlert(&alert); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, alert)

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleAlertDelete deletes a price alert
func (s *Server) handleAlertDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid alert ID")
		return
	}

	if err := s.db.DeletePriceAlert(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleNotificationChannels handles notification channel CRUD
func (s *Server) handleNotificationChannels(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		respondJSON(w, http.StatusOK, cfg.NotificationChannels)

	case http.MethodPost:
		var channel models.NotificationConfig
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		if channel.Type == "" || channel.Target == "" {
			respondError(w, http.StatusBadRequest, "Type and target required")
			return
		}

		if err := s.db.SaveNotificationChannel(cfg.ID, &channel); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, channel)

	case http.MethodPut:
		var channel models.NotificationConfig
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid JSON")
			return
		}

		if channel.ID == 0 {
			respondError(w, http.StatusBadRequest, "Channel ID required")
			return
		}

		if err := s.db.SaveNotificationChannel(cfg.ID, &channel); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, channel)

	default:
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleNotificationChannelDelete deletes a notification channel
func (s *Server) handleNotificationChannelDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/notification-channels/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	if err := s.db.DeleteNotificationChannel(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleProfiles returns available risk and frequency profiles
func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"risk_profiles":      models.RiskProfiles,
		"frequency_profiles": models.TradeFrequencyProfiles,
	})
}

// handleWebSocket handles WebSocket connections for real-time updates
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		conn.Close()
	}()

	// Get user config for tracked symbols
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		log.Printf("Failed to get config: %v", err)
		return
	}

	if len(cfg.TrackedSymbols) == 0 {
		// Send initial message
		conn.WriteJSON(map[string]string{"type": "info", "message": "No symbols tracked"})
		// Keep connection alive, wait for updates
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
		return
	}

	// Decrypt API key
	apiKey := ""
	if cfg.MarketDataAPIKey != "" {
		apiKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}

	// Create market data provider
	provider, err := market.NewProvider(cfg.MarketDataProvider, apiKey)
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "Provider error: " + err.Error()})
		return
	}

	// Create quote channel
	quoteCh := make(chan models.Quote, 100)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start streaming quotes
	go func() {
		err := provider.StreamQuotes(ctx, cfg.TrackedSymbols, quoteCh)
		if err != nil && err != context.Canceled {
			log.Printf("Stream error: %v", err)
		}
	}()

	// Read goroutine to detect client disconnect
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Check alerts in the background
	go s.checkPriceAlerts(ctx, quoteCh, cfg)

	// Send quotes to client
	for {
		select {
		case <-ctx.Done():
			return
		case quote := <-quoteCh:
			msg := map[string]interface{}{
				"type":  "quote",
				"quote": quote,
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

// checkPriceAlerts checks if any price alerts should be triggered
func (s *Server) checkPriceAlerts(ctx context.Context, quoteCh chan models.Quote, cfg *models.UserConfig) {
	for {
		select {
		case <-ctx.Done():
			return
		case quote := <-quoteCh:
			alerts, err := s.db.GetActiveAlerts()
			if err != nil {
				continue
			}

			for _, alert := range alerts {
				if alert.Symbol != quote.Symbol {
					continue
				}

				var triggered bool
				switch alert.Condition {
				case "above":
					triggered = quote.Price >= alert.Price
				case "below":
					triggered = quote.Price <= alert.Price
				}

				if triggered {
					s.db.TriggerAlert(alert.ID)
					notification := models.Notification{
						Type:    "price_alert",
						Title:   fmt.Sprintf("Price Alert: %s", alert.Symbol),
						Message: fmt.Sprintf("%s is now $%.2f (%s $%.2f)", alert.Symbol, quote.Price, alert.Condition, alert.Price),
						Symbol:  alert.Symbol,
					}
					go s.notifyService.SendToChannels(notification, cfg.NotificationChannels)
				}
			}
		}
	}
}

// BroadcastToClients sends a message to all connected WebSocket clients
func (s *Server) BroadcastToClients(msg interface{}) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for conn := range s.clients {
		conn.WriteJSON(msg)
	}
}

// handleConfigMarket handles market data provider settings (form data for HTMX)
func (s *Server) handleConfigMarket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	provider := r.FormValue("market_data_provider")
	apiKey := r.FormValue("market_data_api_key")

	if provider != "" {
		cfg.MarketDataProvider = provider
	}
	if apiKey != "" {
		encrypted, err := config.Encrypt(apiKey, s.config.EncryptionKey)
		if err != nil {
			htmxError(w, "Failed to encrypt API key")
			return
		}
		cfg.MarketDataAPIKey = encrypted
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, err.Error())
		return
	}

	htmxSuccess(w, "Market settings saved")
}

// handleConfigAI handles AI provider settings (form data for HTMX)
func (s *Server) handleConfigAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	provider := r.FormValue("ai_provider")
	model := r.FormValue("ai_model")
	apiKey := r.FormValue("ai_provider_api_key")

	if provider != "" {
		cfg.AIProvider = provider
	}
	if model != "" {
		cfg.AIModel = model
	}
	if apiKey != "" {
		encrypted, err := config.Encrypt(apiKey, s.config.EncryptionKey)
		if err != nil {
			htmxError(w, "Failed to encrypt API key")
			return
		}
		cfg.AIProviderAPIKey = encrypted
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, err.Error())
		return
	}

	htmxSuccess(w, "AI settings saved")
}

// handleConfigStrategy handles trading strategy settings (form data for HTMX)
func (s *Server) handleConfigStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	riskTolerance := r.FormValue("risk_tolerance")
	tradeFrequency := r.FormValue("trade_frequency")

	if riskTolerance != "" {
		cfg.RiskTolerance = riskTolerance
	}
	if tradeFrequency != "" {
		cfg.TradeFrequency = tradeFrequency
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, err.Error())
		return
	}

	htmxSuccess(w, "Strategy saved")
}

// handleConfigWatchlist handles watchlist settings (form data for HTMX)
func (s *Server) handleConfigWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	symbolsStr := r.FormValue("tracked_symbols")
	if symbolsStr != "" {
		symbols := []string{}
		for _, s := range strings.Split(symbolsStr, ",") {
			s = strings.TrimSpace(strings.ToUpper(s))
			if s != "" {
				symbols = append(symbols, s)
			}
		}
		cfg.TrackedSymbols = symbols
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, err.Error())
		return
	}

	htmxSuccess(w, "Watchlist saved")
}

// handleConfigWatchlistSymbol handles adding/deleting individual watchlist symbols
func (s *Server) handleConfigWatchlistSymbol(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/api/config/watchlist/")
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	if symbol == "" {
		htmxError(w, "Symbol required")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	switch r.Method {
	case http.MethodPost:
		// Add symbol if not already in list
		found := false
		for _, s := range cfg.TrackedSymbols {
			if s == symbol {
				found = true
				break
			}
		}
		if !found {
			cfg.TrackedSymbols = append(cfg.TrackedSymbols, symbol)
		}

	case http.MethodDelete:
		// Remove symbol from list
		newSymbols := []string{}
		for _, s := range cfg.TrackedSymbols {
			if s != symbol {
				newSymbols = append(newSymbols, s)
			}
		}
		cfg.TrackedSymbols = newSymbols

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, err.Error())
		return
	}

	// Return updated watchlist partial
	s.renderWatchlistSettings(w, cfg.TrackedSymbols)
}

// renderWatchlistSettings renders the watchlist-settings partial
func (s *Server) renderWatchlistSettings(w http.ResponseWriter, symbols []string) {
	w.Header().Set("Content-Type", "text/html")

	if len(symbols) == 0 {
		w.Write([]byte(`<div class="text-center py-6">
    <p class="text-sm text-content-muted">No symbols in watchlist</p>
</div>`))
		return
	}

	html := `<div class="space-y-2">`
	for _, sym := range symbols {
		html += fmt.Sprintf(`
  <div class="flex items-center justify-between p-3 bg-bg-tertiary/50 rounded-lg border border-border group hover:border-accent/30 transition-all duration-200">
    <div class="flex items-center gap-3">
      <span class="font-mono font-semibold text-content-primary">%s</span>
    </div>
    <button
      hx-delete="/api/config/watchlist/%s"
      hx-target="#watchlist-items"
      hx-swap="innerHTML"
      hx-confirm="Remove %s from watchlist?"
      class="p-1.5 text-content-muted hover:text-negative hover:bg-negative-bg/50 rounded-lg opacity-0 group-hover:opacity-100 transition-all duration-200"
      aria-label="Remove %s"
    >
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>`, sym, sym, sym, sym)
	}
	html += `\n</div>`

	w.Write([]byte(html))
}

// handleConfigPolling handles polling interval settings
func (s *Server) handleConfigPolling(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	intervalStr := r.FormValue("polling_interval")
	if intervalStr != "" {
		interval, err := strconv.Atoi(intervalStr)
		if err != nil || interval < 5 || interval > 300 {
			htmxError(w, "Polling interval must be between 5 and 300 seconds")
			return
		}
		cfg.PollingInterval = interval
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, err.Error())
		return
	}

	htmxSuccess(w, "Polling settings saved")
}

// handleConfigNotifications handles notification settings (form data for HTMX)
func (s *Server) handleConfigNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	// Handle email
	emailAddr := r.FormValue("email_address")
	emailEnabled := r.FormValue("email_enabled") == "on"
	if emailAddr != "" || emailEnabled {
		s.updateNotificationChannel(cfg.ID, "email", emailAddr, emailEnabled)
	}

	// Handle discord
	discordWebhook := r.FormValue("discord_webhook")
	discordEnabled := r.FormValue("discord_enabled") == "on"
	if discordWebhook != "" || discordEnabled {
		s.updateNotificationChannel(cfg.ID, "discord", discordWebhook, discordEnabled)
	}

	// Handle SMS
	smsPhone := r.FormValue("sms_phone")
	smsEnabled := r.FormValue("sms_enabled") == "on"
	if smsPhone != "" || smsEnabled {
		s.updateNotificationChannel(cfg.ID, "sms", smsPhone, smsEnabled)
	}

	htmxSuccess(w, "Notification settings saved")
}

func (s *Server) updateNotificationChannel(configID int64, channelType, target string, enabled bool) {
	channels, _ := s.db.GetNotificationChannels(configID)
	
	var existing *models.NotificationConfig
	for i := range channels {
		if channels[i].Type == channelType {
			existing = &channels[i]
			break
		}
	}

	if existing != nil {
		existing.Target = target
		existing.Enabled = enabled
		s.db.SaveNotificationChannel(configID, existing)
	} else if target != "" {
		ch := &models.NotificationConfig{
			Type:    channelType,
			Target:  target,
			Enabled: enabled,
		}
		s.db.SaveNotificationChannel(configID, ch)
	}
}

// handleAnalyzeHTMX handles stock analysis for HTMX (returns HTML partial)
func (s *Server) handleAnalyzeHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">Invalid form data</div>`))
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	userContext := r.FormValue("context")

	if symbol == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">Symbol is required</div>`))
		return
	}

	// Get config
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">Failed to load config</div>`))
		return
	}

	// Get market data
	marketAPIKey := ""
	if cfg.MarketDataAPIKey != "" {
		marketAPIKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}
	provider, err := market.NewProvider(cfg.MarketDataProvider, marketAPIKey)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">Market provider error: ` + err.Error() + `</div>`))
		return
	}

	quote, err := provider.GetQuote(r.Context(), symbol)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">Failed to get quote: ` + err.Error() + `</div>`))
		return
	}

	historical, _ := provider.GetHistoricalData(r.Context(), symbol, "1d")

	// Get AI analyzer
	aiAPIKey := cfg.AIProviderAPIKey
	if aiAPIKey != "" {
		aiAPIKey, _ = config.Decrypt(aiAPIKey, s.config.EncryptionKey)
	}
	
	analyzer, err := ai.NewAnalyzer(cfg.AIProvider, aiAPIKey, cfg.AIModel)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">AI provider error: ` + err.Error() + `</div>`))
		return
	}

	// Run analysis
	analysisReq := models.AnalysisRequest{
		Symbol:         symbol,
		CurrentPrice:   quote.Price,
		HistoricalData: historical,
		RiskProfile:    cfg.RiskTolerance,
		TradeFrequency: cfg.TradeFrequency,
		UserContext:    userContext,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	result, err := analyzer.Analyze(ctx, analysisReq)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="text-red-400 p-4">Analysis failed: ` + err.Error() + `</div>`))
		return
	}

	// Save to database
	s.db.SaveAnalysis(result)

	// Return HTML partial
	w.Header().Set("Content-Type", "text/html")
	html := fmt.Sprintf(`
<div class="bg-slate-800 rounded-xl border border-slate-700 p-6">
    <div class="flex items-start justify-between mb-6">
        <div>
            <h2 class="text-2xl font-bold text-white">%s Analysis</h2>
            <p class="text-slate-400 text-sm">%s</p>
        </div>
        <span class="px-4 py-2 rounded-lg text-lg font-bold %s">
            %s
        </span>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div class="bg-slate-700/50 rounded-lg p-4">
            <div class="text-slate-400 text-sm">Confidence</div>
            <div class="text-2xl font-bold text-white">%.0f%%</div>
        </div>
        <div class="bg-slate-700/50 rounded-lg p-4">
            <div class="text-slate-400 text-sm">Current Price</div>
            <div class="text-2xl font-bold text-white">$%.2f</div>
        </div>
        <div class="bg-slate-700/50 rounded-lg p-4">
            <div class="text-slate-400 text-sm">Timeframe</div>
            <div class="text-2xl font-bold text-white">%s</div>
        </div>
    </div>

    <div class="mb-6">
        <h3 class="text-lg font-semibold text-white mb-3">AI Analysis</h3>
        <div class="bg-slate-700/50 rounded-lg p-4 text-slate-300 whitespace-pre-wrap">%s</div>
    </div>
</div>
`, result.Symbol, time.Now().Format("January 02, 2006 at 15:04"),
		getActionClass(result.Action), result.Action,
		result.Confidence*100, quote.Price, result.Timeframe, result.Reasoning)
	
	w.Write([]byte(html))
}

func getActionClass(action string) string {
	switch action {
	case "BUY":
		return "bg-green-500/20 text-green-400"
	case "SELL":
		return "bg-red-500/20 text-red-400"
	case "HOLD":
		return "bg-yellow-500/20 text-yellow-400"
	default:
		return "bg-blue-500/20 text-blue-400"
	}
}

// handleAlertsHTMX handles creating alerts from HTMX forms
func (s *Server) handleAlertsHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, "Invalid form data")
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	condition := r.FormValue("condition")
	priceStr := r.FormValue("target_price")

	if symbol == "" || condition == "" || priceStr == "" {
		htmxError(w, "All fields are required")
		return
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		htmxError(w, "Invalid price")
		return
	}

	alert := &models.PriceAlert{
		Symbol:    symbol,
		Condition: condition,
		Price:     price,
	}

	if err := s.db.SavePriceAlert(alert); err != nil {
		htmxError(w, err.Error())
		return
	}

	// Return updated alerts list
	s.renderAlertsList(w)
}

// handleAlertDeleteHTMX handles deleting alerts and returns updated list
func (s *Server) handleAlertDeleteHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/alerts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		htmxError(w, "Invalid alert ID")
		return
	}

	if err := s.db.DeletePriceAlert(id); err != nil {
		htmxError(w, err.Error())
		return
	}

	s.renderAlertsList(w)
}

func (s *Server) renderAlertsList(w http.ResponseWriter) {
	alerts, _ := s.db.GetActiveAlerts()
	
	w.Header().Set("Content-Type", "text/html")
	
	if len(alerts) == 0 {
		w.Write([]byte(`
<div class="text-center py-12">
    <div class="text-5xl mb-3">🔔</div>
    <p class="text-slate-400">No active alerts</p>
    <p class="text-slate-500 text-sm mt-1">Create an alert to get notified when prices change</p>
</div>
`))
		return
	}

	html := `<div class="space-y-3">`
	for _, a := range alerts {
		icon := "⬆️"
		if a.Condition == "below" {
			icon = "⬇️"
		}
		html += fmt.Sprintf(`
    <div class="flex items-center justify-between p-4 bg-slate-700/50 rounded-lg">
        <div class="flex items-center gap-4">
            <span class="text-2xl">%s</span>
            <div>
                <div class="font-medium text-white">%s</div>
                <div class="text-sm text-slate-400">
                    Price %s $%.2f
                </div>
            </div>
        </div>
        <div class="flex items-center gap-4">
            <span class="px-2 py-1 rounded text-xs font-medium bg-slate-600 text-slate-300">Active</span>
            <button hx-delete="/api/alerts/%d" 
                    hx-target="#alerts-list" 
                    hx-swap="innerHTML"
                    hx-confirm="Delete this alert?"
                    class="text-red-400 hover:text-red-300 text-sm">
                Delete
            </button>
        </div>
    </div>
`, icon, a.Symbol, a.Condition, a.Price, a.ID)
	}
	html += `</div>`
	
	w.Write([]byte(html))
}

// HTMX response helpers
func htmxSuccess(w http.ResponseWriter, message string) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": {"message": "%s", "type": "success"}}`, message))
	w.WriteHeader(http.StatusOK)
}

func htmxError(w http.ResponseWriter, message string) {
	w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": {"message": "%s", "type": "error"}}`, message))
	w.WriteHeader(http.StatusBadRequest)
}

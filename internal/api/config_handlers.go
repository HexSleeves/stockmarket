package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"stockmarket/internal/config"
	"stockmarket/internal/models"
	"stockmarket/internal/web/pages"
)

// handleConfigMarket handles market data provider configuration updates
func (s *Server) handleConfigMarket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, INVALID_FORM_DATA, http.StatusBadRequest)
		return
	}

	provider := r.FormValue("market_data_provider")
	apiKey := r.FormValue("market_data_api_key")

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		http.Error(w, FAILED_TO_GET_CONFIG, http.StatusInternalServerError)
		return
	}

	cfg.MarketDataProvider = provider

	// Only update API key if a new one is provided
	if apiKey != "" {
		encrypted, err := config.Encrypt(apiKey, s.config.EncryptionKey)
		if err != nil {
			http.Error(w, FAILED_TO_ENCRYPT_API_KEY, http.StatusInternalServerError)
			return
		}
		cfg.MarketDataAPIKey = encrypted
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		http.Error(w, FAILED_TO_UPDATE_CONFIG, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleConfigAI handles AI provider configuration updates
func (s *Server) handleConfigAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, INVALID_FORM_DATA, http.StatusBadRequest)
		return
	}

	provider := r.FormValue("ai_provider")
	model := r.FormValue("ai_model")
	apiKey := r.FormValue("ai_provider_api_key")

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		http.Error(w, FAILED_TO_GET_CONFIG, http.StatusInternalServerError)
		return
	}

	cfg.AIProvider = provider
	cfg.AIModel = model

	// Only update API key if a new one is provided
	if apiKey != "" {
		encrypted, err := config.Encrypt(apiKey, s.config.EncryptionKey)
		if err != nil {
			http.Error(w, FAILED_TO_ENCRYPT_API_KEY, http.StatusInternalServerError)
			return
		}
		cfg.AIProviderAPIKey = encrypted
	}

	if err := s.db.UpdateConfig(cfg); err != nil {
		http.Error(w, FAILED_TO_UPDATE_CONFIG, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleConfigStrategy handles trading strategy configuration updates
func (s *Server) handleConfigStrategy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, INVALID_FORM_DATA, http.StatusBadRequest)
		return
	}

	riskTolerance := r.FormValue("risk_tolerance")
	tradeFrequency := r.FormValue("trade_frequency")

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		http.Error(w, FAILED_TO_GET_CONFIG, http.StatusInternalServerError)
		return
	}

	cfg.RiskTolerance = riskTolerance
	cfg.TradeFrequency = tradeFrequency

	if err := s.db.UpdateConfig(cfg); err != nil {
		http.Error(w, FAILED_TO_UPDATE_CONFIG, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleConfigWatchlist handles watchlist updates (adding symbols)
func (s *Server) handleConfigWatchlist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, INVALID_FORM_DATA, http.StatusBadRequest)
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))

	if symbol == "" {
		http.Error(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		http.Error(w, FAILED_TO_GET_CONFIG, http.StatusInternalServerError)
		return
	}

	// Add symbol if not already present
	for _, existing := range cfg.TrackedSymbols {
		if existing == symbol {
			// Already exists, just return the list
			s.renderWatchlistSettings(w, r, cfg.TrackedSymbols)
			return
		}
	}

	cfg.TrackedSymbols = append(cfg.TrackedSymbols, symbol)

	if err := s.db.UpdateConfig(cfg); err != nil {
		http.Error(w, FAILED_TO_UPDATE_CONFIG, http.StatusInternalServerError)
		return
	}

	s.renderWatchlistSettings(w, r, cfg.TrackedSymbols)
}

// handleConfigWatchlistSymbol handles individual symbol deletion
func (s *Server) handleConfigWatchlistSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	// Extract symbol from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/config/watchlist/")
	symbol := strings.ToUpper(strings.TrimSpace(path))

	if symbol == "" {
		http.Error(w, SYMBOL_REQUIRED, http.StatusBadRequest)
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		http.Error(w, FAILED_TO_GET_CONFIG, http.StatusInternalServerError)
		return
	}

	// Remove symbol from tracked list
	newSymbols := []string{}
	for _, s := range cfg.TrackedSymbols {
		if s != symbol {
			newSymbols = append(newSymbols, s)
		}
	}

	cfg.TrackedSymbols = newSymbols

	if err := s.db.UpdateConfig(cfg); err != nil {
		http.Error(w, FAILED_TO_UPDATE_CONFIG, http.StatusInternalServerError)
		return
	}

	s.renderWatchlistSettings(w, r, cfg.TrackedSymbols)
}

// renderWatchlistSettings renders the watchlist items using templ
func (s *Server) renderWatchlistSettings(w http.ResponseWriter, r *http.Request, symbols []string) {
	w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
	pages.WatchlistSettingsItemsPartial(symbols).Render(r.Context(), w)
}

// handleConfigPolling handles polling interval configuration
func (s *Server) handleConfigPolling(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, INVALID_FORM_DATA, http.StatusBadRequest)
		return
	}

	intervalStr := r.FormValue("polling_interval")
	interval, err := strconv.Atoi(intervalStr)
	if err != nil || interval < 1 {
		http.Error(w, INVALID_POLLING_INTERVAL, http.StatusBadRequest)
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		http.Error(w, FAILED_TO_GET_CONFIG, http.StatusInternalServerError)
		return
	}

	cfg.PollingInterval = interval

	if err := s.db.UpdateConfig(cfg); err != nil {
		htmxError(w, FAILED_TO_UPDATE_CONFIG)
		return
	}

	htmxSuccess(w, "Polling interval updated successfully")
}

// handleConfigNotifications handles notification settings updates
func (s *Server) handleConfigNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		htmxError(w, INVALID_FORM_DATA)
		return
	}

	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		htmxError(w, err.Error())
		return
	}

	var updateErrors []string

	// Handle email
	emailAddr := r.FormValue("email_address")
	emailEnabled := r.FormValue("email_enabled") == "on"
	if emailAddr != "" || emailEnabled {
		if err := s.updateNotificationChannel(cfg.ID, "email", emailAddr, emailEnabled); err != nil {
			updateErrors = append(updateErrors, "email")
		}
	}

	// Handle discord
	discordWebhook := r.FormValue("discord_webhook")
	discordEnabled := r.FormValue("discord_enabled") == "on"
	if discordWebhook != "" || discordEnabled {
		if err := s.updateNotificationChannel(cfg.ID, "discord", discordWebhook, discordEnabled); err != nil {
			updateErrors = append(updateErrors, "discord")
		}
	}

	// Handle SMS
	smsPhone := r.FormValue("sms_phone")
	smsEnabled := r.FormValue("sms_enabled") == "on"
	if smsPhone != "" || smsEnabled {
		if err := s.updateNotificationChannel(cfg.ID, "sms", smsPhone, smsEnabled); err != nil {
			updateErrors = append(updateErrors, "sms")
		}
	}

	if len(updateErrors) > 0 {
		htmxError(w, fmt.Sprintf("Failed to update: %s", strings.Join(updateErrors, ", ")))
		return
	}

	htmxSuccess(w, "Notification settings saved")
}

// updateNotificationChannel is a helper for updating individual notification channels
func (s *Server) updateNotificationChannel(configID int64, channelType, target string, enabled bool) error {
	ch := &models.NotificationConfig{
		Type:    channelType,
		Target:  target,
		Enabled: enabled,
	}

	if err := s.db.SaveNotificationChannel(configID, ch); err != nil {
		log.Printf("Failed to update notification channel %s: %v", channelType, err)
		return err
	}
	return nil
}

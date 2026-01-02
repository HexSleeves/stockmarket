package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"stockmarket/internal/config"
	"stockmarket/internal/models"
)

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
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
	}
}

// handleQuote fetches a stock quote
func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"risk_profiles":      models.RiskProfiles,
		"frequency_profiles": models.TradeFrequencyProfiles,
	})
}

// handleWebSocket handles WebSocket connections for real-time updates

package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"stockmarket/internal/config"
	"stockmarket/internal/market"
)

// handleQuote fetches a quote for a symbol
func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/quote/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, SYMBOL_REQUIRED)
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
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/historical/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, SYMBOL_REQUIRED)
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

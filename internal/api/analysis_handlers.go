package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"stockmarket/internal/ai"
	"stockmarket/internal/config"
	"stockmarket/internal/market"
	"stockmarket/internal/models"
	c "stockmarket/internal/web/components"
	"stockmarket/internal/web/pages"
)

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/analyze/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, SYMBOL_REQUIRED)
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
		respondError(w, http.StatusBadRequest, FAILED_TO_GET_QUOTE+": "+err.Error())
		return
	}

	historical, err := provider.GetHistoricalData(ctx, symbol, "1m")
	if err != nil {
		respondError(w, http.StatusBadRequest, FAILED_TO_GET_HISTORICAL_DATA+": "+err.Error())
		return
	}

	// Get AI analyzer
	aiAPIKey := ""
	if cfg.AIProviderAPIKey != "" {
		aiAPIKey, _ = config.Decrypt(cfg.AIProviderAPIKey, s.config.EncryptionKey)
	}

	analyzer, err := ai.NewAnalyzer(cfg.AIProvider, aiAPIKey, cfg.AIModel)
	if err != nil {
		respondError(w, http.StatusBadRequest, FAILED_TO_GET_ANALYZE+": "+err.Error())
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
		respondError(w, http.StatusInternalServerError, FAILED_TO_GET_ANALYZE+": "+err.Error())
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
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
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
		respondError(w, http.StatusMethodNotAllowed, METHOD_NOT_ALLOWED)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/api/analyses/")
	if symbol == "" {
		respondError(w, http.StatusBadRequest, SYMBOL_REQUIRED)
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

// handleAnalyzeHTMX handles HTMX form submissions for stock analysis
func (s *Server) handleAnalyzeHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage(INVALID_FORM_DATA).Render(ctx, w)
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	userContext := r.FormValue("context")

	if symbol == "" {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage(SYMBOL_REQUIRED).Render(ctx, w)
		return
	}

	// Get config
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage(FAILED_TO_GET_CONFIG).Render(ctx, w)
		return
	}

	// Get market data
	marketAPIKey := ""
	if cfg.MarketDataAPIKey != "" {
		marketAPIKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}
	provider, err := market.NewProvider(cfg.MarketDataProvider, marketAPIKey)
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage("Market provider error: "+err.Error()).Render(ctx, w)
		return
	}

	quote, err := provider.GetQuote(ctx, symbol)
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage(FAILED_TO_GET_QUOTE+": "+err.Error()).Render(ctx, w)
		return
	}

	historical, _ := provider.GetHistoricalData(ctx, symbol, "1d")

	// Get AI analyzer
	aiAPIKey := cfg.AIProviderAPIKey
	if aiAPIKey != "" {
		aiAPIKey, _ = config.Decrypt(aiAPIKey, s.config.EncryptionKey)
	}

	analyzer, err := ai.NewAnalyzer(cfg.AIProvider, aiAPIKey, cfg.AIModel)
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage(FAILED_TO_GET_ANALYZE+": "+err.Error()).Render(ctx, w)
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

	analysisCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := analyzer.Analyze(analysisCtx, analysisReq)
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		c.ErrorMessage(FAILED_TO_GET_ANALYZE+": "+err.Error()).Render(ctx, w)
		return
	}

	// Save to database
	s.db.SaveAnalysis(result)

	// Convert to pages.AnalysisResult and render
	analysisResult := pages.AnalysisResult{
		Symbol:     result.Symbol,
		CreatedAt:  time.Now(),
		AIProvider: cfg.AIProvider,
		Recommendation: pages.AnalysisRecommendation{
			Action:      result.Action,
			Confidence:  result.Confidence,
			TargetPrice: result.PriceTargets.Target,
			StopLoss:    result.PriceTargets.StopLoss,
			Reasoning:   result.Reasoning,
		},
		MarketData: &pages.MarketData{
			Price:         quote.Price,
			ChangePercent: quote.ChangePercent,
			Volume:        formatVolume(quote.Volume),
			MarketCap:     "-",
		},
	}

	w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
	pages.AnalysisResultCard(analysisResult).Render(ctx, w)
}

// formatVolume formats a volume number for display
func formatVolume(vol int64) string {
	switch {
	case vol >= 1_000_000_000:
		return strconv.FormatFloat(float64(vol)/1_000_000_000, 'f', 2, 64) + "B"
	case vol >= 1_000_000:
		return strconv.FormatFloat(float64(vol)/1_000_000, 'f', 2, 64) + "M"
	case vol >= 1_000:
		return strconv.FormatFloat(float64(vol)/1_000, 'f', 2, 64) + "K"
	default:
		return strconv.FormatInt(vol, 10)
	}
}

// handleAlertsHTMX handles creating alerts from HTMX forms

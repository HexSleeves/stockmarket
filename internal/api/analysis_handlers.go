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

// handleAlerts handles price alerts CRUD
func (s *Server) handleAnalyzeHTMX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, METHOD_NOT_ALLOWED, http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		w.Write([]byte(`<div class="text-red-400 p-4">Invalid form data</div>`))
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.FormValue("symbol")))
	userContext := r.FormValue("context")

	if symbol == "" {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		w.Write([]byte(`<div class="text-red-400 p-4">Symbol is required</div>`))
		return
	}

	// Get config
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
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
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		w.Write([]byte(`<div class="text-red-400 p-4">Market provider error: ` + err.Error() + `</div>`))
		return
	}

	quote, err := provider.GetQuote(r.Context(), symbol)
	if err != nil {
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
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
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
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
		w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
		w.Write([]byte(`<div class="text-red-400 p-4">Analysis failed: ` + err.Error() + `</div>`))
		return
	}

	// Save to database
	s.db.SaveAnalysis(result)

	// Return HTML partial
	w.Header().Set(HEADER_CONTENT_TYPE, CONTENT_TYPE_HTML)
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

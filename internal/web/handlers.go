package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stockmarket/internal/api"
	"stockmarket/internal/db"
	"stockmarket/internal/market"
	"stockmarket/internal/web/pages"

	"github.com/scmhub/calendar"
)

// Package-level cached calendar (immutable, safe to share)
var nyseCalendar = calendar.XNYS()

// EST timezone for market hours
var estLocation = time.FixedZone("EST", -5*60*60)

// TemplHandlers uses templ components for rendering
type TemplHandlers struct {
	db *db.DB
}

// NewTemplHandlers creates a new templ-based handler
func NewTemplHandlers(database *db.DB) *TemplHandlers {
	return &TemplHandlers{db: database}
}

// Dashboard renders the dashboard page using templ
func (h *TemplHandlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	config, _ := h.db.GetConfig()
	alerts, _ := h.db.GetActiveAlerts()
	recommendations, _ := h.db.GetRecommendationsToday()

	var trackedSymbols []string
	if config != nil {
		trackedSymbols = config.TrackedSymbols
	}

	data := pages.DashboardData{
		MarketOpen:     isMarketOpen(),
		TrackedSymbols: trackedSymbols,
		SignalsToday:   len(recommendations),
		ActiveAlerts:   len(alerts),
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.Dashboard(data).Render(r.Context(), w)
}

// Analysis renders the analysis page using templ
func (h *TemplHandlers) Analysis(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/analysis/")
	if symbol == "/analysis" || symbol == "" {
		symbol = ""
	}

	data := pages.AnalysisPageData{
		Symbol: strings.ToUpper(symbol),
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.AnalysisPage(data).Render(r.Context(), w)
}

// Recommendations renders the recommendations page using templ
func (h *TemplHandlers) Recommendations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.RecommendationsPage().Render(r.Context(), w)
}

// Alerts renders the alerts page using templ
func (h *TemplHandlers) Alerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.AlertsPage().Render(r.Context(), w)
}

// Settings renders the settings page using templ
func (h *TemplHandlers) Settings(w http.ResponseWriter, r *http.Request) {
	config, _ := h.db.GetConfig()

	data := pages.SettingsConfig{
		MarketDataProvider: "yahoo",
		AIProvider:         "openai",
		AIModel:            "gpt-4o",
		RiskTolerance:      "moderate",
		TradeFrequency:     "weekly",
		PollingInterval:    60,
	}

	if config != nil {
		data.MarketDataProvider = config.MarketDataProvider
		data.HasMarketAPIKey = config.HasMarketAPIKey
		data.AIProvider = config.AIProvider
		data.AIModel = config.AIModel
		data.HasAIAPIKey = config.HasAIAPIKey
		data.RiskTolerance = config.RiskTolerance
		data.TradeFrequency = config.TradeFrequency
		data.PollingInterval = config.PollingInterval
		data.TrackedSymbols = config.TrackedSymbols
		data.EmailAddress = config.EmailAddress
		data.EmailEnabled = config.EmailEnabled
		data.DiscordWebhook = config.DiscordWebhook
		data.DiscordEnabled = config.DiscordEnabled
		data.SMSPhone = config.SMSPhone
		data.SMSEnabled = config.SMSEnabled
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.SettingsPage(data).Render(r.Context(), w)
}

// Partial handlers for HTMX

// PartialWatchlist renders the watchlist partial
func (h *TemplHandlers) PartialWatchlist(w http.ResponseWriter, r *http.Request) {
	userConfig, _ := h.db.GetOrCreateConfig()

	var stocks []pages.Stock
	if userConfig != nil && len(userConfig.TrackedSymbols) > 0 {
		// Get the configured market data provider
		provider, err := market.NewProvider(userConfig.MarketDataProvider, userConfig.MarketDataAPIKey)
		if err != nil {
			// Fallback to Yahoo Finance if provider creation fails
			provider = market.NewYahooFinance()
		}

		for _, sym := range userConfig.TrackedSymbols {
			stock := pages.Stock{
				Symbol: sym,
				Name:   sym + " Inc.",
			}

			// Fetch real quote
			quote, err := provider.GetQuote(r.Context(), sym)
			if err == nil && quote != nil {
				stock.Price = quote.Price
				stock.ChangePercent = quote.ChangePercent
			} else {
				// Fallback to placeholder if quote fails
				stock.Price = 0
				stock.ChangePercent = 0
			}

			stocks = append(stocks, stock)
		}
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.WatchlistPartial(stocks).Render(r.Context(), w)
}

// PartialRecommendations renders the recommendations partial
func (h *TemplHandlers) PartialRecommendations(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	recsRaw, _ := h.db.GetRecentRecommendations(limit)

	recs := make([]pages.Recommendation, len(recsRaw))
	for i, rec := range recsRaw {
		recs[i] = pages.Recommendation{
			Symbol:     rec.Symbol,
			Action:     rec.Action,
			Confidence: rec.Confidence,
		}
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.RecommendationsPartial(recs).Render(r.Context(), w)
}

// PartialRecommendationsList renders the full recommendations list
func (h *TemplHandlers) PartialRecommendationsList(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	minConfStr := r.URL.Query().Get("min_confidence")
	symbol := r.URL.Query().Get("symbol")

	var minConf float64
	if minConfStr != "" {
		minConf, _ = strconv.ParseFloat(minConfStr, 64)
	}

	recsRaw, _ := h.db.GetFilteredRecommendations(action, minConf, strings.ToUpper(symbol))

	recs := make([]pages.RecommendationDetail, len(recsRaw))
	for i, rec := range recsRaw {
		recs[i] = pages.RecommendationDetail{
			ID:          rec.ID,
			Symbol:      rec.Symbol,
			Action:      rec.Action,
			Confidence:  rec.Confidence,
			TargetPrice: rec.TargetPrice,
			AIProvider:  rec.AIProvider,
			CreatedAt:   rec.CreatedAt,
		}
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.RecommendationsListPartial(recs).Render(r.Context(), w)
}

// PartialAnalysisHistory renders the analysis history table
func (h *TemplHandlers) PartialAnalysisHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	analysesRaw, _ := h.db.GetRecentAnalyses(limit)

	analyses := make([]pages.Analysis, len(analysesRaw))
	for i, ar := range analysesRaw {
		analyses[i] = pages.Analysis{
			ID:         ar.ID,
			Symbol:     ar.Symbol,
			AIProvider: "AI",
			CreatedAt:  ar.GeneratedAt,
			Recommendation: pages.Recommendation{
				Symbol:     ar.Symbol,
				Action:     ar.Action,
				Confidence: ar.Confidence,
			},
		}
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.AnalysisHistoryPartial(analyses).Render(r.Context(), w)
}

// PartialAnalysisDetail renders a single analysis result
func (h *TemplHandlers) PartialAnalysisDetail(w http.ResponseWriter, r *http.Request) {
	idStr := filepath.Base(r.URL.Path)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	analysis, err := h.db.GetAnalysis(id)
	if err != nil {
		http.Error(w, "Analysis not found", http.StatusNotFound)
		return
	}

	result := pages.AnalysisResult{
		ID:         analysis.ID,
		Symbol:     analysis.Symbol,
		CreatedAt:  analysis.CreatedAt,
		AIProvider: analysis.AIProvider,
		Recommendation: pages.AnalysisRecommendation{
			Action:      analysis.Recommendation.Action,
			Confidence:  analysis.Recommendation.Confidence,
			TargetPrice: analysis.Recommendation.TargetPrice,
			StopLoss:    analysis.Recommendation.StopLoss,
			Reasoning:   analysis.Recommendation.Reasoning,
		},
	}

	if analysis.MarketData != nil {
		result.MarketData = &pages.MarketData{
			Price:         analysis.MarketData.Price,
			ChangePercent: analysis.MarketData.ChangePercent,
			Volume:        formatVolume(analysis.MarketData.Volume),
			MarketCap:     "-", // Not available in Quote
		}
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.AnalysisResultCard(result).Render(r.Context(), w)
}

// PartialAlertsList renders the alerts list
func (h *TemplHandlers) PartialAlertsList(w http.ResponseWriter, r *http.Request) {
	alertsRaw, _ := h.db.GetActiveAlerts()

	alerts := make([]pages.Alert, len(alertsRaw))
	for i, ar := range alertsRaw {
		alerts[i] = pages.Alert{
			ID:          ar.ID,
			Symbol:      ar.Symbol,
			Condition:   ar.Condition,
			TargetPrice: ar.Price,
			Triggered:   ar.Triggered,
		}
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.AlertsListPartial(alerts).Render(r.Context(), w)
}

// PartialQuickAnalyze renders quick analyze buttons
func (h *TemplHandlers) PartialQuickAnalyze(w http.ResponseWriter, r *http.Request) {
	config, _ := h.db.GetConfig()

	var symbols []string
	if config != nil {
		symbols = config.TrackedSymbols
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.QuickAnalyzePartial(symbols).Render(r.Context(), w)
}

// PartialWatchlistAlertButtons renders watchlist buttons for alerts page
func (h *TemplHandlers) PartialWatchlistAlertButtons(w http.ResponseWriter, r *http.Request) {
	config, _ := h.db.GetConfig()

	var symbols []string
	if config != nil {
		symbols = config.TrackedSymbols
	}

	w.Header().Set(api.HEADER_CONTENT_TYPE, api.CONTENT_TYPE_HTML)
	pages.WatchlistAlertButtonsPartial(symbols).Render(r.Context(), w)
}

// formatVolume formats a volume number for display
func formatVolume(vol int64) string {
	if vol >= 1_000_000_000 {
		return fmt.Sprintf("%.2fB", float64(vol)/1_000_000_000)
	}
	if vol >= 1_000_000 {
		return fmt.Sprintf("%.2fM", float64(vol)/1_000_000)
	}
	if vol >= 1_000 {
		return fmt.Sprintf("%.2fK", float64(vol)/1_000)
	}
	return fmt.Sprintf("%d", vol)
}

func isMarketOpen() bool {
	return nyseCalendar.IsOpen(time.Now().In(estLocation))
}

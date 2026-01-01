package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"stockmarket/internal/db"
	"stockmarket/internal/models"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

type Templates struct {
	pages    map[string]*template.Template
	partials *template.Template
	db       *db.DB
}

func NewTemplates(database *db.DB) (*Templates, error) {
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 { return a / b },
	}

	// Parse partials
	partials, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/partials/*.html")
	if err != nil {
		return nil, err
	}

	// Parse each page with layout
	pageNames := []string{"dashboard", "analysis", "recommendations", "alerts", "settings"}
	pages := make(map[string]*template.Template)

	for _, name := range pageNames {
		tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/"+name+".html",
			"templates/partials/*.html")
		if err != nil {
			return nil, err
		}
		pages[name] = tmpl
	}

	return &Templates{
		pages:    pages,
		partials: partials,
		db:       database,
	}, nil
}

type PageData struct {
	Title          string
	Page           string
	MarketOpen     bool
	TrackedSymbols []string
	SignalsToday   int
	ActiveAlerts   int
	Config         *models.AppConfig
	Symbol         string
	Result         *models.Analysis
}

func (t *Templates) renderPage(w http.ResponseWriter, pageName string, data interface{}) {
	tmpl, ok := t.pages[pageName]
	if !ok {
		log.Printf("Page not found: %s", pageName)
		http.Error(w, "Page not found", http.StatusNotFound)
		return
	}
	err := tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (t *Templates) renderPartial(w http.ResponseWriter, name string, data interface{}) {
	tmpl, ok := t.pages["dashboard"] // Use any page template since all have partials
	if !ok {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	// Template names are just the filename without path
	err := tmpl.ExecuteTemplate(w, name+".html", data)
	if err != nil {
		log.Printf("Partial error for %s: %v", name, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (t *Templates) Dashboard(w http.ResponseWriter, r *http.Request) {
	config, _ := t.db.GetConfig()
	alerts, _ := t.db.GetActiveAlerts()
	recommendations, _ := t.db.GetRecommendationsToday()

	var trackedSymbols []string
	if config != nil {
		trackedSymbols = config.TrackedSymbols
	}

	data := PageData{
		Title:          "Dashboard",
		Page:           "dashboard",
		MarketOpen:     isMarketOpen(),
		TrackedSymbols: trackedSymbols,
		SignalsToday:   len(recommendations),
		ActiveAlerts:   len(alerts),
	}

	w.Header().Set("Content-Type", "text/html")
	t.renderPage(w, "dashboard", data)
}

func (t *Templates) Analysis(w http.ResponseWriter, r *http.Request) {
	symbol := strings.TrimPrefix(r.URL.Path, "/analysis/")
	if symbol == "/analysis" || symbol == "" {
		symbol = ""
	}

	data := PageData{
		Title:  "Analysis",
		Page:   "analysis",
		Symbol: strings.ToUpper(symbol),
	}

	w.Header().Set("Content-Type", "text/html")
	t.renderPage(w, "analysis", data)
}

func (t *Templates) Recommendations(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Recommendations",
		Page:  "recommendations",
	}

	w.Header().Set("Content-Type", "text/html")
	t.renderPage(w, "recommendations", data)
}

func (t *Templates) Alerts(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Alerts",
		Page:  "alerts",
	}

	w.Header().Set("Content-Type", "text/html")
	t.renderPage(w, "alerts", data)
}

func (t *Templates) Settings(w http.ResponseWriter, r *http.Request) {
	config, _ := t.db.GetConfig()
	if config == nil {
		config = &models.AppConfig{
			MarketDataProvider: "yahoo",
			AIProvider:         "openai",
			AIModel:            "gpt-4o",
			RiskTolerance:      "moderate",
			TradeFrequency:     "weekly",
		}
	}

	data := PageData{
		Title:  "Settings",
		Page:   "settings",
		Config: config,
	}

	w.Header().Set("Content-Type", "text/html")
	t.renderPage(w, "settings", data)
}

// Partial handlers for HTMX
func (t *Templates) PartialWatchlist(w http.ResponseWriter, r *http.Request) {
	config, _ := t.db.GetConfig()

	type StockInfo struct {
		Symbol        string
		Name          string
		Price         float64
		ChangePercent float64
	}

	stocks := []StockInfo{}
	if config != nil {
		for _, sym := range config.TrackedSymbols {
			stocks = append(stocks, StockInfo{
				Symbol:        sym,
				Name:          sym + " Inc.",
				Price:         150.00,
				ChangePercent: 1.25,
			})
		}
	}

	data := struct{ Stocks []StockInfo }{Stocks: stocks}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "watchlist", data)
}

func (t *Templates) PartialRecommendations(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	recommendations, _ := t.db.GetRecentRecommendations(limit)

	data := struct{ Recommendations []models.Recommendation }{Recommendations: recommendations}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "recommendations", data)
}

func (t *Templates) PartialRecommendationsList(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	minConfStr := r.URL.Query().Get("min_confidence")
	symbol := r.URL.Query().Get("symbol")

	var minConf float64
	if minConfStr != "" {
		minConf, _ = strconv.ParseFloat(minConfStr, 64)
	}

	recommendations, _ := t.db.GetFilteredRecommendations(action, minConf, strings.ToUpper(symbol))

	data := struct{ Recommendations []models.Recommendation }{Recommendations: recommendations}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "recommendations-list", data)
}

func (t *Templates) PartialAnalysisHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	analysesRaw, _ := t.db.GetRecentAnalyses(limit)

	// Convert to Analysis type for template
	analyses := make([]models.Analysis, len(analysesRaw))
	for i, ar := range analysesRaw {
		analyses[i] = models.Analysis{
			ID:         ar.ID,
			Symbol:     ar.Symbol,
			AIProvider: "AI",
			CreatedAt:  ar.GeneratedAt,
			Recommendation: models.Recommendation{
				Action:     ar.Action,
				Confidence: ar.Confidence,
				Reasoning:  ar.Reasoning,
				Timeframe:  ar.Timeframe,
			},
		}
	}

	data := struct{ Analyses []models.Analysis }{Analyses: analyses}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "analysis-history", data)
}

func (t *Templates) PartialAnalysisDetail(w http.ResponseWriter, r *http.Request) {
	idStr := filepath.Base(r.URL.Path)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	analysis, err := t.db.GetAnalysis(id)
	if err != nil {
		http.Error(w, "Analysis not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "analysis-result", analysis)
}

func (t *Templates) PartialAlertsList(w http.ResponseWriter, r *http.Request) {
	alertsRaw, _ := t.db.GetActiveAlerts()

	// Convert to Alert type for template
	alerts := make([]models.Alert, len(alertsRaw))
	for i, ar := range alertsRaw {
		alerts[i] = models.Alert{
			ID:          ar.ID,
			Symbol:      ar.Symbol,
			Condition:   ar.Condition,
			TargetPrice: ar.Price,
			Triggered:   ar.Triggered,
			CreatedAt:   ar.CreatedAt,
		}
	}

	data := struct{ Alerts []models.Alert }{Alerts: alerts}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "alerts-list", data)
}

func (t *Templates) PartialQuickAnalyze(w http.ResponseWriter, r *http.Request) {
	config, _ := t.db.GetConfig()

	var symbols []string
	if config != nil {
		symbols = config.TrackedSymbols
	}

	data := struct{ Symbols []string }{Symbols: symbols}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "quick-analyze", data)
}

func (t *Templates) PartialWatchlistAlertButtons(w http.ResponseWriter, r *http.Request) {
	config, _ := t.db.GetConfig()

	var symbols []string
	if config != nil {
		symbols = config.TrackedSymbols
	}

	data := struct{ Symbols []string }{Symbols: symbols}
	w.Header().Set("Content-Type", "text/html")
	t.renderPartial(w, "watchlist-alert-buttons", data)
}

func isMarketOpen() bool {
	now := time.Now().In(time.FixedZone("EST", -5*60*60))
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	hour := now.Hour()
	minute := now.Minute()
	marketMinutes := hour*60 + minute
	return marketMinutes >= 9*60+30 && marketMinutes < 16*60
}

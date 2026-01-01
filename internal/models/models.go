package models

import "time"

// UserConfig holds all user configuration settings
type UserConfig struct {
	ID                    int64                  `json:"id"`
	MarketDataProvider    string                 `json:"market_data_provider"`    // "alphavantage" | "yahoo" | "finnhub"
	MarketDataAPIKey      string                 `json:"market_data_api_key"`     // encrypted at rest
	AIProvider            string                 `json:"ai_provider"`             // "openai" | "claude" | "gemini"
	AIProviderAPIKey      string                 `json:"ai_provider_api_key"`     // encrypted at rest
	AIModel               string                 `json:"ai_model"`                // e.g., "gpt-4o", "claude-sonnet"
	RiskTolerance         string                 `json:"risk_tolerance"`          // "conservative" | "moderate" | "aggressive"
	TradeFrequency        string                 `json:"trade_frequency"`         // "daily" | "weekly" | "swing"
	TrackedSymbols        []string               `json:"tracked_symbols"`         // e.g., ["AAPL", "GOOGL", "MSFT"]
	PollingInterval       int                    `json:"polling_interval"`        // in seconds, default 30
	NotificationChannels  []NotificationConfig   `json:"notification_channels"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// NotificationConfig holds notification channel settings
type NotificationConfig struct {
	ID      int64    `json:"id"`
	Type    string   `json:"type"`    // "email" | "discord" | "sms"
	Target  string   `json:"target"`  // email address, webhook URL, phone number
	Enabled bool     `json:"enabled"`
	Events  []string `json:"events"` // ["buy_signal", "sell_signal", "price_alert"]
}

// Quote represents a stock quote
type Quote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	Open          float64   `json:"open"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Volume        int64     `json:"volume"`
	PreviousClose float64   `json:"previous_close"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Timestamp     time.Time `json:"timestamp"`
}

// Candle represents OHLCV data
type Candle struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
}

// AnalysisRequest represents a request for AI analysis
type AnalysisRequest struct {
	Symbol         string   `json:"symbol"`
	CurrentPrice   float64  `json:"current_price"`
	HistoricalData []Candle `json:"historical_data"`
	RiskProfile    string   `json:"risk_profile"`
	TradeFrequency string   `json:"trade_frequency"`
	UserContext    string   `json:"user_context"` // optional user notes
}

// AnalysisResponse represents the AI analysis result
type AnalysisResponse struct {
	ID           int64        `json:"id"`
	Symbol       string       `json:"symbol"`
	Action       string       `json:"action"` // "BUY" | "SELL" | "HOLD" | "WATCH"
	Confidence   float64      `json:"confidence"` // 0.0 - 1.0
	Reasoning    string       `json:"reasoning"` // AI explanation
	PriceTargets PriceTargets `json:"price_targets"`
	Risks        []string     `json:"risks"`
	Timeframe    string       `json:"timeframe"`
	GeneratedAt  time.Time    `json:"generated_at"`
}

// PriceTargets holds price target information
type PriceTargets struct {
	Entry    float64 `json:"entry"`
	Target   float64 `json:"target"`
	StopLoss float64 `json:"stop_loss"`
}

// PriceAlert represents a user-defined price alert
type PriceAlert struct {
	ID        int64     `json:"id"`
	Symbol    string    `json:"symbol"`
	Condition string    `json:"condition"` // "above" | "below"
	Price     float64   `json:"price"`
	Triggered bool      `json:"triggered"`
	CreatedAt time.Time `json:"created_at"`
}

// Notification represents a notification to be sent
type Notification struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"` // "buy_signal", "sell_signal", "price_alert"
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Symbol    string    `json:"symbol"`
	SentAt    time.Time `json:"sent_at"`
	Channels  []string  `json:"channels"` // which channels it was sent to
}

// RiskProfile defines analysis behavior based on risk tolerance
type RiskProfile struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PromptModifier string `json:"prompt_modifier"`
}

// TradeFrequencyProfile defines analysis behavior based on trade frequency
type TradeFrequencyProfile struct {
	Name             string `json:"name"`
	AnalysisWindow   string `json:"analysis_window"`
	SignalSensitivity string `json:"signal_sensitivity"`
}

// Risk profiles
var RiskProfiles = map[string]RiskProfile{
	"conservative": {
		Name:           "Conservative",
		Description:    "Capital preservation, blue-chips, low volatility",
		PromptModifier: "Prioritize stability, established companies, dividend yield. Avoid speculative positions. Focus on companies with strong balance sheets, consistent earnings, and proven track records. Recommend only high-confidence, lower-risk opportunities.",
	},
	"moderate": {
		Name:           "Moderate",
		Description:    "Balanced growth/risk, diversified",
		PromptModifier: "Balance growth potential with risk management. Mix of established and growth stocks. Consider both value and momentum factors. Recommend opportunities with reasonable risk-reward ratios.",
	},
	"aggressive": {
		Name:           "Aggressive",
		Description:    "High growth, accepts volatility, momentum plays",
		PromptModifier: "Prioritize high growth potential, momentum indicators. Accept higher volatility for returns. Consider emerging sectors, breakout patterns, and high-beta stocks. Focus on maximum return potential.",
	},
}

// Trade frequency profiles
var TradeFrequencyProfiles = map[string]TradeFrequencyProfile{
	"daily": {
		Name:             "Daily",
		AnalysisWindow:   "Intraday + daily charts",
		SignalSensitivity: "High sensitivity, short-term indicators (RSI, MACD, intraday patterns)",
	},
	"weekly": {
		Name:             "Weekly",
		AnalysisWindow:   "Daily + weekly trends",
		SignalSensitivity: "Medium sensitivity, trend confirmation required",
	},
	"swing": {
		Name:             "Swing",
		AnalysisWindow:   "Multi-week patterns",
		SignalSensitivity: "Low sensitivity, strong trend/reversal signals only",
	},
}

// Recommendation for the HTMX templates
type Recommendation struct {
	ID          int64     `json:"id"`
	Symbol      string    `json:"symbol"`
	Action      string    `json:"action"`
	Confidence  float64   `json:"confidence"`
	TargetPrice float64   `json:"target_price"`
	StopLoss    float64   `json:"stop_loss"`
	Reasoning   string    `json:"reasoning"`
	Timeframe   string    `json:"timeframe"`
	AIProvider  string    `json:"ai_provider"`
	CreatedAt   time.Time `json:"created_at"`
}

// Alert for HTMX templates
type Alert struct {
	ID          int64     `json:"id"`
	Symbol      string    `json:"symbol"`
	Condition   string    `json:"condition"`
	TargetPrice float64   `json:"target_price"`
	Triggered   bool      `json:"triggered"`
	CreatedAt   time.Time `json:"created_at"`
}

// Analysis for HTMX templates
type Analysis struct {
	ID             int64           `json:"id"`
	Symbol         string          `json:"symbol"`
	Recommendation Recommendation  `json:"recommendation"`
	MarketData     *Quote          `json:"market_data"`
	AIProvider     string          `json:"ai_provider"`
	CreatedAt      time.Time       `json:"created_at"`
}

// AppConfig for settings page
type AppConfig struct {
	MarketDataProvider string   `json:"market_data_provider"`
	HasMarketAPIKey    bool     `json:"has_market_api_key"`
	MarketAPIKeyMasked string   `json:"market_api_key_masked"`
	AIProvider         string   `json:"ai_provider"`
	HasAIAPIKey        bool     `json:"has_ai_api_key"`
	AIAPIKeyMasked     string   `json:"ai_api_key_masked"`
	AIModel            string   `json:"ai_model"`
	RiskTolerance      string   `json:"risk_tolerance"`
	TradeFrequency     string   `json:"trade_frequency"`
	TrackedSymbols     []string `json:"tracked_symbols"`
	PollingInterval    int      `json:"polling_interval"` // in seconds
	EmailAddress       string   `json:"email_address"`
	EmailEnabled       bool     `json:"email_enabled"`
	DiscordWebhook     string   `json:"discord_webhook"`
	DiscordEnabled     bool     `json:"discord_enabled"`
	SMSPhone           string   `json:"sms_phone"`
	SMSEnabled         bool     `json:"sms_enabled"`
}

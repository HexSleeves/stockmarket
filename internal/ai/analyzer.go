package ai

import (
	"context"
	"errors"
	"fmt"

	"stockmarket/internal/models"
)

// Analyzer defines the interface for AI analysis providers
type Analyzer interface {
	Analyze(ctx context.Context, req models.AnalysisRequest) (*models.AnalysisResponse, error)
	Name() string
}

// ErrNoAPIKey is returned when no API key is configured
var ErrNoAPIKey = errors.New("no API key configured")

// ErrAnalysisFailed is returned when analysis fails
var ErrAnalysisFailed = errors.New("analysis failed")

// NewAnalyzer creates an AI analyzer based on the provider name
func NewAnalyzer(provider string, apiKey string, model string) (Analyzer, error) {
	switch provider {
	case "openai":
		return NewOpenAI(apiKey, model), nil
	case "claude":
		return NewClaude(apiKey, model), nil
	case "gemini":
		return NewGemini(apiKey, model), nil
	default:
		return nil, errors.New("unknown AI provider: " + provider)
	}
}

// BuildPrompt creates the analysis prompt based on risk profile and trade frequency
func BuildPrompt(req models.AnalysisRequest) string {
	riskProfile := models.RiskProfiles[req.RiskProfile]
	freqProfile := models.TradeFrequencyProfiles[req.TradeFrequency]

	prompt := `You are an expert stock market analyst. Analyze the following stock data and provide a trading recommendation.

Stock: ` + req.Symbol + `
Current Price: $` + formatFloat(req.CurrentPrice) + `

Risk Profile: ` + riskProfile.Name + `
` + riskProfile.PromptModifier + `

Trading Timeframe: ` + freqProfile.Name + `
Analysis Window: ` + freqProfile.AnalysisWindow + `
Signal Sensitivity: ` + freqProfile.SignalSensitivity + `

Historical Data (most recent ` + formatInt(len(req.HistoricalData)) + ` periods):
`

	// Add historical data summary
	if len(req.HistoricalData) > 0 {
		prompt += formatHistoricalSummary(req.HistoricalData)
	}

	if req.UserContext != "" {
		prompt += "\nUser Notes: " + req.UserContext + "\n"
	}

	prompt += `
Provide your analysis in the following JSON format:
{
  "action": "BUY" | "SELL" | "HOLD" | "WATCH",
  "confidence": 0.0-1.0,
  "reasoning": "detailed explanation",
  "price_targets": {
    "entry": price,
    "target": price,
    "stop_loss": price
  },
  "risks": ["risk1", "risk2"],
  "timeframe": "expected time horizon"
}

Respond ONLY with valid JSON, no additional text.`

	return prompt
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

func formatInt(i int) string {
	return fmt.Sprintf("%d", i)
}

func formatHistoricalSummary(candles []models.Candle) string {
	if len(candles) == 0 {
		return "No historical data available\n"
	}

	// Calculate some basic stats
	var high, low float64
	high = candles[0].High
	low = candles[0].Low
	var totalVolume int64

	for _, c := range candles {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
		totalVolume += c.Volume
	}

	avgVolume := totalVolume / int64(len(candles))

	// Calculate price change over period
	latestClose := candles[0].Close
	oldestClose := candles[len(candles)-1].Close
	priceChange := ((latestClose - oldestClose) / oldestClose) * 100

	summary := fmt.Sprintf(`Period High: $%.2f
Period Low: $%.2f
Latest Close: $%.2f
Price Change: %.2f%%
Average Volume: %d

Recent candles:
`, high, low, latestClose, priceChange, avgVolume)

	// Show last 5 candles
	count := 5
	if len(candles) < count {
		count = len(candles)
	}
	for i := 0; i < count; i++ {
		c := candles[i]
		summary += fmt.Sprintf("%s: O:%.2f H:%.2f L:%.2f C:%.2f V:%d\n",
			c.Timestamp.Format("2006-01-02"), c.Open, c.High, c.Low, c.Close, c.Volume)
	}

	return summary
}

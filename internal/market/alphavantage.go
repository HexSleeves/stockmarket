package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"stockmarket/internal/models"
)

const alphaVantageBaseURL = "https://www.alphavantage.co/query"

// AlphaVantage implements the Provider interface for Alpha Vantage API
type AlphaVantage struct {
	apiKey string
	client *http.Client
}

// NewAlphaVantage creates a new Alpha Vantage provider
func NewAlphaVantage(apiKey string) *AlphaVantage {
	return &AlphaVantage{
		apiKey: apiKey,
		client: sharedHTTPClient,
	}
}

// Name returns the provider name
func (av *AlphaVantage) Name() string {
	return "alphavantage"
}

// GetQuote fetches the current quote for a symbol
func (av *AlphaVantage) GetQuote(ctx context.Context, symbol string) (*models.Quote, error) {
	url := fmt.Sprintf("%s?function=GLOBAL_QUOTE&symbol=%s&apikey=%s",
		alphaVantageBaseURL, symbol, av.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := av.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		GlobalQuote struct {
			Symbol           string `json:"01. symbol"`
			Open             string `json:"02. open"`
			High             string `json:"03. high"`
			Low              string `json:"04. low"`
			Price            string `json:"05. price"`
			Volume           string `json:"06. volume"`
			LatestTradingDay string `json:"07. latest trading day"`
			PreviousClose    string `json:"08. previous close"`
			Change           string `json:"09. change"`
			ChangePercent    string `json:"10. change percent"`
		} `json:"Global Quote"`
		Note string `json:"Note"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Check for rate limit
	if result.Note != "" && strings.Contains(result.Note, "API call frequency") {
		return nil, ErrRateLimited
	}

	if result.GlobalQuote.Symbol == "" {
		return nil, ErrInvalidSymbol
	}

	price, _ := strconv.ParseFloat(result.GlobalQuote.Price, 64)
	open, _ := strconv.ParseFloat(result.GlobalQuote.Open, 64)
	high, _ := strconv.ParseFloat(result.GlobalQuote.High, 64)
	low, _ := strconv.ParseFloat(result.GlobalQuote.Low, 64)
	volume, _ := strconv.ParseInt(result.GlobalQuote.Volume, 10, 64)
	prevClose, _ := strconv.ParseFloat(result.GlobalQuote.PreviousClose, 64)
	change, _ := strconv.ParseFloat(result.GlobalQuote.Change, 64)
	changePercent, _ := strconv.ParseFloat(strings.TrimSuffix(result.GlobalQuote.ChangePercent, "%"), 64)

	return &models.Quote{
		Symbol:        symbol,
		Price:         price,
		Open:          open,
		High:          high,
		Low:           low,
		Volume:        volume,
		PreviousClose: prevClose,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Now(),
	}, nil
}

// GetHistoricalData fetches historical OHLCV data
func (av *AlphaVantage) GetHistoricalData(ctx context.Context, symbol string, period string) ([]models.Candle, error) {
	// Map period to Alpha Vantage function
	function := "TIME_SERIES_DAILY"
	outputSize := "compact" // 100 data points

	switch period {
	case "1d", "5d":
		function = "TIME_SERIES_INTRADAY"
	case "1m", "3m":
		outputSize = "compact"
	case "1y", "5y":
		outputSize = "full"
	}

	var url string
	if function == "TIME_SERIES_INTRADAY" {
		url = fmt.Sprintf("%s?function=%s&symbol=%s&interval=5min&outputsize=%s&apikey=%s",
			alphaVantageBaseURL, function, symbol, outputSize, av.apiKey)
	} else {
		url = fmt.Sprintf("%s?function=%s&symbol=%s&outputsize=%s&apikey=%s",
			alphaVantageBaseURL, function, symbol, outputSize, av.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := av.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var rawResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		return nil, err
	}

	// Check for rate limit
	if note, ok := rawResult["Note"].(string); ok && strings.Contains(note, "API call frequency") {
		return nil, ErrRateLimited
	}

	// Find the time series key
	var timeSeriesKey string
	for key := range rawResult {
		if strings.Contains(key, "Time Series") {
			timeSeriesKey = key
			break
		}
	}

	if timeSeriesKey == "" {
		return nil, ErrInvalidSymbol
	}

	timeSeries, ok := rawResult[timeSeriesKey].(map[string]interface{})
	if !ok {
		return nil, ErrAPIError
	}

	var candles []models.Candle
	for dateStr, data := range timeSeries {
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse timestamp
		var timestamp time.Time
		if strings.Contains(dateStr, " ") {
			timestamp, _ = time.Parse("2006-01-02 15:04:05", dateStr)
		} else {
			timestamp, _ = time.Parse("2006-01-02", dateStr)
		}

		open, _ := strconv.ParseFloat(dataMap["1. open"].(string), 64)
		high, _ := strconv.ParseFloat(dataMap["2. high"].(string), 64)
		low, _ := strconv.ParseFloat(dataMap["3. low"].(string), 64)
		close, _ := strconv.ParseFloat(dataMap["4. close"].(string), 64)
		volume, _ := strconv.ParseInt(dataMap["5. volume"].(string), 10, 64)

		candles = append(candles, models.Candle{
			Timestamp: timestamp,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		})
	}

	// Sort by timestamp (newest first) - O(n log n)
	sort.Slice(candles, func(i, j int) bool {
		return candles[i].Timestamp.After(candles[j].Timestamp)
	})

	return candles, nil
}

// StreamQuotes streams real-time quotes (Alpha Vantage doesn't support real-time streaming in free tier)
func (av *AlphaVantage) StreamQuotes(ctx context.Context, symbols []string, ch chan<- models.Quote) error {
	// Alpha Vantage doesn't support WebSocket streaming, so we poll
	ticker := time.NewTicker(15 * time.Second) // Rate limit friendly
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, symbol := range symbols {
				quote, err := av.GetQuote(ctx, symbol)
				if err != nil {
					continue // Skip on error
				}
				select {
				case ch <- *quote:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

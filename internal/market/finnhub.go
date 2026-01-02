package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"stockmarket/internal/models"
)

const finnhubBaseURL = "https://finnhub.io/api/v1"

// Finnhub implements the Provider interface for Finnhub API
type Finnhub struct {
	apiKey string
	client *http.Client
}

// NewFinnhub creates a new Finnhub provider
func NewFinnhub(apiKey string) *Finnhub {
	return &Finnhub{
		apiKey: apiKey,
		client: sharedHTTPClient,
	}
}

// Name returns the provider name
func (f *Finnhub) Name() string {
	return "finnhub"
}

// GetQuote fetches the current quote for a symbol
func (f *Finnhub) GetQuote(ctx context.Context, symbol string) (*models.Quote, error) {
	url := fmt.Sprintf("%s/quote?symbol=%s&token=%s", finnhubBaseURL, symbol, f.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimited
	}
	if resp.StatusCode != 200 {
		return nil, ErrAPIError
	}

	var result struct {
		C  float64 `json:"c"`  // Current price
		D  float64 `json:"d"`  // Change
		Dp float64 `json:"dp"` // Percent change
		H  float64 `json:"h"`  // High price of the day
		L  float64 `json:"l"`  // Low price of the day
		O  float64 `json:"o"`  // Open price of the day
		Pc float64 `json:"pc"` // Previous close price
		T  int64   `json:"t"`  // Timestamp
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.C == 0 && result.H == 0 && result.L == 0 {
		return nil, ErrInvalidSymbol
	}

	return &models.Quote{
		Symbol:        symbol,
		Price:         result.C,
		Open:          result.O,
		High:          result.H,
		Low:           result.L,
		Volume:        0, // Finnhub quote doesn't include volume
		PreviousClose: result.Pc,
		Change:        result.D,
		ChangePercent: result.Dp,
		Timestamp:     time.Unix(result.T, 0),
	}, nil
}

// GetHistoricalData fetches historical OHLCV data
func (f *Finnhub) GetHistoricalData(ctx context.Context, symbol string, period string) ([]models.Candle, error) {
	// Calculate time range based on period
	resolution := "D"
	var from, to time.Time
	to = time.Now()

	switch period {
	case "1d":
		resolution = "5"
		from = to.AddDate(0, 0, -1)
	case "5d":
		resolution = "15"
		from = to.AddDate(0, 0, -5)
	case "1m":
		resolution = "D"
		from = to.AddDate(0, -1, 0)
	case "3m":
		resolution = "D"
		from = to.AddDate(0, -3, 0)
	case "1y":
		resolution = "D"
		from = to.AddDate(-1, 0, 0)
	case "5y":
		resolution = "W"
		from = to.AddDate(-5, 0, 0)
	default:
		resolution = "D"
		from = to.AddDate(0, -1, 0)
	}

	url := fmt.Sprintf("%s/stock/candle?symbol=%s&resolution=%s&from=%d&to=%d&token=%s",
		finnhubBaseURL, symbol, resolution, from.Unix(), to.Unix(), f.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, ErrRateLimited
	}

	var result struct {
		C []float64 `json:"c"` // Close prices
		H []float64 `json:"h"` // High prices
		L []float64 `json:"l"` // Low prices
		O []float64 `json:"o"` // Open prices
		S string    `json:"s"` // Status
		T []int64   `json:"t"` // Timestamps
		V []int64   `json:"v"` // Volume
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.S == "no_data" || len(result.T) == 0 {
		return nil, ErrInvalidSymbol
	}

	var candles []models.Candle
	for i := len(result.T) - 1; i >= 0; i-- {
		var volume int64
		if i < len(result.V) {
			volume = result.V[i]
		}

		candles = append(candles, models.Candle{
			Timestamp: time.Unix(result.T[i], 0),
			Open:      result.O[i],
			High:      result.H[i],
			Low:       result.L[i],
			Close:     result.C[i],
			Volume:    volume,
		})
	}

	return candles, nil
}

// StreamQuotes streams real-time quotes via polling
func (f *Finnhub) StreamQuotes(ctx context.Context, symbols []string, ch chan<- models.Quote) error {
	ticker := time.NewTicker(5 * time.Second) // Finnhub has better rate limits
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, symbol := range symbols {
				quote, err := f.GetQuote(ctx, symbol)
				if err != nil {
					continue
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

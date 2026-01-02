package market

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"stockmarket/internal/models"
)

const yahooBaseURL = "https://query1.finance.yahoo.com/v8/finance"

// YahooFinance implements the Provider interface for Yahoo Finance API
type YahooFinance struct {
	client *http.Client
}

// NewYahooFinance creates a new Yahoo Finance provider
func NewYahooFinance() *YahooFinance {
	return &YahooFinance{
		client: sharedHTTPClient,
	}
}

// Name returns the provider name
func (yf *YahooFinance) Name() string {
	return "yahoo"
}

// GetQuote fetches the current quote for a symbol
func (yf *YahooFinance) GetQuote(ctx context.Context, symbol string) (*models.Quote, error) {
	url := fmt.Sprintf("%s/chart/%s?interval=1m&range=1d", yahooBaseURL, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := yf.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrInvalidSymbol
	}
	if resp.StatusCode != 200 {
		return nil, ErrAPIError
	}

	var result struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice   float64 `json:"regularMarketPrice"`
					PreviousClose        float64 `json:"previousClose"`
					RegularMarketTime    int64   `json:"regularMarketTime"`
					RegularMarketDayHigh float64 `json:"regularMarketDayHigh"`
					RegularMarketDayLow  float64 `json:"regularMarketDayLow"`
					RegularMarketVolume  int64   `json:"regularMarketVolume"`
					RegularMarketOpen    float64 `json:"regularMarketOpen"`
				} `json:"meta"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Chart.Error != nil {
		if result.Chart.Error.Code == "Not Found" {
			return nil, ErrInvalidSymbol
		}
		return nil, fmt.Errorf("%w: %s", ErrAPIError, result.Chart.Error.Description)
	}

	if len(result.Chart.Result) == 0 {
		return nil, ErrInvalidSymbol
	}

	meta := result.Chart.Result[0].Meta
	change := meta.RegularMarketPrice - meta.PreviousClose
	changePercent := (change / meta.PreviousClose) * 100

	return &models.Quote{
		Symbol:        symbol,
		Price:         meta.RegularMarketPrice,
		Open:          meta.RegularMarketOpen,
		High:          meta.RegularMarketDayHigh,
		Low:           meta.RegularMarketDayLow,
		Volume:        meta.RegularMarketVolume,
		PreviousClose: meta.PreviousClose,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     time.Unix(meta.RegularMarketTime, 0),
	}, nil
}

// GetHistoricalData fetches historical OHLCV data
func (yf *YahooFinance) GetHistoricalData(ctx context.Context, symbol string, period string) ([]models.Candle, error) {
	// Map period to Yahoo Finance parameters
	range_ := "1mo"
	interval := "1d"

	switch period {
	case "1d":
		range_ = "1d"
		interval = "5m"
	case "5d":
		range_ = "5d"
		interval = "15m"
	case "1m":
		range_ = "1mo"
		interval = "1d"
	case "3m":
		range_ = "3mo"
		interval = "1d"
	case "1y":
		range_ = "1y"
		interval = "1d"
	case "5y":
		range_ = "5y"
		interval = "1wk"
	}

	url := fmt.Sprintf("%s/chart/%s?interval=%s&range=%s", yahooBaseURL, symbol, interval, range_)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := yf.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrInvalidSymbol
	}

	var result struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Chart.Error != nil || len(result.Chart.Result) == 0 {
		return nil, ErrInvalidSymbol
	}

	r := result.Chart.Result[0]
	if len(r.Indicators.Quote) == 0 {
		return nil, ErrAPIError
	}

	q := r.Indicators.Quote[0]
	var candles []models.Candle

	for i := 0; i < len(r.Timestamp); i++ {
		if i >= len(q.Open) || i >= len(q.High) || i >= len(q.Low) || i >= len(q.Close) || i >= len(q.Volume) {
			break
		}

		candles = append(candles, models.Candle{
			Timestamp: time.Unix(r.Timestamp[i], 0),
			Open:      q.Open[i],
			High:      q.High[i],
			Low:       q.Low[i],
			Close:     q.Close[i],
			Volume:    q.Volume[i],
		})
	}

	// Reverse to get newest first
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	return candles, nil
}

// StreamQuotes streams real-time quotes via polling
func (yf *YahooFinance) StreamQuotes(ctx context.Context, symbols []string, ch chan<- models.Quote) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, symbol := range symbols {
				quote, err := yf.GetQuote(ctx, symbol)
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

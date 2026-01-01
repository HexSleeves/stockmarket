package market

import (
	"context"
	"errors"

	"stockmarket/internal/models"
)

// Provider defines the interface for market data providers
type Provider interface {
	GetQuote(ctx context.Context, symbol string) (*models.Quote, error)
	GetHistoricalData(ctx context.Context, symbol string, period string) ([]models.Candle, error)
	StreamQuotes(ctx context.Context, symbols []string, ch chan<- models.Quote) error
	Name() string
}

// ErrRateLimited is returned when rate limit is exceeded
var ErrRateLimited = errors.New("rate limit exceeded")

// ErrInvalidSymbol is returned when the symbol is not found
var ErrInvalidSymbol = errors.New("invalid symbol")

// ErrAPIError is returned when the API returns an error
var ErrAPIError = errors.New("API error")

// NewProvider creates a market data provider based on the provider name
func NewProvider(name string, apiKey string) (Provider, error) {
	switch name {
	case "alphavantage":
		return NewAlphaVantage(apiKey), nil
	case "yahoo":
		return NewYahooFinance(), nil
	case "finnhub":
		return NewFinnhub(apiKey), nil
	default:
		return nil, errors.New("unknown provider: " + name)
	}
}

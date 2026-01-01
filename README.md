# Stock Market Analysis Platform

A real-time stock market tracking and AI-powered analysis platform built with Go, HTMX, and Tailwind CSS.

## Features

- **Real-time Market Data**: Live price updates via polling
- **AI-Powered Analysis**: Multiple AI providers (OpenAI, Claude, Gemini)
- **Multiple Data Sources**: Alpha Vantage, Yahoo Finance, Finnhub
- **Configurable Risk Profiles**: Conservative, Moderate, Aggressive
- **Trade Frequency Modes**: Daily, Weekly, Swing trading
- **Price Alerts**: Custom price threshold notifications
- **Multi-Channel Notifications**: Email, Discord, SMS (Twilio)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│              Single Go Binary                           │
│  ┌─────────────────────────────────────────────────┐    │
│  │     Go Templates + HTMX + Tailwind CSS          │    │
│  │  Dashboard │ Analysis │ Recommendations │ ...   │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │              REST API + WebSocket               │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────┐ ┌─────────────┐ ┌───────────────┐      │
│  │ Market Data │ │ AI Analyzer │ │ Notifications │      │
│  │  Aggregator │ │   Router    │ │ Email/Discord │      │
│  └─────────────┘ └─────────────┘ └───────────────┘      │
│                         │                               │
│                    SQLite DB                            │
└─────────────────────────────────────────────────────────┘
```

## Tech Stack

- **Backend**: Go 1.21+
- **Frontend**: Go Templates + HTMX 1.9 + Tailwind CSS (CDN)
- **Database**: SQLite with WAL mode
- **AI Providers**: OpenAI GPT-4, Anthropic Claude, Google Gemini
- **Market Data**: Alpha Vantage, Yahoo Finance, Finnhub

## Quick Start

### Development

```bash
go run ./cmd/server
```

Access at http://localhost:8000

### Production

```bash
# Build
go build -o bin/server ./cmd/server

# Run
PORT=5000 ./bin/server
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8000 | Server port |
| DATABASE_PATH | ./stockmarket.db | SQLite database path |
| ENCRYPTION_KEY | (random) | Base64 32-byte key for API key encryption |
| ENVIRONMENT | development | Environment mode |

### Systemd Service

```bash
sudo cp stockmarket.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable stockmarket
sudo systemctl start stockmarket
```

## API Endpoints

### Pages (HTML)
- `GET /` - Dashboard
- `GET /analysis` - Stock analysis
- `GET /recommendations` - Trading recommendations
- `GET /alerts` - Price alerts
- `GET /settings` - Configuration

### API (JSON)
- `GET /api/health` - Health check
- `GET /api/config` - Get configuration
- `POST /api/config/market` - Update market data settings
- `POST /api/config/ai` - Update AI settings
- `POST /api/config/strategy` - Update trading strategy
- `POST /api/config/watchlist` - Update watchlist
- `POST /api/analyze` - Run stock analysis
- `GET /api/recommendations` - Get recommendations
- `POST /api/alerts` - Create price alert
- `DELETE /api/alerts/:id` - Delete alert
- `GET /ws` - WebSocket for real-time updates

### HTMX Partials
- `GET /partials/watchlist` - Watchlist component
- `GET /partials/recommendations` - Recommendations list
- `GET /partials/analysis-history` - Analysis history
- `GET /partials/alerts-list` - Active alerts

## Configuration

1. Go to **Settings** page
2. Configure your AI provider (OpenAI, Claude, or Gemini) with API key
3. Optionally configure market data provider (Yahoo Finance works without key)
4. Set your risk tolerance and trading frequency
5. Add stock symbols to your watchlist

## License

MIT

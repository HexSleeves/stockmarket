package db

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"stockmarket/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the database connection
type DB struct {
	conn *sql.DB

	// Config cache with TTL
	configCache     *models.UserConfig
	configCacheTime time.Time
	configCacheMu   sync.RWMutex
}

// configCacheTTL is how long to cache config before refreshing
const configCacheTTL = 5 * time.Second

// New creates a new database connection and initializes schema
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}

	// Connection pool settings for SQLite
	// SQLite doesn't benefit from many connections, but these prevent resource exhaustion
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(1 * time.Minute)

	// Verify connection is working
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// migrate runs database migrations
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS user_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		market_data_provider TEXT DEFAULT 'alphavantage',
		market_data_api_key TEXT DEFAULT '',
		ai_provider TEXT DEFAULT 'openai',
		ai_provider_api_key TEXT DEFAULT '',
		ai_model TEXT DEFAULT 'gpt-4o',
		risk_tolerance TEXT DEFAULT 'moderate',
		trade_frequency TEXT DEFAULT 'weekly',
		tracked_symbols TEXT DEFAULT '[]',
		polling_interval INTEGER DEFAULT 30,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notification_channels (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		config_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		target TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		events TEXT DEFAULT '[]',
		FOREIGN KEY (config_id) REFERENCES user_config(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS analysis_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL,
		action TEXT NOT NULL,
		confidence REAL NOT NULL,
		reasoning TEXT NOT NULL,
		price_targets TEXT NOT NULL,
		risks TEXT NOT NULL,
		timeframe TEXT NOT NULL,
		generated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS price_alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		symbol TEXT NOT NULL,
		condition TEXT NOT NULL,
		price REAL NOT NULL,
		triggered INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		title TEXT NOT NULL,
		message TEXT NOT NULL,
		symbol TEXT NOT NULL,
		channels TEXT NOT NULL,
		sent_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_analysis_symbol ON analysis_results(symbol);
	CREATE INDEX IF NOT EXISTS idx_analysis_generated ON analysis_results(generated_at);
	CREATE INDEX IF NOT EXISTS idx_alerts_symbol ON price_alerts(symbol);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Run column migrations (ignore errors for existing columns)
	db.conn.Exec(`ALTER TABLE user_config ADD COLUMN polling_interval INTEGER DEFAULT 30`)

	return nil
}

// GetOrCreateConfig gets the user config or creates a default one (with caching)
func (db *DB) GetOrCreateConfig() (*models.UserConfig, error) {
	// Check cache first
	db.configCacheMu.RLock()
	if db.configCache != nil && time.Since(db.configCacheTime) < configCacheTTL {
		// Return a copy to prevent mutation
		cached := *db.configCache
		cached.TrackedSymbols = append([]string{}, db.configCache.TrackedSymbols...)
		cached.NotificationChannels = append([]models.NotificationConfig{}, db.configCache.NotificationChannels...)
		db.configCacheMu.RUnlock()
		return &cached, nil
	}
	db.configCacheMu.RUnlock()

	// Cache miss - fetch from DB
	config, err := db.fetchConfigFromDB()
	if err != nil {
		return nil, err
	}

	// Update cache
	db.configCacheMu.Lock()
	db.configCache = config
	db.configCacheTime = time.Now()
	db.configCacheMu.Unlock()

	// Return a copy
	result := *config
	result.TrackedSymbols = append([]string{}, config.TrackedSymbols...)
	result.NotificationChannels = append([]models.NotificationConfig{}, config.NotificationChannels...)
	return &result, nil
}

// fetchConfigFromDB retrieves config directly from database
func (db *DB) fetchConfigFromDB() (*models.UserConfig, error) {
	var config models.UserConfig
	var trackedSymbolsJSON string

	err := db.conn.QueryRow(`
		SELECT id, market_data_provider, market_data_api_key, ai_provider,
		       ai_provider_api_key, ai_model, risk_tolerance, trade_frequency,
		       tracked_symbols, COALESCE(polling_interval, 30), created_at, updated_at
		FROM user_config LIMIT 1
	`).Scan(
		&config.ID, &config.MarketDataProvider, &config.MarketDataAPIKey,
		&config.AIProvider, &config.AIProviderAPIKey, &config.AIModel,
		&config.RiskTolerance, &config.TradeFrequency, &trackedSymbolsJSON,
		&config.PollingInterval, &config.CreatedAt, &config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Create default config
		result, err := db.conn.Exec(`
			INSERT INTO user_config (tracked_symbols, polling_interval) VALUES ('[]', 30)
		`)
		if err != nil {
			return nil, err
		}
		id, _ := result.LastInsertId()
		config.ID = id
		config.MarketDataProvider = "alphavantage"
		config.AIProvider = "openai"
		config.AIModel = "gpt-4o"
		config.RiskTolerance = "moderate"
		config.TradeFrequency = "weekly"
		config.TrackedSymbols = []string{}
		config.PollingInterval = 30
		config.CreatedAt = time.Now()
		config.UpdatedAt = time.Now()
		return &config, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse tracked symbols
	json.Unmarshal([]byte(trackedSymbolsJSON), &config.TrackedSymbols)

	// Default polling interval if not set
	if config.PollingInterval == 0 {
		config.PollingInterval = 30
	}

	// Load notification channels
	channels, err := db.GetNotificationChannels(config.ID)
	if err != nil {
		return nil, err
	}
	config.NotificationChannels = channels

	return &config, nil
}

// UpdateConfig updates the user configuration
func (db *DB) UpdateConfig(config *models.UserConfig) error {
	trackedSymbolsJSON, _ := json.Marshal(config.TrackedSymbols)

	_, err := db.conn.Exec(`
		UPDATE user_config SET
			market_data_provider = ?,
			market_data_api_key = ?,
			ai_provider = ?,
			ai_provider_api_key = ?,
			ai_model = ?,
			risk_tolerance = ?,
			trade_frequency = ?,
			tracked_symbols = ?,
			polling_interval = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`,
		config.MarketDataProvider, config.MarketDataAPIKey,
		config.AIProvider, config.AIProviderAPIKey, config.AIModel,
		config.RiskTolerance, config.TradeFrequency, string(trackedSymbolsJSON),
		config.PollingInterval, config.ID,
	)

	// Invalidate cache on update
	if err == nil {
		db.InvalidateConfigCache()
	}

	return err
}

// InvalidateConfigCache clears the config cache
func (db *DB) InvalidateConfigCache() {
	db.configCacheMu.Lock()
	db.configCache = nil
	db.configCacheMu.Unlock()
}

// GetNotificationChannels gets all notification channels for a config
func (db *DB) GetNotificationChannels(configID int64) ([]models.NotificationConfig, error) {
	rows, err := db.conn.Query(`
		SELECT id, type, target, enabled, events FROM notification_channels WHERE config_id = ?
	`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []models.NotificationConfig
	for rows.Next() {
		var ch models.NotificationConfig
		var enabled int
		var eventsJSON string
		if err := rows.Scan(&ch.ID, &ch.Type, &ch.Target, &enabled, &eventsJSON); err != nil {
			return nil, err
		}
		ch.Enabled = enabled == 1
		json.Unmarshal([]byte(eventsJSON), &ch.Events)
		channels = append(channels, ch)
	}
	return channels, nil
}

// SaveNotificationChannel saves a notification channel
func (db *DB) SaveNotificationChannel(configID int64, ch *models.NotificationConfig) error {
	eventsJSON, _ := json.Marshal(ch.Events)
	enabled := 0
	if ch.Enabled {
		enabled = 1
	}

	var err error
	if ch.ID == 0 {
		var result sql.Result
		result, err = db.conn.Exec(`
			INSERT INTO notification_channels (config_id, type, target, enabled, events)
			VALUES (?, ?, ?, ?, ?)
		`, configID, ch.Type, ch.Target, enabled, string(eventsJSON))
		if err != nil {
			return err
		}
		ch.ID, _ = result.LastInsertId()
	} else {
		_, err = db.conn.Exec(`
			UPDATE notification_channels SET type = ?, target = ?, enabled = ?, events = ?
			WHERE id = ?
		`, ch.Type, ch.Target, enabled, string(eventsJSON), ch.ID)
	}

	// Invalidate config cache since notification channels are part of config
	if err == nil {
		db.InvalidateConfigCache()
	}

	return err
}

// DeleteNotificationChannel deletes a notification channel
func (db *DB) DeleteNotificationChannel(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM notification_channels WHERE id = ?`, id)
	return err
}

// SaveAnalysis saves an analysis result
func (db *DB) SaveAnalysis(analysis *models.AnalysisResponse) error {
	priceTargetsJSON, _ := json.Marshal(analysis.PriceTargets)
	risksJSON, _ := json.Marshal(analysis.Risks)

	result, err := db.conn.Exec(`
		INSERT INTO analysis_results (symbol, action, confidence, reasoning, price_targets, risks, timeframe)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, analysis.Symbol, analysis.Action, analysis.Confidence, analysis.Reasoning,
		string(priceTargetsJSON), string(risksJSON), analysis.Timeframe)
	if err != nil {
		return err
	}
	analysis.ID, _ = result.LastInsertId()
	return nil
}

// GetRecentAnalyses gets recent analysis results
func (db *DB) GetRecentAnalyses(limit int) ([]models.AnalysisResponse, error) {
	rows, err := db.conn.Query(`
		SELECT id, symbol, action, confidence, reasoning, price_targets, risks, timeframe, generated_at
		FROM analysis_results ORDER BY generated_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.AnalysisResponse
	for rows.Next() {
		var r models.AnalysisResponse
		var priceTargetsJSON, risksJSON string
		if err := rows.Scan(&r.ID, &r.Symbol, &r.Action, &r.Confidence, &r.Reasoning,
			&priceTargetsJSON, &risksJSON, &r.Timeframe, &r.GeneratedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(priceTargetsJSON), &r.PriceTargets)
		json.Unmarshal([]byte(risksJSON), &r.Risks)
		results = append(results, r)
	}
	return results, nil
}

// GetAnalysesForSymbol gets analysis results for a specific symbol
func (db *DB) GetAnalysesForSymbol(symbol string, limit int) ([]models.AnalysisResponse, error) {
	rows, err := db.conn.Query(`
		SELECT id, symbol, action, confidence, reasoning, price_targets, risks, timeframe, generated_at
		FROM analysis_results WHERE symbol = ? ORDER BY generated_at DESC LIMIT ?
	`, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.AnalysisResponse
	for rows.Next() {
		var r models.AnalysisResponse
		var priceTargetsJSON, risksJSON string
		if err := rows.Scan(&r.ID, &r.Symbol, &r.Action, &r.Confidence, &r.Reasoning,
			&priceTargetsJSON, &risksJSON, &r.Timeframe, &r.GeneratedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(priceTargetsJSON), &r.PriceTargets)
		json.Unmarshal([]byte(risksJSON), &r.Risks)
		results = append(results, r)
	}
	return results, nil
}

// SavePriceAlert saves a price alert
func (db *DB) SavePriceAlert(alert *models.PriceAlert) error {
	result, err := db.conn.Exec(`
		INSERT INTO price_alerts (symbol, condition, price) VALUES (?, ?, ?)
	`, alert.Symbol, alert.Condition, alert.Price)
	if err != nil {
		return err
	}
	alert.ID, _ = result.LastInsertId()
	return nil
}

// GetActiveAlerts gets all untriggered price alerts
func (db *DB) GetActiveAlerts() ([]models.PriceAlert, error) {
	rows, err := db.conn.Query(`
		SELECT id, symbol, condition, price, triggered, created_at
		FROM price_alerts WHERE triggered = 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.PriceAlert
	for rows.Next() {
		var a models.PriceAlert
		var triggered int
		if err := rows.Scan(&a.ID, &a.Symbol, &a.Condition, &a.Price, &triggered, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Triggered = triggered == 1
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// TriggerAlert marks an alert as triggered
func (db *DB) TriggerAlert(id int64) error {
	_, err := db.conn.Exec(`UPDATE price_alerts SET triggered = 1 WHERE id = ?`, id)
	return err
}

// DeletePriceAlert deletes a price alert
func (db *DB) DeletePriceAlert(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM price_alerts WHERE id = ?`, id)
	return err
}

// SaveNotification saves a notification record
func (db *DB) SaveNotification(n *models.Notification) error {
	channelsJSON, _ := json.Marshal(n.Channels)
	result, err := db.conn.Exec(`
		INSERT INTO notifications (type, title, message, symbol, channels) VALUES (?, ?, ?, ?, ?)
	`, n.Type, n.Title, n.Message, n.Symbol, string(channelsJSON))
	if err != nil {
		return err
	}
	n.ID, _ = result.LastInsertId()
	return nil
}

// GetRecommendationsToday gets all recommendations from today
func (db *DB) GetRecommendationsToday() ([]models.Recommendation, error) {
	today := time.Now().Truncate(24 * time.Hour)
	rows, err := db.conn.Query(`
		SELECT id, symbol, action, confidence, reasoning, '', 0, '', generated_at, 'unknown'
		FROM analysis_results WHERE generated_at >= ?
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []models.Recommendation
	for rows.Next() {
		var r models.Recommendation
		var reasoning string
		if err := rows.Scan(&r.ID, &r.Symbol, &r.Action, &r.Confidence, &reasoning,
			&r.Timeframe, &r.TargetPrice, &r.Reasoning, &r.CreatedAt, &r.AIProvider); err != nil {
			return nil, err
		}
		if r.Reasoning == "" {
			r.Reasoning = reasoning
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// GetRecentRecommendations gets recent recommendations
func (db *DB) GetRecentRecommendations(limit int) ([]models.Recommendation, error) {
	rows, err := db.conn.Query(`
		SELECT id, symbol, action, confidence, reasoning, '', 0, '', generated_at, 'unknown'
		FROM analysis_results ORDER BY generated_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []models.Recommendation
	for rows.Next() {
		var r models.Recommendation
		var reasoning string
		if err := rows.Scan(&r.ID, &r.Symbol, &r.Action, &r.Confidence, &reasoning,
			&r.Timeframe, &r.TargetPrice, &r.Reasoning, &r.CreatedAt, &r.AIProvider); err != nil {
			return nil, err
		}
		if r.Reasoning == "" {
			r.Reasoning = reasoning
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// GetFilteredRecommendations gets recommendations with filters
func (db *DB) GetFilteredRecommendations(action string, minConfidence float64, symbol string) ([]models.Recommendation, error) {
	query := `SELECT id, symbol, action, confidence, reasoning, '', 0, '', generated_at, 'unknown'
		FROM analysis_results WHERE 1=1`
	args := []interface{}{}

	if action != "" {
		query += " AND action = ?"
		args = append(args, action)
	}
	if minConfidence > 0 {
		query += " AND confidence >= ?"
		args = append(args, minConfidence)
	}
	if symbol != "" {
		query += " AND symbol = ?"
		args = append(args, symbol)
	}
	query += " ORDER BY generated_at DESC LIMIT 100"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []models.Recommendation
	for rows.Next() {
		var r models.Recommendation
		var reasoning string
		if err := rows.Scan(&r.ID, &r.Symbol, &r.Action, &r.Confidence, &reasoning,
			&r.Timeframe, &r.TargetPrice, &r.Reasoning, &r.CreatedAt, &r.AIProvider); err != nil {
			return nil, err
		}
		if r.Reasoning == "" {
			r.Reasoning = reasoning
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// GetAnalysis gets a single analysis by ID
func (db *DB) GetAnalysis(id int64) (*models.Analysis, error) {
	var a models.Analysis
	var priceTargetsJSON, risksJSON string
	err := db.conn.QueryRow(`
		SELECT id, symbol, action, confidence, reasoning, price_targets, risks, timeframe, generated_at
		FROM analysis_results WHERE id = ?
	`, id).Scan(&a.ID, &a.Symbol, &a.Recommendation.Action, &a.Recommendation.Confidence,
		&a.Recommendation.Reasoning, &priceTargetsJSON, &risksJSON, &a.Recommendation.Timeframe, &a.CreatedAt)
	if err != nil {
		return nil, err
	}

	a.AIProvider = "unknown"
	return &a, nil
}

// GetConfig returns the app config for the settings page
func (db *DB) GetConfig() (*models.AppConfig, error) {
	uc, err := db.GetOrCreateConfig()
	if err != nil {
		return nil, err
	}

	config := &models.AppConfig{
		MarketDataProvider: uc.MarketDataProvider,
		HasMarketAPIKey:    uc.MarketDataAPIKey != "",
		AIProvider:         uc.AIProvider,
		HasAIAPIKey:        uc.AIProviderAPIKey != "",
		AIModel:            uc.AIModel,
		RiskTolerance:      uc.RiskTolerance,
		TradeFrequency:     uc.TradeFrequency,
		TrackedSymbols:     uc.TrackedSymbols,
		PollingInterval:    uc.PollingInterval,
	}

	// Get notification channels
	channels, _ := db.GetNotificationChannels(uc.ID)
	for _, ch := range channels {
		switch ch.Type {
		case "email":
			config.EmailAddress = ch.Target
			config.EmailEnabled = ch.Enabled
		case "discord":
			config.DiscordWebhook = ch.Target
			config.DiscordEnabled = ch.Enabled
		case "sms":
			config.SMSPhone = ch.Target
			config.SMSEnabled = ch.Enabled
		}
	}

	return config, nil
}

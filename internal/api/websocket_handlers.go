package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"stockmarket/internal/config"
	"stockmarket/internal/market"
	"stockmarket/internal/models"

	"github.com/gorilla/websocket"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	log.Printf("WebSocket client connected from %s", r.RemoteAddr)

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected from %s", r.RemoteAddr)
	}()

	// Get user config for tracked symbols
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil {
		log.Printf("Failed to get config: %v", err)
		conn.WriteJSON(map[string]string{"type": "error", "message": "Failed to get config"})
		return
	}

	if len(cfg.TrackedSymbols) == 0 {
		// Send initial message
		conn.WriteJSON(map[string]string{"type": "info", "message": "No symbols tracked. Add symbols in Settings."})
		// Keep connection alive, wait for updates
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
		return
	}

	// Send initial message
	conn.WriteJSON(map[string]string{"type": "info", "message": fmt.Sprintf("Tracking %d symbols", len(cfg.TrackedSymbols))})

	// Decrypt API key
	apiKey := ""
	if cfg.MarketDataAPIKey != "" {
		apiKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}

	// Create market data provider
	provider, err := market.NewProvider(cfg.MarketDataProvider, apiKey)
	if err != nil {
		conn.WriteJSON(map[string]string{"type": "error", "message": "Provider error: " + err.Error()})
		return
	}

	// Create quote channel from provider
	providerCh := make(chan models.Quote, 100)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start streaming quotes from provider
	go func() {
		err := provider.StreamQuotes(ctx, cfg.TrackedSymbols, providerCh)
		if err != nil && err != context.Canceled {
			log.Printf("Stream error: %v", err)
		}
	}()

	// Read goroutine to detect client disconnect
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Mutex for safe writes to websocket
	var writeMu sync.Mutex

	// Process quotes and check alerts
	for {
		select {
		case <-ctx.Done():
			return
		case quote := <-providerCh:
			// Send quote to client
			writeMu.Lock()
			err := conn.WriteJSON(map[string]interface{}{
				"type":  "quote",
				"quote": quote,
			})
			writeMu.Unlock()

			if err != nil {
				return
			}

			// Check alerts for this quote
			s.checkAndTriggerAlerts(quote, cfg, conn, &writeMu)
		}
	}
}

// checkAndTriggerAlerts checks if any price alerts should be triggered for a quote
func (s *Server) checkAndTriggerAlerts(quote models.Quote, cfg *models.UserConfig, conn *websocket.Conn, writeMu *sync.Mutex) {
	alerts, err := s.db.GetActiveAlerts()
	if err != nil {
		return
	}

	for _, alert := range alerts {
		if alert.Symbol != quote.Symbol {
			continue
		}

		var triggered bool
		switch alert.Condition {
		case "above":
			triggered = quote.Price >= alert.Price
		case "below":
			triggered = quote.Price <= alert.Price
		}

		if triggered {
			// Mark alert as triggered in database
			s.db.TriggerAlert(alert.ID)

			// Create alert message
			message := fmt.Sprintf("%s is now $%.2f (%s $%.2f)", alert.Symbol, quote.Price, alert.Condition, alert.Price)

			// Send alert to this WebSocket client
			writeMu.Lock()
			conn.WriteJSON(map[string]interface{}{
				"type":    "alert",
				"title":   fmt.Sprintf("Price Alert: %s", alert.Symbol),
				"message": message,
				"symbol":  alert.Symbol,
				"price":   quote.Price,
			})
			writeMu.Unlock()

			// Also broadcast to all other clients
			s.BroadcastAlert(alert.Symbol, message)

			// Send external notifications
			notification := models.Notification{
				Type:    "price_alert",
				Title:   fmt.Sprintf("Price Alert: %s", alert.Symbol),
				Message: message,
				Symbol:  alert.Symbol,
			}
			go s.notifyService.SendToChannels(notification, cfg.NotificationChannels)

			log.Printf("Alert triggered: %s", message)
		}
	}
}

// BroadcastAlert sends an alert message to all connected WebSocket clients
func (s *Server) BroadcastAlert(symbol, message string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	msg := map[string]interface{}{
		"type":    "alert",
		"title":   fmt.Sprintf("Price Alert: %s", symbol),
		"message": message,
		"symbol":  symbol,
	}

	for conn := range s.clients {
		if err := conn.WriteJSON(msg); err != nil {
			// Mark for removal but don't modify map during iteration
			log.Printf("WebSocket write error: %v", err)
		}
	}
}

// BroadcastToClients sends a message to all connected WebSocket clients
func (s *Server) BroadcastToClients(msg interface{}) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for conn := range s.clients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("WebSocket write error: %v", err)
		}
	}
}

// StartPollingService starts a background service that polls market data
// and checks alerts even when no WebSocket clients are connected
func (s *Server) StartPollingService(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.pollAndCheckAlerts(ctx)
			}
		}
	}()
}

// pollAndCheckAlerts polls market data and checks alerts
func (s *Server) pollAndCheckAlerts(ctx context.Context) {
	cfg, err := s.db.GetOrCreateConfig()
	if err != nil || len(cfg.TrackedSymbols) == 0 {
		return
	}

	// Check if polling is enabled
	if cfg.PollingInterval <= 0 {
		return
	}

	// Decrypt API key
	apiKey := ""
	if cfg.MarketDataAPIKey != "" {
		apiKey, _ = config.Decrypt(cfg.MarketDataAPIKey, s.config.EncryptionKey)
	}

	// Create market data provider
	provider, err := market.NewProvider(cfg.MarketDataProvider, apiKey)
	if err != nil {
		return
	}

	// Get quotes for all tracked symbols
	for _, symbol := range cfg.TrackedSymbols {
		quote, err := provider.GetQuote(ctx, symbol)
		if err != nil {
			continue
		}

		// Broadcast quote to all connected clients
		s.BroadcastToClients(map[string]interface{}{
			"type":  "quote",
			"quote": quote,
		})

		// Check alerts
		alerts, err := s.db.GetActiveAlerts()
		if err != nil {
			continue
		}

		for _, alert := range alerts {
			if alert.Symbol != quote.Symbol {
				continue
			}

			var triggered bool
			switch alert.Condition {
			case "above":
				triggered = quote.Price >= alert.Price
			case "below":
				triggered = quote.Price <= alert.Price
			}

			if triggered {
				s.db.TriggerAlert(alert.ID)
				message := fmt.Sprintf("%s is now $%.2f (%s $%.2f)", alert.Symbol, quote.Price, alert.Condition, alert.Price)

				// Broadcast alert to all clients
				s.BroadcastAlert(alert.Symbol, message)

				// Send external notifications
				notification := models.Notification{
					Type:    "price_alert",
					Title:   fmt.Sprintf("Price Alert: %s", alert.Symbol),
					Message: message,
					Symbol:  alert.Symbol,
				}
				go s.notifyService.SendToChannels(notification, cfg.NotificationChannels)

				log.Printf("Alert triggered (polling): %s", message)
			}
		}
	}
}

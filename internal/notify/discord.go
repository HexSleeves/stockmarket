package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"stockmarket/internal/models"
)

// DiscordNotifier sends notifications via Discord webhook
type DiscordNotifier struct {
	client *http.Client
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier() *DiscordNotifier {
	return &DiscordNotifier{
		client: sharedHTTPClient,
	}
}

// Type returns the notifier type
func (d *DiscordNotifier) Type() string {
	return "discord"
}

// Send sends a Discord webhook notification
func (d *DiscordNotifier) Send(notification models.Notification, target string) error {
	if target == "" {
		fmt.Println("[DISCORD] No webhook URL provided, skipping")
		return nil
	}
	fmt.Printf("[DISCORD] Sending to webhook: %s...\n", target[:50])

	// Choose color based on notification type
	color := 0x808080 // gray
	switch notification.Type {
	case "buy_signal":
		color = 0x00FF00 // green
	case "sell_signal":
		color = 0xFF0000 // red
	case "price_alert":
		color = 0xFFFF00 // yellow
	}

	webhook := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       notification.Title,
				"description": notification.Message,
				"color":       color,
				"fields": []map[string]interface{}{
					{
						"name":   "Symbol",
						"value":  notification.Symbol,
						"inline": true,
					},
					{
						"name":   "Type",
						"value":  notification.Type,
						"inline": true,
					},
				},
				"timestamp": time.Now().Format(time.RFC3339),
				"footer": map[string]string{
					"text": "Stock Market Analysis Platform",
				},
			},
		},
	}

	jsonBody, err := json.Marshal(webhook)
	if err != nil {
		return err
	}

	resp, err := d.client.Post(target, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: discord returned status %d", ErrNotificationFailed, resp.StatusCode)
	}

	return nil
}

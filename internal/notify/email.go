package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"stockmarket/internal/models"
)

// EmailNotifier sends notifications via email (using Resend API)
type EmailNotifier struct {
	apiKey    string
	fromEmail string
	client    *http.Client
}

// NewEmailNotifier creates a new email notifier using Resend
func NewEmailNotifier(config map[string]string) *EmailNotifier {
	apiKey := config["resend_api_key"]
	if apiKey == "" {
		apiKey = os.Getenv("RESEND_API_KEY")
	}

	fromEmail := config["from_email"]
	if fromEmail == "" {
		fromEmail = "StockAI <alerts@resend.dev>" // Default Resend sender
	}

	return &EmailNotifier{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		client:    sharedHTTPClient,
	}
}

// Type returns the notifier type
func (e *EmailNotifier) Type() string {
	return "email"
}

// Send sends an email notification via Resend API
func (e *EmailNotifier) Send(notification models.Notification, target string) error {
	if e.apiKey == "" {
		// Log but don't fail - email not configured
		fmt.Printf("[EMAIL] Would send to %s: %s - %s\n", target, notification.Title, notification.Message)
		return nil
	}

	// Build the email payload for Resend
	payload := map[string]interface{}{
		"from":    e.fromEmail,
		"to":      []string{target},
		"subject": notification.Title,
		"html":    formatEmailBody(notification),
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: failed to marshal email payload: %v", ErrNotificationFailed, err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("%w: failed to create request: %v", ErrNotificationFailed, err)
	}

	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: failed to send email: %v", ErrNotificationFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("%w: resend returned status %d: %v", ErrNotificationFailed, resp.StatusCode, errResp)
	}

	fmt.Printf("[EMAIL] Successfully sent email to %s\n", target)
	return nil
}

func formatEmailBody(n models.Notification) string {
	// Choose color based on notification type
	color := "#6366f1" // default indigo
	switch n.Type {
	case "buy_signal":
		color = "#22c55e" // green
	case "sell_signal":
		color = "#ef4444" // red
	case "price_alert":
		color = "#eab308" // yellow
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f3f4f6;">
  <table role="presentation" style="width: 100%%; border-collapse: collapse;">
    <tr>
      <td style="padding: 40px 20px;">
        <table role="presentation" style="max-width: 600px; margin: 0 auto; background: white; border-radius: 12px; overflow: hidden; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);">
          <!-- Header -->
          <tr>
            <td style="background: linear-gradient(135deg, #1e1b4b 0%%, #312e81 100%%); padding: 30px; text-align: center;">
              <h1 style="margin: 0; color: white; font-size: 24px; font-weight: 600;">📈 StockAI Alert</h1>
            </td>
          </tr>
          <!-- Alert Badge -->
          <tr>
            <td style="padding: 30px 30px 0 30px; text-align: center;">
              <span style="display: inline-block; background: %s; color: white; padding: 8px 16px; border-radius: 20px; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px;">%s</span>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 30px;">
              <h2 style="margin: 0 0 10px 0; color: #111827; font-size: 20px; font-weight: 600;">%s</h2>
              <p style="margin: 0 0 20px 0; color: #6b7280; font-size: 16px; line-height: 1.5;">%s</p>
              <table role="presentation" style="width: 100%%; background: #f9fafb; border-radius: 8px; padding: 20px;">
                <tr>
                  <td style="padding: 10px 20px;">
                    <p style="margin: 0; color: #9ca3af; font-size: 12px; text-transform: uppercase; letter-spacing: 0.5px;">Symbol</p>
                    <p style="margin: 5px 0 0 0; color: #111827; font-size: 18px; font-weight: 600;">%s</p>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 20px 30px; background: #f9fafb; text-align: center; border-top: 1px solid #e5e7eb;">
              <p style="margin: 0; color: #9ca3af; font-size: 12px;">Sent by StockAI • Stock Market Analysis Platform</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>
`, color, n.Type, n.Title, n.Message, n.Symbol)
}

package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"stockmarket/internal/models"
)

// SMSNotifier sends notifications via Twilio SMS
type SMSNotifier struct {
	accountSID string
	authToken  string
	fromNumber string
	client     *http.Client
}

// NewSMSNotifier creates a new SMS notifier (Twilio)
func NewSMSNotifier(config map[string]string) *SMSNotifier {
	accountSID := config["twilio_account_sid"]
	if accountSID == "" {
		accountSID = os.Getenv("TWILIO_ACCOUNT_SID")
	}

	authToken := config["twilio_auth_token"]
	if authToken == "" {
		authToken = os.Getenv("TWILIO_AUTH_TOKEN")
	}

	fromNumber := config["twilio_from_number"]
	if fromNumber == "" {
		fromNumber = os.Getenv("TWILIO_FROM_NUMBER")
	}

	return &SMSNotifier{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
		client:     sharedHTTPClient,
	}
}

// Type returns the notifier type
func (s *SMSNotifier) Type() string {
	return "sms"
}

// Send sends an SMS notification via Twilio
func (s *SMSNotifier) Send(notification models.Notification, target string) error {
	if s.accountSID == "" {
		// Log but don't fail - SMS not configured
		fmt.Printf("[SMS] Would send to %s: %s - %s\n", target, notification.Title, notification.Message)
		return nil
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSID)

	message := fmt.Sprintf("%s\n%s: %s", notification.Title, notification.Symbol, notification.Message)
	if len(message) > 160 {
		message = message[:157] + "..."
	}

	data := url.Values{}
	data.Set("To", target)
	data.Set("From", s.fromNumber)
	data.Set("Body", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(s.accountSID, s.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotificationFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		fmt.Printf("[SMS] Twilio error response: %v\n", errResp)
		return fmt.Errorf("%w: twilio returned status %d: %v", ErrNotificationFailed, resp.StatusCode, errResp)
	}

	return nil
}

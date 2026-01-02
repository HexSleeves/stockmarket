package notify

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"stockmarket/internal/models"
)

// Shared HTTP client with optimized transport for all notifiers
var sharedHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

// Notifier defines the interface for notification dispatchers
type Notifier interface {
	Send(notification models.Notification, target string) error
	Type() string
}

// ErrNotificationFailed is returned when notification fails
var ErrNotificationFailed = errors.New("notification failed")

// NewNotifier creates a notifier based on the type
func NewNotifier(notifType string, config map[string]string) (Notifier, error) {
	switch notifType {
	case "email":
		return NewEmailNotifier(config), nil
	case "discord":
		return NewDiscordNotifier(), nil
	case "sms":
		return NewSMSNotifier(config), nil
	default:
		return nil, errors.New("unknown notifier type: " + notifType)
	}
}

// Service manages sending notifications to configured channels
type Service struct {
	notifiers map[string]Notifier
}

// NewService creates a new notification service
func NewService() *Service {
	return &Service{
		notifiers: make(map[string]Notifier),
	}
}

// RegisterNotifier registers a notifier
func (s *Service) RegisterNotifier(n Notifier) {
	s.notifiers[n.Type()] = n
}

// SendToChannels sends a notification to all enabled channels
func (s *Service) SendToChannels(notification models.Notification, channels []models.NotificationConfig) []error {
	var errs []error

	log.Printf("[NOTIFY] Sending notification type=%s to %d channels", notification.Type, len(channels))

	for _, ch := range channels {
		if !ch.Enabled {
			log.Printf("[NOTIFY] Skipping disabled channel: %s", ch.Type)
			continue
		}

		// Check if this event should trigger the channel
		eventMatch := false
		for _, event := range ch.Events {
			if event == notification.Type {
				eventMatch = true
				break
			}
		}
		if !eventMatch {
			log.Printf("[NOTIFY] Channel %s doesn't handle event %s (events: %v)", ch.Type, notification.Type, ch.Events)
			continue
		}

		notifier, ok := s.notifiers[ch.Type]
		if !ok {
			log.Printf("[NOTIFY] No notifier registered for type: %s", ch.Type)
			errs = append(errs, errors.New("no notifier for type: "+ch.Type))
			continue
		}

		log.Printf("[NOTIFY] Sending %s notification to %s", ch.Type, ch.Target)
		if err := notifier.Send(notification, ch.Target); err != nil {
			log.Printf("[NOTIFY] Failed to send %s notification: %v", ch.Type, err)
			errs = append(errs, err)
		} else {
			log.Printf("[NOTIFY] Successfully sent %s notification", ch.Type)
		}
	}

	return errs
}

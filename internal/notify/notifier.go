package notify

import (
	"errors"

	"stockmarket/internal/models"
)

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

	for _, ch := range channels {
		if !ch.Enabled {
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
			continue
		}

		notifier, ok := s.notifiers[ch.Type]
		if !ok {
			errs = append(errs, errors.New("no notifier for type: "+ch.Type))
			continue
		}

		if err := notifier.Send(notification, ch.Target); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

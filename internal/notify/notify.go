package notify

import (
	"fmt"
	"strings"
)

// EventType classifies a notification event.
type EventType string

// EventReleaseComplete fires once the SCM release has been successfully published.
const EventReleaseComplete EventType = "release_complete"

// Event carries the data each channel needs to format its message.
type Event struct {
	Type    EventType
	Tag     string // e.g. "v1.2.3"
	Version string // e.g. "1.2.3"
	URL     string // SCM release URL; may be empty
	Body    string // changelog markdown
}

// Notifier dispatches a release event to one or more notification channels.
type Notifier interface {
	Notify(event Event) error
}

// Build constructs a Notifier from cfg, fanning out to all active channels.
// Returns nil if no channels are configured.
func Build(cfg Config) (Notifier, error) {
	var channels []Notifier

	if cfg.Email != nil {
		ch, err := newEmail(*cfg.Email)
		if err != nil {
			return nil, fmt.Errorf("email notifier: %w", err)
		}
		channels = append(channels, ch)
	}

	if cfg.Telegram != nil {
		ch, err := newTelegram(*cfg.Telegram)
		if err != nil {
			return nil, fmt.Errorf("telegram notifier: %w", err)
		}
		channels = append(channels, ch)
	}

	if cfg.Desktop {
		channels = append(channels, &desktop{})
	}

	if cfg.Slack != nil {
		ch, err := newSlack(*cfg.Slack)
		if err != nil {
			return nil, fmt.Errorf("slack notifier: %w", err)
		}
		channels = append(channels, ch)
	}

	if cfg.Webhook != nil {
		ch, err := newWebhook(*cfg.Webhook)
		if err != nil {
			return nil, fmt.Errorf("webhook notifier: %w", err)
		}
		channels = append(channels, ch)
	}

	if len(channels) == 0 {
		return nil, nil
	}

	return &multi{channels: channels}, nil
}

// multi fans out a Notify call to all configured channels.
// All channels are attempted; errors are collected and returned together.
type multi struct {
	channels []Notifier
}

func (m *multi) Notify(event Event) error {
	var errs []string
	for _, ch := range m.channels {
		if err := ch.Notify(event); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

package notify

import (
	"fmt"
	"strings"

	"github.com/mxstzdev/releasar-cli/internal/log"
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
func Build(cfg Config, log *log.Channel) (Notifier, error) {
	var channels []Notifier
	var names []string

	if cfg.Email != nil {
		ch, err := newEmail(*cfg.Email, log)
		if err != nil {
			return nil, fmt.Errorf("email notifier: %w", err)
		}
		channels = append(channels, ch)
		names = append(names, "email")
	}

	if cfg.Telegram != nil {
		ch, err := newTelegram(*cfg.Telegram, log)
		if err != nil {
			return nil, fmt.Errorf("telegram notifier: %w", err)
		}
		channels = append(channels, ch)
		names = append(names, "telegram")
	}

	if cfg.Desktop {
		channels = append(channels, newDesktop(log))
		names = append(names, "desktop")
	}

	if cfg.Slack != nil {
		ch, err := newSlack(*cfg.Slack, log)
		if err != nil {
			return nil, fmt.Errorf("slack notifier: %w", err)
		}
		channels = append(channels, ch)
		names = append(names, "slack")
	}

	if cfg.Webhook != nil {
		ch, err := newWebhook(*cfg.Webhook, log)
		if err != nil {
			return nil, fmt.Errorf("webhook notifier: %w", err)
		}
		channels = append(channels, ch)
		names = append(names, "webhook")
	}

	if len(channels) == 0 {
		return nil, nil
	}

	log.Debug("notification channels configured", map[string]any{"channels": names})
	return &multi{channels: channels, names: names, log: log}, nil
}

// multi fans out a Notify call to all configured channels.
// All channels are attempted; errors are collected and returned together.
type multi struct {
	channels []Notifier
	names    []string
	log      *log.Channel
}

func (m *multi) Notify(event Event) error {
	var errs []string
	for i, ch := range m.channels {
		var name string
		if i < len(m.names) {
			name = m.names[i]
		}
		if m.log != nil {
			m.log.Debug("dispatching notification", map[string]any{"channel": name})
		}
		if err := ch.Notify(event); err != nil {
			if m.log != nil {
				m.log.Warn("notification channel failed", map[string]any{"channel": name, "error": err})
			}
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

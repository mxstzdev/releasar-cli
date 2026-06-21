package notify

import (
	"fmt"
	"net/smtp"
	"strings"

	"github.com/mxstzdev/releasar-cli/internal/log"
)

type email struct {
	cfg EmailConfig
	log *log.Channel
}

func newEmail(cfg EmailConfig, log *log.Channel) (*email, error) {
	if cfg.SMTPHost == "" {
		return nil, fmt.Errorf("smtpHost is required")
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("from address is required")
	}
	if len(cfg.To) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}
	return &email{cfg: cfg, log: log}, nil
}

func (e *email) Notify(event Event) error {
	subject := applyTemplate(e.cfg.Subject, event)

	body := event.Body
	if body == "" {
		body = fmt.Sprintf("Released %s", event.Tag)
	}
	if event.URL != "" {
		body += "\n\n" + event.URL
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		e.cfg.From,
		strings.Join(e.cfg.To, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", e.cfg.SMTPHost, e.cfg.SMTPPort)
	var auth smtp.Auth
	if e.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", e.cfg.SMTPUser, e.cfg.SMTPPass, e.cfg.SMTPHost)
	}

	if err := smtp.SendMail(addr, auth, e.cfg.From, e.cfg.To, []byte(msg)); err != nil {
		e.log.Error("Email notification failed", map[string]any{"addr": addr, "error": err})
		return fmt.Errorf("sending email: %w", err)
	}
	e.log.Debug("Email notification sent", map[string]any{"addr": addr, "recipients": len(e.cfg.To)})
	return nil
}

// applyTemplate replaces {{tag}} and {{version}} placeholders in s.
func applyTemplate(s string, event Event) string {
	s = strings.ReplaceAll(s, "{{tag}}", event.Tag)
	s = strings.ReplaceAll(s, "{{version}}", event.Version)
	return s
}

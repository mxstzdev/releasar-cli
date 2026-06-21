package notify

// Config holds resolved credentials for all notification channels.
// All token/password fields carry the actual value (not an env var name);
// env var resolution happens in cmd/release.go before Build is called.
type Config struct {
	Email    *EmailConfig
	Telegram *TelegramConfig
	Desktop  bool
	Slack    *SlackConfig
	Webhook  *WebhookConfig
}

// EmailConfig configures the SMTP email channel.
type EmailConfig struct {
	SMTPHost string
	SMTPPort int // defaults to 587 when zero
	SMTPUser string
	SMTPPass string
	From     string
	To       []string
	Subject  string // template: {{tag}}, {{version}}
}

// TelegramConfig configures the Telegram Bot API channel.
type TelegramConfig struct {
	Token  string
	ChatID string
}

// SlackConfig configures the Slack incoming webhook channel.
type SlackConfig struct {
	WebhookURL string
}

// WebhookConfig configures the generic JSON POST webhook channel.
type WebhookConfig struct {
	URL     string
	Headers map[string]string
}

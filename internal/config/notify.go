package config

// NotifyConfig holds post-release notification channel configuration.
// Each sub-config is a pointer; nil means the channel is disabled.
type NotifyConfig struct {
	Email    *EmailNotifyConfig
	Telegram *TelegramNotifyConfig
	Desktop  *DesktopNotifyConfig
	Slack    *SlackNotifyConfig
	Webhook  *WebhookNotifyConfig
}

// EmailNotifyConfig configures the SMTP email channel.
type EmailNotifyConfig struct {
	SMTPHost    string
	SMTPPort    int    // defaults to 587
	SMTPUserEnv string // env var name holding the SMTP username
	SMTPPassEnv string // env var name holding the SMTP password
	From        string
	To          []string
	Subject     string // template: {{tag}}, {{version}}
}

// TelegramNotifyConfig configures the Telegram Bot API channel.
type TelegramNotifyConfig struct {
	TokenEnv string // env var name holding the bot token
	ChatID   string
}

// DesktopNotifyConfig enables native OS desktop notifications via beeep.
// Presence of this block in config is sufficient to enable the channel.
type DesktopNotifyConfig struct{}

// SlackNotifyConfig configures the Slack incoming webhook channel.
type SlackNotifyConfig struct {
	WebhookEnv string // env var name holding the webhook URL
}

// WebhookNotifyConfig configures the generic JSON POST webhook channel.
type WebhookNotifyConfig struct {
	URL        string
	Headers    map[string]string // static header values
	HeadersEnv map[string]string // header name → env var name; resolved at runtime
}

// --- raw deserialization types ---

type rawNotifyConfig struct {
	Email    *rawEmailNotifyConfig    `json:"email"`
	Telegram *rawTelegramNotifyConfig `json:"telegram"`
	Desktop  *rawDesktopNotifyConfig  `json:"desktop"`
	Slack    *rawSlackNotifyConfig    `json:"slack"`
	Webhook  *rawWebhookNotifyConfig  `json:"webhook"`
}

type rawEmailNotifyConfig struct {
	SMTPHost    string   `json:"smtpHost"`
	SMTPPort    int      `json:"smtpPort"`
	SMTPUserEnv string   `json:"smtpUserEnv"`
	SMTPPassEnv string   `json:"smtpPassEnv"`
	From        string   `json:"from"`
	To          []string `json:"to"`
	Subject     string   `json:"subject"`
}

type rawTelegramNotifyConfig struct {
	TokenEnv string `json:"tokenEnv"`
	ChatID   string `json:"chatId"`
}

type rawDesktopNotifyConfig struct{}

type rawSlackNotifyConfig struct {
	WebhookEnv string `json:"webhookEnv"`
}

type rawWebhookNotifyConfig struct {
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	HeadersEnv map[string]string `json:"headersEnv"`
}

package notify

import (
	"errors"
	"testing"

	"github.com/mxstzdev/releasar-cli/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNotifier struct {
	received []Event
	err      error
}

func (f *fakeNotifier) Notify(event Event) error {
	f.received = append(f.received, event)
	return f.err
}

func TestBuild_ReturnsNilWhenNoChannels(t *testing.T) {
	n, err := Build(Config{}, log.Nop())
	require.NoError(t, err)
	assert.Nil(t, n)
}

func TestMulti_Notify_FansOutToAllChannels(t *testing.T) {
	a := &fakeNotifier{}
	b := &fakeNotifier{}
	m := &multi{channels: []Notifier{a, b}}

	event := Event{Type: EventReleaseComplete, Tag: "v1.0.0", Version: "1.0.0"}
	require.NoError(t, m.Notify(event))

	assert.Len(t, a.received, 1)
	assert.Equal(t, event, a.received[0])
	assert.Len(t, b.received, 1)
	assert.Equal(t, event, b.received[0])
}

func TestMulti_Notify_AccumulatesErrors(t *testing.T) {
	a := &fakeNotifier{err: errors.New("channel A failed")}
	b := &fakeNotifier{err: errors.New("channel B failed")}
	m := &multi{channels: []Notifier{a, b}}

	err := m.Notify(Event{Type: EventReleaseComplete, Tag: "v1.0.0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel A failed")
	assert.Contains(t, err.Error(), "channel B failed")
}

func TestMulti_Notify_ContinuesAfterPartialError(t *testing.T) {
	a := &fakeNotifier{err: errors.New("channel A failed")}
	b := &fakeNotifier{}
	m := &multi{channels: []Notifier{a, b}}

	err := m.Notify(Event{Type: EventReleaseComplete, Tag: "v1.0.0"})
	require.Error(t, err)

	// b must still have received the event despite a failing
	assert.Len(t, b.received, 1)
}

func TestEmail_RequiresConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     EmailConfig
		wantMsg string
	}{
		{
			name:    "missing smtpHost",
			cfg:     EmailConfig{From: "a@b.com", To: []string{"c@d.com"}},
			wantMsg: "smtpHost",
		},
		{
			name:    "missing from",
			cfg:     EmailConfig{SMTPHost: "smtp.example.com", To: []string{"c@d.com"}},
			wantMsg: "from",
		},
		{
			name:    "missing recipients",
			cfg:     EmailConfig{SMTPHost: "smtp.example.com", From: "a@b.com"},
			wantMsg: "recipient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newEmail(tt.cfg, log.Nop())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestTelegram_RequiresConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     TelegramConfig
		wantMsg string
	}{
		{
			name:    "missing token",
			cfg:     TelegramConfig{ChatID: "-100123"},
			wantMsg: "token",
		},
		{
			name:    "missing chatId",
			cfg:     TelegramConfig{Token: "bot-token"},
			wantMsg: "chatId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTelegram(tt.cfg, log.Nop())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestWebhook_RequiresURL(t *testing.T) {
	_, err := newWebhook(WebhookConfig{}, log.Nop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestSlack_RequiresWebhookURL(t *testing.T) {
	_, err := newSlack(SlackConfig{}, log.Nop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook URL")
}

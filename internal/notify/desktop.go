package notify

import (
	"fmt"

	"github.com/gen2brain/beeep"
	"github.com/mxstzdev/releasar-cli/internal/log"
)

type desktop struct {
	log *log.Channel
}

func newDesktop(log *log.Channel) *desktop {
	return &desktop{log: log}
}

func (d *desktop) Notify(event Event) error {
	title := fmt.Sprintf("Released %s", event.Tag)
	msg := "Release complete"
	if event.URL != "" {
		msg = event.URL
	}
	if err := beeep.Notify(title, msg, ""); err != nil {
		d.log.Error("Desktop notification failed", map[string]any{"error": err})
		return fmt.Errorf("desktop notification: %w", err)
	}
	d.log.Debug("Desktop notification sent", map[string]any{"tag": event.Tag})
	return nil
}

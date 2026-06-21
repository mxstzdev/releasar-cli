package notify

import (
	"fmt"

	"github.com/gen2brain/beeep"
)

type desktop struct{}

func (d *desktop) Notify(event Event) error {
	title := fmt.Sprintf("Released %s", event.Tag)
	msg := "Release complete"
	if event.URL != "" {
		msg = event.URL
	}
	if err := beeep.Notify(title, msg, ""); err != nil {
		return fmt.Errorf("desktop notification: %w", err)
	}
	return nil
}

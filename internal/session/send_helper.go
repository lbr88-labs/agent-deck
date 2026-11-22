package session

import (
	"context"
)

// NotifyWatcher queues an event for the recipient resolved by the active UI profile.
func NotifyWatcher(profile, recipient, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), wakeNudgeSendTimeout)
	defer cancel()
	_, err := queueInboxMessage(ctx, profile, "system:watchdog", recipient, body)
	return err
}

package session

import (
	"context"
	"log/slog"
	"time"
)

// Issue #1225 Tier-2 WIRING. inbox_nudge.go holds the platform-independent
// policy core (WakeNudger: debounced, idle-only, best-effort). This file wires
// it into the live producer commit path so an IDLE conductor is woken to drain
// the MOMENT a completion durably lands — instead of waiting up to ~14 min for
// its next heartbeat. The trigger is event-driven (fired synchronously from the
// commit chokepoint, commitEventToInbox), NOT a poll loop.
//
// Everything here is best-effort: a nil wiring, a busy/non-conductor parent, or
// a failed send is harmless because the durable record is still drained on the
// parent's next Stop/heartbeat. Wake ≠ deliver.

// defaultWakeNudgeDebounce bounds how often a single idle parent is woken by a
// write-triggered nudge. A burst of N children completing near-simultaneously
// collapses to ONE wake; the parent's Stop-hook drain then consumes every
// pending record in that single woken turn, so a suppressed nudge loses no
// delivery. The window NEVER delays the FIRST completion (Nudge sends the first
// and only debounces subsequent ones), so it adds zero latency to the common
// single-completion case. ~500ms is the audit-fleet sweet spot: long enough to
// kill a thundering-herd of send-keys, short enough to be imperceptible — and
// well under the ~100-300ms Claude pane-pickup cost that dominates real latency.
const defaultWakeNudgeDebounce = 500 * time.Millisecond

// wakeNudgeMessage is the prompt fired into an idle conductor's pane to wake it.
// The content does not affect delivery — taking ANY turn runs the conductor's
// Stop-hook drain, which is what actually consumes the durable inbox. The text
// only tells the conductor WHY it woke so it acts on the queue immediately.
const wakeNudgeMessage = "[INBOX] A child just committed a completion to your inbox — drain it and act on each item now."

// wakeNudgeWiring carries the platform hooks the Tier-2 wake-nudge needs. It is
// kept out of the WakeNudger policy core so the idle-only/debounced policy stays
// testable without tmux: tests inject spy probes, production wires the live
// status probe + a best-effort no-wait pane send.
type wakeNudgeWiring struct {
	nudger *WakeNudger
	now    func() time.Time
	isIdle func(parent *Instance) bool
	send   func(parent *Instance, profile string) error
}

// defaultWakeNudgeWiring is the production wiring: a debounced nudger, the wall
// clock, the conductor-scoped idle probe, and a best-effort non-blocking pane
// send.
func defaultWakeNudgeWiring() *wakeNudgeWiring {
	return &wakeNudgeWiring{
		nudger: NewWakeNudger(defaultWakeNudgeDebounce),
		now:    time.Now,
		isIdle: parentIsNudgeableIdle,
		send:   sendWakeNudge,
	}
}

// fireWakeNudge invokes the Tier-2 wake-nudge for a parent that just had a
// completion durably committed. It is best-effort and MUST NOT affect the commit
// result: a nil wiring, a non-conductor/busy parent, or a send error are all
// swallowed (the durable record still drains on the next turn/heartbeat). A
// panic in the injected probe/send is recovered so a wake bug can never take
// down the producer.
func (n *TransitionNotifier) fireWakeNudge(parent *Instance, event TransitionNotificationEvent) {
	w := n.wake
	if w == nil || w.nudger == nil || parent == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			commsLog.Warn("wake_nudge_panic_recovered",
				slog.String("parent", parent.ID), slog.Any("panic", r))
		}
	}()

	now := time.Now()
	if w.now != nil {
		now = w.now()
	}
	profile := event.Profile
	isIdle := func() bool { return w.isIdle != nil && w.isIdle(parent) }
	send := func() error {
		if w.send == nil {
			return nil
		}
		return w.send(parent, profile)
	}
	if _, err := w.nudger.Nudge(parent.ID, now, isIdle, send); err != nil {
		// Best-effort: a failed wake is harmless. Log once at debug-ish level so
		// the operator can see WHY a pane wasn't woken without it being an error.
		commsLog.Warn("wake_nudge_send_failed",
			slog.String("parent", parent.ID), slog.String("error", err.Error()))
	}
}

// parentIsNudgeableIdle limits reminder traffic to idle conductors. Busy
// conductors already drain their completion inbox at the turn boundary.
//
// parent.Status is read directly (not re-probed): the conductor branch of
// resolveParentNotificationTarget freshly UpdateStatus()'d this same instance
// moments earlier on the commit path, so a second tmux round-trip would add cost
// without adding freshness.
func parentIsNudgeableIdle(parent *Instance) bool {
	if parent == nil || !isConductorSessionTitle(parent.Title) {
		return false
	}
	switch parent.Status {
	case StatusIdle, StatusWaiting:
		return true
	default:
		return false
	}
}

// sendWakeNudge schedules an inbox reminder off the producer commit path.
func sendWakeNudge(parent *Instance, profile string) error {
	if parent == nil {
		return nil
	}
	go func(profile, ref string) {
		if err := sendWakeNudgeNoWait(profile, ref); err != nil {
			commsLog.Warn("wake_nudge_dispatch_failed",
				slog.String("parent", ref), slog.String("error", err.Error()))
		}
	}(profile, parent.ID)
	return nil
}

// Bound the detached write so a stalled inbox cannot accumulate wake goroutines.
const wakeNudgeSendTimeout = 5 * time.Second

func sendWakeNudgeNoWait(profile, recipient string) error {
	ctx, cancel := context.WithTimeout(context.Background(), wakeNudgeSendTimeout)
	defer cancel()
	_, err := queueInboxMessage(ctx, profile, "system:completion-wake", recipient, wakeNudgeMessage)
	return err
}

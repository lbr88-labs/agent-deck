package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/logging"
	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// EngineConfig holds the configuration for the watcher Engine.
type EngineConfig struct {
	// DB is the state database for event persistence and dedup.
	DB *statedb.StateDB

	// Router routes events to conductors based on sender.
	Router *Router

	// MaxEventsPerWatcher limits the number of stored events per watcher (pruning threshold).
	MaxEventsPerWatcher int

	// HealthCheckInterval controls how often adapter health checks run.
	// Set to 0 to disable the health check loop (useful in tests).
	HealthCheckInterval time.Duration

	// Logger is the structured logger. Defaults to logging.ForComponent(logging.CompWatcher).
	Logger *slog.Logger
}

// eventEnvelope wraps an Event with metadata for the single-writer goroutine.
// This avoids modifying the public Event struct with internal routing fields.
type eventEnvelope struct {
	event     Event
	watcherID string
	tracker   *HealthTracker
}

// adapterEntry holds a registered adapter with its associated metadata.
type adapterEntry struct {
	adapter   WatcherAdapter
	config    AdapterConfig
	watcherID string
	tracker   *HealthTracker
	cancel    context.CancelFunc
}

// Engine orchestrates the watcher event pipeline: adapter goroutines produce Events,
// a single-writer goroutine serializes DB writes via a buffered channel, dedup is
// handled by INSERT OR IGNORE, and the router determines event routing.
//
// Lifecycle: NewEngine -> RegisterAdapter (1..N) -> Start -> Stop.
type Engine struct {
	cfg      EngineConfig
	adapters []adapterEntry

	// eventCh is the internal channel from adapter goroutines to the single-writer.
	// Capacity 64 per D-12 / T-13-06.
	eventCh chan eventEnvelope

	// routedEventCh is the exported channel for TUI consumption (D-20).
	// Successfully persisted events are forwarded here.
	routedEventCh chan Event

	// healthCh is the exported channel for health state updates (D-20).
	healthCh chan HealthState

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	log    *slog.Logger
}

// NewEngine creates a new Engine with the provided configuration.
// Call RegisterAdapter to add adapters, then Start to begin processing.
func NewEngine(cfg EngineConfig) *Engine {
	logger := cfg.Logger
	if logger == nil {
		logger = logging.ForComponent(logging.CompWatcher)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		cfg:           cfg,
		adapters:      make([]adapterEntry, 0),
		eventCh:       make(chan eventEnvelope, 64),
		routedEventCh: make(chan Event, 64),
		healthCh:      make(chan HealthState, 16),
		ctx:           ctx,
		cancel:        cancel,
		log:           logger,
	}
}

// RegisterAdapter adds an adapter to the engine. Must be called before Start.
// watcherID is the statedb watcher ID used for SaveWatcherEvent persistence.
// maxSilenceMinutes is the threshold for silence detection in the health tracker.
func (e *Engine) RegisterAdapter(watcherID string, adapter WatcherAdapter, config AdapterConfig, maxSilenceMinutes int) {
	tracker := NewHealthTracker(config.Name, maxSilenceMinutes)
	e.adapters = append(e.adapters, adapterEntry{
		adapter:   adapter,
		config:    config,
		watcherID: watcherID,
		tracker:   tracker,
	})
}

// Start begins the event pipeline. For each registered adapter, it calls Setup,
// then launches an adapter goroutine. It also starts the single-writer goroutine
// and optionally the health check loop.
func (e *Engine) Start() error {
	for i := range e.adapters {
		entry := &e.adapters[i]

		if err := entry.adapter.Setup(e.ctx, entry.config); err != nil {
			e.log.Warn("adapter_setup_failed",
				slog.String("watcher", entry.config.Name),
				slog.String("type", entry.config.Type),
				slog.String("error", err.Error()),
			)
			continue
		}

		adapterCtx, adapterCancel := context.WithCancel(e.ctx)
		entry.cancel = adapterCancel

		e.wg.Add(1)
		go e.runAdapter(adapterCtx, entry)
	}

	// Single-writer goroutine serializes all DB writes (D-13).
	e.wg.Add(1)
	go e.writerLoop()

	// Health check loop (optional, disabled when HealthCheckInterval is 0).
	if e.cfg.HealthCheckInterval > 0 {
		e.wg.Add(1)
		go e.healthLoop()
	}

	return nil
}

// runAdapter runs a single adapter's Listen loop in its own goroutine.
// Events are wrapped in envelopes and sent to the single-writer via eventCh.
func (e *Engine) runAdapter(ctx context.Context, entry *adapterEntry) {
	defer e.wg.Done()

	// Create an intermediary channel for the adapter to send events to.
	// We wrap each event in an envelope before forwarding to the engine's eventCh.
	adapterCh := make(chan Event, 64)

	// Launch a goroutine to forward events from the adapter channel to the engine channel.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for evt := range adapterCh {
			env := eventEnvelope{
				event:     evt,
				watcherID: entry.watcherID,
				tracker:   entry.tracker,
			}
			// Non-blocking send to prevent adapter goroutine hang if channel full (T-13-06).
			select {
			case e.eventCh <- env:
			default:
				e.log.Warn("event_channel_full",
					slog.String("watcher", entry.config.Name),
					slog.String("sender", evt.Sender),
				)
			}
		}
	}()

	err := entry.adapter.Listen(ctx, adapterCh)
	close(adapterCh)
	<-done

	if err != nil && ctx.Err() == nil {
		e.log.Error("adapter_listen_error",
			slog.String("watcher", entry.config.Name),
			slog.String("error", err.Error()),
		)
		entry.tracker.RecordError()
	}
}

// writerLoop is the single-writer goroutine that serializes all DB writes (D-13).
// It reads event envelopes from eventCh, performs dedup via SaveWatcherEvent,
// routes events, updates health trackers, and forwards persisted events to routedEventCh.
func (e *Engine) writerLoop() {
	defer e.wg.Done()

	for {
		select {
		case env, ok := <-e.eventCh:
			if !ok {
				return
			}

			// Route the event via the router (D-08).
			var routedTo string
			if e.cfg.Router != nil {
				result := e.cfg.Router.Match(env.event.Sender)
				if result != nil {
					routedTo = result.Conductor
				}
			}

			// Persist with dedup via INSERT OR IGNORE (D-10, D-23).
			inserted, err := e.cfg.DB.SaveWatcherEvent(
				env.watcherID,
				env.event.DedupKey(),
				env.event.Sender,
				env.event.Subject,
				routedTo,
				"", // sessionID: populated later when session is launched
				e.cfg.MaxEventsPerWatcher,
			)

			if err != nil {
				e.log.Error("save_event_failed",
					slog.String("watcher_id", env.watcherID),
					slog.String("sender", env.event.Sender),
					slog.String("error", err.Error()),
				)
				env.tracker.RecordError()
				continue
			}

			if inserted {
				// New event: update health tracker and forward to TUI (D-14).
				env.tracker.RecordEvent()

				// Non-blocking send to routedEventCh for TUI consumption.
				select {
				case e.routedEventCh <- env.event:
				default:
					e.log.Debug("routed_event_channel_full",
						slog.String("sender", env.event.Sender),
					)
				}
			}

		case <-e.ctx.Done():
			return
		}
	}
}

// healthLoop periodically checks adapter health and emits HealthState snapshots.
func (e *Engine) healthLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.cfg.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for i := range e.adapters {
				entry := &e.adapters[i]

				if err := entry.adapter.HealthCheck(); err != nil {
					entry.tracker.SetAdapterHealth(false)
					entry.tracker.RecordError()
				} else {
					entry.tracker.SetAdapterHealth(true)
				}

				state := entry.tracker.Check()

				// Non-blocking send to healthCh.
				select {
				case e.healthCh <- state:
				default:
					e.log.Debug("health_channel_full",
						slog.String("watcher", entry.config.Name),
					)
				}
			}

		case <-e.ctx.Done():
			return
		}
	}
}

// Stop cancels all adapter contexts, calls Teardown on each adapter,
// and waits for all goroutines to exit. Safe to call multiple times.
func (e *Engine) Stop() {
	// Cancel root context, which propagates to all derived adapter contexts.
	e.cancel()

	// Best-effort teardown of all adapters.
	for i := range e.adapters {
		entry := &e.adapters[i]
		if err := entry.adapter.Teardown(); err != nil {
			e.log.Warn("adapter_teardown_error",
				slog.String("watcher", entry.config.Name),
				slog.String("error", err.Error()),
			)
		}
	}

	// Wait for all goroutines (adapters + writer + health) to exit.
	e.wg.Wait()
}

// EventCh returns a read-only channel of routed events for TUI consumption (D-20).
func (e *Engine) EventCh() <-chan Event {
	return e.routedEventCh
}

// HealthCh returns a read-only channel of health state updates for TUI consumption (D-20).
func (e *Engine) HealthCh() <-chan HealthState {
	return e.healthCh
}

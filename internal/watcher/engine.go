package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/asheshgoplani/agent-deck/internal/logging"
	"github.com/asheshgoplani/agent-deck/internal/statedb"
)

// eventEnvelope wraps an Event with metadata needed by the writerLoop
// so we can route and persist without modifying the public Event type.
type eventEnvelope struct {
	event     Event
	watcherID string
	tracker   *HealthTracker
}

// adapterEntry holds a registered adapter and its associated state.
type adapterEntry struct {
	adapter   WatcherAdapter
	config    AdapterConfig
	watcherID string
	tracker   *HealthTracker
	cancel    context.CancelFunc
}

// EngineConfig configures an Engine instance.
type EngineConfig struct {
	// DB is the StateDB used to persist events via SaveWatcherEvent.
	DB *statedb.StateDB

	// Router routes incoming events to conductors based on sender.
	Router *Router

	// MaxEventsPerWatcher is the maximum number of events to keep per watcher in the DB.
	MaxEventsPerWatcher int

	// HealthCheckInterval is how often to run adapter health checks.
	// Set to 0 to disable the health check loop (useful in tests).
	HealthCheckInterval time.Duration

	// Logger overrides the default component logger. Optional.
	Logger *slog.Logger
}

// Engine orchestrates the full event pipeline:
//   - Adapter goroutines produce Events via WatcherAdapter.Listen
//   - A single-writer goroutine serializes all DB writes through a buffered channel
//   - Dedup is handled by INSERT OR IGNORE in SaveWatcherEvent
//   - The router determines event routing (routed_to field in DB)
//   - Health trackers are updated after each write
type Engine struct {
	cfg      EngineConfig
	adapters []*adapterEntry
	// eventCh receives envelopes from adapter goroutines
	eventCh chan eventEnvelope
	// routedEventCh delivers successfully saved events to TUI consumers (D-20)
	routedEventCh chan Event
	// healthCh delivers health state snapshots to TUI consumers (D-20)
	healthCh chan HealthState
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	log      *slog.Logger
	mu       sync.Mutex // protects adapters slice during RegisterAdapter vs Start
}

// NewEngine creates a new Engine with the given configuration.
// Call RegisterAdapter to add adapters before calling Start.
func NewEngine(cfg EngineConfig) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	logger := cfg.Logger
	if logger == nil {
		logger = logging.ForComponent(logging.CompWatcher)
	}

	return &Engine{
		cfg:           cfg,
		eventCh:       make(chan eventEnvelope, 64),
		routedEventCh: make(chan Event, 64),
		healthCh:      make(chan HealthState, 16),
		ctx:           ctx,
		cancel:        cancel,
		log:           logger,
	}
}

// RegisterAdapter adds an adapter to the engine. Must be called before Start.
// watcherID is the statedb watcher ID used for event persistence.
// maxSilenceMinutes controls when the health tracker reports silence warnings.
func (e *Engine) RegisterAdapter(watcherID string, adapter WatcherAdapter, config AdapterConfig, maxSilenceMinutes int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	tracker := NewHealthTracker(config.Name, maxSilenceMinutes)
	e.adapters = append(e.adapters, &adapterEntry{
		adapter:   adapter,
		config:    config,
		watcherID: watcherID,
		tracker:   tracker,
	})
}

// Start initializes all registered adapters and begins the event processing pipeline.
// Adapter goroutines, the single-writer goroutine, and (optionally) the health check
// goroutine are all launched here. Returns an error only if no adapters were registered.
func (e *Engine) Start() error {
	e.mu.Lock()
	entries := make([]*adapterEntry, len(e.adapters))
	copy(entries, e.adapters)
	e.mu.Unlock()

	for _, entry := range entries {
		if err := entry.adapter.Setup(e.ctx, entry.config); err != nil {
			e.log.Error("adapter_setup_failed",
				slog.String("watcher", entry.config.Name),
				slog.String("error", err.Error()),
			)
			// Skip this adapter; others continue
			continue
		}

		adapterCtx, adapterCancel := context.WithCancel(e.ctx)
		entry.cancel = adapterCancel

		e.wg.Add(1)
		go e.runAdapter(adapterCtx, entry)
	}

	e.wg.Add(1)
	go e.writerLoop()

	if e.cfg.HealthCheckInterval > 0 {
		e.wg.Add(1)
		go e.healthLoop()
	}

	return nil
}

// runAdapter calls adapter.Listen in a goroutine, wrapping each received Event
// in an eventEnvelope before forwarding it to the single-writer goroutine.
func (e *Engine) runAdapter(ctx context.Context, entry *adapterEntry) {
	defer e.wg.Done()

	// adapterEventCh receives raw events from the adapter
	adapterEventCh := make(chan Event, 64)

	// Drain goroutine: wrap events in envelopes and forward to the engine channel
	var drainWg sync.WaitGroup
	drainWg.Add(1)
	go func() {
		defer drainWg.Done()
		for evt := range adapterEventCh {
			envelope := eventEnvelope{
				event:     evt,
				watcherID: entry.watcherID,
				tracker:   entry.tracker,
			}
			select {
			case e.eventCh <- envelope:
			case <-ctx.Done():
				return
			}
		}
	}()

	if err := entry.adapter.Listen(ctx, adapterEventCh); err != nil {
		if ctx.Err() == nil {
			// Error before context was cancelled — record it
			e.log.Error("adapter_listen_error",
				slog.String("watcher", entry.config.Name),
				slog.String("error", err.Error()),
			)
			entry.tracker.RecordError()
		}
	}

	// Close adapterEventCh to signal drain goroutine to exit
	close(adapterEventCh)
	drainWg.Wait()
}

// writerLoop is the single-writer goroutine that serializes all DB writes.
// It reads eventEnvelopes from eventCh, calls SaveWatcherEvent for dedup,
// updates health trackers, and forwards new events to routedEventCh.
//
// Per D-13: single writer prevents SQLite contention.
// Per D-10: INSERT OR IGNORE is the dedup mechanism.
func (e *Engine) writerLoop() {
	defer e.wg.Done()

	for {
		select {
		case envelope := <-e.eventCh:
			e.processEnvelope(envelope)
		case <-e.ctx.Done():
			// Drain remaining envelopes before exiting
			for {
				select {
				case envelope := <-e.eventCh:
					e.processEnvelope(envelope)
				default:
					return
				}
			}
		}
	}
}

// processEnvelope routes and persists a single event envelope.
func (e *Engine) processEnvelope(envelope eventEnvelope) {
	evt := envelope.event
	watcherID := envelope.watcherID
	tracker := envelope.tracker

	// Route: find conductor for this sender
	routedTo := ""
	if result := e.cfg.Router.Match(evt.Sender); result != nil {
		routedTo = result.Conductor
	}

	inserted, err := e.cfg.DB.SaveWatcherEvent(
		watcherID,
		evt.DedupKey(),
		evt.Sender,
		evt.Subject,
		routedTo,
		"",
		e.cfg.MaxEventsPerWatcher,
	)
	if err != nil {
		e.log.Error("save_watcher_event_failed",
			slog.String("watcher_id", watcherID),
			slog.String("sender", evt.Sender),
			slog.String("error", err.Error()),
		)
		tracker.RecordError()
		return
	}

	if inserted {
		tracker.RecordEvent()
		// Non-blocking send to TUI channel (T-13-06 mitigation)
		select {
		case e.routedEventCh <- evt:
		default:
			e.log.Warn("routed_event_ch_full",
				slog.String("watcher_id", watcherID),
				slog.String("sender", evt.Sender),
			)
		}
	}
	// If not inserted: duplicate, silently discarded (dedup succeeded)
}

// healthLoop periodically calls HealthCheck on each adapter and emits HealthState
// snapshots to healthCh for TUI consumption.
func (e *Engine) healthLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.cfg.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.mu.Lock()
			entries := make([]*adapterEntry, len(e.adapters))
			copy(entries, e.adapters)
			e.mu.Unlock()

			for _, entry := range entries {
				if err := entry.adapter.HealthCheck(); err != nil {
					entry.tracker.SetAdapterHealth(false)
					entry.tracker.RecordError()
				} else {
					entry.tracker.SetAdapterHealth(true)
				}

				state := entry.tracker.Check()
				select {
				case e.healthCh <- state:
				default:
					// Health channel full; drop oldest state (non-critical)
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
	// Cancel root context — propagates to all adapter contexts (D-18)
	e.cancel()

	// Best-effort teardown for each adapter
	e.mu.Lock()
	entries := make([]*adapterEntry, len(e.adapters))
	copy(entries, e.adapters)
	e.mu.Unlock()

	for _, entry := range entries {
		if err := entry.adapter.Teardown(); err != nil {
			e.log.Error("adapter_teardown_failed",
				slog.String("watcher", entry.config.Name),
				slog.String("error", err.Error()),
			)
		}
	}

	// Wait for all goroutines to exit (goleak test enforces this)
	e.wg.Wait()
}

// EventCh returns a read-only channel that receives events that were
// successfully persisted (de-duplicated) to the database. Intended for
// TUI consumption (D-20).
func (e *Engine) EventCh() <-chan Event {
	return e.routedEventCh
}

// HealthCh returns a read-only channel that receives HealthState snapshots
// emitted by the health check loop. Intended for TUI consumption (D-20).
func (e *Engine) HealthCh() <-chan HealthState {
	return e.healthCh
}

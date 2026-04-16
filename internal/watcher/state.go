package watcher

import (
	"fmt"
	"time"
)

type HealthSample struct {
	TS time.Time `json:"ts"`
	OK bool      `json:"ok"`
}

type WatcherState struct {
	LastEventTS    time.Time      `json:"last_event_ts"`
	ErrorCount     int            `json:"error_count"`
	AdapterHealthy bool           `json:"adapter_healthy"`
	HealthWindow   []HealthSample `json:"health_window"`
	DedupCursor    string         `json:"dedup_cursor"`
}

func LoadState(name string) (*WatcherState, error) { return nil, fmt.Errorf("not implemented (RED)") }
func SaveState(name string, s *WatcherState) error { return fmt.Errorf("not implemented (RED)") }

package ui

import "testing"

func TestNewHomeDoesNotStartBackgroundWorkersInTests(t *testing.T) {
	if homeBackgroundWorkersEnabled {
		t.Fatal("TestMain must disable Home background workers")
	}

	home := NewHome()
	t.Cleanup(func() {
		home.cancel()
		if home.storage != nil {
			_ = home.storage.Close()
		}
	})

	if home.statusWorkerDone != nil {
		t.Fatal("status worker channel is active in test mode")
	}
	if home.storageWatcher != nil {
		t.Fatal("storage watcher started in test mode")
	}
}

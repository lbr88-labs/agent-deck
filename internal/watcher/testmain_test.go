package watcher

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("AGENTDECK_PROFILE", "_test")
	os.Exit(m.Run())
}

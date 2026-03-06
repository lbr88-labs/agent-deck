package main

import (
	"fmt"

	"github.com/asheshgoplani/agent-deck/internal/session"
)

func applyGeminiCLIYoloOverride(inst *session.Instance, enabled bool) error {
	if !enabled || inst == nil {
		return nil
	}
	if inst.Tool != "gemini" {
		return fmt.Errorf("--yolo only works with Gemini sessions (-c gemini)")
	}
	inst.SetGeminiYoloMode(true)
	return nil
}

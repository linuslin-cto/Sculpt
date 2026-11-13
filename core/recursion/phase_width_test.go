package recursion_test

import (
	"testing"

	"sculpt/internal/config"
)

func TestPhaseWidthDefault(t *testing.T) {
	if config.PhaseWidth < 1 {
		t.Fatalf("PhaseWidth=%d", config.PhaseWidth)
	}
	// Documented Python default is 2 unless HARNESS2_PHASE_WIDTH overrides.
	if config.PhaseWidth != 2 && testing.Short() {
		t.Skip("env override in effect")
	}
}

package runtime

import (
	"testing"

	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/utils"
)

// ---- scoreToConfidence -------------------------------------------------------

func TestScoreToConfidence(t *testing.T) {
	cases := []struct {
		score float64
		want  int
	}{
		{0.0, 0},
		{-1.0, 0},
		{0.001, 1},  // > 0 but < 0.01
		{0.009, 1},
		{0.01, 3},   // >= 0.01
		{0.049, 3},
		{0.05, 5},   // >= 0.05
		{0.099, 5},
		{0.1, 7},    // >= 0.1
		{0.499, 7},
		{0.5, 9},    // >= 0.5
		{0.999, 9},
		{1.0, 10},   // >= 1.0
		{1.5, 10},   // over 1.0 still 10
	}
	for _, tc := range cases {
		got := scoreToConfidence(tc.score)
		if got != tc.want {
			t.Errorf("scoreToConfidence(%v) = %d want %d", tc.score, got, tc.want)
		}
	}
}

// ---- shouldPause ------------------------------------------------------------

func newTestRuntime(cfg config.Config) *Runtime {
	var buf noopWriter
	logger := utils.NewLogger(&buf)
	return New(cfg, &MockPage{}, logger)
}

// noopWriter satisfies io.Writer for constructing a Logger without real output.
type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestShouldPause_DebugContinue(t *testing.T) {
	rt := newTestRuntime(config.Default())
	rt.debugContinue = true

	// Even with empty breakLines (pause-every-step mode) debugContinue wins.
	cmd := dsl.Command{LineNum: 5}
	if rt.shouldPause(cmd) {
		t.Error("shouldPause should return false when debugContinue=true")
	}
}

func TestShouldPause_EmptyBreakLines_PausesEveryStep(t *testing.T) {
	cfg := config.Default()
	// No BreakLines → empty map → pause on every step.
	rt := newTestRuntime(cfg)

	for _, lineNum := range []int{1, 10, 99, 0} {
		cmd := dsl.Command{LineNum: lineNum}
		if !rt.shouldPause(cmd) {
			t.Errorf("shouldPause should return true for line %d when breakLines is empty", lineNum)
		}
	}
}

func TestShouldPause_SpecificBreakLines(t *testing.T) {
	cfg := config.Default()
	cfg.BreakLines = []int{10, 20}
	rt := newTestRuntime(cfg)

	cases := []struct {
		lineNum int
		want    bool
	}{
		{10, true},
		{20, true},
		{5, false},
		{15, false},
		{0, false},
	}
	for _, tc := range cases {
		cmd := dsl.Command{LineNum: tc.lineNum}
		got := rt.shouldPause(cmd)
		if got != tc.want {
			t.Errorf("shouldPause(line=%d) = %v want %v", tc.lineNum, got, tc.want)
		}
	}
}

func TestShouldPause_DebugContinue_OverridesBreakLines(t *testing.T) {
	cfg := config.Default()
	cfg.BreakLines = []int{10, 20}
	rt := newTestRuntime(cfg)
	rt.debugContinue = true

	// Registered breakpoint lines must not pause when debugContinue is set.
	for _, lineNum := range []int{10, 20} {
		cmd := dsl.Command{LineNum: lineNum}
		if rt.shouldPause(cmd) {
			t.Errorf("shouldPause(line=%d) should be false when debugContinue=true", lineNum)
		}
	}
}

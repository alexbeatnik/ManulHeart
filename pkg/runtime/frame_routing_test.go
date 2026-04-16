package runtime

import (
	"testing"

	"github.com/manulengineer/manulheart/pkg/dom"
)

func TestRoutedFrameIndex_UsesExactFrameWhenPresent(t *testing.T) {
	if got := routedFrameIndex(dom.ElementSnapshot{FrameIndex: 2}, 3); got != 2 {
		t.Fatalf("routedFrameIndex = %d, want 2", got)
	}
}

func TestRoutedFrameIndex_FallsBackToMainFrameWhenStale(t *testing.T) {
	if got := routedFrameIndex(dom.ElementSnapshot{FrameIndex: 999}, 3); got != 0 {
		t.Fatalf("routedFrameIndex = %d, want 0", got)
	}
}

func TestRoutedFrameIndex_FallsBackToMainFrameWhenMissing(t *testing.T) {
	if got := routedFrameIndex(dom.ElementSnapshot{}, 3); got != 0 {
		t.Fatalf("routedFrameIndex = %d, want 0", got)
	}
}

func TestRoutedFrameIndex_FallsBackToMainFrameWhenNegative(t *testing.T) {
	if got := routedFrameIndex(dom.ElementSnapshot{FrameIndex: -1}, 3); got != 0 {
		t.Fatalf("routedFrameIndex = %d, want 0", got)
	}
}

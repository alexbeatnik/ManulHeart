package runtime

import "github.com/manulengineer/manulheart/pkg/dom"

// routedFrameIndex returns the concrete frame index to use for an element.
// Missing, negative, or stale indexes fall back to the main frame (0).
func routedFrameIndex(el dom.ElementSnapshot, frameCount int) int {
	if frameCount <= 0 {
		return 0
	}
	if el.FrameIndex < 0 || el.FrameIndex >= frameCount {
		return 0
	}
	return el.FrameIndex
}

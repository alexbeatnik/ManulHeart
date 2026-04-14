// Package config provides runtime configuration for ManulHeart.
package config

import "time"

// Config holds all runtime configuration for a ManulHeart execution session.
type Config struct {
	// CDPEndpoint is the WebSocket URL of the Chrome DevTools Protocol endpoint.
	// Example: "http://127.0.0.1:9222"
	CDPEndpoint string

	// DefaultTimeout is the maximum time to wait for any single operation.
	DefaultTimeout time.Duration

	// NavigationTimeout is the maximum time to wait for page navigation to settle.
	NavigationTimeout time.Duration

	// VerifyTimeout is the maximum time to wait when polling for text/element presence.
	VerifyTimeout time.Duration

	// ScoringThreshold is the minimum normalized score [0.0–1.0] required for
	// a candidate to be accepted as the target. Candidates below this are rejected.
	ScoringThreshold float64

	// MaxCandidates is the maximum number of candidates returned from a page probe.
	MaxCandidates int

	// Verbose enables detailed structured logging of each resolution step.
	Verbose bool

	// ExplainAll forces full explainability output even for successful resolutions.
	ExplainAll bool
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		CDPEndpoint:      "http://127.0.0.1:9222",
		DefaultTimeout:   30 * time.Second,
		NavigationTimeout: 15 * time.Second,
		VerifyTimeout:    10 * time.Second,
		ScoringThreshold: 0.15,
		MaxCandidates:    200,
		Verbose:          false,
		ExplainAll:       false,
	}
}

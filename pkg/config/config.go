// Package config holds the engine-wide runtime configuration for ManulHeart.
// Each hunt execution gets a Config passed through the runtime stack.
package config

import "time"

// Config is the engine-wide runtime configuration.
// It is populated from CLI flags before any hunt is executed.
type Config struct {
	// CDPEndpoint is the Chrome DevTools Protocol HTTP endpoint.
	// Example: "http://127.0.0.1:9222"
	CDPEndpoint string

	// Headless runs the browser in headless mode.
	Headless bool

	// Verbose enables verbose logging.
	Verbose bool

	// DebugMode pauses execution between each DSL command for interactive stepping.
	DebugMode bool

	// ExplainMode prints a full scoring breakdown for every targeted element.
	ExplainMode bool

	// DefaultTimeout is the per-command context deadline.
	DefaultTimeout time.Duration

	// Screenshot controls when screenshots are taken: "none", "on-fail", or "always".
	Screenshot string

	// HTMLReport enables generation of an HTML execution report.
	HTMLReport bool

	// Retries is the number of times to retry a failed targeting command.
	Retries int

	// DisableCache disables the DOM snapshot cache (forces re-probe on every command).
	DisableCache bool

	// Tags filters which @tag-annotated commands to execute.
	// Empty slice means run all commands.
	Tags []string
}

// Default returns a Config with sensible production defaults.
func Default() Config {
	return Config{
		DefaultTimeout: 30 * time.Second,
		Retries:        0,
	}
}

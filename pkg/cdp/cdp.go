// Package cdp provides a Chrome DevTools Protocol client for ManulHeart.
//
// This package implements the low-level WebSocket messenger and the
// command-level CDP calls (Navigate, Evaluate, Click, etc.) used by
// pkg/browser/cdp_backend.go.
//
// STATUS: Implementation in progress. The API shape is frozen; the
// underlying WebSocket transport is being implemented.
package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ── Types ──────────────────────────────────────────────────────────────────────

// Conn is a live WebSocket connection to a Chrome DevTools Protocol target.
// It is safe for concurrent use from multiple goroutines.
type Conn struct {
	wsURL string
}

// Target represents a single debuggable browser target (page, worker, etc.).
type Target struct {
	// ID is the unique target ID assigned by Chrome.
	ID string `json:"id"`
	// Type is the target type ("page", "background_page", "worker", etc.)
	Type string `json:"type"`
	// Title is the page title.
	Title string `json:"title"`
	// URL is the current URL of the target.
	URL string `json:"url"`
	// WSURL is the WebSocket debugger URL for this target.
	WSURL string `json:"webSocketDebuggerUrl"`
}

// KeyEventParams holds the browser input event parameters for keyboard dispatch.
type KeyEventParams struct {
	Type                  string `json:"type"`
	Key                   string `json:"key"`
	Code                  string `json:"code,omitempty"`
	WindowsVirtualKeyCode int    `json:"windowsVirtualKeyCode,omitempty"`
	Modifiers             int    `json:"modifiers,omitempty"`
}

// ── Connection ─────────────────────────────────────────────────────────────────

// ListTargets queries the Chrome HTTP endpoint and returns all available targets.
func ListTargets(_ context.Context, endpoint string) ([]Target, error) {
	return nil, fmt.Errorf("cdp.ListTargets: not yet implemented (endpoint=%s)", endpoint)
}

// FindPageTarget returns the first page-type target from a list.
func FindPageTarget(targets []Target) (Target, error) {
	for _, t := range targets {
		if t.Type == "page" {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("cdp.FindPageTarget: no page target found among %d targets", len(targets))
}

// DialTarget establishes a WebSocket connection to the given target URL.
func DialTarget(_ context.Context, wsURL string) (*Conn, error) {
	return nil, fmt.Errorf("cdp.DialTarget: not yet implemented (url=%s)", wsURL)
}

// Close terminates the WebSocket connection.
func (c *Conn) Close() error {
	if c == nil {
		return nil
	}
	return nil
}

// ── CDP Commands ───────────────────────────────────────────────────────────────

// Navigate instructs the browser to navigate to the given URL.
func Navigate(_ context.Context, _ *Conn, url string) error {
	return fmt.Errorf("cdp.Navigate: not yet implemented (url=%s)", url)
}

// Evaluate runs JavaScript in the page context and returns the result.
func Evaluate(_ context.Context, _ *Conn, _ string) (interface{}, error) {
	return nil, fmt.Errorf("cdp.Evaluate: not yet implemented")
}

// CallFunctionOn calls a JS function string with a JSON-serialized argument.
func CallFunctionOn(_ context.Context, _ *Conn, _ string, _ interface{}) (interface{}, error) {
	return nil, fmt.Errorf("cdp.CallFunctionOn: not yet implemented")
}

// Click dispatches a mouse click at the given page coordinates.
func Click(_ context.Context, _ *Conn, x, y float64) error {
	return fmt.Errorf("cdp.Click: not yet implemented (x=%.0f y=%.0f)", x, y)
}

// DoubleClick dispatches a double-click at the given page coordinates.
func DoubleClick(_ context.Context, _ *Conn, x, y float64) error {
	return fmt.Errorf("cdp.DoubleClick: not yet implemented")
}

// RightClick dispatches a right-click (contextmenu) at the given page coordinates.
func RightClick(_ context.Context, _ *Conn, x, y float64) error {
	return fmt.Errorf("cdp.RightClick: not yet implemented")
}

// Hover dispatches a mousemove event at the given page coordinates.
func Hover(_ context.Context, _ *Conn, x, y float64) error {
	return fmt.Errorf("cdp.Hover: not yet implemented")
}

// DragAndDrop dispatches a drag-and-drop sequence.
func DragAndDrop(_ context.Context, _ *Conn, fromX, fromY, toX, toY float64) error {
	return fmt.Errorf("cdp.DragAndDrop: not yet implemented")
}

// FocusByXPath focuses an element resolved by the given XPath.
func FocusByXPath(_ context.Context, _ *Conn, xpath string) error {
	return fmt.Errorf("cdp.FocusByXPath: not yet implemented (xpath=%s)", xpath)
}

// SetInputValue sets the value of an input element resolved by XPath.
func SetInputValue(_ context.Context, _ *Conn, xpath, value string) error {
	return fmt.Errorf("cdp.SetInputValue: not yet implemented")
}

// ScrollIntoView scrolls a given XPath element into the viewport.
func ScrollIntoView(_ context.Context, _ *Conn, xpath string) error {
	return fmt.Errorf("cdp.ScrollIntoView: not yet implemented")
}

// ScrollPage scrolls the page or a container element in the given direction.
func ScrollPage(_ context.Context, _ *Conn, direction, container string) error {
	return fmt.Errorf("cdp.ScrollPage: not yet implemented")
}

// SetFileInput sets the file path(s) on a file input resolved by XPath.
func SetFileInput(_ context.Context, _ *Conn, xpath string, filePaths []string) error {
	return fmt.Errorf("cdp.SetFileInput: not yet implemented")
}

// Screenshot captures a PNG screenshot of the current viewport.
func Screenshot(_ context.Context, _ *Conn) ([]byte, error) {
	return nil, fmt.Errorf("cdp.Screenshot: not yet implemented")
}

// WaitForResponse waits for a network response whose URL matches the given pattern.
func WaitForResponse(_ context.Context, _ *Conn, urlPattern string, _ time.Duration) error {
	return fmt.Errorf("cdp.WaitForResponse: not yet implemented (pattern=%s)", urlPattern)
}

// HighlightElement draws a debug overlay on the element resolved by XPath.
func HighlightElement(_ context.Context, _ *Conn, xpath string, durationMS int) error {
	return fmt.Errorf("cdp.HighlightElement: not yet implemented")
}

// GetElementCenter returns the centre coordinates of an element resolved by XPath.
func GetElementCenter(_ context.Context, _ *Conn, xpath string) (x, y float64, err error) {
	return 0, 0, fmt.Errorf("cdp.GetElementCenter: not yet implemented (xpath=%s)", xpath)
}

// DispatchKeyEvent sends a keyboard event to the currently focused element.
func DispatchKeyEvent(_ context.Context, _ *Conn, eventType string, params KeyEventParams) error {
	return fmt.Errorf("cdp.DispatchKeyEvent: not yet implemented (type=%s key=%s)", eventType, params.Key)
}

// GetCurrentURL returns the current URL of the page.
func GetCurrentURL(_ context.Context, _ *Conn) (string, error) {
	return "", fmt.Errorf("cdp.GetCurrentURL: not yet implemented")
}

// WaitForLoad is available but ManulHeart prefers JS-polling WaitForLoad
// in cdp_backend.go to avoid race conditions on cached pages.
func WaitForLoad(_ context.Context, _ *Conn) error {
	return fmt.Errorf("cdp.WaitForLoad: not yet implemented")
}

// ── JSON helpers ───────────────────────────────────────────────────────────────

// MustMarshal marshals v to JSON, panicking on error (used in tests only).
func MustMarshal(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

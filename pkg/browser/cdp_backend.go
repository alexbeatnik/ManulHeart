// Package browser — CDP backend implementation.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/manulengineer/manulheart/pkg/cdp"
)

// CDPBrowser is the Chrome DevTools Protocol implementation of Browser.
type CDPBrowser struct {
	endpoint string
}

// NewCDPBrowser creates a CDPBrowser connected to the given HTTP endpoint.
// Example endpoint: "http://127.0.0.1:9222"
func NewCDPBrowser(endpoint string) *CDPBrowser {
	return &CDPBrowser{endpoint: endpoint}
}

// FirstPage attaches to the first available page target in the running Chrome instance.
func (b *CDPBrowser) FirstPage(ctx context.Context) (Page, error) {
	targets, err := cdp.ListTargets(ctx, b.endpoint)
	if err != nil {
		return nil, fmt.Errorf("browser: list targets: %w", err)
	}
	target, err := cdp.FindPageTarget(targets)
	if err != nil {
		return nil, fmt.Errorf("browser: find page target: %w", err)
	}
	conn, err := cdp.DialTarget(ctx, target.WSURL)
	if err != nil {
		return nil, fmt.Errorf("browser: dial target %q: %w", target.WSURL, err)
	}
	return &CDPPage{conn: conn}, nil
}

// NewPage is not yet implemented for CDP (requires Target.createTarget).
func (b *CDPBrowser) NewPage(ctx context.Context) (Page, error) {
	return nil, fmt.Errorf("browser: NewPage not yet implemented for CDP backend")
}

// Close is a no-op for CDPBrowser — we don't own the browser process.
func (b *CDPBrowser) Close() error { return nil }

// ── CDPPage ───────────────────────────────────────────────────────────────────

// CDPPage is the CDP implementation of Page.
type CDPPage struct {
	conn *cdp.Conn
}

func (p *CDPPage) Navigate(ctx context.Context, url string) error {
	if err := cdp.Navigate(ctx, p.conn, url); err != nil {
		return err
	}
	// Use JS-polling WaitForLoad instead of the event-based cdp.WaitForLoad.
	// The event-based approach misses Page.loadEventFired when the page loads
	// from cache (very fast) before the handler is registered.
	return p.WaitForLoad(ctx)
}

func (p *CDPPage) EvalJS(ctx context.Context, expr string) ([]byte, error) {
	raw, err := cdp.Evaluate(ctx, p.conn, expr)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		b, e := json.Marshal(v)
		return b, e
	}
}

func (p *CDPPage) CallProbe(ctx context.Context, fn string, arg any) ([]byte, error) {
	raw, err := cdp.CallFunctionOn(ctx, p.conn, fn, arg)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		b, e := json.Marshal(v)
		return b, e
	}
}

func (p *CDPPage) Click(ctx context.Context, x, y float64) error {
	return cdp.Click(ctx, p.conn, x, y)
}

func (p *CDPPage) FocusByXPath(ctx context.Context, xpath string) error {
	return cdp.FocusByXPath(ctx, p.conn, xpath)
}

func (p *CDPPage) SetInputValue(ctx context.Context, xpath, value string) error {
	return cdp.SetInputValue(ctx, p.conn, xpath, value)
}

func (p *CDPPage) ScrollIntoView(ctx context.Context, xpath string) error {
	return cdp.ScrollIntoView(ctx, p.conn, xpath)
}

func (p *CDPPage) ScrollPage(ctx context.Context, direction, container string) error {
	return cdp.ScrollPage(ctx, p.conn, direction, container)
}

func (p *CDPPage) DoubleClick(ctx context.Context, x, y float64) error {
	return cdp.DoubleClick(ctx, p.conn, x, y)
}

func (p *CDPPage) RightClick(ctx context.Context, x, y float64) error {
	return cdp.RightClick(ctx, p.conn, x, y)
}

func (p *CDPPage) Hover(ctx context.Context, x, y float64) error {
	return cdp.Hover(ctx, p.conn, x, y)
}

func (p *CDPPage) DragAndDrop(ctx context.Context, fromX, fromY, toX, toY float64) error {
	return cdp.DragAndDrop(ctx, p.conn, fromX, fromY, toX, toY)
}

func (p *CDPPage) SetFileInput(ctx context.Context, xpath string, filePaths []string) error {
	return cdp.SetFileInput(ctx, p.conn, xpath, filePaths)
}

func (p *CDPPage) Screenshot(ctx context.Context) ([]byte, error) {
	return cdp.Screenshot(ctx, p.conn)
}

func (p *CDPPage) WaitForResponse(ctx context.Context, urlPattern string, timeout time.Duration) error {
	return cdp.WaitForResponse(ctx, p.conn, urlPattern, timeout)
}

func (p *CDPPage) HighlightElement(ctx context.Context, xpath string, durationMS int) error {
	return cdp.HighlightElement(ctx, p.conn, xpath, durationMS)
}

func (p *CDPPage) GetElementCenter(ctx context.Context, xpath string) (float64, float64, error) {
	return cdp.GetElementCenter(ctx, p.conn, xpath)
}

func (p *CDPPage) DispatchKey(ctx context.Context, key string, modifiers int) error {
	params := cdp.KeyEventParams{
		Key:                   key,
		WindowsVirtualKeyCode: keyToVirtualCode(key),
		Modifiers:             modifiers,
	}
	if err := cdp.DispatchKeyEvent(ctx, p.conn, "keyDown", params); err != nil {
		return err
	}
	return cdp.DispatchKeyEvent(ctx, p.conn, "keyUp", params)
}

// keyToVirtualCode maps common key names to Windows virtual key codes.
func keyToVirtualCode(key string) int {
	codes := map[string]int{
		"Enter": 13, "Tab": 9, "Escape": 27, "Backspace": 8,
		"Delete": 46, "ArrowUp": 38, "ArrowDown": 40,
		"ArrowLeft": 37, "ArrowRight": 39, "Home": 36, "End": 35,
		"PageUp": 33, "PageDown": 34, "Space": 32, "F1": 112,
		"F2": 113, "F3": 114, "F4": 115, "F5": 116, "F6": 117,
		"F7": 118, "F8": 119, "F9": 120, "F10": 121, "F11": 122, "F12": 123,
		"a": 65, "b": 66, "c": 67, "v": 86, "x": 88, "z": 90,
	}
	if code, ok := codes[key]; ok {
		return code
	}
	// Default: try first character as uppercase ASCII
	if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
		return int(key[0]) - 32
	}
	return 0
}

func (p *CDPPage) CurrentURL(ctx context.Context) (string, error) {
	return cdp.GetCurrentURL(ctx, p.conn)
}

func (p *CDPPage) WaitForLoad(ctx context.Context) error {
	// Poll document.readyState via JS instead of relying on CDP event registration.
	// Event-based approach misses loadEventFired when the page loads before we
	// register the handler. JS polling is always accurate.
	const pollInterval = 150 * time.Millisecond
	for {
		raw, err := cdp.Evaluate(ctx, p.conn, "document.readyState")
		if err == nil && raw != nil {
			var state string
			var b []byte
			switch v := raw.(type) {
			case []byte:
				b = v
			case string:
				b = []byte(v)
			default:
				b, _ = json.Marshal(v)
			}
			if json.Unmarshal(b, &state) == nil && state == "complete" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (p *CDPPage) Wait(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *CDPPage) Close() error {
	return p.conn.Close()
}

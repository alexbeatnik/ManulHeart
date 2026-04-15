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
	return raw, nil
}

func (p *CDPPage) CallProbe(ctx context.Context, fn string, arg any) ([]byte, error) {
	raw, err := cdp.CallFunctionOn(ctx, p.conn, fn, arg)
	if err != nil {
		return nil, err
	}
	return raw, nil
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
		if err == nil {
			var state string
			if json.Unmarshal(raw, &state) == nil && state == "complete" {
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

func (p *CDPPage) Wait(_ context.Context, duration time.Duration) error {
	time.Sleep(duration)
	return nil
}

func (p *CDPPage) Close() error {
	return p.conn.Close()
}

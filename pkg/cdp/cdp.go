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

// ── CDP Commands ───────────────────────────────────────────────────────────────

// Navigate instructs the browser to navigate to the given URL.
func Navigate(ctx context.Context, c *Conn, url string) error {
	_, err := c.Call(ctx, "Page.navigate", map[string]interface{}{"url": url})
	return err
}

// Evaluate runs JavaScript in the page context and returns the result.
func Evaluate(ctx context.Context, c *Conn, expression string) (interface{}, error) {
	res, err := c.Call(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}
	
	// {"result": {"type": "...", "value": ...}}
	var wrap struct {
		Result struct {
			Value interface{} `json:"value"`
			Type  string      `json:"type"`
		} `json:"result"`
		ExceptionDetails interface{} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(res, &wrap); err != nil {
		return nil, fmt.Errorf("unmarshal evaluate result: %w", err)
	}
	if wrap.ExceptionDetails != nil {
		return nil, fmt.Errorf("js exception: %v", wrap.ExceptionDetails)
	}
	return wrap.Result.Value, nil
}

// CallFunctionOn calls a JS function string with a JSON-serialized argument.
func CallFunctionOn(ctx context.Context, c *Conn, objectId string, arg interface{}) (interface{}, error) {
	// For now, if we don't use real objectId, we just map it to an evaluation.
	return nil, fmt.Errorf("cdp.CallFunctionOn: not fully implemented")
}

// Click dispatches a mouse click at the given page coordinates.
func Click(ctx context.Context, c *Conn, x, y float64) error {
	// MousePressed
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mousePressed",
		"button":     "left",
		"x":          x,
		"y":          y,
		"clickCount": 1,
	})
	if err != nil {
		return err
	}
	// MouseReleased
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mouseReleased",
		"button":     "left",
		"x":          x,
		"y":          y,
		"clickCount": 1,
	})
	return err
}

// DoubleClick dispatches a double-click at the given page coordinates.
func DoubleClick(ctx context.Context, c *Conn, x, y float64) error {
	// Press 1
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mousePressed", "button": "left", "x": x, "y": y, "clickCount": 1,
	})
	if err != nil { return err }
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mouseReleased", "button": "left", "x": x, "y": y, "clickCount": 1,
	})
	if err != nil { return err }
	
	// Press 2
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mousePressed", "button": "left", "x": x, "y": y, "clickCount": 2,
	})
	if err != nil { return err }
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mouseReleased", "button": "left", "x": x, "y": y, "clickCount": 2,
	})
	return err
}

// RightClick dispatches a right-click (contextmenu) at the given page coordinates.
func RightClick(ctx context.Context, c *Conn, x, y float64) error {
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mousePressed",
		"button":     "right",
		"x":          x,
		"y":          y,
		"clickCount": 1,
	})
	if err != nil {
		return err
	}
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mouseReleased",
		"button":     "right",
		"x":          x,
		"y":          y,
		"clickCount": 1,
	})
	return err
}

// Hover dispatches a mousemove event at the given page coordinates.
func Hover(ctx context.Context, c *Conn, x, y float64) error {
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mouseMoved",
		"x":    x,
		"y":    y,
	})
	return err
}

// DragAndDrop dispatches a drag-and-drop sequence.
func DragAndDrop(ctx context.Context, c *Conn, fromX, fromY, toX, toY float64) error {
	// Mouse move to start
	if err := Hover(ctx, c, fromX, fromY); err != nil {
		return err
	}
	// Press
	_, err := c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mousePressed", "button": "left", "x": fromX, "y": fromY, "clickCount": 1,
	})
	if err != nil {
		return err
	}
	
	// Move slowly (simulate multiple steps?) Simple 1-step move:
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mouseMoved", "button": "left", "x": toX, "y": toY,
	})
	if err != nil {
		return err
	}
	
	// Release
	_, err = c.Call(ctx, "Input.dispatchMouseEvent", map[string]interface{}{
		"type": "mouseReleased", "button": "left", "x": toX, "y": toY, "clickCount": 1,
	})
	return err
}

// FocusByXPath focuses an element resolved by the given XPath.
func FocusByXPath(ctx context.Context, c *Conn, xpath string) error {
	js := fmt.Sprintf(`
		var el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) el.focus();
	`, xpath)
	_, err := Evaluate(ctx, c, js)
	return err
}

// SetInputValue sets the value of an input element resolved by XPath.
func SetInputValue(ctx context.Context, c *Conn, xpath, value string) error {
	js := fmt.Sprintf(`
		var el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) {
			el.value = %q;
			el.dispatchEvent(new Event('input', { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
		}
	`, xpath, value)
	_, err := Evaluate(ctx, c, js)
	return err
}

// ScrollIntoView scrolls a given XPath element into the viewport.
func ScrollIntoView(ctx context.Context, c *Conn, xpath string) error {
	js := fmt.Sprintf(`
		var el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) el.scrollIntoView({block: "center", inline: "center"});
	`, xpath)
	_, err := Evaluate(ctx, c, js)
	return err
}

// ScrollPage scrolls the page or a container element in the given direction.
// Currently only direction="up" or "down" are well supported.
func ScrollPage(ctx context.Context, c *Conn, direction, container string) error {
	// A basic implementation. In prod this would use Input.synthesizeScrollGesture or JS.
	var amount = 500
	if direction == "up" {
		amount = -500
	}
	js := fmt.Sprintf(`window.scrollBy(0, %d);`, amount)
	if container != "" {
		// A rudimentary xpath selection.
		js = fmt.Sprintf(`
			var el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
			if (el) el.scrollBy(0, %d);
		`, container, amount)
	}
	_, err := Evaluate(ctx, c, js)
	return err
}

// SetFileInput sets the file path(s) on a file input resolved by XPath.
func SetFileInput(ctx context.Context, c *Conn, xpath string, filePaths []string) error {
	// Requires DOM node ID resolution to call DOM.setFileInputFiles
	return fmt.Errorf("cdp.SetFileInput: not yet implemented (requires DOM objectId)")
}

// Screenshot captures a PNG screenshot of the current viewport.
func Screenshot(ctx context.Context, c *Conn) ([]byte, error) {
	res, err := c.Call(ctx, "Page.captureScreenshot", map[string]interface{}{
		"format": "png",
	})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Data []byte `json:"data"` // base64 encoded by chrome, auto-decoded by Go's []byte unmarshal!
	}
	if err := json.Unmarshal(res, &wrap); err != nil {
		return nil, fmt.Errorf("unmarshal screenshot: %w", err)
	}
	return wrap.Data, nil
}

// WaitForResponse waits for a network response whose URL matches the given pattern.
func WaitForResponse(ctx context.Context, c *Conn, urlPattern string, timeout time.Duration) error {
	// Enable network tracking first
	_, err := c.Call(ctx, "Network.enable", nil)
	if err != nil {
		return fmt.Errorf("Network.enable: %w", err)
	}
	
	sub := c.Subscribe()
	defer c.Unsubscribe(sub)
	defer c.Call(context.Background(), "Network.disable", nil)

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctxTimeout.Done():
			return fmt.Errorf("timeout waiting for response pattern %q", urlPattern)
		case event := <-sub:
			if event.Method == "Network.responseReceived" {
				var received struct {
					Response struct {
						URL string `json:"url"`
					} `json:"response"`
				}
				if err := json.Unmarshal(event.Params, &received); err == nil {
					// Extremely simple suffix/substring match
					if len(received.Response.URL) >= len(urlPattern) && 
						received.Response.URL[len(received.Response.URL)-len(urlPattern):] == urlPattern {
						return nil
					}
				}
			}
		}
	}
}

// HighlightElement draws a debug overlay on the element resolved by XPath.
func HighlightElement(ctx context.Context, c *Conn, xpath string, durationMS int) error {
	// Use JS to draw an outline
	js := fmt.Sprintf(`
		var el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) {
			var old = el.style.outline;
			el.style.outline = "2px solid red";
			setTimeout(() => { el.style.outline = old; }, %d);
		}
	`, xpath, durationMS)
	_, err := Evaluate(ctx, c, js)
	return err
}

// GetElementCenter returns the centre coordinates of an element resolved by XPath.
func GetElementCenter(ctx context.Context, c *Conn, xpath string) (x, y float64, err error) {
	js := fmt.Sprintf(`
		var el = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!el) {
			throw new Error("Element not found");
		}
		var rect = el.getBoundingClientRect();
		JSON.stringify({x: rect.x + rect.width/2, y: rect.y + rect.height/2});
	`, xpath)
	
	val, err := Evaluate(ctx, c, js)
	if err != nil {
		return 0, 0, err
	}
	
	var coords struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	
	// val should be a string containing JSON due to JSON.stringify
	str, ok := val.(string)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected evaluate result format: %T", val)
	}
	if err := json.Unmarshal([]byte(str), &coords); err != nil {
		return 0, 0, err
	}
	return coords.X, coords.Y, nil
}

// DispatchKeyEvent sends a keyboard event to the currently focused element.
func DispatchKeyEvent(ctx context.Context, c *Conn, eventType string, params KeyEventParams) error {
	_, err := c.Call(ctx, "Input.dispatchKeyEvent", params)
	return err
}

// GetCurrentURL returns the current URL of the page.
func GetCurrentURL(ctx context.Context, c *Conn) (string, error) {
	val, err := Evaluate(ctx, c, "window.location.href")
	if err != nil {
		return "", err
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("unexpected evaluation result for URL: %v", val)
}

// WaitForLoad is available but ManulHeart prefers JS-polling WaitForLoad
// in cdp_backend.go to avoid race conditions on cached pages.
func WaitForLoad(ctx context.Context, c *Conn) error {
	return nil // Handled in cdp_backend.go 
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

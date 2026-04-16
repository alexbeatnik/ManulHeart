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
	args := []map[string]interface{}{}
	if arg != nil {
		args = append(args, map[string]interface{}{"value": arg})
	}

// If objectId was meant to be a real remote object ID, we could pass it.
	// We'll evaluate it unconditionally in the default context:
	var expr string
	if arg == nil {
		expr = objectId
	} else {
		expr = fmt.Sprintf("(%s)(%s)", objectId, MustMarshalString(arg))
	}
	res, err := c.Call(ctx, "Runtime.evaluate", map[string]interface{}{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err == nil {
		// Just parse Evaluate wrapper
		var wrap struct {
			Result struct {
				Value interface{} `json:"value"`
			} `json:"result"`
			ExceptionDetails interface{} `json:"exceptionDetails"`
		}
		if json.Unmarshal(res, &wrap) == nil {
			if wrap.ExceptionDetails != nil {
				return nil, fmt.Errorf("js exception: %v", wrap.ExceptionDetails)
			}
			return wrap.Result.Value, nil
		}
	}
	
	return nil, err
}

func MustMarshalString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil || string(b) == "null" {
		return "undefined"
	}
	return string(b)
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

// Focus focuses the element resolved by ID or XPath.
func (c *Conn) Focus(ctx context.Context, id int, xpath string) error {
	js := fmt.Sprintf(`
		var el = (window.__manulReg && window.__manulReg[%d]) || document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) el.focus();
	`, id, xpath)
	_, err := Evaluate(ctx, c, js)
	return err
}

// SetInputValue sets the value of an input element resolved by ID or XPath.
// Uses the native HTMLInputElement/HTMLTextAreaElement value setter to
// bypass framework-level overrides (React, Vue, etc.) that intercept
// the value property on individual elements.
func (c *Conn) SetInputValue(ctx context.Context, id int, xpath, value string) error {
	js := fmt.Sprintf(`
		var el = (window.__manulReg && window.__manulReg[%[1]d]) || document.evaluate(%[2]q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) {
			var targetEl = el;
			if (el.tagName !== 'INPUT' && el.tagName !== 'TEXTAREA' && el.tagName !== 'SELECT' && el.getAttribute('contenteditable') !== 'true') {
				if (el.tagName === 'LABEL' && el.htmlFor) {
					targetEl = document.getElementById(el.htmlFor) || targetEl;
				} else {
					var child = el.querySelector('input, textarea, select');
					if (child) {
						targetEl = child;
					} else if (el.nextElementSibling) {
						var next = el.nextElementSibling.matches('input, textarea, select') ? el.nextElementSibling : el.nextElementSibling.querySelector('input, textarea, select');
						if (next) targetEl = next;
					} else if (el.parentElement && el.parentElement.nextElementSibling) {
						var nextParent = el.parentElement.nextElementSibling;
						var pChild = nextParent.matches('input, textarea, select') ? nextParent : nextParent.querySelector('input, textarea, select');
						if (pChild) targetEl = pChild;
					}
				}
			}
			el = targetEl;

			// Use the native value setter so React/Vue/Angular state updates fire.
			var proto = Object.getPrototypeOf(el);
			var nativeSetter = null;
			while (proto && proto !== Object.prototype) {
				var desc = Object.getOwnPropertyDescriptor(proto, 'value');
				if (desc && desc.set) {
					nativeSetter = desc.set;
					break;
				}
				proto = Object.getPrototypeOf(proto);
			}
			if (nativeSetter) {
				nativeSetter.call(el, %[3]q);
			} else {
				el.value = %[3]q;
			}
			el.dispatchEvent(new Event('input', { bubbles: true }));
			el.dispatchEvent(new Event('change', { bubbles: true }));
			console.log("SetInputValue:", el.tagName, el.id, "to", el.value);
		}
	`, id, xpath, value)
	_, err := Evaluate(ctx, c, js)
	return err
}

// ScrollIntoView scrolls the element resolved by ID or XPath into the viewport.
func (c *Conn) ScrollIntoView(ctx context.Context, id int, xpath string) error {
	js := fmt.Sprintf(`
		var el = (window.__manulReg && window.__manulReg[%d]) || document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) el.scrollIntoView({block: "center", inline: "center"});
	`, id, xpath)
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

// SetFileInput sets the file paths on a file input element resolved by ID or XPath.
func (c *Conn) SetFileInput(ctx context.Context, id int, xpath string, filePaths []string) error {
	// First resolve the backend node ID for DOM.setFileInputFiles
	js := fmt.Sprintf(`(function() {
		var el = (window.__manulReg && window.__manulReg[%d]) || document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		return el;
	})()`, id, xpath)
	
	val, err := Evaluate(ctx, c, js)
	if err != nil {
		return err
	}
	
	// val is a map[string]interface{} from Evaluate/CallFunctionOn
	obj, ok := val.(map[string]interface{})
	if !ok {
		return fmt.Errorf("SetFileInput: failed to resolve element to CDP node")
	}
	
	objectId, ok := obj["objectId"].(string)
	if !ok {
		return fmt.Errorf("SetFileInput: element has no objectId")
	}
	
	// Get the backend node ID
	rawRes, err := c.Call(ctx, "DOM.requestNode", map[string]interface{}{
		"objectId": objectId,
	})
	if err != nil {
		return err
	}
	
	var res struct {
		NodeId int `json:"nodeId"`
	}
	if err := json.Unmarshal(rawRes, &res); err != nil {
		return fmt.Errorf("SetFileInput: unmarshal requestNode: %w", err)
	}
	
	_, err = c.Call(ctx, "DOM.setFileInputFiles", map[string]interface{}{
		"nodeId": res.NodeId,
		"files":  filePaths,
	})
	return err
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

// HighlightElement injects a temporary border highlight for debugging.
func (c *Conn) HighlightElement(ctx context.Context, id int, xpath string, durationMS int) error {
	js := fmt.Sprintf(`
		var el = (window.__manulReg && window.__manulReg[%d]) || document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (el) {
			var old = el.style.outline;
			el.style.outline = '4px solid #ff4444';
			setTimeout(() => { el.style.outline = old; }, %d);
		}
	`, id, xpath, durationMS)
	_, err := Evaluate(ctx, c, js)
	return err
}

// GetElementCenter returns the centre coordinates of an element resolved by ID or XPath.
func (c *Conn) GetElementCenter(ctx context.Context, id int, xpath string) (x, y float64, err error) {
	js := fmt.Sprintf(`
		var el = (window.__manulReg && window.__manulReg[%d]) || document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!el) {
			throw new Error("Element not found");
		}
		el.scrollIntoView({behavior: 'instant', block: 'center', inline: 'center'});
		var rect = el.getBoundingClientRect();
		// If it's still outside, we might need a small delay, but instant scroll usually is synchronous.
		JSON.stringify({x: rect.x + rect.width/2, y: rect.y + rect.height/2});
	`, id, xpath)
	
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

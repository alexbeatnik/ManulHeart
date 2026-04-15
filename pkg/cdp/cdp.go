// Package cdp provides a low-level Chrome DevTools Protocol transport,
// session management, and domain method wrappers.
//
// This package is strictly the browser backend transport layer.
// It knows nothing about DSL commands, heuristics, or scoring.
// All DOM intelligence lives in pkg/core, pkg/heuristics, and pkg/scorer.
package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Target describes a browser target (tab/page) returned by the CDP /json endpoint.
type Target struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
	WSURL string `json:"webSocketDebuggerUrl"`
}

// Message is a raw CDP protocol message.
type Message struct {
	ID     int64          `json:"id,omitempty"`
	Method string         `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *CDPError      `json:"error,omitempty"`
	// SessionID is present for session-multiplexed messages.
	SessionID string `json:"sessionId,omitempty"`
}

// CDPError represents a CDP protocol-level error.
type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (e *CDPError) Error() string {
	if e.Data != "" {
		return fmt.Sprintf("CDP error %d: %s (%s)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("CDP error %d: %s", e.Code, e.Message)
}

// Conn is a WebSocket-backed CDP connection to a single browser target.
type Conn struct {
	ws      *websocket.Conn
	mu      sync.Mutex
	counter atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan *Message

	eventMu      sync.RWMutex
	eventHandlers map[string][]func(json.RawMessage)

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// DialTarget opens a WebSocket CDP connection to the given target WebSocket URL.
func DialTarget(ctx context.Context, wsURL string) (*Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	ws, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cdp: dial %q: %w", wsURL, err)
	}

	connCtx, cancel := context.WithCancel(ctx)
	c := &Conn{
		ws:            ws,
		pending:       make(map[int64]chan *Message),
		eventHandlers: make(map[string][]func(json.RawMessage)),
		ctx:           connCtx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

// Close tears down the CDP connection.
func (c *Conn) Close() error {
	c.cancel()
	err := c.ws.Close()
	<-c.done
	return err
}

// Call sends a CDP command and waits for the response.
func (c *Conn) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.counter.Add(1)

	var rawParams json.RawMessage
	if params != nil {
		var err error
		rawParams, err = json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("cdp: marshal params: %w", err)
		}
	}

	msg := Message{
		ID:     id,
		Method: method,
		Params: rawParams,
	}

	ch := make(chan *Message, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	c.mu.Lock()
	err := c.ws.WriteJSON(msg)
	c.mu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("cdp: send %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("cdp: %s: %w", method, ctx.Err())
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// OnEvent registers a handler for a CDP event method name.
func (c *Conn) OnEvent(method string, handler func(json.RawMessage)) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()
	c.eventHandlers[method] = append(c.eventHandlers[method], handler)
}

// readLoop reads CDP messages and dispatches them.
func (c *Conn) readLoop() {
	defer close(c.done)
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			// Connection closed — fail all pending calls
			c.pendingMu.Lock()
			for id, ch := range c.pending {
				ch <- &Message{ID: id, Error: &CDPError{Code: -1, Message: "connection closed"}}
				delete(c.pending, id)
			}
			c.pendingMu.Unlock()
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		if msg.ID > 0 {
			c.pendingMu.Lock()
			ch, ok := c.pending[msg.ID]
			if ok {
				delete(c.pending, msg.ID)
			}
			c.pendingMu.Unlock()
			if ok {
				ch <- &msg
			}
			continue
		}

		if msg.Method != "" {
			c.eventMu.RLock()
			handlers := c.eventHandlers[msg.Method]
			c.eventMu.RUnlock()
			for _, h := range handlers {
				go h(msg.Params)
			}
		}
	}
}

// ── Target discovery ──────────────────────────────────────────────────────────

// ListTargets queries the CDP HTTP endpoint for available page targets.
func ListTargets(ctx context.Context, endpoint string) ([]Target, error) {
	url := endpoint + "/json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cdp: list targets at %q: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var targets []Target
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("cdp: parse targets: %w", err)
	}
	return targets, nil
}

// FindPageTarget returns the first target with type "page", or an error if none.
func FindPageTarget(targets []Target) (*Target, error) {
	for i := range targets {
		if targets[i].Type == "page" {
			return &targets[i], nil
		}
	}
	return nil, fmt.Errorf("cdp: no page target found (got %d targets)", len(targets))
}

// ── Domain helpers ────────────────────────────────────────────────────────────

// Navigate sends Page.navigate and waits for the response.
func Navigate(ctx context.Context, conn *Conn, url string) error {
	_, err := conn.Call(ctx, "Page.navigate", map[string]any{"url": url})
	return err
}

// EvaluateResult is the result of Runtime.evaluate.
type EvaluateResult struct {
	Value            json.RawMessage `json:"value"`
	Type             string          `json:"type"`
	UnserializableValue string       `json:"unserializableValue,omitempty"`
}

// Evaluate runs a JavaScript expression in the page and returns its result.
// The expression must return a JSON-serializable value.
func Evaluate(ctx context.Context, conn *Conn, expression string) (json.RawMessage, error) {
	result, err := conn.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":            expression,
		"returnByValue":         true,
		"awaitPromise":          true,
		"userGesture":           true,
	})
	if err != nil {
		return nil, fmt.Errorf("cdp: evaluate: %w", err)
	}

	// Unwrap the Runtime.evaluate envelope: { result: { value: ... } }
	var envelope struct {
		Result struct {
			Value            json.RawMessage `json:"value"`
			Type             string          `json:"type"`
			SubType          string          `json:"subtype"`
			Description      string          `json:"description"`
			UnserializableValue string       `json:"unserializableValue"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return nil, fmt.Errorf("cdp: unmarshal evaluate result: %w", err)
	}
	if envelope.ExceptionDetails != nil {
		return nil, fmt.Errorf("cdp: js exception: %s", envelope.ExceptionDetails.Text)
	}
	return envelope.Result.Value, nil
}

// CallFunctionOn calls a serialized JS function expression with JSON arguments
// in the page context. The function is called as: (<expr>)(<args>)
func CallFunctionOn(ctx context.Context, conn *Conn, fnExpr string, args any) (json.RawMessage, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("cdp: marshal args: %w", err)
	}
	// Invoke via Runtime.evaluate: (fn)(args) pattern
	expr := fmt.Sprintf("(%s)(%s)", fnExpr, string(argsJSON))
	return Evaluate(ctx, conn, expr)
}

// WaitForLoad enables Page domain events and waits for Page.loadEventFired.
func WaitForLoad(ctx context.Context, conn *Conn, timeout time.Duration) error {
	// Enable Page events
	if _, err := conn.Call(ctx, "Page.enable", nil); err != nil {
		return fmt.Errorf("cdp: Page.enable: %w", err)
	}

	loadCh := make(chan struct{}, 1)
	conn.OnEvent("Page.loadEventFired", func(_ json.RawMessage) {
		select {
		case loadCh <- struct{}{}:
		default:
		}
	})

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-loadCh:
		return nil
	case <-timeoutCtx.Done():
		// Load may have already fired before we registered — that is acceptable.
		return nil
	}
}

// DispatchMouseEvent dispatches a mouse event at viewport coordinates (x, y).
func DispatchMouseEvent(ctx context.Context, conn *Conn, eventType string, x, y float64) error {
	_, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":       eventType,
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	return err
}

// Click synthesizes a left-click at viewport coordinates (x, y).
func Click(ctx context.Context, conn *Conn, x, y float64) error {
	if err := DispatchMouseEvent(ctx, conn, "mousePressed", x, y); err != nil {
		return err
	}
	return DispatchMouseEvent(ctx, conn, "mouseReleased", x, y)
}

// Focus focuses a DOM element specified by its XPath by evaluating JS.
func FocusByXPath(ctx context.Context, conn *Conn, xpath string) error {
	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (el) { el.focus(); return true; }
		return false;
	})()`, xpath)
	raw, err := Evaluate(ctx, conn, expr)
	if err != nil {
		return err
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil || !ok {
		return fmt.Errorf("cdp: focus failed for xpath %q", xpath)
	}
	return nil
}

// TypeText dispatches key events for every rune in the text string.
func TypeText(ctx context.Context, conn *Conn, text string) error {
	for _, ch := range text {
		params := map[string]any{
			"type": "keyDown",
			"text": string(ch),
		}
		if _, err := conn.Call(ctx, "Input.dispatchKeyEvent", params); err != nil {
			return err
		}
		params["type"] = "keyUp"
		if _, err := conn.Call(ctx, "Input.dispatchKeyEvent", params); err != nil {
			return err
		}
	}
	return nil
}

// SetInputValue sets an input element's value directly via JS (faster than key events
// for long strings, and fires the correct input/change events).
func SetInputValue(ctx context.Context, conn *Conn, xpath, value string) error {
	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return false;
		el.focus();
		if (el.isContentEditable || el.getAttribute('contenteditable') === 'true') {
			el.innerText = %q;
		} else {
			const proto = el.tagName === 'TEXTAREA'
				? window.HTMLTextAreaElement.prototype
				: window.HTMLInputElement.prototype;
			const nativeInputValueSetter = Object.getOwnPropertyDescriptor(proto, 'value');
			if (nativeInputValueSetter && nativeInputValueSetter.set) {
				nativeInputValueSetter.set.call(el, %q);
			} else {
				el.value = %q;
			}
		}
		el.dispatchEvent(new Event('input', {bubbles:true}));
		el.dispatchEvent(new Event('change', {bubbles:true}));
		return true;
	})()`, xpath, value, value, value)
	raw, err := Evaluate(ctx, conn, expr)
	if err != nil {
		return err
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil || !ok {
		return fmt.Errorf("cdp: set value failed for xpath %q", xpath)
	}
	return nil
}

// GetCurrentURL returns the current page URL.
func GetCurrentURL(ctx context.Context, conn *Conn) (string, error) {
	raw, err := Evaluate(ctx, conn, "window.location.href")
	if err != nil {
		return "", err
	}
	var url string
	if err := json.Unmarshal(raw, &url); err != nil {
		return "", err
	}
	return url, nil
}

// ScrollIntoView scrolls the element at the given XPath into view.
func ScrollIntoView(ctx context.Context, conn *Conn, xpath string) error {
	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (el) { el.scrollIntoView({behavior:'instant', block:'center'}); return true; }
		return false;
	})()`, xpath)
	_, err := Evaluate(ctx, conn, expr)
	return err
}

// ScrollPage scrolls the page or a container element by the viewport height.
// direction is "down" (+1) or "up" (-1). container is a CSS selector or empty for window.
func ScrollPage(ctx context.Context, conn *Conn, direction string, container string) error {
	sign := 1
	if direction == "up" {
		sign = -1
	}
	var expr string
	if container != "" {
		expr = fmt.Sprintf(`(() => {
			// Try CSS selector first, then text-based search for scrollable container
			let el = document.querySelector(%q);
			if (!el) {
				const query = %q;
				// Find scrollable elements containing the text
				const candidates = Array.from(document.querySelectorAll('*')).filter(e => {
					const st = getComputedStyle(e);
					const scrollable = e.scrollHeight > e.clientHeight && 
						(st.overflow === 'auto' || st.overflow === 'scroll' || 
						 st.overflowY === 'auto' || st.overflowY === 'scroll');
					return scrollable && e.textContent.toLowerCase().includes(query);
				});
				// Prefer the innermost scrollable container
				el = candidates.length > 0 ? candidates[candidates.length - 1] : null;
			}
			if (!el) {
				// Last resort: find any element matching text
				el = Array.from(document.querySelectorAll('*')).find(e =>
					e.textContent.toLowerCase().includes(%q));
			}
			if (el) {
				if (%d > 0) {
					el.scrollTop = el.scrollHeight;
				} else {
					el.scrollTop = 0;
				}
				return true;
			}
			return false;
		})()`, container, strings.ToLower(container), strings.ToLower(container), sign)
	} else {
		expr = fmt.Sprintf(`(() => {
			window.scrollBy(0, %d * window.innerHeight);
			return true;
		})()`, sign)
	}
	_, err := Evaluate(ctx, conn, expr)
	return err
}

// DispatchKeyEvent dispatches a single CDP Input.dispatchKeyEvent.
func DispatchKeyEvent(ctx context.Context, conn *Conn, eventType string, params map[string]any) error {
	p := map[string]any{"type": eventType}
	for k, v := range params {
		p[k] = v
	}
	_, err := conn.Call(ctx, "Input.dispatchKeyEvent", p)
	return err
}

// DoubleClick synthesizes a double-click at viewport coordinates (x, y).
func DoubleClick(ctx context.Context, conn *Conn, x, y float64) error {
	// First click
	if err := Click(ctx, conn, x, y); err != nil {
		return err
	}
	// Second click with clickCount: 2
	params := map[string]any{
		"type":       "mousePressed",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 2,
	}
	if _, err := conn.Call(ctx, "Input.dispatchMouseEvent", params); err != nil {
		return err
	}
	params["type"] = "mouseReleased"
	_, err := conn.Call(ctx, "Input.dispatchMouseEvent", params)
	return err
}

// RightClick synthesizes a right-click (context menu) at viewport coordinates (x, y).
func RightClick(ctx context.Context, conn *Conn, x, y float64) error {
	if _, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":       "mousePressed",
		"x":          x,
		"y":          y,
		"button":     "right",
		"clickCount": 1,
	}); err != nil {
		return err
	}
	_, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":       "mouseReleased",
		"x":          x,
		"y":          y,
		"button":     "right",
		"clickCount": 1,
	})
	return err
}

// Hover moves the mouse to viewport coordinates (x, y) without clicking.
func Hover(ctx context.Context, conn *Conn, x, y float64) error {
	_, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved",
		"x":    x,
		"y":    y,
	})
	return err
}

// DragAndDrop synthesizes a drag from (fromX, fromY) to (toX, toY).
// Uses Input.dispatchMouseEvent with proper timing and button state
// to work with jQuery UI draggable, HTML5 drag, and other frameworks.
func DragAndDrop(ctx context.Context, conn *Conn, fromX, fromY, toX, toY float64) error {
	// Move to source first (establishes hover state)
	if _, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved", "x": fromX, "y": fromY,
		"pointerType": "mouse",
	}); err != nil {
		return err
	}
	time.Sleep(80 * time.Millisecond)

	// Press at source
	if _, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type": "mousePressed", "x": fromX, "y": fromY,
		"button": "left", "buttons": 1, "clickCount": 1,
		"pointerType": "mouse",
	}); err != nil {
		return err
	}
	time.Sleep(150 * time.Millisecond)

	// Move 2px to exceed default distance threshold
	if _, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved", "x": fromX + 2, "y": fromY,
		"button": "left", "buttons": 1,
		"pointerType": "mouse",
	}); err != nil {
		return err
	}
	time.Sleep(60 * time.Millisecond)

	// Move to target with intermediate steps
	steps := 15
	for i := 1; i <= steps; i++ {
		ratio := float64(i) / float64(steps)
		mx := fromX + (toX-fromX)*ratio
		my := fromY + (toY-fromY)*ratio
		if _, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type": "mouseMoved", "x": mx, "y": my,
			"button": "left", "buttons": 1,
			"pointerType": "mouse",
		}); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(80 * time.Millisecond)

	// Release at target
	_, err := conn.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type": "mouseReleased", "x": toX, "y": toY,
		"button": "left", "buttons": 0, "clickCount": 1,
		"pointerType": "mouse",
	})
	return err
}

// SetFileInput sets file paths on a file input element via DOM.setFileInputFiles.
func SetFileInput(ctx context.Context, conn *Conn, xpath string, filePaths []string) error {
	// First resolve the XPath to a DOM nodeId.
	docResult, err := conn.Call(ctx, "DOM.getDocument", map[string]any{"depth": 0})
	if err != nil {
		return fmt.Errorf("cdp: DOM.getDocument: %w", err)
	}
	var doc struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := json.Unmarshal(docResult, &doc); err != nil {
		return fmt.Errorf("cdp: parse document: %w", err)
	}

	// Evaluate XPath to get the remote object
	expr := fmt.Sprintf(`document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue`, xpath)
	result, evalErr := conn.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression": expr,
	})
	if evalErr != nil {
		return fmt.Errorf("cdp: eval xpath: %w", evalErr)
	}
	var evalEnvelope struct {
		Result struct {
			ObjectID string `json:"objectId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(result, &evalEnvelope); err != nil || evalEnvelope.Result.ObjectID == "" {
		return fmt.Errorf("cdp: file input element not found at %q", xpath)
	}

	// Resolve the object to a DOM node
	descResult, err := conn.Call(ctx, "DOM.describeNode", map[string]any{
		"objectId": evalEnvelope.Result.ObjectID,
	})
	if err != nil {
		return fmt.Errorf("cdp: DOM.describeNode: %w", err)
	}
	var descEnvelope struct {
		Node struct {
			BackendNodeID int `json:"backendNodeId"`
		} `json:"node"`
	}
	if err := json.Unmarshal(descResult, &descEnvelope); err != nil {
		return fmt.Errorf("cdp: parse node description: %w", err)
	}

	// Set the files
	_, err = conn.Call(ctx, "DOM.setFileInputFiles", map[string]any{
		"files":         filePaths,
		"backendNodeId": descEnvelope.Node.BackendNodeID,
	})
	return err
}

// Screenshot captures a PNG screenshot of the current page.
// Returns the raw PNG bytes.
func Screenshot(ctx context.Context, conn *Conn) ([]byte, error) {
	result, err := conn.Call(ctx, "Page.captureScreenshot", map[string]any{
		"format": "png",
	})
	if err != nil {
		return nil, fmt.Errorf("cdp: screenshot: %w", err)
	}
	var envelope struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return nil, fmt.Errorf("cdp: parse screenshot: %w", err)
	}
	// CDP returns base64-encoded data
	decoded, err := base64.StdEncoding.DecodeString(envelope.Data)
	if err != nil {
		return nil, fmt.Errorf("cdp: decode screenshot: %w", err)
	}
	return decoded, nil
}

// WaitForResponse waits for a network response matching the URL pattern.
func WaitForResponse(ctx context.Context, conn *Conn, urlPattern string, timeout time.Duration) error {
	// Enable Network domain
	if _, err := conn.Call(ctx, "Network.enable", nil); err != nil {
		return fmt.Errorf("cdp: Network.enable: %w", err)
	}

	matched := make(chan struct{}, 1)
	conn.OnEvent("Network.responseReceived", func(params json.RawMessage) {
		var resp struct {
			Response struct {
				URL string `json:"url"`
			} `json:"response"`
		}
		if err := json.Unmarshal(params, &resp); err == nil {
			if strings.Contains(resp.Response.URL, urlPattern) {
				select {
				case matched <- struct{}{}:
				default:
				}
			}
		}
	})

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-matched:
		return nil
	case <-timeoutCtx.Done():
		return fmt.Errorf("cdp: wait for response %q: timed out", urlPattern)
	}
}

// HighlightElement injects a temporary border highlight around the given XPath.
func HighlightElement(ctx context.Context, conn *Conn, xpath string, durationMS int) error {
	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return false;
		const prev = el.style.outline;
		el.style.outline = '3px solid magenta';
		el.scrollIntoView({behavior:'smooth', block:'center'});
		setTimeout(() => { el.style.outline = prev; }, %d);
		return true;
	})()`, xpath, durationMS)
	_, err := Evaluate(ctx, conn, expr)
	return err
}

// GetElementCenter returns the center viewport coordinates for the element at xpath.
func GetElementCenter(ctx context.Context, conn *Conn, xpath string) (float64, float64, error) {
	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return null;
		const rect = el.getBoundingClientRect();
		return {x: rect.left + rect.width/2, y: rect.top + rect.height/2};
	})()`, xpath)
	raw, err := Evaluate(ctx, conn, expr)
	if err != nil {
		return 0, 0, err
	}
	var pos struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal(raw, &pos); err != nil {
		return 0, 0, fmt.Errorf("cdp: cannot get center for %q", xpath)
	}
	return pos.X, pos.Y, nil
}

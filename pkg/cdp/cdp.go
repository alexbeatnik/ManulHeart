// Package cdp provides a low-level Chrome DevTools Protocol transport,
// session management, and domain method wrappers.
//
// This package is strictly the browser backend transport layer.
// It knows nothing about DSL commands, heuristics, or scoring.
// All DOM intelligence lives in pkg/core, pkg/heuristics, and pkg/scorer.
package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
			const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
				window.HTMLInputElement.prototype, 'value') ||
				Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value');
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
		if (el) { el.scrollIntoView({behavior:'smooth', block:'center'}); return true; }
		return false;
	})()`, xpath)
	_, err := Evaluate(ctx, conn, expr)
	return err
}

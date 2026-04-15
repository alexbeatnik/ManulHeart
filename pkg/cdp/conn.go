package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// msgReq represents a JSON-RPC 2.0 request.
type msgReq struct {
	ID     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// msgResp represents a JSON-RPC 2.0 response or event.
type msgResp struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *msgError       `json:"error,omitempty"`
}

type msgError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Conn is a live WebSocket connection to a Chrome DevTools Protocol target.
// It is safe for concurrent use from multiple goroutines.
type Conn struct {
	wsURL string
	ws    *websocket.Conn
	mu      sync.Mutex
	writeMu sync.Mutex
	idSeq   int

	// pending maps message IDs to channels waiting for the response.
	pending map[int]chan *msgResp

	// events is a channel broadcasting all received events.
	// Many listeners can subscribe. For now we use a simple fanout or just bare handling.
	eventSubs []chan *msgResp
	subsMu    sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// DialTarget establishes a WebSocket connection to the given target URL.
func DialTarget(ctx context.Context, wsURL string) (*Conn, error) {
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial %s: %w", wsURL, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	c := &Conn{
		wsURL:   wsURL,
		ws:      ws,
		pending: make(map[int]chan *msgResp),
		ctx:     ctx,
		cancel:  cancel,
	}

	go c.readLoop()
	return c, nil
}

// Close terminates the WebSocket connection.
func (c *Conn) Close() error {
	if c == nil {
		return nil
	}
	c.cancel()
	if c.ws != nil {
		return c.ws.Close()
	}
	return nil
}

func (c *Conn) readLoop() {
	defer c.cancel()
	for {
		_, msg, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		var resp msgResp
		if err := json.Unmarshal(msg, &resp); err != nil {
			// fmt.Printf("cdp readLoop unmarshal error: %v, msg=%s\n", err, string(msg))
			continue
		}
		// fmt.Printf("cdp readLoop: %s\n", string(msg))

		if resp.ID != 0 {
			// It's a response to a request
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()
			if ok {
				ch <- &resp
			}
		} else if resp.Method != "" {
			// It's an event
			c.subsMu.Lock()
			for _, sub := range c.eventSubs {
				select {
				case sub <- &resp:
				default:
				}
			}
			c.subsMu.Unlock()
		}
	}
}

// Subscribe returns a channel that receives CDP events.
func (c *Conn) Subscribe() <-chan *msgResp {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	ch := make(chan *msgResp, 64)
	c.eventSubs = append(c.eventSubs, ch)
	return ch
}

// Unsubscribe removes an event listener channel.
func (c *Conn) Unsubscribe(ch chan *msgResp) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	for i, sub := range c.eventSubs {
		if sub == ch {
			c.eventSubs = append(c.eventSubs[:i], c.eventSubs[i+1:]...)
			break
		}
	}
}

// Call sends a JSON-RPC request and waits for the response.
func (c *Conn) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.idSeq++
	id := c.idSeq
	ch := make(chan *msgResp, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	req := msgReq{
		ID:     id,
		Method: method,
		Params: params,
	}

	c.writeMu.Lock()
	err := c.ws.WriteJSON(req)
	c.writeMu.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("cdp write: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("connection closed")
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("cdp error: code=%d message=%s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// ListTargets queries the Chrome HTTP endpoint and returns all available targets.
func ListTargets(ctx context.Context, endpoint string) ([]Target, error) {
	url := endpoint
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = fmt.Sprintf("http://%s", endpoint)
	}
	url = strings.TrimSuffix(url, "/") + "/json/list"
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}
	defer resp.Body.Close()

	var targets []Target
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, fmt.Errorf("decode targets: %w", err)
	}
	return targets, nil
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

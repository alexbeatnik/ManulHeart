package cdp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startMockCDP spins up a local WebSocket endpoint that echoes JSON-RPC
// requests back as responses and lets tests inject CDP events.
func startMockCDP(t *testing.T) (wsURL string, eventsIn chan<- json.RawMessage, stop func()) {
	t.Helper()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	events := make(chan json.RawMessage, 16)
	var (
		mu       sync.Mutex
		liveConn *websocket.Conn
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		mu.Lock()
		liveConn = conn
		mu.Unlock()

		go func() {
			for ev := range events {
				mu.Lock()
				c := liveConn
				mu.Unlock()
				if c == nil {
					return
				}
				_ = c.WriteMessage(websocket.TextMessage, ev)
			}
		}()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				mu.Lock()
				liveConn = nil
				mu.Unlock()
				return
			}
			var req msgReq
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			resp := msgResp{ID: req.ID, Result: json.RawMessage(`{"ok":true}`)}
			b, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, b)
		}
	}))

	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	stop = func() {
		close(events)
		srv.Close()
	}
	return wsURL, events, stop
}

func TestConn_CallReturnsResult(t *testing.T) {
	wsURL, _, stop := startMockCDP(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialTarget(ctx, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	res, err := c.Call(ctx, "Test.method", nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if string(res) != `{"ok":true}` {
		t.Fatalf("unexpected result: %s", string(res))
	}
}

func TestConn_ParallelCallsUniqueIDs(t *testing.T) {
	wsURL, _, stop := startMockCDP(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := DialTarget(ctx, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	const N = 64
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Call(ctx, "Test.method", nil); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("call err: %v", err)
	}
}

func TestConn_SubscribeReceivesEvents(t *testing.T) {
	wsURL, events, stop := startMockCDP(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialTarget(ctx, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	sub := c.Subscribe()
	defer sub.Close()

	// Give the connection a moment to be live on the server side.
	time.Sleep(50 * time.Millisecond)
	events <- json.RawMessage(`{"method":"Test.event","params":{"k":"v"}}`)

	select {
	case ev, ok := <-sub.C():
		if !ok {
			t.Fatalf("subscription closed unexpectedly")
		}
		if ev.Method != "Test.event" {
			t.Fatalf("unexpected event method: %s", ev.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("did not receive event")
	}
}

func TestConn_SubscriptionCloseIdempotent(t *testing.T) {
	wsURL, _, stop := startMockCDP(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialTarget(ctx, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	sub := c.Subscribe()
	sub.Close()
	sub.Close() // must not panic
}

func TestConn_CloseUnblocksSubscribers(t *testing.T) {
	wsURL, _, stop := startMockCDP(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialTarget(ctx, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	sub := c.Subscribe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range sub.C() {
		}
	}()

	_ = c.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("subscriber did not exit after Close()")
	}
}

func TestConn_CloseIsIdempotent(t *testing.T) {
	wsURL, _, stop := startMockCDP(t)
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialTarget(ctx, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	// Second close must not panic and must return without error.
	if err := c.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestConn_ParentCtxCancelTearsDownConn(t *testing.T) {
	wsURL, _, stop := startMockCDP(t)
	defer stop()

	parent, cancel := context.WithCancel(context.Background())
	c, err := DialTarget(parent, wsURL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	cancel()

	// Pending Call must observe connection closure.
	_, err = c.Call(context.Background(), "Test.method", nil)
	if err == nil {
		t.Fatalf("expected error after parent cancel")
	}
}

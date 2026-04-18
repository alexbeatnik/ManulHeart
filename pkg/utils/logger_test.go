package utils

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// ---- StripANSIWriter --------------------------------------------------------

func TestStripANSIWriter_StripsEscapeCodes(t *testing.T) {
	var buf bytes.Buffer
	w := StripANSIWriter{W: &buf}

	msg := "\033[31mhello\033[0m world"
	n, err := w.Write([]byte(msg))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned n=%d want %d", n, len(msg))
	}
	got := buf.String()
	if strings.Contains(got, "\033") {
		t.Errorf("output still contains ANSI: %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "world") {
		t.Errorf("text content missing: %q", got)
	}
}

func TestStripANSIWriter_NoEscapes(t *testing.T) {
	var buf bytes.Buffer
	w := StripANSIWriter{W: &buf}
	n, err := w.Write([]byte("clean text"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len("clean text") {
		t.Errorf("n=%d", n)
	}
	if buf.String() != "clean text" {
		t.Errorf("got %q", buf.String())
	}
}

func TestStripANSIWriter_AlwaysReturnsInputLen(t *testing.T) {
	var buf bytes.Buffer
	w := StripANSIWriter{W: &buf}
	// Write with embedded escape: output is shorter but n should still be len(p).
	p := []byte("\033[1mA\033[0m")
	n, err := w.Write(p)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(p) {
		t.Errorf("n=%d want %d", n, len(p))
	}
}

// ---- NewLogger --------------------------------------------------------------

func TestNewLogger_WritesToOut(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{out: &buf, mu: &sync.Mutex{}}
	l.Info("hello %s", "world")
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("output missing: %q", buf.String())
	}
}

func TestNewLogger_WritesToFile(t *testing.T) {
	var outBuf, fileBuf bytes.Buffer
	l := &Logger{
		out:  &outBuf,
		file: StripANSIWriter{W: &fileBuf},
		mu:   &sync.Mutex{},
	}
	l.Warn("danger!")
	if !strings.Contains(outBuf.String(), "danger!") {
		t.Errorf("stdout missing warn: %q", outBuf.String())
	}
	if !strings.Contains(fileBuf.String(), "danger!") {
		t.Errorf("file missing warn: %q", fileBuf.String())
	}
	// File output must not contain ANSI codes.
	if strings.Contains(fileBuf.String(), "\033") {
		t.Errorf("file contains ANSI: %q", fileBuf.String())
	}
}

// ---- WithLevel / Debug gating -----------------------------------------------

func TestWithLevel_DebugGated(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{out: &buf, mu: &sync.Mutex{}, level: LogLevelInfo}

	l.Debug("should not appear")
	if strings.Contains(buf.String(), "should not appear") {
		t.Error("Debug logged at LogLevelInfo")
	}

	l2 := l.WithLevel(LogLevelDebug)
	l2.Debug("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("Debug not logged at LogLevelDebug")
	}
}

// ---- WithPrefix -------------------------------------------------------------

func TestWithPrefix_PrependedToOutput(t *testing.T) {
	var buf bytes.Buffer
	base := &Logger{out: &buf, mu: &sync.Mutex{}}
	child := WithPrefix(base, "[w1] ")
	child.Info("step done")
	got := buf.String()
	if !strings.Contains(got, "[w1] step done") {
		t.Errorf("prefix missing: %q", got)
	}
}

func TestWithPrefix_SharesMutex(t *testing.T) {
	var buf bytes.Buffer
	base := &Logger{out: &buf, mu: &sync.Mutex{}}
	c1 := WithPrefix(base, "[a] ")
	c2 := WithPrefix(base, "[b] ")
	if c1.mu != c2.mu {
		t.Error("children must share the parent mutex pointer")
	}
}

func TestWithPrefix_NilParent(t *testing.T) {
	// Should not panic; uses default stdout logger.
	child := WithPrefix(nil, "[x] ")
	if child == nil {
		t.Error("WithPrefix(nil,...) returned nil")
	}
}

// ---- Concurrent writes (race detector) --------------------------------------

func TestLogger_ConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{out: &buf, mu: &sync.Mutex{}}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Info("goroutine %d", n)
		}(i)
	}
	wg.Wait()
}

// ---- Semantic helpers -------------------------------------------------------

func TestSemanticMethods_Smoke(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{out: &buf, mu: &sync.Mutex{}}

	l.Startup("claude-3", "chromium")
	l.BlockStart("Login Block")
	l.BlockPass("Login Block")
	l.BlockFail("Login Block")
	l.ActionStart("Click Login")
	l.ActionPass(1.23)
	l.ActionFail(nil)
	l.ActionWarn("retry")
	l.HeuristicDetail(0.75, "aria-label match")
	l.ActionDetail("⌨️", "typed %q", "admin")

	out := buf.String()
	for _, needle := range []string{
		"ManulEngine", "Login Block", "BLOCK PASS", "BLOCK FAIL",
		"ACTION START", "ACTION PASS", "ACTION FAIL", "ACTION WARN",
		"DOM HEURISTICS", "typed",
	} {
		if !strings.Contains(out, needle) {
			t.Errorf("output missing %q", needle)
		}
	}
}

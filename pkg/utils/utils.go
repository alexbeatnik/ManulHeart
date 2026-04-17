// Package utils provides logging, error helpers, and shared utilities.
package utils

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel controls verbosity.
type LogLevel int

const (
	LogLevelInfo  LogLevel = 0
	LogLevelDebug LogLevel = 1
)

// Logger is a simple structured logger for ManulHeart execution output.
//
// A Logger may carry a prefix (e.g. "[w3] ") that is emitted before every
// message. Use WithPrefix to derive a child logger that shares the parent's
// writer and level. Prefixes are nested: WithPrefix(parent, "B") on a
// parent already prefixed "A" yields a child whose lines start with "AB".
type Logger struct {
	level  LogLevel
	writer io.Writer
	prefix string
}

// NewLogger creates a new Logger writing to the given writer at the given level.
func NewLogger(level LogLevel, w io.Writer) *Logger {
	if w == nil {
		w = os.Stdout
	}
	return &Logger{level: level, writer: w}
}

// WithPrefix returns a child logger sharing the parent's writer and level,
// but prepending prefix to every message. If parent is nil, a default
// stdout/info logger is used.
func WithPrefix(parent *Logger, prefix string) *Logger {
	if parent == nil {
		parent = NewLogger(LogLevelInfo, nil)
	}
	return &Logger{
		level:  parent.level,
		writer: parent.writer,
		prefix: parent.prefix + prefix,
	}
}

// Info logs an informational message.
func (l *Logger) Info(format string, args ...any) {
	l.log("INFO", format, args...)
}

// Debug logs a debug message (only emitted at LogLevelDebug).
func (l *Logger) Debug(format string, args ...any) {
	if l.level >= LogLevelDebug {
		l.log("DEBUG", format, args...)
	}
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, args ...any) {
	l.log("WARN", format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) {
	l.log("ERROR", format, args...)
}

func (l *Logger) log(level, format string, args ...any) {
	ts := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.writer, "[%s] [%s] %s%s\n", ts, level, l.prefix, msg)
}

// ResolutionError is returned when the targeting pipeline cannot resolve an element.
type ResolutionError struct {
	Target      string
	Reason      string
	Candidates  int
	BestScore   float64
}

func (e *ResolutionError) Error() string {
	return fmt.Sprintf("cannot resolve element %q: %s (candidates=%d, best_score=%.3f)",
		e.Target, e.Reason, e.Candidates, e.BestScore)
}

// ActionError is returned when a resolved element action fails.
type ActionError struct {
	Action  string
	Target  string
	Cause   error
}

func (e *ActionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("action %q on %q failed: %v", e.Action, e.Target, e.Cause)
	}
	return fmt.Sprintf("action %q on %q failed", e.Action, e.Target)
}

func (e *ActionError) Unwrap() error { return e.Cause }

// ParseError is returned by the DSL parser.
type ParseError struct {
	Line    int
	Text    string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at line %d (%q): %s", e.Line, e.Text, e.Message)
}

// Package utils provides logging, error helpers, and shared utilities.
// Logger is defined in logger.go.
package utils

import "fmt"

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

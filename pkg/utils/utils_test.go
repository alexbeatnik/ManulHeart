package utils

import (
	"errors"
	"testing"
)

// ---- ResolutionError --------------------------------------------------------

func TestResolutionError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *ResolutionError
		want string
	}{
		{
			"basic",
			&ResolutionError{Target: "Login button", Reason: "no match", Candidates: 3, BestScore: 0.042},
			`cannot resolve element "Login button": no match (candidates=3, best_score=0.042)`,
		},
		{
			"zero candidates",
			&ResolutionError{Target: "Submit", Reason: "empty DOM", Candidates: 0, BestScore: 0.0},
			`cannot resolve element "Submit": empty DOM (candidates=0, best_score=0.000)`,
		},
		{
			"high score",
			&ResolutionError{Target: "x", Reason: "below threshold", Candidates: 100, BestScore: 0.123},
			`cannot resolve element "x": below threshold (candidates=100, best_score=0.123)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

// ---- ActionError ------------------------------------------------------------

func TestActionError_Error(t *testing.T) {
	cause := errors.New("CDP timeout")

	cases := []struct {
		name string
		err  *ActionError
		want string
	}{
		{
			"with cause",
			&ActionError{Action: "click", Target: "Login button", Cause: cause},
			`action "click" on "Login button" failed: CDP timeout`,
		},
		{
			"without cause",
			&ActionError{Action: "fill", Target: "username field", Cause: nil},
			`action "fill" on "username field" failed`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestActionError_Unwrap(t *testing.T) {
	cause := errors.New("sentinel")
	ae := &ActionError{Action: "click", Target: "btn", Cause: cause}

	if !errors.Is(ae, cause) {
		t.Error("errors.Is should find cause via Unwrap")
	}
}

func TestActionError_Unwrap_Nil(t *testing.T) {
	ae := &ActionError{Action: "click", Target: "btn", Cause: nil}
	if ae.Unwrap() != nil {
		t.Error("Unwrap should return nil when Cause is nil")
	}
}

// ---- ParseError -------------------------------------------------------------

func TestParseError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *ParseError
		want string
	}{
		{
			"normal",
			&ParseError{Line: 5, Text: "CLICK login", Message: "unknown command"},
			`parse error at line 5 ("CLICK login"): unknown command`,
		},
		{
			"line 1",
			&ParseError{Line: 1, Text: "", Message: "empty line"},
			`parse error at line 1 (""): empty line`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

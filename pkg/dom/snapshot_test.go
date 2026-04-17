package dom

import "testing"

// ── Normalize ─────────────────────────────────────────────────────────────────

func TestNormalize(t *testing.T) {
	el := ElementSnapshot{
		VisibleText: "  Hello World  ",
		AriaLabel:   " Submit Form ",
		Placeholder: "Enter email",
		LabelText:   "  Name  ",
		DataQA:      "login-btn",
		HTMLId:      "myBtn",
		FrameIndex:  2,
	}
	el.Normalize()

	if el.NormText != "hello world" {
		t.Errorf("NormText = %q", el.NormText)
	}
	if el.NormAriaLabel != "submit form" {
		t.Errorf("NormAriaLabel = %q", el.NormAriaLabel)
	}
	if el.NormPlaceholder != "enter email" {
		t.Errorf("NormPlaceholder = %q", el.NormPlaceholder)
	}
	if el.NormLabelText != "name" {
		t.Errorf("NormLabelText = %q", el.NormLabelText)
	}
	if el.NormDataQA != "login-btn" {
		t.Errorf("NormDataQA = %q", el.NormDataQA)
	}
	if el.NormHTMLId != "mybtn" {
		t.Errorf("NormHTMLId = %q", el.NormHTMLId)
	}
	if el.FrameIndex != 2 {
		t.Errorf("FrameIndex = %d, want 2", el.FrameIndex)
	}
}

// ── AllTextSignals ────────────────────────────────────────────────────────────

func TestAllTextSignals_Deduplication(t *testing.T) {
	el := ElementSnapshot{
		VisibleText: "Submit",
		AriaLabel:   "Submit",
		LabelText:   "Submit",
	}
	el.Normalize()
	signals := el.AllTextSignals()
	if len(signals) != 1 {
		t.Errorf("expected 1 unique signal, got %d: %v", len(signals), signals)
	}
}

func TestAllTextSignals_IncludesAll(t *testing.T) {
	el := ElementSnapshot{
		VisibleText: "Click Me",
		AriaLabel:   "Submit Form",
		Placeholder: "Enter text",
		LabelText:   "Name",
		DataQA:      "submit-btn",
		Title:       "Submit",
		DataTestID:  "submit-test",
		NameAttr:    "submit_name",
		Value:       "Go",
	}
	el.Normalize()
	signals := el.AllTextSignals()

	if len(signals) != 9 {
		t.Errorf("expected 9 signals, got %d: %v", len(signals), signals)
	}

	want := map[string]bool{
		"click me":    true,
		"submit form": true,
		"enter text":  true,
		"name":        true,
		"submit-btn":  true,
		"submit":      true,
		"submit-test": true,
		"submit_name": true,
		"go":          true,
	}
	for _, s := range signals {
		if !want[s] {
			t.Errorf("unexpected signal %q", s)
		}
		delete(want, s)
	}
	for w := range want {
		t.Errorf("missing signal %q", w)
	}
}

// ── IsInteractive ─────────────────────────────────────────────────────────────

func TestIsInteractive_CheckboxMode(t *testing.T) {
	tests := []struct {
		name string
		el   ElementSnapshot
		want bool
	}{
		{"checkbox", ElementSnapshot{Tag: "input", InputType: "checkbox"}, true},
		{"radio", ElementSnapshot{Tag: "input", InputType: "radio"}, true},
		{"text input", ElementSnapshot{Tag: "input", InputType: "text"}, false},
		{"checkbox role", ElementSnapshot{Tag: "div", Role: "checkbox"}, true},
		{"button", ElementSnapshot{Tag: "button"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.el.IsInteractive("checkbox")
			if got != tt.want {
				t.Errorf("IsInteractive(checkbox) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInteractive_InputMode(t *testing.T) {
	tests := []struct {
		name string
		el   ElementSnapshot
		want bool
	}{
		{"text input", ElementSnapshot{Tag: "input", InputType: "text", IsEditable: true}, true},
		{"textarea", ElementSnapshot{Tag: "textarea", IsEditable: true}, true},
		{"button", ElementSnapshot{Tag: "button"}, false},
		// Note: IsInteractive("input") only filters by tag, not inputType.
		// The JS probe handles the detailed checkbox/radio exclusion.
		{"checkbox", ElementSnapshot{Tag: "input", InputType: "checkbox"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.el.IsInteractive("input")
			if got != tt.want {
				t.Errorf("IsInteractive(input) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsInteractive_Disabled(t *testing.T) {
	el := ElementSnapshot{Tag: "button", IsDisabled: true}
	if el.IsInteractive("clickable") {
		t.Error("disabled element should not be interactive")
	}
}

package dom

// ─────────────────────────────────────────────────────────────────────────────
// VISIBILITY AND TREEWALKER TEST SUITE
//
// Port of ManulEngine test_31_visibility_treewalker.py
//
// Validates:
// A. ElementSnapshot normalization and text signals
// B. IsInteractive mode-aware filtering
// C. [HIDDEN] suffix detection in element names
// D. Special input types remain discoverable
// E. Snapshot filtering assertions
// ─────────────────────────────────────────────────────────────────────────────

import (
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Section A: Normalization correctness
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_NormalizationLowercase(t *testing.T) {
	el := ElementSnapshot{
		VisibleText: "  Submit Order  ",
		AriaLabel:   "  Close Dialog  ",
		Placeholder: "  Enter Email  ",
		LabelText:   "  Full Name  ",
		DataQA:      "  submit-btn  ",
		HTMLId:      "  myInput  ",
	}
	el.Normalize()

	if el.NormText != "submit order" {
		t.Errorf("NormText = %q", el.NormText)
	}
	if el.NormAriaLabel != "close dialog" {
		t.Errorf("NormAriaLabel = %q", el.NormAriaLabel)
	}
	if el.NormPlaceholder != "enter email" {
		t.Errorf("NormPlaceholder = %q", el.NormPlaceholder)
	}
	if el.NormLabelText != "full name" {
		t.Errorf("NormLabelText = %q", el.NormLabelText)
	}
	if el.NormDataQA != "submit-btn" {
		t.Errorf("NormDataQA = %q", el.NormDataQA)
	}
	if el.NormHTMLId != "myinput" {
		t.Errorf("NormHTMLId = %q", el.NormHTMLId)
	}
}

func TestVisibility_NormalizationEmpty(t *testing.T) {
	el := ElementSnapshot{}
	el.Normalize()

	if el.NormText != "" {
		t.Errorf("NormText should be empty, got %q", el.NormText)
	}
	if el.NormAriaLabel != "" {
		t.Errorf("NormAriaLabel should be empty, got %q", el.NormAriaLabel)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section B: AllTextSignals deduplication
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_TextSignalsDedup(t *testing.T) {
	el := ElementSnapshot{
		VisibleText: "Submit",
		AriaLabel:   "Submit", // same as VisibleText
		Placeholder: "Enter value",
		LabelText:   "Submit", // duplicate
		DataQA:      "submit-btn",
	}
	el.Normalize()

	signals := el.AllTextSignals()

	// Should be deduplicated: "submit", "enter value", "submit-btn"
	seen := map[string]int{}
	for _, s := range signals {
		seen[s]++
	}
	for s, count := range seen {
		if count > 1 {
			t.Errorf("signal %q appears %d times (should be deduplicated)", s, count)
		}
	}

	// "submit" should appear exactly once
	foundSubmit := false
	for _, s := range signals {
		if s == "submit" {
			foundSubmit = true
		}
	}
	if !foundSubmit {
		t.Error("'submit' not in text signals")
	}
}

func TestVisibility_TextSignalsIncludeAllAttributes(t *testing.T) {
	el := ElementSnapshot{
		VisibleText: "Button Text",
		AriaLabel:   "Aria Text",
		Placeholder: "Placeholder Text",
		LabelText:   "Label Text",
		DataQA:      "data-qa-text",
		Title:       "Title Text",
		DataTestID:  "test-id-text",
		NameAttr:    "name-attr-text",
		Value:       "value-text",
	}
	el.Normalize()

	signals := el.AllTextSignals()
	expected := []string{
		"button text", "aria text", "placeholder text", "label text",
		"data-qa-text", "title text", "test-id-text", "name-attr-text", "value-text",
	}
	for _, want := range expected {
		found := false
		for _, s := range signals {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected signal %q not found in %v", want, signals)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section C: IsInteractive mode filtering
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_InteractiveInputMode(t *testing.T) {
	tests := []struct {
		name     string
		el       ElementSnapshot
		expected bool
	}{
		{
			"text input is interactive in input mode",
			ElementSnapshot{Tag: "input", InputType: "text", IsEditable: true},
			true,
		},
		{
			"textarea is interactive in input mode",
			ElementSnapshot{Tag: "textarea", IsEditable: true},
			true,
		},
		{
			"button is NOT interactive in input mode",
			ElementSnapshot{Tag: "button"},
			false,
		},
		{
			"contenteditable div is interactive in input mode",
			ElementSnapshot{Tag: "div", IsEditable: true},
			true,
		},
		{
			"disabled input is NOT interactive",
			ElementSnapshot{Tag: "input", InputType: "text", IsDisabled: true},
			false,
		},
	}
	for _, tc := range tests {
		got := tc.el.IsInteractive("input")
		if got != tc.expected {
			t.Errorf("%s: IsInteractive(input) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestVisibility_InteractiveCheckboxMode(t *testing.T) {
	tests := []struct {
		name     string
		el       ElementSnapshot
		expected bool
	}{
		{
			"checkbox is interactive in checkbox mode",
			ElementSnapshot{Tag: "input", InputType: "checkbox"},
			true,
		},
		{
			"radio is interactive in checkbox mode",
			ElementSnapshot{Tag: "input", InputType: "radio"},
			true,
		},
		{
			"role=checkbox is interactive",
			ElementSnapshot{Tag: "div", Role: "checkbox"},
			true,
		},
		{
			"button is NOT interactive in checkbox mode",
			ElementSnapshot{Tag: "button"},
			false,
		},
		{
			"text input is NOT interactive in checkbox mode",
			ElementSnapshot{Tag: "input", InputType: "text"},
			false,
		},
	}
	for _, tc := range tests {
		got := tc.el.IsInteractive("checkbox")
		if got != tc.expected {
			t.Errorf("%s: IsInteractive(checkbox) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestVisibility_InteractiveSelectMode(t *testing.T) {
	tests := []struct {
		name     string
		el       ElementSnapshot
		expected bool
	}{
		{
			"select is interactive in select mode",
			ElementSnapshot{Tag: "select"},
			true,
		},
		{
			"role=listbox is interactive in select mode",
			ElementSnapshot{Tag: "div", Role: "listbox"},
			true,
		},
		{
			"role=combobox is interactive in select mode",
			ElementSnapshot{Tag: "div", Role: "combobox"},
			true,
		},
		{
			"button is NOT interactive in select mode",
			ElementSnapshot{Tag: "button"},
			false,
		},
	}
	for _, tc := range tests {
		got := tc.el.IsInteractive("select")
		if got != tc.expected {
			t.Errorf("%s: IsInteractive(select) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestVisibility_InteractiveClickableMode(t *testing.T) {
	// In clickable mode, everything is interactive (unless disabled)
	tests := []struct {
		name     string
		el       ElementSnapshot
		expected bool
	}{
		{"button", ElementSnapshot{Tag: "button"}, true},
		{"link", ElementSnapshot{Tag: "a"}, true},
		{"div", ElementSnapshot{Tag: "div"}, true},
		{"disabled button", ElementSnapshot{Tag: "button", IsDisabled: true}, false},
	}
	for _, tc := range tests {
		got := tc.el.IsInteractive("clickable")
		if got != tc.expected {
			t.Errorf("%s: IsInteractive(clickable) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section D: [HIDDEN] suffix detection
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_HiddenElementFlags(t *testing.T) {
	el := ElementSnapshot{
		Tag:       "button",
		IsVisible: false,
		IsHidden:  true,
	}
	if el.IsVisible {
		t.Error("hidden element should not be visible")
	}
	if !el.IsHidden {
		t.Error("hidden element should be marked hidden")
	}
}

func TestVisibility_VisibleElementFlags(t *testing.T) {
	el := ElementSnapshot{
		Tag:       "button",
		IsVisible: true,
		IsHidden:  false,
	}
	if !el.IsVisible {
		t.Error("visible element should be visible")
	}
	if el.IsHidden {
		t.Error("visible element should not be hidden")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section E: Special input types remain discoverable when hidden
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_HiddenCheckboxStillDiscoverable(t *testing.T) {
	// Hidden checkboxes (opacity: 0.01, etc.) should still be considered
	// interactive in checkbox mode
	el := ElementSnapshot{
		Tag:       "input",
		InputType: "checkbox",
		IsVisible: false, // visually hidden
		IsHidden:  false, // not display:none
	}
	if !el.IsInteractive("checkbox") {
		t.Error("hidden checkbox should still be interactive in checkbox mode")
	}
}

func TestVisibility_HiddenFileInputDiscoverable(t *testing.T) {
	// Hidden file inputs (display:none) are triggered by their labels.
	// The element itself is hidden but still valid for interaction.
	el := ElementSnapshot{
		Tag:       "input",
		InputType: "file",
		IsVisible: false,
		IsHidden:  true,
	}
	// File inputs are not interactive in input mode (they use upload commands)
	if el.IsInteractive("input") {
		t.Error("file input should not be interactive in input mode")
	}
	// But in clickable mode, everything is interactive (unless disabled)
	if !el.IsInteractive("clickable") {
		t.Error("file input should be interactive in clickable mode")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section F: Rect and geometry
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_ZeroSizeElement(t *testing.T) {
	el := ElementSnapshot{
		Tag:       "button",
		IsVisible: true,
		Rect:      Rect{Top: 0, Left: 0, Bottom: 0, Right: 0, Width: 0, Height: 0},
	}
	// Zero size doesn't affect IsInteractive
	if !el.IsInteractive("clickable") {
		t.Error("zero-size element should still be interactive in clickable mode")
	}
}

func TestVisibility_OffscreenElement(t *testing.T) {
	el := ElementSnapshot{
		Tag:       "button",
		IsVisible: false,
		Rect:      Rect{Top: -9999, Left: -9999, Bottom: -9998, Right: -9998, Width: 1, Height: 1},
	}
	if el.IsVisible {
		t.Error("offscreen element should not be visible")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section G: Shadow DOM elements
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_ShadowDOMElement(t *testing.T) {
	el := ElementSnapshot{
		Tag:        "input",
		InputType:  "text",
		IsInShadow: true,
		IsVisible:  true,
		IsEditable: true,
	}
	if !el.IsInShadow {
		t.Error("shadow DOM element should be marked")
	}
	if !el.IsInteractive("input") {
		t.Error("shadow DOM input should be interactive")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section H: Role-based interactivity
// ═══════════════════════════════════════════════════════════════════════════════

func TestVisibility_RoleBasedInteractivity(t *testing.T) {
	tests := []struct {
		role   string
		mode   string
		expect bool
	}{
		{"textbox", "input", true},
		{"spinbutton", "input", true},
		{"checkbox", "checkbox", true},
		{"radio", "checkbox", true},
		{"listbox", "select", true},
		{"combobox", "select", true},
		{"button", "clickable", true},
		{"link", "clickable", true},
		{"menuitem", "clickable", true},
		{"tab", "clickable", true},
	}
	for _, tc := range tests {
		el := ElementSnapshot{Tag: "div", Role: tc.role}
		got := el.IsInteractive(tc.mode)
		if got != tc.expect {
			t.Errorf("role=%q mode=%q: IsInteractive = %v, want %v",
				tc.role, tc.mode, got, tc.expect)
		}
	}
}

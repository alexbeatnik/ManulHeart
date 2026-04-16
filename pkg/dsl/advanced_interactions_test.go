package dsl

// ─────────────────────────────────────────────────────────────────────────────
// ADVANCED INTERACTIONS TEST SUITE
//
// Port of ManulEngine test_23_advanced_interactions.py — command classification
// for PRESS, RIGHT CLICK, UPLOAD, WAIT FOR, and related patterns.
//
// The Python version has two parts:
// 1. Pure classifications (classify_step / parseCommand) — ported here
// 2. Mocked Playwright handler tests — NOT portable (browser-specific)
//
// Validates:
// A. PRESS ENTER still maps to CmdPress
// B. PRESS Key variants (Escape, Control+A, Tab, etc.)
// C. PRESS Key ON 'Target' — targeted press
// D. RIGHT CLICK classification
// E. UPLOAD classification
// F. WAIT FOR element explicit waits
// G. Existing keywords (NAVIGATE, Click, DONE) still work
// H. Keywords inside quoted labels must NOT misclassify
// ─────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"testing"
)

// parseOne parses a single DSL line and returns the Command.
func parseOne(t *testing.T, line string) Command {
	t.Helper()
	cmd, err := parseCommand(line, 1)
	if err != nil {
		t.Fatalf("parseCommand(%q) failed: %v", line, err)
	}
	return cmd
}

// ── Section 1: PRESS variants ────────────────────────────────────────────────

func TestAdvanced_PressEnter(t *testing.T) {
	cmd := parseOne(t, "PRESS ENTER")
	if cmd.Type != CmdPress {
		t.Errorf("PRESS ENTER → type=%s, want %s", cmd.Type, CmdPress)
	}
	if cmd.PressKey != "ENTER" {
		t.Errorf("PRESS ENTER → key=%q, want %q", cmd.PressKey, "ENTER")
	}
}

func TestAdvanced_PressEscape(t *testing.T) {
	cmd := parseOne(t, "PRESS Escape")
	if cmd.Type != CmdPress {
		t.Errorf("PRESS Escape → type=%s, want %s", cmd.Type, CmdPress)
	}
	if cmd.PressKey != "Escape" {
		t.Errorf("PRESS Escape → key=%q, want %q", cmd.PressKey, "Escape")
	}
}

func TestAdvanced_PressControlA(t *testing.T) {
	cmd := parseOne(t, "PRESS Control+A")
	if cmd.Type != CmdPress {
		t.Errorf("type=%s, want %s", cmd.Type, CmdPress)
	}
	if cmd.PressKey != "Control+A" {
		t.Errorf("key=%q, want %q", cmd.PressKey, "Control+A")
	}
}

func TestAdvanced_PressTab(t *testing.T) {
	cmd := parseOne(t, "PRESS Tab")
	if cmd.Type != CmdPress {
		t.Errorf("type=%s, want %s", cmd.Type, CmdPress)
	}
	if cmd.PressKey != "Tab" {
		t.Errorf("key=%q, want %q", cmd.PressKey, "Tab")
	}
}

func TestAdvanced_PressArrowDownOnTarget(t *testing.T) {
	cmd := parseOne(t, "PRESS ArrowDown ON 'Search Input'")
	if cmd.Type != CmdPress {
		t.Errorf("type=%s, want %s", cmd.Type, CmdPress)
	}
	if cmd.PressKey != "ArrowDown" {
		t.Errorf("key=%q, want %q", cmd.PressKey, "ArrowDown")
	}
	if cmd.PressTarget != "Search Input" {
		t.Errorf("target=%q, want %q", cmd.PressTarget, "Search Input")
	}
}

func TestAdvanced_PressShiftTabOnTarget(t *testing.T) {
	cmd := parseOne(t, "PRESS Shift+Tab ON 'Username'")
	if cmd.Type != CmdPress {
		t.Errorf("type=%s, want %s", cmd.Type, CmdPress)
	}
	if cmd.PressKey != "Shift+Tab" {
		t.Errorf("key=%q, want %q", cmd.PressKey, "Shift+Tab")
	}
	if cmd.PressTarget != "Username" {
		t.Errorf("target=%q, want %q", cmd.PressTarget, "Username")
	}
}

// ── Section 2: RIGHT CLICK ──────────────────────────────────────────────────

func TestAdvanced_RightClick(t *testing.T) {
	cmd := parseOne(t, "RIGHT CLICK 'Image'")
	if cmd.Type != CmdRightClick {
		t.Errorf("type=%s, want %s", cmd.Type, CmdRightClick)
	}
	if cmd.Target != "Image" {
		t.Errorf("target=%q, want %q", cmd.Target, "Image")
	}
}

func TestAdvanced_RightClickThe(t *testing.T) {
	cmd := parseOne(t, "RIGHT CLICK the 'Context Menu Area'")
	if cmd.Type != CmdRightClick {
		t.Errorf("type=%s, want %s", cmd.Type, CmdRightClick)
	}
	if cmd.Target != "Context Menu Area" {
		t.Errorf("target=%q, want %q", cmd.Target, "Context Menu Area")
	}
}

func TestAdvanced_RightClickMixedCase(t *testing.T) {
	cmd := parseOne(t, "Right Click 'Menu'")
	if cmd.Type != CmdRightClick {
		t.Errorf("type=%s, want %s", cmd.Type, CmdRightClick)
	}
}

// ── Section 3: UPLOAD ────────────────────────────────────────────────────────

func TestAdvanced_Upload(t *testing.T) {
	cmd := parseOne(t, "UPLOAD 'avatar.png' to 'Profile Picture'")
	if cmd.Type != CmdUpload {
		t.Errorf("type=%s, want %s", cmd.Type, CmdUpload)
	}
	if cmd.UploadFile != "avatar.png" {
		t.Errorf("file=%q, want %q", cmd.UploadFile, "avatar.png")
	}
	if cmd.Target != "Profile Picture" {
		t.Errorf("target=%q, want %q", cmd.Target, "Profile Picture")
	}
}

func TestAdvanced_UploadNoNumber(t *testing.T) {
	cmd := parseOne(t, "UPLOAD 'file.pdf' to 'Dropzone'")
	if cmd.Type != CmdUpload {
		t.Errorf("type=%s, want %s", cmd.Type, CmdUpload)
	}
}

func TestAdvanced_UploadMixedCase(t *testing.T) {
	cmd := parseOne(t, "Upload 'data.csv' to 'Import'")
	if cmd.Type != CmdUpload {
		t.Errorf("type=%s, want %s", cmd.Type, CmdUpload)
	}
}

// ── Section 4: WAIT FOR element ──────────────────────────────────────────────

func TestAdvanced_WaitForVisible(t *testing.T) {
	cmd := parseOne(t, `Wait for "Welcome, User" to be visible`)
	if cmd.Type != CmdWaitFor {
		t.Errorf("type=%s, want %s", cmd.Type, CmdWaitFor)
	}
	if cmd.Target != "Welcome, User" {
		t.Errorf("target=%q, want %q", cmd.Target, "Welcome, User")
	}
	if cmd.WaitForState != "visible" {
		t.Errorf("state=%q, want %q", cmd.WaitForState, "visible")
	}
}

func TestAdvanced_WaitForDisappear(t *testing.T) {
	cmd := parseOne(t, "Wait for 'Loading...' to disappear")
	if cmd.Type != CmdWaitFor {
		t.Errorf("type=%s, want %s", cmd.Type, CmdWaitFor)
	}
	if cmd.WaitForState != "disappear" {
		t.Errorf("state=%q, want %q", cmd.WaitForState, "disappear")
	}
}

func TestAdvanced_WaitForHidden(t *testing.T) {
	cmd := parseOne(t, `Wait for "Submit" to be hidden`)
	if cmd.Type != CmdWaitFor {
		t.Errorf("type=%s, want %s", cmd.Type, CmdWaitFor)
	}
	if cmd.WaitForState != "hidden" {
		t.Errorf("state=%q, want %q", cmd.WaitForState, "hidden")
	}
}

// ── Section 5: Existing keywords still work ──────────────────────────────────

func TestAdvanced_NavigateStillWorks(t *testing.T) {
	cmd := parseOne(t, "NAVIGATE to https://x.com")
	if cmd.Type != CmdNavigate {
		t.Errorf("type=%s, want %s", cmd.Type, CmdNavigate)
	}
}

func TestAdvanced_ClickStillWorks(t *testing.T) {
	cmd := parseOne(t, "Click 'Submit'")
	if cmd.Type != CmdClick {
		t.Errorf("type=%s, want %s", cmd.Type, CmdClick)
	}
}

func TestAdvanced_DoubleClickParsed(t *testing.T) {
	cmd := parseOne(t, "DOUBLE CLICK the 'Double Click Me' button")
	if cmd.Type != CmdDoubleClick {
		t.Errorf("type=%s, want %s", cmd.Type, CmdDoubleClick)
	}
}

func TestAdvanced_HoverParsed(t *testing.T) {
	cmd := parseOne(t, "HOVER over the 'Mouse Hover' button")
	if cmd.Type != CmdHover {
		t.Errorf("type=%s, want %s", cmd.Type, CmdHover)
	}
}

// ── Section 6: Keywords inside quotes must NOT misclassify ───────────────────

func TestAdvanced_PressInsideQuotes(t *testing.T) {
	cmd := parseOne(t, "Click 'Press Here' button")
	if cmd.Type != CmdClick {
		t.Errorf("'Press Here' should be click, got type=%s", cmd.Type)
	}
}

func TestAdvanced_UploadInsideQuotes(t *testing.T) {
	cmd := parseOne(t, "Click the 'Upload Logo' button")
	if cmd.Type != CmdClick {
		t.Errorf("'Upload Logo' should be click, got type=%s", cmd.Type)
	}
}

func TestAdvanced_NavigateInsideQuotes(t *testing.T) {
	// "Fill 'Navigate Away' field with 'test'" — should be fill, not navigate
	cmd := parseOne(t, "Fill 'Navigate Away' field with 'test'")
	if cmd.Type != CmdFill {
		t.Errorf("'Navigate Away' should be fill, got type=%s", cmd.Type)
	}
}

// ── Section 7: STEP block label ──────────────────────────────────────────────

func TestAdvanced_StepBlockAssigned(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"STEP 1: Login\n" +
			"    NAVIGATE to 'https://example.com'\n" +
			"    Fill 'Username' field with 'admin'\n" +
			"STEP 2: Verify\n" +
			"    VERIFY that 'Dashboard' is present\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	// Commands inside STEP 1 should have StepBlock set
	for _, cmd := range h.Commands {
		if cmd.Type == CmdNavigate || cmd.Type == CmdFill {
			if !strings.HasPrefix(cmd.StepBlock, "STEP 1") {
				t.Errorf("cmd %q should be in STEP 1, got StepBlock=%q", cmd.Raw, cmd.StepBlock)
			}
		}
		if cmd.Type == CmdVerify {
			if !strings.HasPrefix(cmd.StepBlock, "STEP 2") {
				t.Errorf("cmd %q should be in STEP 2, got StepBlock=%q", cmd.Raw, cmd.StepBlock)
			}
		}
	}
}

// ── Section 8: Verify field parsing ──────────────────────────────────────────

func TestAdvanced_VerifyFieldHasText(t *testing.T) {
	cmd := parseOne(t, "Verify 'Save me' button has text 'Save me'")
	if cmd.Type != CmdVerifyField {
		t.Errorf("type=%s, want %s", cmd.Type, CmdVerifyField)
	}
	if cmd.VerifyFieldKind != "text" {
		t.Errorf("fieldKind=%q, want %q", cmd.VerifyFieldKind, "text")
	}
	if cmd.Target != "Save me" {
		t.Errorf("target=%q, want %q", cmd.Target, "Save me")
	}
	if cmd.Value != "Save me" {
		t.Errorf("value=%q, want %q", cmd.Value, "Save me")
	}
}

func TestAdvanced_VerifyFieldHasValue(t *testing.T) {
	cmd := parseOne(t, "Verify 'Profile Email' field has value 'captain@manul.com'")
	if cmd.Type != CmdVerifyField {
		t.Errorf("type=%s, want %s", cmd.Type, CmdVerifyField)
	}
	if cmd.VerifyFieldKind != "value" {
		t.Errorf("fieldKind=%q, want %q", cmd.VerifyFieldKind, "value")
	}
}

func TestAdvanced_VerifyFieldHasPlaceholder(t *testing.T) {
	cmd := parseOne(t, "Verify 'Login' field has placeholder 'Login/Email'")
	if cmd.Type != CmdVerifyField {
		t.Errorf("type=%s, want %s", cmd.Type, CmdVerifyField)
	}
	if cmd.VerifyFieldKind != "placeholder" {
		t.Errorf("fieldKind=%q, want %q", cmd.VerifyFieldKind, "placeholder")
	}
}

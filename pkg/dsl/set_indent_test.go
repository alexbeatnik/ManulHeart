package dsl

// ─────────────────────────────────────────────────────────────────────────────
// SET AND INDENT TEST SUITE
//
// Port of ManulEngine test_38_set_and_indent.py
//
// Validates:
// A. Indentation robustness — indented lines parse identically to flush ones
// B. SET {var} = value command parsing
// C. SET + variable coexistence with @var: declarations
// D. SET not confused with 'Settings'/'Reset' inside quoted labels
// E. Tab indentation handling
// ─────────────────────────────────────────────────────────────────────────────

import (
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Section A: Indentation robustness
// ═══════════════════════════════════════════════════════════════════════════════

func TestSetAndIndent_IndentedLinesParseCorrectly(t *testing.T) {
	src := `STEP 1: Login
    NAVIGATE to https://example.com/login
    Fill 'Username' field with 'admin'
    Fill 'Password' field with 'secret'
    Click the 'Login' button
    VERIFY that 'Welcome' is present

STEP 2: Extract data
    EXTRACT the 'Price' into {price}
    SET {discount} = 10 percent
    WAIT 2
    SCROLL DOWN
    PRESS ENTER
DONE.
`
	hunt := mustParse(t, src)

	// Count by type to ensure all indented lines parsed correctly
	counts := map[CommandType]int{}
	for _, cmd := range hunt.Commands {
		counts[cmd.Type]++
	}

	if counts[CmdNavigate] != 1 {
		t.Errorf("navigate = %d, want 1", counts[CmdNavigate])
	}
	if counts[CmdFill] != 2 {
		t.Errorf("fill = %d, want 2", counts[CmdFill])
	}
	if counts[CmdClick] != 1 {
		t.Errorf("click = %d, want 1", counts[CmdClick])
	}
	if counts[CmdVerify] != 1 {
		t.Errorf("verify = %d, want 1", counts[CmdVerify])
	}
	if counts[CmdExtract] != 1 {
		t.Errorf("extract = %d, want 1", counts[CmdExtract])
	}
	if counts[CmdSet] != 1 {
		t.Errorf("set = %d, want 1", counts[CmdSet])
	}
	if counts[CmdWait] != 1 {
		t.Errorf("wait = %d, want 1", counts[CmdWait])
	}
	if counts[CmdScroll] != 1 {
		t.Errorf("scroll = %d, want 1", counts[CmdScroll])
	}
	if counts[CmdPress] != 1 {
		t.Errorf("press = %d, want 1", counts[CmdPress])
	}
}

func TestSetAndIndent_TabsAndSpacesIdentical(t *testing.T) {
	// Tabs should produce the same parse results as spaces
	srcSpaces := `STEP 1: Test
    Fill 'Name' field with 'John'
    Click the 'Save' button
`
	srcTabs := "STEP 1: Test\n\tFill 'Name' field with 'John'\n\tClick the 'Save' button\n"

	huntSpaces := mustParse(t, srcSpaces)
	huntTabs := mustParse(t, srcTabs)

	if len(huntSpaces.Commands) != len(huntTabs.Commands) {
		t.Fatalf("spaces: %d commands, tabs: %d commands",
			len(huntSpaces.Commands), len(huntTabs.Commands))
	}
	for i, cmdS := range huntSpaces.Commands {
		cmdT := huntTabs.Commands[i]
		if cmdS.Type != cmdT.Type {
			t.Errorf("cmd[%d]: spaces=%s, tabs=%s", i, cmdS.Type, cmdT.Type)
		}
		if cmdS.Target != cmdT.Target {
			t.Errorf("cmd[%d]: spaces target=%q, tabs target=%q", i, cmdS.Target, cmdT.Target)
		}
		if cmdS.Value != cmdT.Value {
			t.Errorf("cmd[%d]: spaces value=%q, tabs value=%q", i, cmdS.Value, cmdT.Value)
		}
	}
}

func TestSetAndIndent_MixedIndentation(t *testing.T) {
	src := "STEP 1: Mixed\n    Fill 'Name' field with 'John'\n\t\tClick the 'Save' button\n  VERIFY that 'Saved' is present\n"
	hunt := mustParse(t, src)
	if len(hunt.Commands) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(hunt.Commands))
	}
	if hunt.Commands[0].Type != CmdFill {
		t.Errorf("cmd[0] = %s, want fill", hunt.Commands[0].Type)
	}
	if hunt.Commands[1].Type != CmdClick {
		t.Errorf("cmd[1] = %s, want click", hunt.Commands[1].Type)
	}
	if hunt.Commands[2].Type != CmdVerify {
		t.Errorf("cmd[2] = %s, want verify", hunt.Commands[2].Type)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section B: SET command — recognition and parsing
// ═══════════════════════════════════════════════════════════════════════════════

func TestSetAndIndent_SetWithBracesAndQuotedValue(t *testing.T) {
	cmd := mustParseLine(t, "SET {user_email} = 'admin@test.com'")
	if cmd.Type != CmdSet {
		t.Errorf("type = %s, want %s", cmd.Type, CmdSet)
	}
	if cmd.SetVar != "user_email" {
		t.Errorf("var = %q, want 'user_email'", cmd.SetVar)
	}
	if cmd.SetValue != "admin@test.com" {
		t.Errorf("value = %q, want 'admin@test.com'", cmd.SetValue)
	}
}

func TestSetAndIndent_SetBareKeyAndUnquotedValue(t *testing.T) {
	cmd := mustParseLine(t, "SET token = abc123")
	if cmd.Type != CmdSet {
		t.Errorf("type = %s, want %s", cmd.Type, CmdSet)
	}
	if cmd.SetVar != "token" {
		t.Errorf("var = %q, want 'token'", cmd.SetVar)
	}
	if cmd.SetValue != "abc123" {
		t.Errorf("value = %q, want 'abc123'", cmd.SetValue)
	}
}

func TestSetAndIndent_SetWithDoubleQuotedValue(t *testing.T) {
	cmd := mustParseLine(t, `SET {greeting} = "Hello World"`)
	if cmd.Type != CmdSet {
		t.Errorf("type = %s, want %s", cmd.Type, CmdSet)
	}
	if cmd.SetVar != "greeting" {
		t.Errorf("var = %q, want 'greeting'", cmd.SetVar)
	}
	if cmd.SetValue != "Hello World" {
		t.Errorf("value = %q, want 'Hello World'", cmd.SetValue)
	}
}

func TestSetAndIndent_SetWithBracesUnquotedValue(t *testing.T) {
	cmd := mustParseLine(t, "SET {token} = abc123")
	if cmd.Type != CmdSet {
		t.Errorf("type = %s, want %s", cmd.Type, CmdSet)
	}
	if cmd.SetVar != "token" {
		t.Errorf("var = %q", cmd.SetVar)
	}
	if cmd.SetValue != "abc123" {
		t.Errorf("value = %q", cmd.SetValue)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section C: SET must NOT match words inside quoted labels
// ═══════════════════════════════════════════════════════════════════════════════

func TestSetAndIndent_SettingsNotConfused(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Settings' button")
	if cmd.Type == CmdSet {
		t.Error("'Settings' inside quotes should NOT be classified as SET")
	}
	if cmd.Type != CmdClick {
		t.Errorf("type = %s, want click", cmd.Type)
	}
}

func TestSetAndIndent_ResetNotConfused(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Reset' button")
	if cmd.Type == CmdSet {
		t.Error("'Reset' inside quotes should NOT be classified as SET")
	}
	if cmd.Type != CmdClick {
		t.Errorf("type = %s, want click", cmd.Type)
	}
}

func TestSetAndIndent_OffsetNotConfused(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Offset' button")
	if cmd.Type == CmdSet {
		t.Error("'Offset' inside quotes should NOT be classified as SET")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section D: SET + @var: coexistence
// ═══════════════════════════════════════════════════════════════════════════════

func TestSetAndIndent_VarAndSetCoexist(t *testing.T) {
	src := `@var: {base_url} = https://example.com
@var: {email} = admin@test.com

STEP 1: Setup
    SET {token} = abc123
    NAVIGATE to {base_url}
    Fill 'Email' field with '{email}'
DONE.
`
	hunt := mustParse(t, src)

	// @var: declarations parsed
	if hunt.Vars["base_url"] != "https://example.com" {
		t.Errorf("var base_url = %q", hunt.Vars["base_url"])
	}
	if hunt.Vars["email"] != "admin@test.com" {
		t.Errorf("var email = %q", hunt.Vars["email"])
	}

	// SET command should be present
	hasSET := false
	for _, cmd := range hunt.Commands {
		if cmd.Type == CmdSet {
			hasSET = true
			if cmd.SetVar != "token" {
				t.Errorf("SET var = %q, want 'token'", cmd.SetVar)
			}
			if cmd.SetValue != "abc123" {
				t.Errorf("SET value = %q, want 'abc123'", cmd.SetValue)
			}
		}
	}
	if !hasSET {
		t.Error("SET command not found in parsed commands")
	}

	// @var substitution should be applied
	for _, cmd := range hunt.Commands {
		if cmd.Type == CmdNavigate {
			if cmd.URL != "https://example.com" {
				t.Errorf("NAVIGATE URL = %q, want substituted value", cmd.URL)
			}
		}
		if cmd.Type == CmdFill && cmd.Target == "Email" {
			if cmd.Value != "admin@test.com" {
				t.Errorf("Fill Email value = %q, want substituted value", cmd.Value)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section E: Indented hunt file with all header types
// ═══════════════════════════════════════════════════════════════════════════════

func TestSetAndIndent_FullIndentedHuntFile(t *testing.T) {
	src := `@context: Indentation test
@title: indent-test
@var: {email} = admin@test.com
@tags: smoke, regression

STEP 1: Login
    NAVIGATE to https://example.com
    Fill 'Email' field with '{email}'
    Click the 'Submit' button
    VERIFY that 'Success' is present
DONE.
`
	hunt := mustParse(t, src)

	if hunt.Context != "Indentation test" {
		t.Errorf("context = %q", hunt.Context)
	}
	if hunt.Title != "indent-test" {
		t.Errorf("title = %q", hunt.Title)
	}
	if hunt.Vars["email"] != "admin@test.com" {
		t.Errorf("var email = %q", hunt.Vars["email"])
	}
	if len(hunt.Tags) != 2 {
		t.Errorf("tags count = %d, want 2", len(hunt.Tags))
	}

	hasNavigate := false
	hasFill := false
	hasClick := false
	hasVerify := false
	for _, cmd := range hunt.Commands {
		switch cmd.Type {
		case CmdNavigate:
			hasNavigate = true
		case CmdFill:
			hasFill = true
			if cmd.Value != "admin@test.com" {
				t.Errorf("Fill value = %q, want substituted email", cmd.Value)
			}
		case CmdClick:
			hasClick = true
		case CmdVerify:
			hasVerify = true
		}
	}
	if !hasNavigate {
		t.Error("NAVIGATE not parsed from indented file")
	}
	if !hasFill {
		t.Error("Fill not parsed from indented file")
	}
	if !hasClick {
		t.Error("Click not parsed from indented file")
	}
	if !hasVerify {
		t.Error("VERIFY not parsed from indented file")
	}
}

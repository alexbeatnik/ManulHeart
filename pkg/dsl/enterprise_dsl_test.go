package dsl

// ─────────────────────────────────────────────────────────────────────────────
// ENTERPRISE DSL TEST SUITE
//
// Port of ManulEngine test_37_enterprise_dsl.py
//
// Validates:
// A. Command type classification for new DSL keywords
// B. Strict VERIFY forms (text, value, placeholder)
// C. Data-driven testing (@data: parsing)
// D. Report-compatible structures
// E. @import/@export/@schedule header parsing
// F. Backward compatibility of Hunt structure
// ─────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Section A: Command type classification — new DSL keywords
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_VerifySoftlyClassified(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY SOFTLY that 'Welcome' is present")
	if cmd.Type != CmdVerifySoft {
		t.Errorf("VERIFY SOFTLY should be %s, got %s", CmdVerifySoft, cmd.Type)
	}
	if cmd.VerifyText != "Welcome" {
		t.Errorf("verify text = %q, want 'Welcome'", cmd.VerifyText)
	}
}

func TestEnterpriseDSL_VerifySoftlyNegated(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY SOFTLY that 'Error' is NOT present")
	if cmd.Type != CmdVerifySoft {
		t.Errorf("type = %s, want %s", cmd.Type, CmdVerifySoft)
	}
	if !cmd.VerifyNegated {
		t.Error("should be negated")
	}
}

func TestEnterpriseDSL_WaitForResponseClassified(t *testing.T) {
	cmd := mustParseLine(t, `WAIT FOR RESPONSE "/api/users"`)
	if cmd.Type != CmdWaitForResponse {
		t.Errorf("type = %s, want %s", cmd.Type, CmdWaitForResponse)
	}
	if cmd.WaitResponseURL != "/api/users" {
		t.Errorf("response URL = %q, want '/api/users'", cmd.WaitResponseURL)
	}
}

func TestEnterpriseDSL_WaitForElementClassified(t *testing.T) {
	tests := []struct {
		line  string
		state string
	}{
		{"Wait for 'Welcome, User' to be visible", "visible"},
		{"Wait for 'Loading...' to be hidden", "hidden"},
	}
	for _, tc := range tests {
		cmd := mustParseLine(t, tc.line)
		if cmd.Type != CmdWaitFor {
			t.Errorf("%q: type = %s, want %s", tc.line, cmd.Type, CmdWaitFor)
		}
		if cmd.WaitForState != tc.state {
			t.Errorf("%q: state = %q, want %q", tc.line, cmd.WaitForState, tc.state)
		}
	}
}

func TestEnterpriseDSL_PlainWaitStillWorks(t *testing.T) {
	cmd := mustParseLine(t, "WAIT 3")
	if cmd.Type != CmdWait {
		t.Errorf("type = %s, want %s", cmd.Type, CmdWait)
	}
	if cmd.WaitSeconds != 3 {
		t.Errorf("wait seconds = %f, want 3", cmd.WaitSeconds)
	}
}

func TestEnterpriseDSL_PlainVerifyStillWorks(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY that 'Welcome' is present")
	if cmd.Type != CmdVerify {
		t.Errorf("type = %s, want %s", cmd.Type, CmdVerify)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section B: Strict VERIFY forms — text, placeholder, value
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_StrictVerifyText(t *testing.T) {
	cmd := mustParseLine(t, "Verify 'Save me' button has text 'Save me'")
	if cmd.Type != CmdVerifyField {
		t.Errorf("type = %s, want %s", cmd.Type, CmdVerifyField)
	}
	if cmd.VerifyFieldKind != "text" {
		t.Errorf("verify field kind = %q, want 'text'", cmd.VerifyFieldKind)
	}
	if cmd.Target != "Save me" {
		t.Errorf("target = %q, want 'Save me'", cmd.Target)
	}
	if cmd.Value != "Save me" {
		t.Errorf("value = %q, want 'Save me'", cmd.Value)
	}
}

func TestEnterpriseDSL_StrictVerifyPlaceholder(t *testing.T) {
	cmd := mustParseLine(t, `Verify 'Login' field has placeholder 'Login/Email'`)
	if cmd.Type != CmdVerifyField {
		t.Errorf("type = %s, want %s", cmd.Type, CmdVerifyField)
	}
	if cmd.VerifyFieldKind != "placeholder" {
		t.Errorf("verify field kind = %q, want 'placeholder'", cmd.VerifyFieldKind)
	}
	if cmd.Target != "Login" {
		t.Errorf("target = %q, want 'Login'", cmd.Target)
	}
	if cmd.Value != "Login/Email" {
		t.Errorf("value = %q, want 'Login/Email'", cmd.Value)
	}
}

func TestEnterpriseDSL_StrictVerifyValue(t *testing.T) {
	cmd := mustParseLine(t, `Verify 'Profile Email' field has value 'captain@manul.com'`)
	if cmd.Type != CmdVerifyField {
		t.Errorf("type = %s, want %s", cmd.Type, CmdVerifyField)
	}
	if cmd.VerifyFieldKind != "value" {
		t.Errorf("verify field kind = %q, want 'value'", cmd.VerifyFieldKind)
	}
	if cmd.Target != "Profile Email" {
		t.Errorf("target = %q, want 'Profile Email'", cmd.Target)
	}
	if cmd.Value != "captain@manul.com" {
		t.Errorf("value = %q, want 'captain@manul.com'", cmd.Value)
	}
}

func TestEnterpriseDSL_LegacyVerifyNotStrictAssert(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY that 'Welcome' is present")
	if cmd.Type == CmdVerifyField {
		t.Error("legacy VERIFY should NOT be classified as verify_field")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section C: Data-driven testing (@data: parsing)
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_DataHeaderParsing(t *testing.T) {
	src := `@context: Test data-driven
@title: DDT Test
@data: test_data.json
@var: {base_url} = https://example.com

STEP 1: Login
    NAVIGATE to https://example.com
    Fill 'Email' field with '{email}'
DONE.
`
	hunt := mustParse(t, src)
	if hunt.DataFile != "test_data.json" {
		t.Errorf("data file = %q, want 'test_data.json'", hunt.DataFile)
	}
	if hunt.Context != "Test data-driven" {
		t.Errorf("context = %q", hunt.Context)
	}
	if hunt.Title != "DDT Test" {
		t.Errorf("title = %q", hunt.Title)
	}
	if hunt.Vars["base_url"] != "https://example.com" {
		t.Errorf("var base_url = %q", hunt.Vars["base_url"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section D: @schedule: header parsing
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_ScheduleHeaderParsing(t *testing.T) {
	src := `@context: Scheduled test
@title: scheduled
@schedule: every 1h
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if hunt.Schedule != "every 1h" {
		t.Errorf("schedule = %q, want 'every 1h'", hunt.Schedule)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section E: @export: header parsing
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_ExportHeaderParsing(t *testing.T) {
	src := `@context: Export test
@title: export
@export: Login, Setup
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if len(hunt.Exports) != 2 {
		t.Fatalf("expected 2 exports, got %d", len(hunt.Exports))
	}
	if hunt.Exports[0] != "Login" {
		t.Errorf("export[0] = %q", hunt.Exports[0])
	}
	if hunt.Exports[1] != "Setup" {
		t.Errorf("export[1] = %q", hunt.Exports[1])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section F: @import: header parsing
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_ImportHeaderParsing(t *testing.T) {
	src := `@import: Login from lib/auth.hunt
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if len(hunt.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(hunt.Imports))
	}
	imp := hunt.Imports[0]
	if imp.Source != "lib/auth.hunt" {
		t.Errorf("import source = %q", imp.Source)
	}
	if len(imp.Names) != 1 || imp.Names[0] != "Login" {
		t.Errorf("import names = %v", imp.Names)
	}
}

func TestEnterpriseDSL_ImportMultipleBlocks(t *testing.T) {
	src := `@import: Login, Logout from lib/auth.hunt
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if len(hunt.Imports) != 1 {
		t.Fatalf("expected 1 import directive, got %d", len(hunt.Imports))
	}
	imp := hunt.Imports[0]
	if len(imp.Names) != 2 {
		t.Fatalf("expected 2 import names, got %d", len(imp.Names))
	}
	if imp.Names[0] != "Login" || imp.Names[1] != "Logout" {
		t.Errorf("import names = %v", imp.Names)
	}
}

func TestEnterpriseDSL_ImportWithAlias(t *testing.T) {
	src := `@import: Login as AuthLogin from lib/auth.hunt
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if len(hunt.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(hunt.Imports))
	}
	imp := hunt.Imports[0]
	if len(imp.Names) != 1 || imp.Names[0] != "Login" {
		t.Errorf("import names = %v", imp.Names)
	}
	if imp.Aliases["Login"] != "AuthLogin" {
		t.Errorf("alias for Login = %q, want 'AuthLogin'", imp.Aliases["Login"])
	}
}

func TestEnterpriseDSL_ImportWildcard(t *testing.T) {
	src := `@import: * from lib/common.hunt
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if len(hunt.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(hunt.Imports))
	}
	if hunt.Imports[0].Names[0] != "*" {
		t.Errorf("import names = %v, want ['*']", hunt.Imports[0].Names)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section G: Hunt structure backward compatibility
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_HuntStructureFields(t *testing.T) {
	src := `@context: Full structure test
@title: full_test
@tags: smoke, regression
@data: data.json
@schedule: every 1h
@export: Login, Checkout
@var: {base_url} = https://example.com
@import: Auth from lib/auth.hunt

STEP 1: Navigate
    NAVIGATE to https://example.com

STEP 2: Fill form
    Fill 'Email' field with 'test@test.com'
    VERIFY that 'Success' is present

DONE.
`
	hunt := mustParse(t, src)

	// Verify all header fields populated
	if hunt.Context != "Full structure test" {
		t.Errorf("context = %q", hunt.Context)
	}
	if hunt.Title != "full_test" {
		t.Errorf("title = %q", hunt.Title)
	}
	if len(hunt.Tags) != 2 {
		t.Errorf("tags count = %d", len(hunt.Tags))
	}
	if hunt.DataFile != "data.json" {
		t.Errorf("data = %q", hunt.DataFile)
	}
	if hunt.Schedule != "every 1h" {
		t.Errorf("schedule = %q", hunt.Schedule)
	}
	if len(hunt.Exports) != 2 {
		t.Errorf("exports count = %d", len(hunt.Exports))
	}
	if hunt.Vars["base_url"] != "https://example.com" {
		t.Errorf("var base_url = %q", hunt.Vars["base_url"])
	}
	if len(hunt.Imports) != 1 {
		t.Errorf("imports count = %d", len(hunt.Imports))
	}

	// Commands should be present
	if len(hunt.Commands) < 3 {
		t.Errorf("expected ≥3 commands, got %d", len(hunt.Commands))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section H: Keywords inside quotes should not trigger classification
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_QuotedKeywordsNotMisclassified(t *testing.T) {
	tests := []struct {
		line     string
		wantType CommandType
	}{
		{"Click the 'VERIFY' button", CmdClick},
		{"Click the 'WAIT' button", CmdClick},
		{"Fill 'SET' field with 'hello'", CmdFill},
	}
	for _, tc := range tests {
		cmd := mustParseLine(t, tc.line)
		if cmd.Type != tc.wantType {
			t.Errorf("%q: type = %s, want %s", tc.line, cmd.Type, tc.wantType)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section I: Verify checked/enabled/disabled variants
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_VerifyChecked(t *testing.T) {
	tests := []struct {
		line    string
		state   string
		negated bool
	}{
		{"VERIFY that 'Monday' is checked", "checked", false},
		{"VERIFY that 'Accept' is NOT checked", "checked", true},
		{"VERIFY that 'Submit' is ENABLED", "enabled", false},
		{"VERIFY that 'Submit' is DISABLED", "disabled", false},
	}
	for _, tc := range tests {
		cmd := mustParseLine(t, tc.line)
		if cmd.Type != CmdVerifyField {
			t.Errorf("%q: type = %s, want %s", tc.line, cmd.Type, CmdVerifyField)
		}
		if cmd.VerifyState != tc.state {
			t.Errorf("%q: VerifyState = %q, want %q", tc.line, cmd.VerifyState, tc.state)
		}
		if cmd.VerifyNegated != tc.negated {
			t.Errorf("%q: VerifyNegated = %t, want %t", tc.line, cmd.VerifyNegated, tc.negated)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section J: NAVIGATE with variable substitution
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_NavigateWithVarSubstitution(t *testing.T) {
	src := `@var: {base_url} = https://example.com
NAVIGATE to {base_url}/login
`
	hunt := mustParse(t, src)
	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(hunt.Commands))
	}
	if hunt.Commands[0].URL != "https://example.com/login" {
		t.Errorf("URL = %q, want 'https://example.com/login'", hunt.Commands[0].URL)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section K: Multiple @var: declarations
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_MultipleVarDeclarations(t *testing.T) {
	src := `@var: {email} = admin@test.com
@var: {password} = secret123
@var: {base} = https://example.com
NAVIGATE to {base}
Fill 'Email' field with '{email}'
Fill 'Password' field with '{password}'
`
	hunt := mustParse(t, src)
	if hunt.Vars["email"] != "admin@test.com" {
		t.Errorf("email = %q", hunt.Vars["email"])
	}
	if hunt.Vars["password"] != "secret123" {
		t.Errorf("password = %q", hunt.Vars["password"])
	}
	if hunt.Vars["base"] != "https://example.com" {
		t.Errorf("base = %q", hunt.Vars["base"])
	}

	// Verify substitution in commands
	if len(hunt.Commands) < 3 {
		t.Fatalf("expected ≥3 commands, got %d", len(hunt.Commands))
	}
	if hunt.Commands[0].URL != "https://example.com" {
		t.Errorf("NAVIGATE URL = %q", hunt.Commands[0].URL)
	}
	if hunt.Commands[1].Value != "admin@test.com" {
		t.Errorf("Fill Email value = %q", hunt.Commands[1].Value)
	}
	if hunt.Commands[2].Value != "secret123" {
		t.Errorf("Fill Password value = %q", hunt.Commands[2].Value)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section L: Complex hunt file round-trip
// ═══════════════════════════════════════════════════════════════════════════════

func TestEnterpriseDSL_ComplexHuntRoundTrip(t *testing.T) {
	src := `@context: E-commerce flow
@title: checkout_test
@tags: smoke, checkout, critical
@data: checkout_data.csv
@schedule: every 30m
@export: Login, AddToCart
@var: {base} = https://shop.example.com
@import: Auth from lib/auth.hunt

STEP 1: Setup
    NAVIGATE to {base}
    VERIFY that 'Welcome' is present

STEP 2: Login
    Fill 'Email' field with 'user@test.com'
    Fill 'Password' field with 'pass123'
    Click the 'Login' button
    VERIFY that 'Dashboard' is present

STEP 3: Add to cart
    Click the 'Add to Cart' button NEAR 'Product Name'
    VERIFY SOFTLY that 'Added' is present
    SCROLL DOWN

STEP 4: Checkout
    Click the 'Checkout' button
    Select 'Express' from the 'Shipping' dropdown
    Fill 'Card Number' field with '4242424242424242'
    Click the 'Pay Now' button
    VERIFY that 'Order Confirmed' is present

DONE.
`
	hunt := mustParse(t, src)

	// Headers
	if hunt.Context != "E-commerce flow" {
		t.Errorf("context = %q", hunt.Context)
	}
	if len(hunt.Tags) != 3 {
		t.Errorf("tags = %v", hunt.Tags)
	}
	if hunt.DataFile != "checkout_data.csv" {
		t.Errorf("data = %q", hunt.DataFile)
	}
	if hunt.Schedule != "every 30m" {
		t.Errorf("schedule = %q", hunt.Schedule)
	}
	if len(hunt.Exports) != 2 {
		t.Errorf("exports = %v", hunt.Exports)
	}
	if len(hunt.Imports) != 1 {
		t.Errorf("imports = %d", len(hunt.Imports))
	}

	// Count command types
	counts := map[CommandType]int{}
	for _, c := range hunt.Commands {
		counts[c.Type]++
	}

	if counts[CmdNavigate] != 1 {
		t.Errorf("navigate count = %d", counts[CmdNavigate])
	}
	if counts[CmdFill] != 3 {
		t.Errorf("fill count = %d, want 3", counts[CmdFill])
	}
	if counts[CmdClick] != 4 {
		t.Errorf("click count = %d, want 4", counts[CmdClick])
	}
	if counts[CmdVerify] != 3 {
		t.Errorf("verify count = %d, want 3", counts[CmdVerify])
	}
	if counts[CmdVerifySoft] != 1 {
		t.Errorf("verify_soft count = %d, want 1", counts[CmdVerifySoft])
	}
	if counts[CmdSelect] != 1 {
		t.Errorf("select count = %d, want 1", counts[CmdSelect])
	}
	if counts[CmdScroll] != 1 {
		t.Errorf("scroll count = %d, want 1", counts[CmdScroll])
	}

	// Check NEAR qualifier was parsed
	found := false
	for _, c := range hunt.Commands {
		if c.NearAnchor != "" {
			found = true
			if !strings.Contains(c.NearAnchor, "Product Name") {
				t.Errorf("near anchor = %q", c.NearAnchor)
			}
		}
	}
	if !found {
		t.Error("no command with NEAR qualifier found")
	}
}

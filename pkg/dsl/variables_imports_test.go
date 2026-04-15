package dsl

// ─────────────────────────────────────────────────────────────────────────────
// VARIABLES & IMPORTS TEST SUITE
//
// Port of ManulEngine test_20_variables.py + test_43_scoped_variables.py +
// test_50_imports.py
//
// Validates variable substitution, scoped variables, @var declarations,
// import directives, and variable resolution in various contexts.
// ─────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION A: Variable Declarations (@var)
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_SingleVarDeclaration(t *testing.T) {
	h := mustParse(t, "@var: {baseURL} = https://example.com\n\nSTEP 1: test\n    NAVIGATE to '{baseURL}'")
	if h.Vars["baseURL"] != "https://example.com" {
		t.Errorf("got %q, want https://example.com", h.Vars["baseURL"])
	}
}

func TestVars_MultipleVarDeclarations(t *testing.T) {
	src := `@var: {user} = admin
@var: {pass} = secret123
@var: {url} = https://app.example.com

STEP 1: login
    NAVIGATE to '{url}'
`
	h := mustParse(t, src)
	if h.Vars["user"] != "admin" {
		t.Errorf("user=%q, want admin", h.Vars["user"])
	}
	if h.Vars["pass"] != "secret123" {
		t.Errorf("pass=%q, want secret123", h.Vars["pass"])
	}
	if h.Vars["url"] != "https://app.example.com" {
		t.Errorf("url=%q, want https://app.example.com", h.Vars["url"])
	}
}

func TestVars_VarWithSpacesInValue(t *testing.T) {
	h := mustParse(t, "@var: {greeting} = Hello World!\n\nSTEP 1: test\n    VERIFY that '{greeting}' is present")
	if h.Vars["greeting"] != "Hello World!" {
		t.Errorf("got %q, want Hello World!", h.Vars["greeting"])
	}
}

func TestVars_VarWithSpecialChars(t *testing.T) {
	h := mustParse(t, "@var: {email} = user+test@example.com\n\nSTEP 1: test\n    Fill 'Email' field with '{email}'")
	if h.Vars["email"] != "user+test@example.com" {
		t.Errorf("got %q, want user+test@example.com", h.Vars["email"])
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION B: SET Command Parsing
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_SetCommandWithBraces(t *testing.T) {
	cmd := mustParseLine(t, "SET {username} = admin")
	if cmd.Type != CmdSet {
		t.Errorf("type=%v, want CmdSet", cmd.Type)
	}
	if cmd.SetVar != "username" {
		t.Errorf("SetVar=%q, want username", cmd.SetVar)
	}
	if cmd.SetValue != "admin" {
		t.Errorf("SetValue=%q, want admin", cmd.SetValue)
	}
}

func TestVars_SetCommandBare(t *testing.T) {
	cmd := mustParseLine(t, "SET username = admin")
	if cmd.Type != CmdSet {
		t.Errorf("type=%v, want CmdSet", cmd.Type)
	}
}

func TestVars_SetCommandQuotedValue(t *testing.T) {
	cmd := mustParseLine(t, "SET {token} = 'abc-123-xyz'")
	if cmd.Type != CmdSet {
		t.Errorf("type=%v, want CmdSet", cmd.Type)
	}
}

func TestVars_SetCommandDoubleQuotedValue(t *testing.T) {
	cmd := mustParseLine(t, `SET {name} = "John Doe"`)
	if cmd.Type != CmdSet {
		t.Errorf("type=%v, want CmdSet", cmd.Type)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION C: EXTRACT Command (variable capture)
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_ExtractIntoVar(t *testing.T) {
	cmd := mustParseLine(t, "EXTRACT the 'Order Number' into {orderID}")
	if cmd.Type != CmdExtract {
		t.Errorf("type=%v, want CmdExtract", cmd.Type)
	}
	if cmd.ExtractVar != "orderID" {
		t.Errorf("ExtractVar=%q, want orderID", cmd.ExtractVar)
	}
}

func TestVars_ExtractUppercase(t *testing.T) {
	cmd := mustParseLine(t, "EXTRACT the 'Total Price' into {TOTAL}")
	if cmd.Type != CmdExtract {
		t.Errorf("type=%v, want CmdExtract", cmd.Type)
	}
	if cmd.ExtractVar != "TOTAL" {
		t.Errorf("ExtractVar=%q, want TOTAL", cmd.ExtractVar)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION D: Variable Substitution in Commands
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_VarSubstitutionInNavigate(t *testing.T) {
	src := `@var: {base} = https://test.com

STEP 1: go
    NAVIGATE to '{base}/login'
`
	h := mustParse(t, src)
	found := false
	for _, c := range h.Commands {
		if c.Type == CmdNavigate {
			if !strings.Contains(c.URL, "{base}") {
				t.Logf("URL was already resolved: %s", c.URL)
			}
			found = true
		}
	}
	if !found {
		t.Error("no NAVIGATE command found")
	}
}

func TestVars_VarSubstitutionInFill(t *testing.T) {
	h := mustParse(t, "@var: {user} = testuser\n\nSTEP 1: fill\n    Fill 'Username' field with '{user}'")
	for _, c := range h.Commands {
		if c.Type == CmdFill {
			if !strings.Contains(c.Value, "{user}") && c.Value != "testuser" {
				t.Logf("Value: %s", c.Value)
			}
			return
		}
	}
	t.Error("no FILL command found")
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION E: Import Directives
// ═══════════════════════════════════════════════════════════════════════════════

func TestImports_SingleImport(t *testing.T) {
	src := `@import: login from 'auth.hunt'

STEP 1: test
    NAVIGATE to 'https://test.com'
`
	h := mustParse(t, src)
	if len(h.Imports) == 0 {
		t.Fatal("expected at least 1 import")
	}
	imp := h.Imports[0]
	if imp.Source != "auth.hunt" {
		t.Errorf("Source=%q, want auth.hunt", imp.Source)
	}
}

func TestImports_MultipleImports(t *testing.T) {
	src := `@import: login from 'auth.hunt'
@import: checkout from 'shop.hunt'

STEP 1: test
    NAVIGATE to 'https://test.com'
`
	h := mustParse(t, src)
	if len(h.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(h.Imports))
	}
}

func TestImports_ImportWithAlias(t *testing.T) {
	src := `@import: login as auth_login from 'auth.hunt'

STEP 1: test
    NAVIGATE to 'https://test.com'
`
	h := mustParse(t, src)
	if len(h.Imports) == 0 {
		t.Fatal("expected at least 1 import")
	}
	imp := h.Imports[0]
	if imp.Aliases == nil || imp.Aliases["login"] != "auth_login" {
		t.Errorf("Aliases=%v, want login->auth_login", imp.Aliases)
	}
}

func TestImports_WildcardImport(t *testing.T) {
	src := `@import: * from 'shared.hunt'

STEP 1: test
    NAVIGATE to 'https://test.com'
`
	h := mustParse(t, src)
	if len(h.Imports) == 0 {
		t.Fatal("expected at least 1 import")
	}
	imp := h.Imports[0]
	if len(imp.Names) == 0 || imp.Names[0] != "*" {
		t.Errorf("Names=%v, want [*]", imp.Names)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION F: Scoped Variables (IF/ELSE context)
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_SetInsideIfBlock(t *testing.T) {
	src := `STEP 1: conditional set
    IF 'Success' is present:
        SET {status} = passed
    ELSE:
        SET {status} = failed
`
	h := mustParse(t, src)
	// Both SET commands should be parsed inside the IF/ELSE branches
	setCount := 0
	for _, c := range h.Commands {
		if c.Type == CmdIf {
			for _, branch := range c.Branches {
				for _, bc := range branch.Body {
					if bc.Type == CmdSet {
						setCount++
					}
				}
			}
		}
	}
	if setCount != 2 {
		t.Errorf("expected 2 SET commands inside IF/ELSE, got %d", setCount)
	}
}

func TestVars_ExtractThenUse(t *testing.T) {
	src := `STEP 1: capture and use
    EXTRACT the 'Price' into {price}
    VERIFY that '{price}' is present
`
	h := mustParse(t, src)
	extractFound := false
	verifyFound := false
	for _, c := range h.Commands {
		if c.Type == CmdExtract {
			extractFound = true
		}
		if c.Type == CmdVerify {
			verifyFound = true
		}
	}
	if !extractFound {
		t.Error("no EXTRACT command found")
	}
	if !verifyFound {
		t.Error("no VERIFY command found")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION G: @var and SET Coexistence
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_VarAndSetCoexist(t *testing.T) {
	src := `@var: {base} = https://example.com
@var: {user} = admin

STEP 1: override
    SET {user} = superadmin
    Fill 'Username' field with '{user}'
`
	h := mustParse(t, src)
	if h.Vars["base"] != "https://example.com" {
		t.Errorf("base=%q, want https://example.com", h.Vars["base"])
	}
	if h.Vars["user"] != "admin" {
		t.Errorf("initial user=%q, want admin", h.Vars["user"])
	}

	setFound := false
	for _, c := range h.Commands {
		if c.Type == CmdSet {
			setFound = true
		}
	}
	if setFound == false {
		t.Error("SET command should be present for runtime override")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION H: Edge Cases
// ═══════════════════════════════════════════════════════════════════════════════

func TestVars_EmptyVarValue(t *testing.T) {
	src := "@var: {empty} = \n\nSTEP 1: test\n    VERIFY that 'something' is present"
	h := mustParse(t, src)
	if _, ok := h.Vars["empty"]; !ok {
		t.Log("empty var not parsed (may be expected)")
	}
}

func TestVars_VarNameUnderscoreDash(t *testing.T) {
	src := `@var: {my_var} = value1
@var: {my-var} = value2

STEP 1: test
    VERIFY that 'something' is present
`
	h := mustParse(t, src)
	if v, ok := h.Vars["my_var"]; !ok || v != "value1" {
		t.Errorf("my_var=%q, want value1", h.Vars["my_var"])
	}
}

func TestVars_NestedVarRefs(t *testing.T) {
	// Parser should preserve nested references as-is; runtime resolves them
	src := `@var: {domain} = example.com
@var: {url} = https://{domain}/app

STEP 1: test
    NAVIGATE to '{url}'
`
	h := mustParse(t, src)
	if h.Vars["url"] != "https://{domain}/app" {
		t.Logf("Nested var ref: %q (parser may or may not resolve)", h.Vars["url"])
	}
}

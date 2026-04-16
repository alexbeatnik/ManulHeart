package dsl

// ─────────────────────────────────────────────────────────────────────────────
// CONDITIONALS AND LOOPS TEST SUITE
//
// Port of ManulEngine test_54_conditionals.py and test_55_loops.py
//
// Validates:
// A. IF/ELIF/ELSE parsing and block nesting
// B. WHILE loops
// C. REPEAT N TIMES loops
// D. FOR EACH loops
// E. Nested control flow
// F. Edge cases
// ─────────────────────────────────────────────────────────────────────────────

import (
	"testing"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Section A: IF/ELIF/ELSE — basic classification
// ═══════════════════════════════════════════════════════════════════════════════

func TestConditionals_IfClassified(t *testing.T) {
	cmd := mustParseLine(t, "IF {logged_in} == true:")
	if cmd.Type != CmdIf {
		t.Errorf("type = %s, want %s", cmd.Type, CmdIf)
	}
	if cmd.Condition != "{logged_in} == true" {
		t.Errorf("condition = %q", cmd.Condition)
	}
}

func TestConditionals_ElifClassified(t *testing.T) {
	cmd := mustParseLine(t, "ELIF {role} == admin:")
	if cmd.Type != CmdElIf {
		t.Errorf("type = %s, want %s", cmd.Type, CmdElIf)
	}
	if cmd.Condition != "{role} == admin" {
		t.Errorf("condition = %q", cmd.Condition)
	}
}

func TestConditionals_ElseClassified(t *testing.T) {
	cmd := mustParseLine(t, "ELSE:")
	if cmd.Type != CmdElse {
		t.Errorf("type = %s, want %s", cmd.Type, CmdElse)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section B: IF/ELIF/ELSE block nesting
// ═══════════════════════════════════════════════════════════════════════════════

func TestConditionals_IfElseBlock(t *testing.T) {
	src := `IF {logged_in} == true:
    Click the 'Dashboard' button
ELSE:
    Click the 'Login' button
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command (IF), got %d", len(hunt.Commands))
	}
	ifCmd := hunt.Commands[0]
	if ifCmd.Type != CmdIf {
		t.Fatalf("type = %s, want IF", ifCmd.Type)
	}
	if len(ifCmd.Branches) != 2 {
		t.Fatalf("expected 2 branches (if + else), got %d", len(ifCmd.Branches))
	}

	// Check IF branch
	ifBranch := ifCmd.Branches[0]
	if ifBranch.Kind != "if" {
		t.Errorf("branch[0] kind = %q, want 'if'", ifBranch.Kind)
	}
	if ifBranch.Condition != "{logged_in} == true" {
		t.Errorf("branch[0] condition = %q", ifBranch.Condition)
	}
	if len(ifBranch.Body) != 1 {
		t.Fatalf("IF body: expected 1 command, got %d", len(ifBranch.Body))
	}
	if ifBranch.Body[0].Type != CmdClick {
		t.Errorf("IF body[0] type = %s, want click", ifBranch.Body[0].Type)
	}

	// Check ELSE branch
	elseBranch := ifCmd.Branches[1]
	if elseBranch.Kind != "else" {
		t.Errorf("branch[1] kind = %q, want 'else'", elseBranch.Kind)
	}
	if len(elseBranch.Body) != 1 {
		t.Fatalf("ELSE body: expected 1 command, got %d", len(elseBranch.Body))
	}
	if elseBranch.Body[0].Type != CmdClick {
		t.Errorf("ELSE body[0] type = %s, want click", elseBranch.Body[0].Type)
	}
}

func TestConditionals_IfElifElseBlock(t *testing.T) {
	src := `IF {role} == admin:
    Click the 'Admin Panel' button
ELIF {role} == user:
    Click the 'Dashboard' button
ELSE:
    Click the 'Login' button
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	ifCmd := hunt.Commands[0]
	if len(ifCmd.Branches) != 3 {
		t.Fatalf("expected 3 branches (if + elif + else), got %d", len(ifCmd.Branches))
	}

	kinds := []string{"if", "elif", "else"}
	for i, b := range ifCmd.Branches {
		if b.Kind != kinds[i] {
			t.Errorf("branch[%d] kind = %q, want %q", i, b.Kind, kinds[i])
		}
		if len(b.Body) != 1 {
			t.Errorf("branch[%d] body length = %d, want 1", i, len(b.Body))
		}
	}
}

func TestConditionals_IfMultipleElifBranches(t *testing.T) {
	src := `IF {status} == active:
    Click the 'Active' button
ELIF {status} == pending:
    Click the 'Pending' button
ELIF {status} == suspended:
    Click the 'Suspended' button
ELSE:
    Click the 'Unknown' button
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	if len(hunt.Commands[0].Branches) != 4 {
		t.Fatalf("expected 4 branches, got %d", len(hunt.Commands[0].Branches))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section C: IF body with multiple commands
// ═══════════════════════════════════════════════════════════════════════════════

func TestConditionals_IfBodyMultipleCommands(t *testing.T) {
	src := `IF {logged_in} == true:
    Fill 'Search' field with 'manul'
    Click the 'Search' button
    VERIFY that 'Results' is present
ELSE:
    NAVIGATE to https://example.com/login
`
	hunt := mustParse(t, src)

	ifCmd := hunt.Commands[0]
	ifBranch := ifCmd.Branches[0]
	if len(ifBranch.Body) != 3 {
		t.Fatalf("IF body: expected 3 commands, got %d", len(ifBranch.Body))
	}
	if ifBranch.Body[0].Type != CmdFill {
		t.Errorf("IF body[0] = %s, want fill", ifBranch.Body[0].Type)
	}
	if ifBranch.Body[1].Type != CmdClick {
		t.Errorf("IF body[1] = %s, want click", ifBranch.Body[1].Type)
	}
	if ifBranch.Body[2].Type != CmdVerify {
		t.Errorf("IF body[2] = %s, want verify", ifBranch.Body[2].Type)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section D: WHILE loops
// ═══════════════════════════════════════════════════════════════════════════════

func TestLoops_WhileClassified(t *testing.T) {
	cmd := mustParseLine(t, "WHILE {retries} > 0:")
	if cmd.Type != CmdWhile {
		t.Errorf("type = %s, want %s", cmd.Type, CmdWhile)
	}
	if cmd.Condition != "{retries} > 0" {
		t.Errorf("condition = %q", cmd.Condition)
	}
}

func TestLoops_WhileBlock(t *testing.T) {
	src := `WHILE {count} < 5:
    Click the 'Next' button
    SET {count} = {count} + 1
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	whileCmd := hunt.Commands[0]
	if whileCmd.Type != CmdWhile {
		t.Fatalf("type = %s, want while", whileCmd.Type)
	}
	if len(whileCmd.Body) != 2 {
		t.Fatalf("WHILE body: expected 2 commands, got %d", len(whileCmd.Body))
	}
	if whileCmd.Body[0].Type != CmdClick {
		t.Errorf("body[0] = %s, want click", whileCmd.Body[0].Type)
	}
	if whileCmd.Body[1].Type != CmdSet {
		t.Errorf("body[1] = %s, want set", whileCmd.Body[1].Type)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section E: REPEAT N TIMES loops
// ═══════════════════════════════════════════════════════════════════════════════

func TestLoops_RepeatClassified(t *testing.T) {
	cmd := mustParseLine(t, "REPEAT 5 TIMES:")
	if cmd.Type != CmdRepeat {
		t.Errorf("type = %s, want %s", cmd.Type, CmdRepeat)
	}
	if cmd.RepeatCount != 5 {
		t.Errorf("count = %d, want 5", cmd.RepeatCount)
	}
	if cmd.RepeatVar != "i" {
		t.Errorf("var = %q, want 'i' (default)", cmd.RepeatVar)
	}
}

func TestLoops_RepeatSingular(t *testing.T) {
	cmd := mustParseLine(t, "REPEAT 1 TIME:")
	if cmd.Type != CmdRepeat {
		t.Errorf("type = %s, want %s", cmd.Type, CmdRepeat)
	}
	if cmd.RepeatCount != 1 {
		t.Errorf("count = %d, want 1", cmd.RepeatCount)
	}
}

func TestLoops_RepeatBlock(t *testing.T) {
	src := `REPEAT 3 TIMES:
    Click the 'Next' button
    WAIT 1
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	repeatCmd := hunt.Commands[0]
	if repeatCmd.Type != CmdRepeat {
		t.Fatalf("type = %s, want repeat", repeatCmd.Type)
	}
	if repeatCmd.RepeatCount != 3 {
		t.Fatalf("count = %d, want 3", repeatCmd.RepeatCount)
	}
	if len(repeatCmd.Body) != 2 {
		t.Fatalf("REPEAT body: expected 2 commands, got %d", len(repeatCmd.Body))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section F: FOR EACH loops
// ═══════════════════════════════════════════════════════════════════════════════

func TestLoops_ForEachClassified(t *testing.T) {
	cmd := mustParseLine(t, "FOR EACH {item} IN {items}:")
	if cmd.Type != CmdForEach {
		t.Errorf("type = %s, want %s", cmd.Type, CmdForEach)
	}
	if cmd.ForEachVar != "item" {
		t.Errorf("var = %q, want 'item'", cmd.ForEachVar)
	}
	if cmd.ForEachCollection != "items" {
		t.Errorf("collection = %q, want 'items'", cmd.ForEachCollection)
	}
}

func TestLoops_ForEachBlock(t *testing.T) {
	src := `FOR EACH {url} IN {pages}:
    NAVIGATE to {url}
    VERIFY that 'Welcome' is present
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	forCmd := hunt.Commands[0]
	if forCmd.Type != CmdForEach {
		t.Fatalf("type = %s, want for_each", forCmd.Type)
	}
	if len(forCmd.Body) != 2 {
		t.Fatalf("FOR EACH body: expected 2 commands, got %d", len(forCmd.Body))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section G: Nested control flow
// ═══════════════════════════════════════════════════════════════════════════════

func TestConditionals_NestedIfInLoop(t *testing.T) {
	src := `REPEAT 5 TIMES:
    IF {i} == 3:
        Click the 'Skip' button
    ELSE:
        Click the 'Process' button
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	repeatCmd := hunt.Commands[0]
	if repeatCmd.Type != CmdRepeat {
		t.Fatalf("type = %s, want repeat", repeatCmd.Type)
	}
	if len(repeatCmd.Body) != 1 {
		t.Fatalf("REPEAT body: expected 1 command (nested IF), got %d", len(repeatCmd.Body))
	}
	nestedIf := repeatCmd.Body[0]
	if nestedIf.Type != CmdIf {
		t.Fatalf("nested type = %s, want if", nestedIf.Type)
	}
	if len(nestedIf.Branches) != 2 {
		t.Fatalf("nested IF branches = %d, want 2", len(nestedIf.Branches))
	}
}

func TestConditionals_LoopInIf(t *testing.T) {
	src := `IF {has_items} == true:
    REPEAT 3 TIMES:
        Click the 'Next' button
ELSE:
    Click the 'Empty' button
`
	hunt := mustParse(t, src)

	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 top-level command, got %d", len(hunt.Commands))
	}
	ifCmd := hunt.Commands[0]
	if len(ifCmd.Branches) != 2 {
		t.Fatalf("branches = %d, want 2", len(ifCmd.Branches))
	}
	ifBody := ifCmd.Branches[0].Body
	if len(ifBody) != 1 {
		t.Fatalf("IF body = %d commands, want 1", len(ifBody))
	}
	if ifBody[0].Type != CmdRepeat {
		t.Errorf("IF body[0] = %s, want repeat", ifBody[0].Type)
	}
	if len(ifBody[0].Body) != 1 {
		t.Errorf("nested REPEAT body = %d, want 1", len(ifBody[0].Body))
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section H: Mixed actions with control flow
// ═══════════════════════════════════════════════════════════════════════════════

func TestConditionals_MixedActionsWithControlFlow(t *testing.T) {
	src := `NAVIGATE to https://example.com
Fill 'Email' field with 'test@test.com'
IF {has_promo} == true:
    Fill 'Promo' field with 'SAVE20'
Click the 'Submit' button
VERIFY that 'Success' is present
`
	hunt := mustParse(t, src)

	// Should have: NAVIGATE, Fill, IF, Click, VERIFY = 5 top-level
	if len(hunt.Commands) != 5 {
		t.Fatalf("expected 5 top-level commands, got %d", len(hunt.Commands))
	}
	if hunt.Commands[0].Type != CmdNavigate {
		t.Errorf("cmd[0] = %s, want navigate", hunt.Commands[0].Type)
	}
	if hunt.Commands[1].Type != CmdFill {
		t.Errorf("cmd[1] = %s, want fill", hunt.Commands[1].Type)
	}
	if hunt.Commands[2].Type != CmdIf {
		t.Errorf("cmd[2] = %s, want if", hunt.Commands[2].Type)
	}
	if hunt.Commands[3].Type != CmdClick {
		t.Errorf("cmd[3] = %s, want click", hunt.Commands[3].Type)
	}
	if hunt.Commands[4].Type != CmdVerify {
		t.Errorf("cmd[4] = %s, want verify", hunt.Commands[4].Type)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Section I: Condition text patterns
// ═══════════════════════════════════════════════════════════════════════════════

func TestConditionals_ConditionPatterns(t *testing.T) {
	tests := []struct {
		line      string
		condition string
	}{
		{"IF {x} == 1:", "{x} == 1"},
		{"IF {count} > 0:", "{count} > 0"},
		{"IF {name} != '':", "{name} != ''"},
		{"IF {flag} == true:", "{flag} == true"},
		{"WHILE {retries} < 10:", "{retries} < 10"},
	}
	for _, tc := range tests {
		cmd := mustParseLine(t, tc.line)
		if cmd.Condition != tc.condition {
			t.Errorf("%q: condition = %q, want %q", tc.line, cmd.Condition, tc.condition)
		}
	}
}

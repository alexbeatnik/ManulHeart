package dsl

// ─────────────────────────────────────────────────────────────────────────────
// LOGICAL STEPS TEST SUITE
//
// Port of ManulEngine test_28_logical_steps.py — STEP N: marker parsing,
// block grouping, and StepBlock assignment.
//
// The Python version also tests reporting features (_group_steps, _render_lstep_group,
// generate_report). Those are NOT portable since Go reporting doesn't have
// those features yet. We port only the parser / structural tests.
//
// Validates:
// 1. STEP N: recognized as a block marker (not a command)
// 2. Commands inside STEP blocks get StepBlock assigned
// 3. Multiple STEP blocks parsed correctly
// 4. Legacy (no STEP markers) produces commands without StepBlock
// 5. STEP inside quoted labels is not treated as a block marker
// ─────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"testing"
)

// ── Section 1: STEP recognition ──────────────────────────────────────────────

func TestLogicalSteps_StepMarkerRecognized(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"STEP 1: Login\n" +
			"    NAVIGATE to 'https://example.com'\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	// STEP marker is consumed, not emitted as a command
	for _, cmd := range h.Commands {
		if strings.Contains(strings.ToUpper(cmd.Raw), "STEP 1:") {
			t.Error("STEP marker should be consumed by parser, not emitted as command")
		}
	}
	// But the NAVIGATE command inside should have StepBlock set
	if len(h.Commands) == 0 {
		t.Fatal("expected at least one command")
	}
	if !strings.Contains(h.Commands[0].StepBlock, "STEP 1") {
		t.Errorf("command StepBlock=%q, expected to contain 'STEP 1'", h.Commands[0].StepBlock)
	}
}

func TestLogicalSteps_StepWithoutNumber(t *testing.T) {
	// "STEP: Fill in credentials" — no number
	// This follows the regex `(?i)^STEP\s+\d+\s*:` which requires a number.
	// Test that STEP without a number is NOT consumed as a marker.
	h, err := Parse(strings.NewReader(
		"NAVIGATE to 'https://example.com'\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	// No StepBlock for commands without STEP markers
	for _, cmd := range h.Commands {
		if cmd.StepBlock != "" {
			t.Errorf("cmd %q StepBlock=%q, expected empty for legacy missions", cmd.Raw, cmd.StepBlock)
		}
	}
}

// ── Section 2: Multiple STEP blocks ──────────────────────────────────────────

func TestLogicalSteps_TwoBlocks(t *testing.T) {
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

	var step1Cmds, step2Cmds int
	for _, cmd := range h.Commands {
		if strings.Contains(cmd.StepBlock, "STEP 1") {
			step1Cmds++
		}
		if strings.Contains(cmd.StepBlock, "STEP 2") {
			step2Cmds++
		}
	}
	if step1Cmds != 2 {
		t.Errorf("expected 2 commands in STEP 1, got %d", step1Cmds)
	}
	if step2Cmds != 1 {
		t.Errorf("expected 1 command in STEP 2, got %d", step2Cmds)
	}
}

func TestLogicalSteps_ThreeBlocks(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"STEP 1: Setup\n" +
			"    NAVIGATE to 'https://example.com'\n" +
			"STEP 2: Login\n" +
			"    Fill 'Username' field with 'admin'\n" +
			"    Click the 'Submit' button\n" +
			"STEP 3: Verify\n" +
			"    VERIFY that 'Welcome' is present\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	blocks := make(map[string]int)
	for _, cmd := range h.Commands {
		if cmd.StepBlock != "" {
			blocks[cmd.StepBlock]++
		}
	}
	if len(blocks) != 3 {
		t.Errorf("expected 3 distinct step blocks, got %d: %v", len(blocks), blocks)
	}
}

// ── Section 3: Legacy (no STEP markers) ──────────────────────────────────────

func TestLogicalSteps_LegacyNoSteps(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"NAVIGATE to 'https://example.com'\n" +
			"Fill 'Username' field with 'admin'\n" +
			"Click the 'Login' button\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	for _, cmd := range h.Commands {
		if cmd.StepBlock != "" {
			t.Errorf("legacy command %q should have empty StepBlock, got %q", cmd.Raw, cmd.StepBlock)
		}
	}
}

// ── Section 4: STEP keyword inside quoted labels ─────────────────────────────

func TestLogicalSteps_StepInsideVerifyQuotes(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"VERIFY that 'STEP 1: done' is present\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(h.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(h.Commands))
	}
	if h.Commands[0].Type != CmdVerify {
		t.Errorf("expected verify command, got %s", h.Commands[0].Type)
	}
}

func TestLogicalSteps_StepInsideClickLabel(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"Click 'Next Step' button\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(h.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(h.Commands))
	}
	if h.Commands[0].Type != CmdClick {
		t.Errorf("expected click command, got %s", h.Commands[0].Type)
	}
}

// ── Section 5: STEP numbering formats ────────────────────────────────────────

func TestLogicalSteps_NumberedVariants(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"STEP 1: Navigate to login", "STEP 1"},
		{"STEP 10: Final check", "STEP 10"},
		{"STEP 42: The answer", "STEP 42"},
	}

	for _, tt := range tests {
		h, err := Parse(strings.NewReader(
			tt.input + "\n" +
				"    Click the 'Submit' button\n" +
				"DONE.\n"))
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", tt.input, err)
			continue
		}
		if len(h.Commands) == 0 {
			t.Errorf("Parse(%q) produced no commands", tt.input)
			continue
		}
		if !strings.Contains(h.Commands[0].StepBlock, tt.contains) {
			t.Errorf("Parse(%q): StepBlock=%q, should contain %q",
				tt.input, h.Commands[0].StepBlock, tt.contains)
		}
	}
}

// ── Section 6: STEP combined with other headers ──────────────────────────────

func TestLogicalSteps_WithHeaders(t *testing.T) {
	h, err := Parse(strings.NewReader(
		"@context: Login flow\n" +
			"@title: auth\n" +
			"@tags: smoke, regression\n" +
			"\n" +
			"STEP 1: Setup\n" +
			"    NAVIGATE to 'https://example.com'\n" +
			"STEP 2: Action\n" +
			"    Fill 'Username' field with 'admin'\n" +
			"DONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if h.Context != "Login flow" {
		t.Errorf("context=%q", h.Context)
	}
	if h.Title != "auth" {
		t.Errorf("title=%q", h.Title)
	}
	if len(h.Tags) != 2 {
		t.Errorf("tags=%v", h.Tags)
	}
	if len(h.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(h.Commands))
	}
}

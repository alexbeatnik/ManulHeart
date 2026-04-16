package dsl

import (
	"strings"
	"testing"
)

func TestParse_Steps(t *testing.T) {
	input := `
        1. STEP 1: Navigate to the login page
        NAVIGATE to https://example.com
        
        2. STEP 2: Login
        FILL "User" with "admin"
        CLICK "Submit"
        
        STEP: Finalize
        DONE.
    `
	hunt, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(hunt.Commands) != 3 { // NAVIGATE, FILL, CLICK
		t.Errorf("expected 3 commands, got %d", len(hunt.Commands))
	}

	if hunt.Commands[0].StepBlock != "1. STEP 1: Navigate to the login page" {
		t.Errorf("wrong step block for cmd 0: %q", hunt.Commands[0].StepBlock)
	}

	if hunt.Commands[1].StepBlock != "2. STEP 2: Login" {
		t.Errorf("wrong step block for cmd 1: %q", hunt.Commands[1].StepBlock)
	}

	if hunt.Commands[2].StepBlock != "2. STEP 2: Login" {
		t.Errorf("wrong step block for cmd 2: %q", hunt.Commands[2].StepBlock)
	}
}

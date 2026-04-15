package dsl

import (
	"strings"
	"testing"
)

func TestParse_Tags(t *testing.T) {
	input := `
        @context: Login flow
        @title: auth
        @tags: smoke, auth, regression

        1. NAVIGATE to https://example.com
        2. DONE.
    `
	hunt, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	expectedTags := []string{"smoke", "auth", "regression"}
	if len(hunt.Tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %v", len(expectedTags), hunt.Tags)
	}
	for i, tag := range hunt.Tags {
		if tag != expectedTags[i] {
			t.Errorf("expected tag %d to be %s, got %s", i, expectedTags[i], tag)
		}
	}

	// Tags should NOT be in command bodies
	for _, cmd := range hunt.Commands {
		if strings.Contains(cmd.Raw, "@tags") {
			t.Errorf("command %q should not contain @tags", cmd.Raw)
		}
	}
}

func TestParse_InLineTags(t *testing.T) {
	input := `
        @TAG: smoke
        CLICK the "Login" button

        @TAGS: slow, nightly
        WAIT 5
    `
	hunt, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(hunt.Commands) != 2 {
		t.Errorf("expected 2 commands, got %d", len(hunt.Commands))
	}

	if len(hunt.Commands[0].Tags) != 1 || hunt.Commands[0].Tags[0] != "smoke" {
		t.Errorf("expected [smoke] tags for cmd 0, got %v", hunt.Commands[0].Tags)
	}

	if len(hunt.Commands[1].Tags) != 2 || hunt.Commands[1].Tags[0] != "slow" || hunt.Commands[1].Tags[1] != "nightly" {
		t.Errorf("expected [slow nightly] tags for cmd 1, got %v", hunt.Commands[1].Tags)
	}
}

package record

import (
	"strings"
	"testing"
)

func TestEventToDSL(t *testing.T) {
	tests := []struct {
		ev   Event
		want string
	}{
		{Event{Type: "click", Data: map[string]interface{}{"target": "Login", "tag": "BUTTON"}}, "Click the 'Login' button"},
		{Event{Type: "click", Data: map[string]interface{}{"target": "Home", "tag": "A"}}, "Click the 'Home' link"},
		{Event{Type: "fill", Data: map[string]interface{}{"target": "Email", "value": "a@b.com"}}, "Fill 'Email' field with 'a@b.com'"},
		{Event{Type: "press", Data: map[string]interface{}{"key": "Enter"}}, "PRESS Enter"},
		{Event{Type: "press", Data: map[string]interface{}{"key": "Escape"}}, "PRESS Escape"},
		{Event{Type: "unknown", Data: map[string]interface{}{}}, ""},
		{Event{Type: "click", Data: map[string]interface{}{"target": "Submit", "tag": "INPUT"}}, "Click the 'Submit' button"},
		{Event{Type: "click", Data: map[string]interface{}{"target": "About Us", "tag": "A"}}, "Click the 'About Us' link"},
	}
	for _, tt := range tests {
		got := eventToDSL(tt.ev)
		if got != tt.want {
			t.Fatalf("eventToDSL(%+v) = %q, want %q", tt.ev, got, tt.want)
		}
	}
}

func TestBuildHunt(t *testing.T) {
	url := "https://example.com"
	events := []Event{
		{Type: "click", Data: map[string]interface{}{"target": "Login", "tag": "BUTTON"}},
		{Type: "fill", Data: map[string]interface{}{"target": "Email", "value": "a@b.com"}},
	}
	hunt := buildHunt(url, events)
	if !strings.Contains(hunt, "NAVIGATE to https://example.com") {
		t.Fatal("missing NAVIGATE")
	}
	if !strings.Contains(hunt, "Click the 'Login' button") {
		t.Fatal("missing click action")
	}
	if !strings.Contains(hunt, "Fill 'Email' field with 'a@b.com'") {
		t.Fatal("missing fill action")
	}
}

func TestBuildHunt_NoEvents(t *testing.T) {
	hunt := buildHunt("https://empty.com", nil)
	if !strings.Contains(hunt, "NAVIGATE to https://empty.com") {
		t.Fatal("missing NAVIGATE")
	}
	if !strings.Contains(hunt, "DONE.") {
		t.Fatal("missing DONE")
	}
	// Should only have STEP 1 (NAVIGATE) and no other steps
	c := strings.Count(hunt, "STEP")
	if c != 1 {
		t.Fatalf("expected 1 STEP for empty events, got %d", c)
	}
}

func TestBuildHunt_DeduplicatesFills(t *testing.T) {
	events := []Event{
		{Type: "fill", Data: map[string]interface{}{"target": "Email", "value": "first"}},
		{Type: "fill", Data: map[string]interface{}{"target": "Email", "value": "second"}},
	}
	hunt := buildHunt("https://example.com", events)
	c := strings.Count(hunt, "Fill 'Email'")
	if c != 1 {
		t.Fatalf("expected 1 Fill 'Email' after dedup, got %d", c)
	}
}

func TestBuildHunt_PreservesOrder(t *testing.T) {
	events := []Event{
		{Type: "click", Data: map[string]interface{}{"target": "A", "tag": "BUTTON"}},
		{Type: "click", Data: map[string]interface{}{"target": "B", "tag": "BUTTON"}},
		{Type: "click", Data: map[string]interface{}{"target": "C", "tag": "BUTTON"}},
	}
	hunt := buildHunt("https://example.com", events)
	idxA := strings.Index(hunt, "Click the 'A' button")
	idxB := strings.Index(hunt, "Click the 'B' button")
	idxC := strings.Index(hunt, "Click the 'C' button")
	if idxA == -1 || idxB == -1 || idxC == -1 {
		t.Fatal("missing expected actions")
	}
	if !(idxA < idxB && idxB < idxC) {
		t.Fatal("actions not in expected order")
	}
}

func TestBuildHunt_MixedEvents(t *testing.T) {
	events := []Event{
		{Type: "click", Data: map[string]interface{}{"target": "Login", "tag": "BUTTON"}},
		{Type: "fill", Data: map[string]interface{}{"target": "Email", "value": "a@b.com"}},
		{Type: "press", Data: map[string]interface{}{"key": "Enter"}},
		{Type: "click", Data: map[string]interface{}{"target": "Submit", "tag": "BUTTON"}},
	}
	hunt := buildHunt("https://example.com", events)
	checks := []string{
		"Click the 'Login' button",
		"Fill 'Email' field with 'a@b.com'",
		"PRESS Enter",
		"Click the 'Submit' button",
	}
	for _, check := range checks {
		if !strings.Contains(hunt, check) {
			t.Fatalf("missing action: %s", check)
		}
	}
}

func TestBuildHunt_SkipsUnknownEvents(t *testing.T) {
	events := []Event{
		{Type: "click", Data: map[string]interface{}{"target": "Login", "tag": "BUTTON"}},
		{Type: "scroll", Data: map[string]interface{}{"direction": "down"}},
		{Type: "press", Data: map[string]interface{}{"key": "Enter"}},
	}
	hunt := buildHunt("https://example.com", events)
	if strings.Contains(hunt, "scroll") {
		t.Fatal("unknown event type should not appear in hunt")
	}
}

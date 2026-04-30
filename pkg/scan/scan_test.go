package scan

import (
	"strings"
	"testing"
)

func TestIsUseful(t *testing.T) {
	if isUseful("", "button") {
		t.Fatal("empty identifier should be skipped")
	}
	if isUseful("click", "button") {
		t.Fatal("'click' should be skipped")
	}
	if isUseful("https://example.com", "link") {
		t.Fatal("URL identifier should be skipped")
	}
	if !isUseful("Login", "button") {
		t.Fatal("'Login' should be useful")
	}
	if isUseful("×", "button") {
		t.Fatal("'×' should be skipped")
	}
	if isUseful("menu", "button") {
		t.Fatal("'menu' should be skipped")
	}
	if isUseful("go", "link") {
		t.Fatal("'go' should be skipped")
	}
	if isUseful("   ", "button") {
		t.Fatal("whitespace-only should be skipped")
	}
	if !isUseful("Add to Cart", "button") {
		t.Fatal("'Add to Cart' should be useful")
	}
}

func TestMapToStep(t *testing.T) {
	if !strings.Contains(mapToStep("input", "Email"), "Fill") {
		t.Fatal("input should map to Fill")
	}
	if !strings.Contains(mapToStep("select", "Country"), "Select") {
		t.Fatal("select should map to Select")
	}
	if !strings.Contains(mapToStep("link", "Home"), "link") {
		t.Fatal("link should map to Click link")
	}
	if !strings.Contains(mapToStep("button", "Submit"), "button") {
		t.Fatal("button should map to Click button")
	}
	if !strings.Contains(mapToStep("checkbox", "Agree"), "Check") {
		t.Fatal("checkbox should map to Check")
	}
	if !strings.Contains(mapToStep("radio", "Option A"), "radio button") {
		t.Fatal("radio should map to radio button")
	}
}

func TestBuildHunt(t *testing.T) {
	url := "https://example.com"
	els := []Element{
		{Type: "button", Identifier: "Login"},
		{Type: "input", Identifier: "Email"},
		{Type: "button", Identifier: "Login"}, // duplicate
	}
	hunt := BuildHunt(url, els)
	if !strings.Contains(hunt, "NAVIGATE to https://example.com") {
		t.Fatal("missing NAVIGATE step")
	}
	c := strings.Count(hunt, "STEP")
	if c != 4 { // 1 navigate + 1 wait + 2 actions
		t.Fatalf("expected 4 STEP lines, got %d", c)
	}
}

func TestBuildHunt_NoElements(t *testing.T) {
	hunt := BuildHunt("https://empty.com", nil)
	if !strings.Contains(hunt, "NAVIGATE to https://empty.com") {
		t.Fatal("missing NAVIGATE")
	}
	if !strings.Contains(hunt, "WAIT 2") {
		t.Fatal("missing WAIT step")
	}
	if !strings.Contains(hunt, "DONE.") {
		t.Fatal("missing DONE")
	}
	// navigate + wait + done = 2 STEP mentions (STEP 1 and STEP 2)
	c := strings.Count(hunt, "STEP")
	if c != 2 {
		t.Fatalf("expected 2 STEP lines for empty elements, got %d", c)
	}
}

func TestBuildHunt_SkipsUseless(t *testing.T) {
	els := []Element{
		{Type: "button", Identifier: "Login"},
		{Type: "button", Identifier: "click"}, // skipped
		{Type: "button", Identifier: ""},      // skipped
	}
	hunt := BuildHunt("https://example.com", els)
	// Only Login should appear as an action
	actionCount := strings.Count(hunt, "Click the 'Login' button")
	if actionCount != 1 {
		t.Fatalf("expected 1 Login action, got %d", actionCount)
	}
}

func TestBuildHunt_SkipsUrlIdentifiers(t *testing.T) {
	els := []Element{
		{Type: "link", Identifier: "https://evil.com"}, // skipped
		{Type: "link", Identifier: "Home"},             // kept
	}
	hunt := BuildHunt("https://example.com", els)
	if strings.Contains(hunt, "evil.com") {
		t.Fatal("should skip URL identifiers")
	}
	if !strings.Contains(hunt, "Click the 'Home' link") {
		t.Fatal("should include non-URL identifiers")
	}
}

func TestBuildHunt_SkipsDuplicates(t *testing.T) {
	els := []Element{
		{Type: "button", Identifier: "Buy Now"},
		{Type: "button", Identifier: "Buy Now"},
		{Type: "button", Identifier: "Buy Now"},
	}
	hunt := BuildHunt("https://example.com", els)
	c := strings.Count(hunt, "Buy Now")
	if c != 1 {
		t.Fatalf("expected 1 'Buy Now' after dedup, got %d", c)
	}
}

func TestBuildHunt_AllTypes(t *testing.T) {
	els := []Element{
		{Type: "input", Identifier: "Username"},
		{Type: "select", Identifier: "Country"},
		{Type: "checkbox", Identifier: "Agree"},
		{Type: "radio", Identifier: "Male"},
		{Type: "link", Identifier: "Home"},
		{Type: "button", Identifier: "Next"},
	}
	hunt := BuildHunt("https://example.com", els)
	checks := []string{
		"Fill 'Username'",
		"Select 'Option' from the 'Country' dropdown",
		"Check the checkbox for 'Agree'",
		"Click the radio button for 'Male'",
		"Click the 'Home' link",
		"Click the 'Next' button",
	}
	for _, check := range checks {
		if !strings.Contains(hunt, check) {
			t.Fatalf("missing action: %s", check)
		}
	}
}

func TestBuildHunt_LongLabelsSkipped(t *testing.T) {
	longLabel := strings.Repeat("a", 100)
	els := []Element{
		{Type: "button", Identifier: longLabel},
	}
	hunt := BuildHunt("https://example.com", els)
	if strings.Contains(hunt, longLabel) {
		t.Fatal("labels > 80 chars should be skipped")
	}
}

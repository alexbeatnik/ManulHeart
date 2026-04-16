package dsl

import (
	"strings"
	"testing"
)

// ── Parse helpers ─────────────────────────────────────────────────────────────

func mustParse(t *testing.T, src string) *Hunt {
	t.Helper()
	hunt, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return hunt
}

func mustParseLine(t *testing.T, line string) Command {
	t.Helper()
	hunt := mustParse(t, line)
	if len(hunt.Commands) == 0 {
		t.Fatalf("no commands parsed from %q", line)
	}
	return hunt.Commands[0]
}

// ── Headers ───────────────────────────────────────────────────────────────────

func TestParseHeaders(t *testing.T) {
	src := `@context: testing context
@title: my_suite
@var: {base} = https://example.com
`
	hunt := mustParse(t, src)
	if hunt.Context != "testing context" {
		t.Errorf("context = %q", hunt.Context)
	}
	if hunt.Title != "my_suite" {
		t.Errorf("title = %q", hunt.Title)
	}
	if hunt.Vars["base"] != "https://example.com" {
		t.Errorf("var base = %q", hunt.Vars["base"])
	}
}

func TestParseTags(t *testing.T) {
	src := `@tags: smoke, regression, ui`
	hunt := mustParse(t, src)
	if len(hunt.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(hunt.Tags))
	}
	if hunt.Tags[0] != "smoke" || hunt.Tags[1] != "regression" || hunt.Tags[2] != "ui" {
		t.Errorf("tags = %v", hunt.Tags)
	}
}

// ── NAVIGATE ──────────────────────────────────────────────────────────────────

func TestParseNavigate(t *testing.T) {
	cmd := mustParseLine(t, "NAVIGATE to https://example.com/page")
	if cmd.Type != CmdNavigate {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.URL != "https://example.com/page" {
		t.Errorf("url = %q", cmd.URL)
	}
}

func TestParseNavigateQuoted(t *testing.T) {
	cmd := mustParseLine(t, "NAVIGATE to 'https://example.com/page'")
	if cmd.URL != "https://example.com/page" {
		t.Errorf("url = %q", cmd.URL)
	}
}

// ── CLICK ─────────────────────────────────────────────────────────────────────

func TestParseClickButton(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Submit' button")
	if cmd.Type != CmdClick {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Submit" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.TypeHint != "button" {
		t.Errorf("hint = %q", cmd.TypeHint)
	}
	if cmd.InteractionMode != ModeClickable {
		t.Errorf("mode = %s", cmd.InteractionMode)
	}
}

func TestParseClickLink(t *testing.T) {
	cmd := mustParseLine(t, "CLICK on the '2' link")
	if cmd.Type != CmdClick {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "2" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.TypeHint != "link" {
		t.Errorf("hint = %q, want 'link'", cmd.TypeHint)
	}
}

func TestParseClickElement(t *testing.T) {
	cmd := mustParseLine(t, "CLICK the 'Scrolling DropDown' element")
	if cmd.Target != "Scrolling DropDown" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.TypeHint != "element" {
		t.Errorf("hint = %q", cmd.TypeHint)
	}
}

// ── DOUBLE CLICK ──────────────────────────────────────────────────────────────

func TestParseDoubleClick(t *testing.T) {
	cmd := mustParseLine(t, "DOUBLE CLICK the 'Copy Text' button")
	if cmd.Type != CmdDoubleClick {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Copy Text" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.TypeHint != "button" {
		t.Errorf("hint = %q", cmd.TypeHint)
	}
}

// ── FILL / TYPE ───────────────────────────────────────────────────────────────

func TestParseFill(t *testing.T) {
	cmd := mustParseLine(t, "FILL 'Name' field with 'Mega Manul'")
	if cmd.Type != CmdFill {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Name" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.Value != "Mega Manul" {
		t.Errorf("value = %q", cmd.Value)
	}
	if cmd.InteractionMode != ModeInput {
		t.Errorf("mode = %s", cmd.InteractionMode)
	}
}

func TestParseType(t *testing.T) {
	cmd := mustParseLine(t, "TYPE 'Shadow Boss' into the 'Shadow Root' field")
	if cmd.Type != CmdType {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Value != "Shadow Boss" {
		t.Errorf("value = %q", cmd.Value)
	}
	if cmd.Target != "Shadow Root" {
		t.Errorf("target = %q", cmd.Target)
	}
}

// ── SELECT ────────────────────────────────────────────────────────────────────

func TestParseSelect(t *testing.T) {
	cmd := mustParseLine(t, "SELECT 'Japan' from the 'Country' dropdown")
	if cmd.Type != CmdSelect {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Value != "Japan" {
		t.Errorf("value = %q", cmd.Value)
	}
	if cmd.Target != "Country" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.InteractionMode != ModeSelect {
		t.Errorf("mode = %s", cmd.InteractionMode)
	}
}

// ── CHECK / UNCHECK ───────────────────────────────────────────────────────────

func TestParseCheck(t *testing.T) {
	cmd := mustParseLine(t, "CHECK the checkbox for 'Monday'")
	if cmd.Type != CmdCheck {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Monday" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.InteractionMode != ModeCheckbox {
		t.Errorf("mode = %s", cmd.InteractionMode)
	}
}

func TestParseCheckNumericTarget(t *testing.T) {
	cmd := mustParseLine(t, "CHECK the checkbox for '7'")
	if cmd.Type != CmdCheck {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "7" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.InteractionMode != ModeCheckbox {
		t.Errorf("mode = %s", cmd.InteractionMode)
	}
}

func TestParseUncheck(t *testing.T) {
	cmd := mustParseLine(t, "Uncheck the checkbox for 'Auto-Renew'")
	if cmd.Type != CmdUncheck {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Auto-Renew" {
		t.Errorf("target = %q", cmd.Target)
	}
}

// ── VERIFY ────────────────────────────────────────────────────────────────────

func TestParseVerifyPresent(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY that 'Mega Manul' is present")
	if cmd.Type != CmdVerify {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.VerifyText != "Mega Manul" {
		t.Errorf("verifyText = %q", cmd.VerifyText)
	}
	if cmd.VerifyNegated {
		t.Error("should not be negated")
	}
}

func TestParseVerifyNotPresent(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY that 'Error' is NOT present")
	if cmd.Type != CmdVerify {
		t.Errorf("type = %s", cmd.Type)
	}
	if !cmd.VerifyNegated {
		t.Error("should be negated")
	}
}

func TestParseVerifySoftly(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY SOFTLY that 'Warning' is present")
	if cmd.Type != CmdVerifySoft {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.VerifyText != "Warning" {
		t.Errorf("verifyText = %q", cmd.VerifyText)
	}
}

func TestParseVerifyChecked(t *testing.T) {
	cmd := mustParseLine(t, "VERIFY that 'Monday' is checked")
	if cmd.Type != CmdVerify {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.VerifyText != "Monday" {
		t.Errorf("verifyText = %q", cmd.VerifyText)
	}
}

// ── SCROLL ────────────────────────────────────────────────────────────────────

func TestParseScrollDown(t *testing.T) {
	cmd := mustParseLine(t, "SCROLL DOWN")
	if cmd.Type != CmdScroll {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.ScrollDirection != "down" {
		t.Errorf("direction = %q", cmd.ScrollDirection)
	}
}

func TestParseScrollInsideContainer(t *testing.T) {
	cmd := mustParseLine(t, "SCROLL DOWN inside the 'list'")
	if cmd.Type != CmdScroll {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.ScrollContainer != "list" {
		t.Errorf("container = %q", cmd.ScrollContainer)
	}
}

// ── PRESS ─────────────────────────────────────────────────────────────────────

func TestParsePressEnter(t *testing.T) {
	cmd := mustParseLine(t, "PRESS ENTER")
	if cmd.Type != CmdPress {
		t.Errorf("type = %s", cmd.Type)
	}
	if !strings.EqualFold(cmd.PressKey, "ENTER") {
		t.Errorf("key = %q", cmd.PressKey)
	}
}

func TestParsePressCombo(t *testing.T) {
	cmd := mustParseLine(t, "PRESS Control+A")
	if cmd.Type != CmdPress {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.PressKey != "Control+A" {
		t.Errorf("key = %q", cmd.PressKey)
	}
}

// ── EXTRACT ───────────────────────────────────────────────────────────────────

func TestParseExtract(t *testing.T) {
	cmd := mustParseLine(t, "EXTRACT the 'CPU of Chrome' into {chrome_cpu}")
	if cmd.Type != CmdExtract {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "CPU of Chrome" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.ExtractVar != "chrome_cpu" {
		t.Errorf("var = %q", cmd.ExtractVar)
	}
}

// ── SET ───────────────────────────────────────────────────────────────────────

func TestParseSet(t *testing.T) {
	cmd := mustParseLine(t, "SET {greeting} = Hello World")
	if cmd.Type != CmdSet {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.SetVar != "greeting" {
		t.Errorf("var = %q", cmd.SetVar)
	}
	if cmd.SetValue != "Hello World" {
		t.Errorf("value = %q", cmd.SetValue)
	}
}

// ── WAIT ──────────────────────────────────────────────────────────────────────

func TestParseWait(t *testing.T) {
	cmd := mustParseLine(t, "WAIT 2")
	if cmd.Type != CmdWait {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.WaitSeconds != 2 {
		t.Errorf("seconds = %f", cmd.WaitSeconds)
	}
}

func TestParseWaitForVisible(t *testing.T) {
	cmd := mustParseLine(t, "Wait for 'Loading' to be visible")
	if cmd.Type != CmdWaitFor {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Loading" {
		t.Errorf("target = %q", cmd.Target)
	}
	if cmd.WaitForState != "visible" {
		t.Errorf("state = %q", cmd.WaitForState)
	}
}

// ── HOVER ─────────────────────────────────────────────────────────────────────

func TestParseHover(t *testing.T) {
	cmd := mustParseLine(t, "HOVER over the 'Profile' element")
	if cmd.Type != CmdHover {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "Profile" {
		t.Errorf("target = %q", cmd.Target)
	}
}

// ── RIGHT CLICK ───────────────────────────────────────────────────────────────

func TestParseRightClick(t *testing.T) {
	cmd := mustParseLine(t, "RIGHT CLICK the 'File' element")
	if cmd.Type != CmdRightClick {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.Target != "File" {
		t.Errorf("target = %q", cmd.Target)
	}
}

// ── DRAG ──────────────────────────────────────────────────────────────────────

func TestParseDrag(t *testing.T) {
	cmd := mustParseLine(t, "DRAG the element 'Drag me to my target' and drop it into 'Drop here'")
	if cmd.Type != CmdDrag {
		t.Errorf("type = %s", cmd.Type)
	}
	if cmd.DragSource != "Drag me to my target" {
		t.Errorf("source = %q", cmd.DragSource)
	}
	if cmd.DragTarget != "Drop here" {
		t.Errorf("dropTarget = %q", cmd.DragTarget)
	}
}

// ── NEAR / ON / INSIDE qualifiers ─────────────────────────────────────────────

func TestParseNearQualifier(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Edit' button NEAR 'John Doe'")
	if cmd.NearAnchor != "John Doe" {
		t.Errorf("near = %q", cmd.NearAnchor)
	}
}

func TestParseOnRegion(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Logo' link ON HEADER")
	if cmd.OnRegion != "header" {
		t.Errorf("region = %q", cmd.OnRegion)
	}
}

func TestParseInsideQualifier(t *testing.T) {
	cmd := mustParseLine(t, "Click the 'Delete' button INSIDE 'Actions' row with 'John'")
	if cmd.InsideContainer != "Actions" {
		t.Errorf("inside = %q", cmd.InsideContainer)
	}
	if cmd.InsideRowText != "John" {
		t.Errorf("rowText = %q", cmd.InsideRowText)
	}
}

// ── STEP blocks and DONE ─────────────────────────────────────────────────────

func TestParseStepBlocks(t *testing.T) {
	src := `
STEP 1: First step
    NAVIGATE to https://example.com

STEP 2: Second step
    CLICK the 'Login' button
DONE.
`
	hunt := mustParse(t, src)
	if len(hunt.Commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(hunt.Commands))
	}
	if hunt.Commands[0].StepBlock != "STEP 1: First step" {
		t.Errorf("step[0] block = %q", hunt.Commands[0].StepBlock)
	}
	if hunt.Commands[1].StepBlock != "STEP 2: Second step" {
		t.Errorf("step[1] block = %q", hunt.Commands[1].StepBlock)
	}
}

// ── Comments and blank lines ─────────────────────────────────────────────────

func TestParseIgnoresComments(t *testing.T) {
	src := `# This is a comment
    # Indented comment
NAVIGATE to https://example.com
`
	hunt := mustParse(t, src)
	if len(hunt.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(hunt.Commands))
	}
}

// ── PRINT ─────────────────────────────────────────────────────────────────────

func TestParsePrint(t *testing.T) {
	cmd := mustParseLine(t, "PRINT 'Hello World'")
	if cmd.Type != CmdPrint {
		t.Errorf("type = %s", cmd.Type)
	}
}

// ── Variable substitution ─────────────────────────────────────────────────────

func TestParseVariableSubstitution(t *testing.T) {
	src := `@var: {url} = https://example.com
NAVIGATE to {url}
`
	hunt := mustParse(t, src)
	if len(hunt.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(hunt.Commands))
	}
	if hunt.Commands[0].URL != "https://example.com" {
		t.Errorf("url = %q, expected substituted value", hunt.Commands[0].URL)
	}
}

// ── Full .hunt parse ──────────────────────────────────────────────────────────

func TestParseMegaHuntStructure(t *testing.T) {
	src := `@context: Test context
@title: test_suite

STEP 1: Navigate
    NAVIGATE to https://example.com

STEP 2: Inputs
    FILL 'Name' field with 'John'
    FILL 'Email' field with 'john@test.com'
    VERIFY that 'John' is present

STEP 3: Interactions
    CLICK the 'Submit' button
    SELECT 'Option A' from the 'Dropdown' dropdown
    CHECK the checkbox for 'Terms'

STEP 4: Advanced
    DOUBLE CLICK the 'Copy' button
    DRAG the element 'Source' and drop it into 'Target'
    EXTRACT the 'Price' into {price}
    SCROLL DOWN
    SCROLL DOWN inside the 'list'

DONE.
`
	hunt := mustParse(t, src)
	if hunt.Title != "test_suite" {
		t.Errorf("title = %q", hunt.Title)
	}
	if hunt.Context != "Test context" {
		t.Errorf("context = %q", hunt.Context)
	}

	// Count by type
	counts := map[CommandType]int{}
	for _, cmd := range hunt.Commands {
		counts[cmd.Type]++
	}

	expect := map[CommandType]int{
		CmdNavigate:    1,
		CmdFill:        2,
		CmdVerify:      1,
		CmdClick:       1,
		CmdSelect:      1,
		CmdCheck:       1,
		CmdDoubleClick: 1,
		CmdDrag:        1,
		CmdExtract:     1,
		CmdScroll:      2,
	}
	for typ, want := range expect {
		if counts[typ] != want {
			t.Errorf("count(%s) = %d, want %d", typ, counts[typ], want)
		}
	}
}

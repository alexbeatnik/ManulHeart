// Package dsl provides the parser and command model for Manul-style .hunt files.
//
// The parser reads .hunt files into a structured AST that the runtime executes.
// It is deliberately simple but designed to be extended with variables,
// conditional blocks, imports, page abstractions, and hooks.
//
// Supported syntax (case-insensitive):
//
//	@context: description text
//	@title: suite name
//	@var: {name} = value
//
//	STEP 1: description
//	    NAVIGATE to 'https://example.com'
//	    Click the 'Login' button
//	    Fill 'Email' field with 'user@example.com'
//	    Type 'hello' into the 'Search' field
//	    Select 'Option' from the 'Dropdown' dropdown
//	    WAIT 2
//	    VERIFY that 'Welcome' is present
//	    VERIFY that 'Error' is NOT present
//	DONE.
package dsl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// CommandType classifies the kind of action a DSL command represents.
type CommandType string

const (
	CmdNavigate    CommandType = "navigate"
	CmdClick       CommandType = "click"
	CmdDoubleClick CommandType = "double_click"
	CmdFill        CommandType = "fill"
	CmdType        CommandType = "type"
	CmdSelect      CommandType = "select"
	CmdCheck       CommandType = "check"
	CmdUncheck     CommandType = "uncheck"
	CmdVerify      CommandType = "verify"
	CmdVerifySoft  CommandType = "verify_softly"
	CmdVerifyField CommandType = "verify_field"
	CmdWait        CommandType = "wait"
	CmdWaitFor     CommandType = "wait_for"
	CmdScroll      CommandType = "scroll"
	CmdPress       CommandType = "press"
	CmdExtract     CommandType = "extract"
	CmdSet         CommandType = "set"
	CmdIf          CommandType = "if"
	CmdElIf        CommandType = "elif"
	CmdElse        CommandType = "else"
	CmdEndIf       CommandType = "endif"
	CmdWhile       CommandType = "while"
	CmdEndWhile    CommandType = "endwhile"
	CmdRepeat      CommandType = "repeat"
	CmdEndRepeat   CommandType = "endrepeat"
	CmdUnknown     CommandType = "unknown"
)

// InteractionMode describes the expected interaction surface for element resolution.
type InteractionMode string

const (
	ModeClickable InteractionMode = "clickable"
	ModeInput     InteractionMode = "input"
	ModeSelect    InteractionMode = "select"
	ModeCheckbox  InteractionMode = "checkbox"
	ModeNone      InteractionMode = "none"
)

// Command is a single parsed DSL instruction.
type Command struct {
	// Raw is the original line text, preserved for explainability.
	Raw string
	// LineNumber is the 1-based line number in the source file.
	LineNumber int
	// Type is the classified command kind.
	Type CommandType
	// InteractionMode is the expected element interaction surface.
	InteractionMode InteractionMode
	// Target is the plain-English target expression (the text inside quotes).
	Target string
	// TypeHint is the optional element type hint (button, link, field, etc.).
	TypeHint string
	// Value is the text to type/fill, for input commands.
	Value string
	// URL is the navigation destination, for navigate commands.
	URL string
	// WaitSeconds is the duration for wait commands.
	WaitSeconds float64
	// VerifyText is the text to verify is present/absent on the page.
	VerifyText string
	// VerifyNegated is true for "VERIFY that 'X' is NOT present".
	VerifyNegated bool
	// VerifySoft is true for VERIFY SOFTLY — non-fatal, continues on failure.
	VerifySoft bool
	// VerifyFieldKind is "text", "value", or "placeholder" for VERIFY HAS TEXT/VALUE.
	VerifyFieldKind string
	// StepBlock is the STEP N label this command belongs to.
	StepBlock string
	// NearAnchor is the plain-English anchor text for NEAR contextual qualifier.
	NearAnchor string
	// OnRegion is the region qualifier: "header" or "footer".
	OnRegion string
	// InsideContainer is the INSIDE 'Container' qualifier target.
	InsideContainer string
	// InsideRowText is the optional row text for INSIDE 'Container' row with 'Text'.
	InsideRowText string
	// ScrollContainer is the target container for SCROLL DOWN inside 'container'.
	ScrollContainer string
	// ScrollDirection is "down" or "up" for SCROLL commands.
	ScrollDirection string
	// PressKey is the key or combo for PRESS commands (e.g. "Enter", "Control+A").
	PressKey string
	// PressTarget is the optional element for PRESS Key ON 'Target'.
	PressTarget string
	// ExtractVar is the variable name for EXTRACT into {var}.
	ExtractVar string
	// SetVar is the variable name for SET {var} = value.
	SetVar string
	// SetValue is the value for SET assignment.
	SetValue string
	// Condition is the raw condition text for IF/ELIF/WHILE.
	Condition string
	// RepeatCount is the iteration count for REPEAT N TIMES.
	RepeatCount int
	// RepeatVar is the loop variable name for REPEAT (default "i").
	RepeatVar string
	// WaitForState is "visible", "hidden", or "disappear" for WAIT FOR.
	WaitForState string
}

// Hunt is the parsed representation of a complete .hunt file.
type Hunt struct {
	// SourcePath is the file path, empty if parsed from a reader.
	SourcePath string
	// Title is the @title: header value.
	Title string
	// Context is the @context: header value.
	Context string
	// Vars holds file-level @var: declarations.
	Vars map[string]string
	// Commands is the ordered list of parsed commands.
	Commands []Command
}

// ── regex patterns (compiled once) ─────────────────────────────────────────

var (
	reQuoted    = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
	reStep      = regexp.MustCompile(`(?i)^STEP\s+\d+\s*:`)
	reNear      = regexp.MustCompile(`(?i)\bNEAR\s+(?:"([^"]*)"|'([^']*)')`)
	reOnRegion  = regexp.MustCompile(`(?i)\bON\s+(HEADER|FOOTER)\b`)
	reInside    = regexp.MustCompile(`(?i)\bINSIDE\s+(?:"([^"]*)"|'([^']*)')`)
	reInsideRow = regexp.MustCompile(`(?i)\brow\s+with\s+(?:"([^"]*)"|'([^']*)')`)
	reNavigate  = regexp.MustCompile(`(?i)\bNAVIGATE\b`)
	reClick     = regexp.MustCompile(`(?i)\bCLICK\b`)
	reDoubleClk = regexp.MustCompile(`(?i)\bDOUBLE\s+CLICK\b`)
	reFill      = regexp.MustCompile(`(?i)\bFILL\b`)
	reType      = regexp.MustCompile(`(?i)\bTYPE\b`)
	reSelect    = regexp.MustCompile(`(?i)\bSELECT\b`)
	reCheck     = regexp.MustCompile(`(?i)\bCHECK\b`)
	reUncheck   = regexp.MustCompile(`(?i)\bUNCHECK\b`)
	reVerify    = regexp.MustCompile(`(?i)\bVERIFY\b`)
	reVerifySoft = regexp.MustCompile(`(?i)\bVERIFY\s+SOFTLY\b`)
	reVerifyHas = regexp.MustCompile(`(?i)\bHAS\s+(TEXT|VALUE|PLACEHOLDER)\b`)
	reWait      = regexp.MustCompile(`(?i)^WAIT\s+(\d+(?:\.\d+)?)`)
	reWaitFor   = regexp.MustCompile(`(?i)^WAIT\s+FOR\b`)
	reWaitState = regexp.MustCompile(`(?i)\bTO\s+BE\s+(VISIBLE|HIDDEN)\b|\bTO\s+(DISAPPEAR)\b`)
	reNotPres   = regexp.MustCompile(`(?i)\bNOT\s+PRESENT\b`)
	reScroll    = regexp.MustCompile(`(?i)^SCROLL\b`)
	reScrollDir = regexp.MustCompile(`(?i)\b(DOWN|UP)\b`)
	reScrollIn  = regexp.MustCompile(`(?i)\binside\s+(?:the\s+)?(?:"([^"]*)"|'([^']*)')`)
	rePress     = regexp.MustCompile(`(?i)^PRESS\b`)
	rePressOn   = regexp.MustCompile(`(?i)\bON\s+(?:"([^"]*)"|'([^']*)')`)
	reExtract   = regexp.MustCompile(`(?i)^EXTRACT\b`)
	reExtractVar = regexp.MustCompile(`(?i)\binto\s+\{(\w+)\}`)
	reSet       = regexp.MustCompile(`(?i)^SET\s+\{?(\w+)\}?\s*=\s*(.+)`)
	reIf        = regexp.MustCompile(`(?i)^IF\s+(.+):\s*$`)
	reElIf      = regexp.MustCompile(`(?i)^ELIF\s+(.+):\s*$`)
	reElse      = regexp.MustCompile(`(?i)^ELSE\s*:\s*$`)
	reEndIf     = regexp.MustCompile(`(?i)^ENDIF\s*$`)
	reWhile     = regexp.MustCompile(`(?i)^WHILE\s+(.+):\s*$`)
	reEndWhile  = regexp.MustCompile(`(?i)^ENDWHILE\s*$`)
	reRepeat    = regexp.MustCompile(`(?i)^REPEAT\s+(\d+)\s+TIMES?\b`)
	reRepeatVar = regexp.MustCompile(`(?i)\bas\s+\{?(\w+)\}?`)
	reEndRepeat = regexp.MustCompile(`(?i)^ENDREPEAT\s*$`)
	reHeader    = regexp.MustCompile(`(?i)^@(\w+):\s*(.*)`)
	reVar       = regexp.MustCompile(`(?i)^@var:\s*\{(\w+)\}\s*=\s*(.*)`)

	// element type hints recognized after the quoted target
	typeHints = map[string]bool{
		"button": true, "link": true, "field": true, "dropdown": true,
		"checkbox": true, "radio": true, "element": true, "input": true,
		"textarea": true, "select": true, "option": true,
	}
)

// quotedMatch returns the captured text from a reQuoted or reNear match.
// The regex alternation produces two groups: [1] from double-quoted, [2] from
// single-quoted.  Exactly one of them will be non-empty.
func quotedMatch(m []string, groupOffset int) string {
	if m[groupOffset] != "" {
		return m[groupOffset]
	}
	return m[groupOffset+1]
}

// ParseFile reads and parses a .hunt file from disk.
func ParseFile(path string) (*Hunt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open hunt file %q: %w", path, err)
	}
	defer f.Close()

	hunt, err := Parse(f)
	if err != nil {
		return nil, err
	}
	hunt.SourcePath = path
	return hunt, nil
}

// Parse reads and parses .hunt content from an io.Reader.
func Parse(r io.Reader) (*Hunt, error) {
	hunt := &Hunt{
		Vars: make(map[string]string),
	}

	scanner := bufio.NewScanner(r)
	lineNum := 0
	currentStep := ""

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		// Skip blank lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// DONE. terminates the file
		if strings.EqualFold(trimmed, "done.") || strings.EqualFold(trimmed, "done") {
			break
		}

		// @var: declarations
		if m := reVar.FindStringSubmatch(trimmed); m != nil {
			hunt.Vars[m[1]] = strings.TrimSpace(m[2])
			continue
		}

		// @header: declarations
		if m := reHeader.FindStringSubmatch(trimmed); m != nil {
			key := strings.ToLower(m[1])
			val := strings.TrimSpace(m[2])
			switch key {
			case "title":
				hunt.Title = val
			case "context":
				hunt.Context = val
			}
			continue
		}

		// STEP N: label
		if reStep.MatchString(trimmed) {
			currentStep = trimmed
			continue
		}

		// Parse command
		cmd, err := parseCommand(trimmed, lineNum)
		if err != nil {
			return nil, err
		}
		cmd.StepBlock = currentStep
		// Apply variable substitution to all text fields
		applyVars(&cmd, hunt.Vars)
		hunt.Commands = append(hunt.Commands, cmd)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading hunt file: %w", err)
	}

	return hunt, nil
}

// parseCommand classifies and extracts fields from a single DSL line.
func parseCommand(line string, lineNum int) (Command, error) {
	cmd := Command{
		Raw:        line,
		LineNumber: lineNum,
	}

	upper := strings.ToUpper(line)

	// ── Control flow (check before action commands) ───────────────────

	if m := reIf.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdIf
		cmd.InteractionMode = ModeNone
		cmd.Condition = strings.TrimSpace(m[1])
		return cmd, nil
	}
	if m := reElIf.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdElIf
		cmd.InteractionMode = ModeNone
		cmd.Condition = strings.TrimSpace(m[1])
		return cmd, nil
	}
	if reElse.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdElse
		cmd.InteractionMode = ModeNone
		return cmd, nil
	}
	if reEndIf.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdEndIf
		cmd.InteractionMode = ModeNone
		return cmd, nil
	}
	if m := reWhile.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdWhile
		cmd.InteractionMode = ModeNone
		cmd.Condition = strings.TrimSpace(m[1])
		return cmd, nil
	}
	if reEndWhile.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdEndWhile
		cmd.InteractionMode = ModeNone
		return cmd, nil
	}
	if m := reRepeat.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdRepeat
		cmd.InteractionMode = ModeNone
		cmd.RepeatCount, _ = strconv.Atoi(m[1])
		cmd.RepeatVar = "i"
		if rv := reRepeatVar.FindStringSubmatch(line); rv != nil {
			cmd.RepeatVar = rv[1]
		}
		return cmd, nil
	}
	if reEndRepeat.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdEndRepeat
		cmd.InteractionMode = ModeNone
		return cmd, nil
	}

	// ── SET {var} = value ─────────────────────────────────────────────

	if m := reSet.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdSet
		cmd.InteractionMode = ModeNone
		cmd.SetVar = m[1]
		val := strings.TrimSpace(m[2])
		// Strip wrapping quotes from the value if present.
		if (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) ||
			(strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) {
			val = val[1 : len(val)-1]
		}
		cmd.SetValue = val
		return cmd, nil
	}

	switch {
	// NAVIGATE to 'url'  OR  NAVIGATE to https://...
	case reNavigate.MatchString(upper):
		cmd.Type = CmdNavigate
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.URL = quotedMatch(m, 1)
		} else {
			// bare URL after "to" (no quotes)
			parts := strings.Fields(line)
			for i, p := range parts {
				if strings.EqualFold(p, "to") && i+1 < len(parts) {
					cmd.URL = parts[i+1]
					break
				}
			}
		}

	// SCROLL [DOWN|UP] [inside 'container']
	case reScroll.MatchString(upper):
		cmd.Type = CmdScroll
		cmd.InteractionMode = ModeNone
		cmd.ScrollDirection = "down" // default
		if m := reScrollDir.FindStringSubmatch(line); m != nil {
			cmd.ScrollDirection = strings.ToLower(m[1])
		}
		if m := reScrollIn.FindStringSubmatch(line); m != nil {
			cmd.ScrollContainer = quotedMatch(m, 1)
		}

	// PRESS Key [ON 'Target']
	case rePress.MatchString(upper):
		cmd.Type = CmdPress
		cmd.InteractionMode = ModeNone
		// Extract key: everything between PRESS and (optional) ON 'target' or end
		pressText := strings.TrimSpace(line)
		// Remove "PRESS " prefix (case-insensitive)
		pressText = pressText[6:] // len("PRESS ") = 6
		pressText = strings.TrimSpace(pressText)
		if m := rePressOn.FindStringSubmatch(line); m != nil {
			cmd.PressTarget = quotedMatch(m, 1)
			// Remove the ON 'target' part from pressText
			onIdx := rePressOn.FindStringIndex(line)
			if onIdx != nil {
				pressText = strings.TrimSpace(line[6:onIdx[0]])
			}
		}
		cmd.PressKey = strings.TrimSpace(pressText)

	// EXTRACT the 'Target' into {var}
	case reExtract.MatchString(upper):
		cmd.Type = CmdExtract
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.Target = quotedMatch(m, 1)
		}
		if m := reExtractVar.FindStringSubmatch(line); m != nil {
			cmd.ExtractVar = m[1]
		}

	// WAIT FOR 'element' TO BE VISIBLE|HIDDEN|DISAPPEAR (before plain WAIT)
	case reWaitFor.MatchString(strings.TrimSpace(line)):
		cmd.Type = CmdWaitFor
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.Target = quotedMatch(m, 1)
		}
		cmd.WaitForState = "visible" // default
		if m := reWaitState.FindStringSubmatch(line); m != nil {
			if m[1] != "" {
				cmd.WaitForState = strings.ToLower(m[1])
			} else if m[2] != "" {
				cmd.WaitForState = "disappear"
			}
		}

	// WAIT N
	case reWait.MatchString(strings.TrimSpace(line)):
		cmd.Type = CmdWait
		cmd.InteractionMode = ModeNone
		m := reWait.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil {
			cmd.WaitSeconds, _ = strconv.ParseFloat(m[1], 64)
		}

	// VERIFY SOFTLY that 'text' is [NOT] present (before plain VERIFY)
	case reVerifySoft.MatchString(upper):
		cmd.Type = CmdVerifySoft
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.VerifyText = quotedMatch(m, 1)
		}
		cmd.VerifyNegated = reNotPres.MatchString(upper)
		cmd.VerifySoft = true

	// VERIFY 'Target' field HAS TEXT|VALUE|PLACEHOLDER 'expected'
	// VERIFY that 'text' is [NOT] present
	case reVerify.MatchString(upper):
		if reVerifyHas.MatchString(upper) {
			cmd.Type = CmdVerifyField
			cmd.InteractionMode = ModeNone
			quotes := reQuoted.FindAllStringSubmatch(line, -1)
			if len(quotes) >= 2 {
				cmd.Target = quotedMatch(quotes[0], 1)
				cmd.Value = quotedMatch(quotes[1], 1)
			} else if len(quotes) == 1 {
				cmd.Target = quotedMatch(quotes[0], 1)
			}
			if m := reVerifyHas.FindStringSubmatch(upper); m != nil {
				cmd.VerifyFieldKind = strings.ToLower(m[1])
			}
		} else {
			cmd.Type = CmdVerify
			cmd.InteractionMode = ModeNone
			if m := reQuoted.FindStringSubmatch(line); m != nil {
				cmd.VerifyText = quotedMatch(m, 1)
			}
			cmd.VerifyNegated = reNotPres.MatchString(upper)
		}

	// UNCHECK (before CHECK to avoid substring match)
	case reUncheck.MatchString(upper):
		cmd.Type = CmdUncheck
		cmd.InteractionMode = ModeCheckbox
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	// CHECK
	case reCheck.MatchString(upper):
		cmd.Type = CmdCheck
		cmd.InteractionMode = ModeCheckbox
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	// DOUBLE CLICK (before CLICK)
	case reDoubleClk.MatchString(upper):
		cmd.Type = CmdDoubleClick
		cmd.InteractionMode = ModeClickable
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	// SELECT 'option' from 'target' dropdown
	case reSelect.MatchString(upper):
		cmd.Type = CmdSelect
		cmd.InteractionMode = ModeSelect
		quotes := reQuoted.FindAllStringSubmatch(line, -1)
		if len(quotes) >= 2 {
			cmd.Value = quotedMatch(quotes[0], 1)
			cmd.Target = quotedMatch(quotes[1], 1)
		} else if len(quotes) == 1 {
			cmd.Target = quotedMatch(quotes[0], 1)
		}
		cmd.TypeHint = "dropdown"

	// FILL 'target' field with 'value'
	case reFill.MatchString(upper):
		cmd.Type = CmdFill
		cmd.InteractionMode = ModeInput
		quotes := reQuoted.FindAllStringSubmatch(line, -1)
		if len(quotes) >= 2 {
			cmd.Target = quotedMatch(quotes[0], 1)
			cmd.Value = quotedMatch(quotes[1], 1)
		} else if len(quotes) == 1 {
			cmd.Target = quotedMatch(quotes[0], 1)
		}
		cmd.TypeHint = extractHintAfterTarget(line, cmd.Target)
		if cmd.TypeHint == "" {
			cmd.TypeHint = "field"
		}

	// TYPE 'value' into 'target'
	case reType.MatchString(upper):
		cmd.Type = CmdType
		cmd.InteractionMode = ModeInput
		quotes := reQuoted.FindAllStringSubmatch(line, -1)
		if len(quotes) >= 2 {
			cmd.Value = quotedMatch(quotes[0], 1)
			cmd.Target = quotedMatch(quotes[1], 1)
		} else if len(quotes) == 1 {
			cmd.Target = quotedMatch(quotes[0], 1)
		}
		cmd.TypeHint = "field"

	// CLICK the 'target' [hint] [NEAR 'anchor'] [ON HEADER|FOOTER] [INSIDE 'container']
	case reClick.MatchString(upper):
		cmd.Type = CmdClick
		cmd.InteractionMode = ModeClickable
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	default:
		cmd.Type = CmdUnknown
	}

	// ── Contextual qualifiers (apply to any target command) ───────────

	if m := reNear.FindStringSubmatch(line); m != nil {
		cmd.NearAnchor = quotedMatch(m, 1)
	}
	if m := reOnRegion.FindStringSubmatch(line); m != nil {
		cmd.OnRegion = strings.ToLower(m[1])
	}
	if m := reInside.FindStringSubmatch(line); m != nil {
		cmd.InsideContainer = quotedMatch(m, 1)
		if m2 := reInsideRow.FindStringSubmatch(line); m2 != nil {
			cmd.InsideRowText = quotedMatch(m2, 1)
		}
	}

	return cmd, nil
}

// extractTargetAndHint returns the quoted target text and optional element-type hint
// for commands like: Click the 'Login' button
func extractTargetAndHint(line string) (target, hint string) {
	m := reQuoted.FindStringSubmatch(line)
	if m != nil {
		target = quotedMatch(m, 1)
	}
	hint = extractHintAfterTarget(line, target)
	return
}

// extractHintAfterTarget finds the element-type hint word after the last quoted target.
func extractHintAfterTarget(line, target string) string {
	if target == "" {
		return ""
	}
	// Find the position of the closing quote of the target (single or double)
	idx := strings.LastIndex(line, "'"+target+"'")
	closingLen := len(target) + 2
	if idx < 0 {
		idx = strings.LastIndex(line, "\""+target+"\"")
	}
	if idx < 0 {
		return ""
	}
	after := strings.ToLower(strings.TrimSpace(line[idx+closingLen:]))
	for word := range typeHints {
		if strings.HasPrefix(after, word) {
			return word
		}
	}
	return ""
}

// applyVars substitutes {varName} placeholders in all text fields of a command.
func applyVars(cmd *Command, vars map[string]string) {
	if len(vars) == 0 {
		return
	}
	sub := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{"+k+"}", v)
		}
		return s
	}
	cmd.URL = sub(cmd.URL)
	cmd.Target = sub(cmd.Target)
	cmd.Value = sub(cmd.Value)
	cmd.VerifyText = sub(cmd.VerifyText)
	cmd.NearAnchor = sub(cmd.NearAnchor)
	cmd.TypeHint = sub(cmd.TypeHint)
	cmd.PressTarget = sub(cmd.PressTarget)
	cmd.ScrollContainer = sub(cmd.ScrollContainer)
	cmd.InsideContainer = sub(cmd.InsideContainer)
	cmd.InsideRowText = sub(cmd.InsideRowText)
	cmd.SetValue = sub(cmd.SetValue)
	cmd.Condition = sub(cmd.Condition)
}

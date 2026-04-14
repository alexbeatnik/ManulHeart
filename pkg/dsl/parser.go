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
	CmdNavigate CommandType = "navigate"
	CmdClick    CommandType = "click"
	CmdFill     CommandType = "fill"
	CmdType     CommandType = "type"
	CmdSelect   CommandType = "select"
	CmdCheck    CommandType = "check"
	CmdUncheck  CommandType = "uncheck"
	CmdVerify   CommandType = "verify"
	CmdWait     CommandType = "wait"
	CmdUnknown  CommandType = "unknown"
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
	// StepBlock is the STEP N label this command belongs to.
	StepBlock string
	// NearAnchor is the plain-English anchor text for NEAR contextual qualifier.
	NearAnchor string
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
	reQuoted    = regexp.MustCompile(`'([^']*)'`)
	reStep      = regexp.MustCompile(`(?i)^STEP\s+\d+\s*:`)
	reNear      = regexp.MustCompile(`(?i)\bNEAR\s+'([^']*)'`)
	reNavigate  = regexp.MustCompile(`(?i)\bNAVIGATE\b`)
	reClick     = regexp.MustCompile(`(?i)\bCLICK\b`)
	reDoubleClk = regexp.MustCompile(`(?i)\bDOUBLE\s+CLICK\b`)
	reFill      = regexp.MustCompile(`(?i)\bFILL\b`)
	reType      = regexp.MustCompile(`(?i)\bTYPE\b`)
	reSelect    = regexp.MustCompile(`(?i)\bSELECT\b`)
	reCheck     = regexp.MustCompile(`(?i)\bCHECK\b`)
	reUncheck   = regexp.MustCompile(`(?i)\bUNCHECK\b`)
	reVerify    = regexp.MustCompile(`(?i)\bVERIFY\b`)
	reWait      = regexp.MustCompile(`(?i)^WAIT\s+(\d+(?:\.\d+)?)`)
	reNotPres   = regexp.MustCompile(`(?i)\bNOT\s+PRESENT\b`)
	reHeader    = regexp.MustCompile(`(?i)^@(\w+):\s*(.*)`)
	reVar       = regexp.MustCompile(`(?i)^@var:\s*\{(\w+)\}\s*=\s*(.*)`)

	// element type hints recognized after the quoted target
	typeHints = map[string]bool{
		"button": true, "link": true, "field": true, "dropdown": true,
		"checkbox": true, "radio": true, "element": true, "input": true,
		"textarea": true, "select": true, "option": true,
	}
)

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

	switch {
	// NAVIGATE to 'url'  OR  NAVIGATE to https://...
	case reNavigate.MatchString(upper):
		cmd.Type = CmdNavigate
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.URL = m[1]
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

	// WAIT N
	case reWait.MatchString(upper):
		cmd.Type = CmdWait
		cmd.InteractionMode = ModeNone
		m := reWait.FindStringSubmatch(line)
		if m != nil {
			cmd.WaitSeconds, _ = strconv.ParseFloat(m[1], 64)
		}

	// VERIFY that 'text' is [NOT] present
	case reVerify.MatchString(upper):
		cmd.Type = CmdVerify
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.VerifyText = m[1]
		}
		cmd.VerifyNegated = reNotPres.MatchString(upper)

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
		cmd.Type = CmdClick
		cmd.InteractionMode = ModeClickable
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	// SELECT 'option' from 'target' dropdown
	case reSelect.MatchString(upper):
		cmd.Type = CmdSelect
		cmd.InteractionMode = ModeSelect
		quotes := reQuoted.FindAllStringSubmatch(line, -1)
		if len(quotes) >= 2 {
			cmd.Value = quotes[0][1]
			cmd.Target = quotes[1][1]
		} else if len(quotes) == 1 {
			cmd.Target = quotes[0][1]
		}
		cmd.TypeHint = "dropdown"

	// FILL 'target' field with 'value'
	case reFill.MatchString(upper):
		cmd.Type = CmdFill
		cmd.InteractionMode = ModeInput
		quotes := reQuoted.FindAllStringSubmatch(line, -1)
		if len(quotes) >= 2 {
			cmd.Target = quotes[0][1]
			cmd.Value = quotes[1][1]
		} else if len(quotes) == 1 {
			cmd.Target = quotes[0][1]
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
			cmd.Value = quotes[0][1]
			cmd.Target = quotes[1][1]
		} else if len(quotes) == 1 {
			cmd.Target = quotes[0][1]
		}
		cmd.TypeHint = "field"

	// CLICK the 'target' [hint] [NEAR 'anchor']
	case reClick.MatchString(upper):
		cmd.Type = CmdClick
		cmd.InteractionMode = ModeClickable
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)
		if m := reNear.FindStringSubmatch(line); m != nil {
			cmd.NearAnchor = m[1]
		}

	default:
		cmd.Type = CmdUnknown
	}

	return cmd, nil
}

// extractTargetAndHint returns the quoted target text and optional element-type hint
// for commands like: Click the 'Login' button
func extractTargetAndHint(line string) (target, hint string) {
	m := reQuoted.FindStringSubmatch(line)
	if m != nil {
		target = m[1]
	}
	hint = extractHintAfterTarget(line, target)
	return
}

// extractHintAfterTarget finds the element-type hint word after the last quoted target.
func extractHintAfterTarget(line, target string) string {
	if target == "" {
		return ""
	}
	// Find the position of the closing quote of the target
	idx := strings.LastIndex(line, "'"+target+"'")
	if idx < 0 {
		return ""
	}
	after := strings.ToLower(strings.TrimSpace(line[idx+len(target)+2:]))
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
}

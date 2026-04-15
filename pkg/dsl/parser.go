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
	CmdSet             CommandType = "set"
	CmdHover           CommandType = "hover"
	CmdRightClick      CommandType = "right_click"
	CmdDrag            CommandType = "drag"
	CmdUpload          CommandType = "upload"
	CmdPrint           CommandType = "print"
	CmdWaitForResponse CommandType = "wait_for_response"
	CmdForEach         CommandType = "for_each"
	CmdEndForEach      CommandType = "end_for_each"
	CmdPause           CommandType = "pause"
	CmdDebugVars       CommandType = "debug_vars"
	CmdIf              CommandType = "if"
	CmdElIf            CommandType = "elif"
	CmdElse            CommandType = "else"
	CmdWhile           CommandType = "while"
	CmdRepeat          CommandType = "repeat"
	CmdUnknown         CommandType = "unknown"
)

// InteractionMode describes the expected interaction surface for element resolution.
type InteractionMode string

const (
	ModeClickable InteractionMode = "clickable"
	ModeInput     InteractionMode = "input"
	ModeSelect    InteractionMode = "select"
	ModeCheckbox  InteractionMode = "checkbox"
	ModeHover     InteractionMode = "hover"
	ModeDrag      InteractionMode = "drag"
	ModeLocate    InteractionMode = "locate"
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
	// WaitResponseURL is the URL pattern for WAIT FOR RESPONSE.
	WaitResponseURL string
	// DragSource is the source element for DRAG AND DROP.
	DragSource string
	// DragTarget is the target element for DRAG AND DROP.
	DragTarget string
	// UploadFile is the file path for UPLOAD commands.
	UploadFile string
	// PrintText is the message for PRINT commands.
	PrintText string
	// ForEachVar is the loop variable name for FOR EACH.
	ForEachVar string
	// ForEachCollection is the collection variable for FOR EACH.
	ForEachCollection string

	// Indent is the indentation level (in characters) of the raw source line.
	Indent int

	// Branches holds the IF/ELIF/ELSE branches (only for CmdIf).
	Branches []IfBranch

	// Body holds the nested commands for WHILE/REPEAT loop bodies and IF branch bodies.
	Body []Command
}

// IfBranch is a single branch in an IF/ELIF/ELSE block.
type IfBranch struct {
	Kind      string    // "if", "elif", "else"
	Condition string
	Body      []Command
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
	// Tags holds file-level @tags: values.
	Tags []string
	// DataFile is the @data: path to a CSV/JSON file for parameterized runs.
	DataFile string
	// Schedule is the @schedule: expression for daemon mode.
	Schedule string
	// Exports holds the @export: block names.
	Exports []string
	// Imports holds @import: directives.
	Imports []ImportDirective
	// Blueprints holds imported step blocks available for USE directives.
	// Key is the block name (or alias), value is the list of commands.
	Blueprints map[string][]Command
	// Commands is the ordered list of parsed commands.
	Commands []Command
}

// ImportDirective describes a single @import: line.
type ImportDirective struct {
	Names   []string          // imported block names (or ["*"] for wildcard)
	Aliases map[string]string // optional aliases (original -> alias)
	Source  string            // source file path
}

// ── regex patterns (compiled once) ─────────────────────────────────────────

var (
	reQuoted    = regexp.MustCompile(`"([^"]*)"|'([^']*)'`)
	reStep      = regexp.MustCompile(`(?i)^STEP\s+\d+\s*:`)
	reNear      = regexp.MustCompile(`(?i)\bNEAR\s+(?:"([^"]*)"|'([^')*]*)')`)
	reOnRegion  = regexp.MustCompile(`(?i)\bON\s+(HEADER|FOOTER)\b`)
	reInside    = regexp.MustCompile(`(?i)\bINSIDE\s+(?:"([^"]*)"|'([^']*)')`)
	reInsideRow = regexp.MustCompile(`(?i)\brow\s+with\s+(?:"([^"]*)"|'([^']*)')`)
	reNavigate  = regexp.MustCompile(`(?i)\bNAVIGATE\b`)
	reClick     = regexp.MustCompile(`(?i)\bCLICK\b`)
	reDoubleClk = regexp.MustCompile(`(?i)\bDOUBLE\s+CLICK\b`)
	reFill      = regexp.MustCompile(`(?i)\bFILL\b`)
	reType      = regexp.MustCompile(`(?i)\bTYPE\b`)
	reSelect    = regexp.MustCompile(`(?i)^SELECT\b`)
	reCheck     = regexp.MustCompile(`(?i)\bCHECK\b`)
	reUncheck   = regexp.MustCompile(`(?i)\bUNCHECK\b`)
	reVerify    = regexp.MustCompile(`(?i)^VERIFY\b`)
	reVerifySoft = regexp.MustCompile(`(?i)^VERIFY\s+SOFTLY\b`)
	reVerifyHas = regexp.MustCompile(`(?i)\bHAS\s+(TEXT|VALUE|PLACEHOLDER)\b`)
	reWait      = regexp.MustCompile(`(?i)^WAIT\s+(\d+(?:\.\d+)?)`)
	reWaitFor   = regexp.MustCompile(`(?i)^WAIT\s+FOR\b`)
	reWaitState = regexp.MustCompile(`(?i)\bTO\s+BE\s+(VISIBLE|HIDDEN)\b|\bTO\s+(DISAPPEAR)\b`)
	reNotPres   = regexp.MustCompile(`(?i)\bNOT\s+PRESENT\b`)
	reScroll    = regexp.MustCompile(`(?i)^SCROLL\b`)
	reScrollDir = regexp.MustCompile(`(?i)\b(DOWN|UP)\b`)
	reScrollIn  = regexp.MustCompile(`(?i)\binside\s+(?:the\s+)?(?:"([^"]*)"|'([^']*)'|(\S.*))`)
	rePress     = regexp.MustCompile(`(?i)^PRESS\b`)
	rePressOn   = regexp.MustCompile(`(?i)\bON\s+(?:"([^"]*)"|'([^']*)')`)
	reExtract   = regexp.MustCompile(`(?i)^EXTRACT\b`)
	reExtractVar = regexp.MustCompile(`(?i)\binto\s+\{(\w+)\}`)
	reSet       = regexp.MustCompile(`(?i)^SET\s+\{?(\w+)\}?\s*=\s*(.+)`)
	reIf        = regexp.MustCompile(`(?i)^IF\s+(.+):\s*$`)
	reElIf      = regexp.MustCompile(`(?i)^ELIF\s+(.+):\s*$`)
	reElse      = regexp.MustCompile(`(?i)^ELSE\s*:\s*$`)
	reWhile     = regexp.MustCompile(`(?i)^WHILE\s+(.+):\s*$`)
	reRepeat    = regexp.MustCompile(`(?i)^REPEAT\s+(\d+)\s+TIMES?\s*:?\s*$`)
	reRepeatVar = regexp.MustCompile(`(?i)\bas\s+\{?(\w+)\}?`)
	reHeader    = regexp.MustCompile(`(?i)^@(\w+):\s*(.*)`)
	reVar       = regexp.MustCompile(`(?i)^@var:\s*\{(\w+)\}\s*=\s*(.*)`)

	// New command patterns
	reHover        = regexp.MustCompile(`(?i)\bHOVER\b`)
	reRightClick   = regexp.MustCompile(`(?i)\bRIGHT\s+CLICK\b`)
	reDrag         = regexp.MustCompile(`(?i)\bDRAG\b`)
	reDragDrop     = regexp.MustCompile(`(?i)\bdrop\s+(?:it\s+)?(?:into|on|onto)\s+(?:"([^"]*)"|'([^')']*)')`)
	reUpload       = regexp.MustCompile(`(?i)^UPLOAD\b`)
	rePrint        = regexp.MustCompile(`(?i)^PRINT\b`)
	reWaitResp     = regexp.MustCompile(`(?i)^WAIT\s+FOR\s+RESPONSE\b`)
	reWaitRespURL  = regexp.MustCompile(`(?:"([^"]*)"|'([^']*)')`) // reuse reQuoted for URL pattern
	reForEach      = regexp.MustCompile(`(?i)^FOR\s+EACH\s+\{?(\w+)\}?\s+IN\s+\{?(\w+)\}?\s*:\s*$`)
	reEndForEach   = regexp.MustCompile(`(?i)^END\s+FOR\s+EACH\s*$`)
	rePause        = regexp.MustCompile(`(?i)^PAUSE\s*$`)
	reDebugVars    = regexp.MustCompile(`(?i)^DEBUG\s+VARS\s*$`)
	reImport       = regexp.MustCompile(`(?i)^@import:\s*(.+)\s+from\s+(.+)$`)
	reTags         = regexp.MustCompile(`(?i)^@tags:\s*(.+)$`)
	reData         = regexp.MustCompile(`(?i)^@data:\s*(.+)$`)
	reSchedule     = regexp.MustCompile(`(?i)^@schedule:\s*(.+)$`)
	reExport       = regexp.MustCompile(`(?i)^@export:\s*(.+)$`)

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
		Vars:    make(map[string]string),
		Imports: nil,
	}

	scanner := bufio.NewScanner(r)
	lineNum := 0
	currentStep := ""

	// Flat list of parsed commands with indentation preserved.
	var flatCmds []rawCmd

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

		// @tags: declarations
		if m := reTags.FindStringSubmatch(trimmed); m != nil {
			for _, t := range strings.Split(m[1], ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					hunt.Tags = append(hunt.Tags, t)
				}
			}
			continue
		}

		// @data: declaration
		if m := reData.FindStringSubmatch(trimmed); m != nil {
			hunt.DataFile = strings.TrimSpace(m[1])
			continue
		}

		// @schedule: declaration
		if m := reSchedule.FindStringSubmatch(trimmed); m != nil {
			hunt.Schedule = strings.TrimSpace(m[1])
			continue
		}

		// @export: declaration
		if m := reExport.FindStringSubmatch(trimmed); m != nil {
			for _, e := range strings.Split(m[1], ",") {
				e = strings.TrimSpace(e)
				if e != "" {
					hunt.Exports = append(hunt.Exports, e)
				}
			}
			continue
		}

		// @import: declaration
		if m := reImport.FindStringSubmatch(trimmed); m != nil {
			directive := parseImportDirective(m[1], strings.TrimSpace(m[2]))
			hunt.Imports = append(hunt.Imports, directive)
			continue
		}

		// @header: declarations
		if m := reHeader.FindStringSubmatch(trimmed); m != nil {
			key := strings.ToLower(m[1])
			val := strings.TrimSpace(m[2])
			switch key {
			case "title", "blueprint":
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

		// Compute indentation (number of leading spaces/tabs as chars).
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		// Parse command
		cmd, err := parseCommand(trimmed, lineNum)
		if err != nil {
			return nil, err
		}
		cmd.StepBlock = currentStep
		cmd.Indent = indent
		// Apply variable substitution to all text fields
		applyVars(&cmd, hunt.Vars)
		flatCmds = append(flatCmds, rawCmd{cmd: cmd, indent: indent})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading hunt file: %w", err)
	}

	// Nest IF/ELIF/ELSE and WHILE/REPEAT blocks by indentation.
	hunt.Commands = nestBlocks(flatCmds)

	return hunt, nil
}

// indentLevel returns the indentation of a rawCmd.
type rawCmd struct {
	cmd    Command
	indent int
}

// nestBlocks converts a flat list of commands (with indentation levels) into
// a tree structure where IF/ELIF/ELSE branches and WHILE/REPEAT bodies are
// nested inside their parent commands. This mirrors ManulEngine's
// indentation-based block detection.
func nestBlocks(cmds []rawCmd) []Command {
	result, _ := nestBlocksFrom(cmds, 0, -1)
	return result
}

// nestBlocksFrom nests commands starting at index `start`, collecting commands
// whose indentation > parentIndent. Returns nested commands and the next index.
func nestBlocksFrom(cmds []rawCmd, start int, parentIndent int) ([]Command, int) {
	var result []Command
	i := start

loop:
	for i < len(cmds) {
		rc := cmds[i]

		// If this line's indent is at or below the parent's indent, we're done
		// with this block (unless parentIndent is -1, meaning top-level).
		if parentIndent >= 0 && rc.indent <= parentIndent {
			break
		}

		switch rc.cmd.Type {
		case CmdIf:
			// Collect IF/ELIF/ELSE branches by indentation.
			ifCmd, nextIdx := consumeIfBlock(cmds, i)
			result = append(result, ifCmd)
			i = nextIdx

		case CmdWhile, CmdRepeat, CmdForEach:
			// Collect loop body: all lines indented deeper than the header.
			headerIndent := rc.indent
			loopCmd := rc.cmd
			i++
			body, nextIdx := nestBlocksFrom(cmds, i, headerIndent)
			loopCmd.Body = body
			result = append(result, loopCmd)
			i = nextIdx

		case CmdElIf, CmdElse:
			// These should only appear inside consumeIfBlock. If we hit them
			// at top level, include them as-is (for single-line parsing).
			// Inside a nested block, treat as end of current block.
			if parentIndent >= 0 {
				break loop
			}
			result = append(result, rc.cmd)
			i++

		default:
			result = append(result, rc.cmd)
			i++
		}
	}
	return result, i
}

// consumeIfBlock parses an IF/ELIF/ELSE block starting at cmds[start].
// Uses indentation to determine branch boundaries.
// Returns the constructed IF Command (with Branches populated) and the next index.
func consumeIfBlock(cmds []rawCmd, start int) (Command, int) {
	ifCmd := cmds[start].cmd
	headerIndent := cmds[start].indent
	i := start

	var branches []IfBranch

	for i < len(cmds) {
		rc := cmds[i]

		// After the first IF, subsequent lines at header indent that are
		// ELIF/ELSE are sibling branches. Lines at lower indent or
		// non-ELIF/ELSE at header indent end the block.
		if i > start && rc.indent <= headerIndent {
			if rc.indent < headerIndent {
				// Lower indent than header → belongs to an outer block.
				break
			}
			// Same indent as header.
			if rc.cmd.Type != CmdElIf && rc.cmd.Type != CmdElse {
				break
			}
		}

		switch rc.cmd.Type {
		case CmdIf:
			if i != start {
				// Nested IF inside a branch body — will be handled by nestBlocksFrom.
				break
			}
			branch := IfBranch{
				Kind:      "if",
				Condition: rc.cmd.Condition,
			}
			i++
			body, nextIdx := nestBlocksFrom(cmds, i, headerIndent)
			branch.Body = body
			branches = append(branches, branch)
			i = nextIdx

		case CmdElIf:
			branch := IfBranch{
				Kind:      "elif",
				Condition: rc.cmd.Condition,
			}
			i++
			body, nextIdx := nestBlocksFrom(cmds, i, headerIndent)
			branch.Body = body
			branches = append(branches, branch)
			i = nextIdx

		case CmdElse:
			branch := IfBranch{
				Kind:      "else",
				Condition: "",
			}
			i++
			body, nextIdx := nestBlocksFrom(cmds, i, headerIndent)
			branch.Body = body
			branches = append(branches, branch)
			i = nextIdx

		default:
			// Should not reach here (handled by the break conditions above).
			break
		}

		// If we hit a non-ELIF/ELSE at header indent, stop.
		if i < len(cmds) && cmds[i].indent <= headerIndent &&
			cmds[i].cmd.Type != CmdElIf && cmds[i].cmd.Type != CmdElse {
			break
		}
	}

	ifCmd.Branches = branches
	return ifCmd, i
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
	if m := reWhile.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdWhile
		cmd.InteractionMode = ModeNone
		cmd.Condition = strings.TrimSpace(m[1])
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

	// FOR EACH {item} IN {collection}:
	if m := reForEach.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		cmd.Type = CmdForEach
		cmd.InteractionMode = ModeNone
		cmd.ForEachVar = m[1]
		cmd.ForEachCollection = m[2]
		return cmd, nil
	}

	// END FOR EACH
	if reEndForEach.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdEndForEach
		cmd.InteractionMode = ModeNone
		return cmd, nil
	}

	// PAUSE
	if rePause.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdPause
		cmd.InteractionMode = ModeNone
		return cmd, nil
	}

	// DEBUG VARS
	if reDebugVars.MatchString(strings.TrimSpace(line)) {
		cmd.Type = CmdDebugVars
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

	// WAIT FOR RESPONSE "url_pattern" (before other WAIT variants)
	case reWaitResp.MatchString(strings.TrimSpace(line)):
		cmd.Type = CmdWaitForResponse
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.WaitResponseURL = quotedMatch(m, 1)
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
			if m[1] != "" {
				cmd.ScrollContainer = m[1]
			} else if m[2] != "" {
				cmd.ScrollContainer = m[2]
			} else if m[3] != "" {
				cmd.ScrollContainer = strings.TrimSpace(m[3])
			}
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

	// PRINT 'message'
	case rePrint.MatchString(strings.TrimSpace(line)):
		cmd.Type = CmdPrint
		cmd.InteractionMode = ModeNone
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.PrintText = quotedMatch(m, 1)
		} else {
			// bare text after PRINT
			cmd.PrintText = strings.TrimSpace(strings.TrimSpace(line)[5:])
		}

	// UPLOAD 'file.pdf' to 'Target'
	case reUpload.MatchString(strings.TrimSpace(line)):
		cmd.Type = CmdUpload
		cmd.InteractionMode = ModeLocate
		quotes := reQuoted.FindAllStringSubmatch(line, -1)
		if len(quotes) >= 2 {
			cmd.UploadFile = quotedMatch(quotes[0], 1)
			cmd.Target = quotedMatch(quotes[1], 1)
		} else if len(quotes) == 1 {
			cmd.UploadFile = quotedMatch(quotes[0], 1)
		}

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

	// RIGHT CLICK (before CLICK to avoid substring match)
	case reRightClick.MatchString(upper):
		cmd.Type = CmdRightClick
		cmd.InteractionMode = ModeClickable
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	// DOUBLE CLICK (before CLICK)
	case reDoubleClk.MatchString(upper):
		cmd.Type = CmdDoubleClick
		cmd.InteractionMode = ModeClickable
		cmd.Target, cmd.TypeHint = extractTargetAndHint(line)

	// DRAG 'Source' and drop [it] on|into|onto 'Target'
	case reDrag.MatchString(upper):
		cmd.Type = CmdDrag
		cmd.InteractionMode = ModeDrag
		// First quoted = source
		if m := reQuoted.FindStringSubmatch(line); m != nil {
			cmd.DragSource = quotedMatch(m, 1)
		}
		// Drop target
		if m := reDragDrop.FindStringSubmatch(line); m != nil {
			cmd.DragTarget = quotedMatch(m, 1)
		}

	// HOVER over 'Target'
	case reHover.MatchString(upper):
		cmd.Type = CmdHover
		cmd.InteractionMode = ModeHover
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
	cmd.PrintText = sub(cmd.PrintText)
	cmd.UploadFile = sub(cmd.UploadFile)
	cmd.DragSource = sub(cmd.DragSource)
	cmd.DragTarget = sub(cmd.DragTarget)
	cmd.WaitResponseURL = sub(cmd.WaitResponseURL)
}

// parseImportDirective parses the names and source from an @import line.
// namesStr: "Login, Logout" or "Login as AuthLogin" or "*"
// source: path to source file
func parseImportDirective(namesStr, source string) ImportDirective {
	d := ImportDirective{
		Source:  source,
		Aliases: make(map[string]string),
	}

	// Strip surrounding quotes from source if present.
	if len(source) >= 2 {
		if (source[0] == '\'' && source[len(source)-1] == '\'') ||
			(source[0] == '"' && source[len(source)-1] == '"') {
			d.Source = source[1 : len(source)-1]
		}
	}

	parts := strings.Split(namesStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Check for "Name as Alias" syntax.
		asParts := regexp.MustCompile(`(?i)\s+as\s+`).Split(p, 2)
		name := strings.TrimSpace(asParts[0])
		d.Names = append(d.Names, name)
		if len(asParts) == 2 {
			d.Aliases[name] = strings.TrimSpace(asParts[1])
		}
	}
	return d
}

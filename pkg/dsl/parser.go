// Package dsl implements the ManulHeart .hunt DSL parser.
//
// A .hunt file is a sequence of natural-language-style automation commands
// with optional @header directives, STEP blocks, and control-flow constructs.
//
// The grammar is intentionally flexible — the parser uses keyword matching
// rather than strict BNF, prioritising human-readability over rigidity.
package dsl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ── Command types ──────────────────────────────────────────────────────────────

// CommandType is the DSL verb for a command.
type CommandType string

const (
	CmdNavigate     CommandType = "NAVIGATE"
	CmdClick        CommandType = "CLICK"
	CmdDoubleClick  CommandType = "DOUBLE_CLICK"
	CmdRightClick   CommandType = "RIGHT_CLICK"
	CmdFill         CommandType = "FILL"
	CmdType         CommandType = "TYPE"
	CmdSelect       CommandType = "SELECT"
	CmdCheck        CommandType = "CHECK"
	CmdUncheck      CommandType = "UNCHECK"
	CmdVerify       CommandType = "VERIFY"
	CmdVerifySoft   CommandType = "VERIFY_SOFT"
	CmdExtract      CommandType = "EXTRACT"
	CmdScroll       CommandType = "SCROLL"
	CmdPress        CommandType = "PRESS"
	CmdWait         CommandType = "WAIT"
	CmdWaitFor      CommandType = "WAIT_FOR"
	CmdWaitResponse CommandType = "WAIT_FOR_RESPONSE"
	CmdHover        CommandType = "HOVER"
	CmdDrag         CommandType = "DRAG"
	CmdSet          CommandType = "SET"
	CmdPrint        CommandType = "PRINT"
	CmdScreenshot   CommandType = "SCREENSHOT"
	CmdHighlight    CommandType = "HIGHLIGHT"
	CmdRepeat       CommandType = "REPEAT"
	CmdForEach      CommandType = "FOR_EACH"
	CmdWhile        CommandType = "WHILE"
	CmdIf           CommandType = "IF"
	CmdElseIf       CommandType = "ELIF"
	CmdElse         CommandType = "ELSE"
	CmdEndIf        CommandType = "END_IF"
	CmdEndFor       CommandType = "END_FOR"
	CmdEndWhile     CommandType = "END_WHILE"
	CmdEndRepeat    CommandType = "END_REPEAT"
	CmdCallStep     CommandType = "CALL_STEP"
	CmdUploadFile   CommandType = "UPLOAD_FILE"
	CmdUnknown      CommandType = "UNKNOWN"
)

// InteractionMode controls which elements are eligible for targeting.
type InteractionMode string

const (
	ModeClickable InteractionMode = "clickable"
	ModeInput     InteractionMode = "input"
	ModeCheckbox  InteractionMode = "checkbox"
	ModeSelect    InteractionMode = "select"
	ModeNone      InteractionMode = "none"
)

// ── Structures ─────────────────────────────────────────────────────────────────

// ImportSpec represents a single @import directive.
type ImportSpec struct {
	// Source is the .hunt file path (relative to the importing file).
	Source string
	// Names is the list of STEP block names to import. ["*"] means all.
	Names []string
	// Aliases maps importing name → local alias.
	Aliases map[string]string
}

// Command is a single parsed DSL instruction.
type Command struct {
	// Type is the DSL verb.
	Type CommandType
	// Raw is the original unparsed DSL text.
	Raw string
	// Verb is the first word of the raw text (normalised lowercase).
	Verb string

	// StepBlock is the STEP label this command belongs to (if any).
	StepBlock string
	// Tags filters execution to specific @tag values.
	Tags []string

	// Target is the human-readable label of the element to interact with.
	Target string
	// TypeHint is the element-type keyword (button, link, field, …).
	TypeHint string
	// InteractionMode is the scoring mode for element disambiguation.
	InteractionMode InteractionMode

	// URL is the navigation destination for NAVIGATE commands.
	URL string
	// Value is the fill/select value.
	Value string

	// NearAnchor is the NEAR qualifier anchor label.
	NearAnchor string
	// OnRegion is the ON <region> qualifier (header, footer, sidebar, …).
	OnRegion string
	// InsideContainer is the INSIDE qualifier container label.
	InsideContainer string
	// InsideRowText is the "with <text>" clarifier for INSIDE.
	InsideRowText string

	// VerifyText is the text to verify for VERIFY commands.
	VerifyText string
	// VerifyNegated is true for "VERIFY that X is NOT present".
	VerifyNegated bool
	// VerifyState is the element state to verify (checked, visible, …).
	VerifyState string

	// ExtractVar is the variable name to store EXTRACT results into.
	ExtractVar string

	// ScrollDirection is "up" or "down" for SCROLL commands.
	ScrollDirection string
	// ScrollContainer is the element label to scroll within.
	ScrollContainer string

	// PressKey is the key combination for PRESS commands.
	PressKey string

	// WaitSeconds is the duration in seconds for WAIT commands.
	WaitSeconds float64
	// WaitForState is the element state for WAIT FOR commands.
	WaitForState string
	// WaitURLPattern is the URL pattern for WAIT FOR RESPONSE.
	WaitURLPattern string

	// SetVar is the variable name for SET commands.
	SetVar string
	// SetValue is the value for SET commands.
	SetValue string

	// DragSource is the source element label for DRAG commands.
	DragSource string
	// DragTarget is the drop target element label for DRAG commands.
	DragTarget string

	// RepeatCount is the iteration count for REPEAT commands.
	RepeatCount int
	// ForEachVar is the loop variable for FOR EACH commands.
	ForEachVar string
	// ForEachList is the collection variable for FOR EACH commands.
	ForEachList string
	// WhileCondition is the condition expression for WHILE commands.
	WhileCondition string
	// IfCondition is the condition expression for IF/ELIF commands.
	IfCondition string

	// Condition is the generic condition string (used by IF/ELIF/WHILE).
	Condition string

	// PrintText is the text to print for PRINT commands.
	PrintText string

	// CallStepName is the step block name to call for CALL_STEP commands.
	CallStepName string

	// UploadFilePath is the file path for UPLOAD_FILE commands.
	UploadFilePath string

	// Body is the block body for control-flow commands (REPEAT, FOR EACH, WHILE, IF).
	Body []Command
	// ElseBody is the else block for IF commands.
	ElseBody []Command
	// ElseIfClauses holds additional ELIF blocks.
	ElseIfClauses []ElseIfClause
}

// ElseIfClause is one ELIF branch in an IF command.
type ElseIfClause struct {
	Condition string
	Body      []Command
}

// Hunt is the fully parsed representation of a .hunt file.
type Hunt struct {
	// SourcePath is the absolute filesystem path to the .hunt file.
	SourcePath string
	// Title is the @title directive value.
	Title string
	// Context is the @context directive value.
	Context string
	// Tags are the file-level @tags.
	Tags []string
	// Vars holds @var declarations and runtime SET values.
	Vars map[string]string
	// Imports contains parsed @import directives.
	Imports []ImportSpec
	// Blueprints holds imported STEP blocks keyed by name.
	Blueprints map[string][]Command
	// Commands is the ordered list of top-level parsed commands.
	Commands []Command
}

// ── Parser ─────────────────────────────────────────────────────────────────────

// Parse reads a .hunt file from r and returns the parsed Hunt.
// Variable substitution of @var declarations is applied during parsing.
func Parse(r io.Reader) (*Hunt, error) {
	hunt := &Hunt{Vars: map[string]string{}}
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := parseLines(hunt, lines); err != nil {
		return nil, err
	}
	return hunt, nil
}

// ParseFile reads a .hunt file from the filesystem and returns the parsed Hunt.
func ParseFile(path string) (*Hunt, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	defer f.Close()
	hunt, err := Parse(f)
	if err != nil {
		return nil, err
	}
	hunt.SourcePath = path
	return hunt, nil
}

// parseLines is the main parsing loop.
func parseLines(hunt *Hunt, lines []string) error {
	var currentStep string
	var currentTags []string

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)

		// Skip blank lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// ── @directives ───────────────────────────────────────────────────────
		if strings.HasPrefix(trimmed, "@") {
			if err := parseDirective(hunt, trimmed); err != nil {
				return fmt.Errorf("line %d: %w", i+1, err)
			}
			continue
		}

		upper := strings.ToUpper(trimmed)

		// ── STEP label ────────────────────────────────────────────────────────
		if strings.HasPrefix(upper, "STEP ") {
			currentStep = strings.TrimSuffix(trimmed, ":")
			// Remove trailing dot from "STEP N. Title" notation.
			continue
		}

		// ── DONE ──────────────────────────────────────────────────────────────
		if upper == "DONE." || upper == "DONE" {
			break
		}

		// ── @tag line ─────────────────────────────────────────────────────────
		if strings.HasPrefix(upper, "@TAG:") || strings.HasPrefix(upper, "@TAGS:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				for _, t := range strings.Split(parts[1], ",") {
					currentTags = append(currentTags, strings.TrimSpace(t))
				}
			}
			continue
		}

		// ── Apply variable substitution ───────────────────────────────────────
		expanded := applyVars(hunt, trimmed)

		// ── Parse command ─────────────────────────────────────────────────────
		cmd := parseCommand(expanded)
		cmd.Raw = trimmed
		cmd.StepBlock = currentStep
		cmd.Tags = append([]string{}, currentTags...)
		currentTags = nil // tags apply only to the immediately following command

		hunt.Commands = append(hunt.Commands, cmd)
	}
	return nil
}

// parseDirective handles @key: value header lines.
func parseDirective(hunt *Hunt, line string) error {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) < 2 {
		return nil
	}
	key := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(parts[0]), "@"))
	value := strings.TrimSpace(parts[1])

	switch key {
	case "title":
		hunt.Title = value
	case "context":
		hunt.Context = value
	case "tags", "tag":
		for _, t := range strings.Split(value, ",") {
			hunt.Tags = append(hunt.Tags, strings.TrimSpace(t))
		}
	case "var":
		// @var: {name} = value
		eqIdx := strings.Index(value, "=")
		if eqIdx < 0 {
			return nil
		}
		varName := strings.Trim(strings.TrimSpace(value[:eqIdx]), "{}")
		varValue := strings.TrimSpace(value[eqIdx+1:])
		hunt.Vars[varName] = varValue
	case "import":
		imp, err := parseImportLine(value)
		if err != nil {
			return err
		}
		hunt.Imports = append(hunt.Imports, imp)
	}
	return nil
}

// parseImportLine parses "@import: Login, Register from 'auth.hunt'" variants.
func parseImportLine(value string) (ImportSpec, error) {
	imp := ImportSpec{Aliases: map[string]string{}}
	// Pattern: <names> from '<file>'
	fromIdx := strings.LastIndex(strings.ToLower(value), " from ")
	if fromIdx < 0 {
		// Bare: @import: 'file.hunt'
		imp.Source = strings.Trim(strings.TrimSpace(value), "'\"")
		imp.Names = []string{"*"}
		return imp, nil
	}
	namesPart := strings.TrimSpace(value[:fromIdx])
	filePart := strings.Trim(strings.TrimSpace(value[fromIdx+6:]), "'\"")
	imp.Source = filePart

	for _, n := range strings.Split(namesPart, ",") {
		n = strings.TrimSpace(n)
		// Check for "OldName as NewName" alias syntax.
		asIdx := strings.Index(strings.ToLower(n), " as ")
		if asIdx >= 0 {
			orig := strings.TrimSpace(n[:asIdx])
			alias := strings.TrimSpace(n[asIdx+4:])
			imp.Names = append(imp.Names, orig)
			imp.Aliases[orig] = alias
		} else {
			imp.Names = append(imp.Names, n)
		}
	}
	return imp, nil
}

// applyVars replaces {var} references with their current values.
func applyVars(hunt *Hunt, s string) string {
	for name, val := range hunt.Vars {
		s = strings.ReplaceAll(s, "{"+name+"}", val)
	}
	return s
}

// parseCommand parses a single expanded DSL line into a Command.
func parseCommand(line string) Command {
	upper := strings.ToUpper(line)
	cmd := Command{Verb: strings.Fields(strings.ToLower(line))[0]}

	switch {
	// ── NAVIGATE ──────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "NAVIGATE TO ") || strings.HasPrefix(upper, "NAVIGATE "):
		cmd.Type = CmdNavigate
		raw := stripPrefix(line, "NAVIGATE TO ", "NAVIGATE ")
		cmd.URL = unquote(raw)

	// ── DOUBLE CLICK ──────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "DOUBLE CLICK "), strings.HasPrefix(upper, "DOUBLECLICK "):
		cmd.Type = CmdDoubleClick
		rest := stripPrefix(line, "DOUBLE CLICK ", "DOUBLECLICK ")
		cmd.Target, cmd.TypeHint, cmd.InteractionMode = parseTarget(rest)
		cmd = parseQualifiers(cmd, rest)

	// ── RIGHT CLICK ───────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "RIGHT CLICK "), strings.HasPrefix(upper, "RIGHTCLICK "):
		cmd.Type = CmdRightClick
		rest := stripPrefix(line, "RIGHT CLICK ", "RIGHTCLICK ")
		cmd.Target, cmd.TypeHint, cmd.InteractionMode = parseTarget(rest)
		cmd = parseQualifiers(cmd, rest)

	// ── CLICK ─────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "CLICK "):
		cmd.Type = CmdClick
		rest := stripPrefix(line, "CLICK ")
		cmd.Target, cmd.TypeHint, cmd.InteractionMode = parseTarget(rest)
		cmd = parseQualifiers(cmd, rest)

	// ── FILL ──────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "FILL ") || strings.HasPrefix(upper, "FILL THE "):
		cmd.Type = CmdFill
		cmd.InteractionMode = ModeInput
		rest := stripPrefix(line, "FILL THE ", "FILL ")
		// "FILL 'Target' field with 'Value'"
		firstQ := extractFirstQuoted(rest)
		cmd.Target = firstQ
		withIdx := strings.Index(strings.ToUpper(rest), " WITH ")
		if withIdx >= 0 {
			cmd.Value = unquote(strings.TrimSpace(rest[withIdx+6:]))
		}

	// ── TYPE ──────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "TYPE "):
		cmd.Type = CmdType
		cmd.InteractionMode = ModeInput
		rest := stripPrefix(line, "TYPE ")
		// "TYPE 'value' into the 'target' field"
		firstQ := extractFirstQuoted(rest)
		cmd.Value = firstQ
		intoIdx := strings.Index(strings.ToUpper(rest), " INTO ")
		if intoIdx >= 0 {
			cmd.Target, _, _ = parseTarget(strings.TrimSpace(rest[intoIdx+6:]))
		}

	// ── SELECT ────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "SELECT "):
		cmd.Type = CmdSelect
		cmd.InteractionMode = ModeSelect
		rest := stripPrefix(line, "SELECT ")
		// "SELECT 'value' from the 'target' dropdown"
		firstQ := extractFirstQuoted(rest)
		cmd.Value = firstQ
		fromIdx := strings.Index(strings.ToUpper(rest), " FROM ")
		if fromIdx >= 0 {
			cmd.Target, cmd.TypeHint, _ = parseTarget(strings.TrimSpace(rest[fromIdx+6:]))
		}

	// ── CHECK ─────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "CHECK "):
		cmd.Type = CmdCheck
		cmd.InteractionMode = ModeCheckbox
		rest := stripPrefix(line, "CHECK ")
		// "CHECK the checkbox for 'target'"
		forIdx := strings.Index(strings.ToUpper(rest), " FOR ")
		if forIdx >= 0 {
			cmd.Target = unquote(strings.TrimSpace(rest[forIdx+5:]))
		} else {
			cmd.Target, cmd.TypeHint, _ = parseTarget(rest)
		}

	// ── UNCHECK ───────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "UNCHECK "):
		cmd.Type = CmdUncheck
		cmd.InteractionMode = ModeCheckbox
		rest := stripPrefix(line, "UNCHECK ")
		forIdx := strings.Index(strings.ToUpper(rest), " FOR ")
		if forIdx >= 0 {
			cmd.Target = unquote(strings.TrimSpace(rest[forIdx+5:]))
		} else {
			cmd.Target, cmd.TypeHint, _ = parseTarget(rest)
		}

	// ── VERIFY SOFTLY ─────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "VERIFY SOFTLY "):
		cmd.Type = CmdVerifySoft
		rest := stripPrefix(line, "VERIFY SOFTLY ")
		cmd.VerifyText = extractVerifyText(rest, &cmd.VerifyNegated, &cmd.VerifyState)

	// ── VERIFY ────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "VERIFY "):
		cmd.Type = CmdVerify
		rest := stripPrefix(line, "VERIFY ")
		cmd.VerifyText = extractVerifyText(rest, &cmd.VerifyNegated, &cmd.VerifyState)

	// ── EXTRACT ───────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "EXTRACT "):
		cmd.Type = CmdExtract
		cmd.InteractionMode = ModeNone
		rest := stripPrefix(line, "EXTRACT ")
		// "EXTRACT the 'Target' into {var}"
		rest = stripPrefix(rest, "THE ", "the ")
		intoIdx := strings.Index(strings.ToUpper(rest), " INTO ")
		if intoIdx >= 0 {
			cmd.Target = unquote(strings.TrimSpace(rest[:intoIdx]))
			varStr := strings.TrimSpace(rest[intoIdx+6:])
			cmd.ExtractVar = strings.Trim(varStr, "{}")
		} else {
			cmd.Target = unquote(rest)
		}

	// ── SCROLL ────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "SCROLL "):
		cmd.Type = CmdScroll
		rest := stripPrefix(line, "SCROLL ")
		upR := strings.ToUpper(rest)
		if strings.HasPrefix(upR, "UP") {
			cmd.ScrollDirection = "up"
			rest = strings.TrimSpace(rest[2:])
		} else {
			cmd.ScrollDirection = "down"
			rest = strings.TrimSpace(strings.TrimPrefix(rest, strings.Fields(rest)[0]))
		}
		insideIdx := strings.Index(strings.ToUpper(rest), " INSIDE ")
		if insideIdx >= 0 {
			cmd.ScrollContainer = unquote(strings.TrimSpace(rest[insideIdx+8:]))
		}

	// ── PRESS ─────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "PRESS "):
		cmd.Type = CmdPress
		cmd.PressKey = strings.TrimSpace(line[6:])

	// ── WAIT FOR RESPONSE ─────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "WAIT FOR RESPONSE "):
		cmd.Type = CmdWaitResponse
		cmd.WaitURLPattern = unquote(strings.TrimSpace(line[18:]))

	// ── WAIT FOR ──────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "WAIT FOR ") || strings.HasPrefix(upper, "WAIT UNTIL "):
		cmd.Type = CmdWaitFor
		rest := stripPrefix(line, "WAIT FOR ", "WAIT UNTIL ")
		// "WAIT FOR 'Element' to be visible/hidden/enabled/…"
		firstQ := extractFirstQuoted(rest)
		cmd.Target = firstQ
		toBeIdx := strings.Index(strings.ToUpper(rest), " TO BE ")
		if toBeIdx >= 0 {
			cmd.WaitForState = strings.ToLower(strings.TrimSpace(rest[toBeIdx+7:]))
		}

	// ── WAIT N ────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "WAIT "):
		cmd.Type = CmdWait
		rest := strings.TrimSpace(line[5:])
		// Strip optional "SECONDS" suffix.
		rest = strings.TrimSuffix(strings.TrimSuffix(strings.ToUpper(rest), " SECONDS"), " SECOND")
		rest = strings.ToLower(rest)
		if n, err := strconv.ParseFloat(strings.Fields(rest)[0], 64); err == nil {
			cmd.WaitSeconds = n
		}

	// ── HOVER ─────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "HOVER "):
		cmd.Type = CmdHover
		rest := stripPrefix(line, "HOVER OVER THE ", "HOVER OVER ", "HOVER THE ", "HOVER ")
		cmd.Target, cmd.TypeHint, cmd.InteractionMode = parseTarget(rest)

	// ── DRAG ──────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "DRAG "):
		cmd.Type = CmdDrag
		rest := stripPrefix(line, "DRAG THE ELEMENT ", "DRAG THE ", "DRAG ")
		// "DRAG '<source>' and drop it into '<target>'"
		andIdx := strings.Index(strings.ToUpper(rest), " AND ")
		if andIdx >= 0 {
			cmd.DragSource = unquote(strings.TrimSpace(rest[:andIdx]))
			dropPart := strings.ToUpper(rest[andIdx:])
			intoIdx := strings.Index(dropPart, " INTO ")
			if intoIdx >= 0 {
				cmd.DragTarget = unquote(strings.TrimSpace(rest[andIdx+intoIdx+6:]))
			}
		}

	// ── SET ───────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "SET "):
		cmd.Type = CmdSet
		rest := strings.TrimSpace(line[4:])
		eqIdx := strings.Index(rest, "=")
		if eqIdx >= 0 {
			cmd.SetVar = strings.Trim(strings.TrimSpace(rest[:eqIdx]), "{}")
			cmd.SetValue = strings.TrimSpace(rest[eqIdx+1:])
		}

	// ── PRINT ─────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "PRINT "):
		cmd.Type = CmdPrint
		cmd.PrintText = unquote(strings.TrimSpace(line[6:]))

	// ── SCREENSHOT ────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "SCREENSHOT"):
		cmd.Type = CmdScreenshot

	// ── HIGHLIGHT ─────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "HIGHLIGHT "):
		cmd.Type = CmdHighlight
		rest := stripPrefix(line, "HIGHLIGHT THE ", "HIGHLIGHT ")
		cmd.Target, cmd.TypeHint, cmd.InteractionMode = parseTarget(rest)

	// ── UPLOAD FILE ───────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "UPLOAD ") || strings.HasPrefix(upper, "UPLOAD_FILE "):
		cmd.Type = CmdUploadFile
		rest := stripPrefix(line, "UPLOAD FILE ", "UPLOAD_FILE ", "UPLOAD ")
		toIdx := strings.Index(strings.ToUpper(rest), " TO ")
		if toIdx >= 0 {
			cmd.UploadFilePath = unquote(strings.TrimSpace(rest[:toIdx]))
			cmd.Target, cmd.TypeHint, cmd.InteractionMode = parseTarget(strings.TrimSpace(rest[toIdx+4:]))
		} else {
			cmd.UploadFilePath = unquote(rest)
		}

	// ── FOR EACH ──────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "FOR EACH "):
		cmd.Type = CmdForEach
		rest := stripPrefix(line, "FOR EACH ")
		inIdx := strings.Index(strings.ToUpper(rest), " IN ")
		if inIdx >= 0 {
			cmd.ForEachVar = strings.Trim(strings.TrimSpace(rest[:inIdx]), "{}")
			cmd.ForEachList = strings.Trim(strings.TrimSpace(strings.TrimSuffix(rest[inIdx+4:], ":")), "{}")
		}

	// ── WHILE ─────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "WHILE "):
		cmd.Type = CmdWhile
		rest := stripPrefix(line, "WHILE ")
		cmd.WhileCondition = strings.TrimSuffix(strings.TrimSpace(rest), ":")
		cmd.Condition = cmd.WhileCondition

	// ── REPEAT ────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "REPEAT "):
		cmd.Type = CmdRepeat
		rest := stripPrefix(line, "REPEAT ")
		// "REPEAT N TIMES:" or "REPEAT N TIME:"
		fields := strings.Fields(rest)
		if len(fields) >= 1 {
			if n, err := strconv.Atoi(fields[0]); err == nil {
				cmd.RepeatCount = n
			}
		}

	// ── IF ────────────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "IF "):
		cmd.Type = CmdIf
		rest := stripPrefix(line, "IF ")
		cmd.IfCondition = strings.TrimSuffix(strings.TrimSpace(rest), ":")
		cmd.Condition = cmd.IfCondition

	// ── ELIF / ELSE IF ────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "ELIF ") || strings.HasPrefix(upper, "ELSE IF "):
		cmd.Type = CmdElseIf
		rest := stripPrefix(line, "ELSE IF ", "ELIF ")
		cmd.IfCondition = strings.TrimSuffix(strings.TrimSpace(rest), ":")
		cmd.Condition = cmd.IfCondition

	// ── ELSE ──────────────────────────────────────────────────────────────────
	case upper == "ELSE:" || upper == "ELSE":
		cmd.Type = CmdElse

	// ── END markers ───────────────────────────────────────────────────────────
	case upper == "END IF" || upper == "END IF:" || upper == "ENDIF":
		cmd.Type = CmdEndIf
	case upper == "END FOR" || upper == "END FOR:" || upper == "ENDFOR" || upper == "END EACH":
		cmd.Type = CmdEndFor
	case upper == "END WHILE" || upper == "END WHILE:" || upper == "ENDWHILE":
		cmd.Type = CmdEndWhile
	case upper == "END REPEAT" || upper == "END REPEAT:" || upper == "ENDREPEAT":
		cmd.Type = CmdEndRepeat

	// ── CALL STEP ─────────────────────────────────────────────────────────────
	case strings.HasPrefix(upper, "CALL ") || strings.HasPrefix(upper, "RUN STEP "):
		cmd.Type = CmdCallStep
		cmd.CallStepName = stripPrefix(line, "RUN STEP ", "CALL ")

	default:
		cmd.Type = CmdUnknown
	}

	return cmd
}

// ── Target parsing helpers ─────────────────────────────────────────────────────

// parseTarget extracts the quoted label, type-hint, and interaction mode
// from a command tail like: "the 'Submit' button", "'Login' link", "on 'Save'".
func parseTarget(s string) (target, typeHint string, mode InteractionMode) {
	mode = ModeClickable
	upper := strings.ToUpper(s)

	// Strip leading "THE ", "ON THE ", "ON ", "FOR THE ", "FOR ".
	s = stripPrefix(s, "THE ", "ON THE ", "ON ", "FOR THE ", "FOR ", "OVER THE ", "OVER ")

	// Extract first quoted string.
	target = extractFirstQuoted(s)
	if target == "" {
		// Unquoted: take text up to a keyword.
		fields := strings.Fields(s)
		if len(fields) > 0 {
			target = fields[0]
		}
	}

	// Detect type-hint keyword.
	_ = upper
	hintsClickable := []string{"button", "link", "tab", "menuitem", "option", "element", "icon", "image"}
	hintsInput := []string{"field", "input", "textbox", "textarea", "text field", "search", "email", "password"}
	hintsSelect := []string{"dropdown", "select", "combobox", "listbox"}
	hintsCheckbox := []string{"checkbox", "radio", "toggle", "switch"}

	sLower := strings.ToLower(s)
	for _, h := range hintsClickable {
		if strings.Contains(sLower, " "+h) || strings.HasSuffix(sLower, " "+h) {
			typeHint = h
			mode = ModeClickable
			return
		}
	}
	for _, h := range hintsInput {
		if strings.Contains(sLower, " "+h) || strings.HasSuffix(sLower, " "+h) {
			typeHint = h
			mode = ModeInput
			return
		}
	}
	for _, h := range hintsSelect {
		if strings.Contains(sLower, " "+h) || strings.HasSuffix(sLower, " "+h) {
			typeHint = h
			mode = ModeSelect
			return
		}
	}
	for _, h := range hintsCheckbox {
		if strings.Contains(sLower, " "+h) || strings.HasSuffix(sLower, " "+h) {
			typeHint = h
			mode = ModeCheckbox
			return
		}
	}
	return
}

// parseQualifiers extracts NEAR / ON <region> / INSIDE qualifiers from a rest string.
func parseQualifiers(cmd Command, rest string) Command {
	upper := strings.ToUpper(rest)
	if nearIdx := strings.Index(upper, " NEAR "); nearIdx >= 0 {
		cmd.NearAnchor = unquote(strings.TrimSpace(rest[nearIdx+6:]))
	}
	if onIdx := strings.Index(upper, " ON "); onIdx >= 0 {
		after := strings.TrimSpace(rest[onIdx+4:])
		if !strings.HasPrefix(strings.ToUpper(after), "THE ") {
			cmd.OnRegion = strings.ToLower(unquote(after))
		}
	}
	if insideIdx := strings.Index(upper, " INSIDE "); insideIdx >= 0 {
		after := strings.TrimSpace(rest[insideIdx+8:])
		after = stripPrefix(after, "THE ")
		withIdx := strings.Index(strings.ToUpper(after), " ROW WITH ")
		if withIdx >= 0 {
			cmd.InsideContainer = unquote(strings.TrimSpace(after[:withIdx]))
			cmd.InsideRowText = unquote(strings.TrimSpace(after[withIdx+10:]))
		} else {
			withIdx = strings.Index(strings.ToUpper(after), " WITH ")
			if withIdx >= 0 {
				cmd.InsideContainer = unquote(strings.TrimSpace(after[:withIdx]))
				cmd.InsideRowText = unquote(strings.TrimSpace(after[withIdx+6:]))
			} else {
				cmd.InsideContainer = unquote(after)
			}
		}
	}
	return cmd
}

// extractVerifyText parses "that 'X' is [NOT] present/checked/visible/…".
func extractVerifyText(rest string, negated *bool, state *string) string {
	rest = stripPrefix(rest, "THAT ", "that ")
	q := extractFirstQuoted(rest)
	upper := strings.ToUpper(rest)
	if strings.Contains(upper, " IS NOT ") || strings.Contains(upper, " ARE NOT ") {
		*negated = true
	}
	for _, st := range []string{"checked", "unchecked", "visible", "hidden", "enabled", "disabled", "selected"} {
		if strings.Contains(upper, " IS "+strings.ToUpper(st)) {
			*state = st
			break
		}
	}
	return q
}

// ── String utilities ───────────────────────────────────────────────────────────

// stripPrefix removes the first matching prefix (case-insensitive) from s.
func stripPrefix(s string, prefixes ...string) string {
	upper := strings.ToUpper(s)
	for _, p := range prefixes {
		if strings.HasPrefix(upper, strings.ToUpper(p)) {
			return s[len(p):]
		}
	}
	return s
}

// unquote removes surrounding single or double quotes from s.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') ||
			(s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// extractFirstQuoted returns the content of the first single- or double-quoted
// substring in s, or empty string if none.
func extractFirstQuoted(s string) string {
	for _, q := range []byte{'\'', '"'} {
		start := strings.IndexByte(s, q)
		if start < 0 {
			continue
		}
		end := strings.IndexByte(s[start+1:], q)
		if end < 0 {
			continue
		}
		return s[start+1 : start+1+end]
	}
	return ""
}

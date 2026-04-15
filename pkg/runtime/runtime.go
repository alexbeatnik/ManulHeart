// Package runtime implements the ManulHeart DSL execution engine.
//
// STATUS: Stub package. The full implementation will live here once the
// CDP backend (pkg/cdp) is production-ready.
package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/explain"
	"github.com/manulengineer/manulheart/pkg/heuristics"
	"github.com/manulengineer/manulheart/pkg/scorer"
	"github.com/manulengineer/manulheart/pkg/utils"
)

// Runtime executes ManulHeart DSL hunts against a live browser page.
type Runtime struct {
	cfg    config.Config
	page   browser.Page
	logger *utils.Logger
	vars   *ScopedVariables
}

// New creates a new Runtime bound to the given Config, Page, and Logger.
func New(cfg config.Config, page browser.Page, logger *utils.Logger) *Runtime {
	return &Runtime{
		cfg:    cfg,
		page:   page,
		logger: logger,
		vars:   NewScopedVariables(),
	}
}

// RunHunt executes all commands in hunt against the bound page.
// It returns an explain.HuntResult summarising the execution.
func (rt *Runtime) RunHunt(ctx context.Context, hunt *dsl.Hunt) (*explain.HuntResult, error) {
	if hunt == nil {
		return nil, fmt.Errorf("runtime: nil hunt")
	}

	// Initialize runtime variables from hunt @vars (Global level)
	for k, v := range hunt.Vars {
		rt.vars.Set(k, v, LevelGlobal)
	}

	result := &explain.HuntResult{
		HuntFile: hunt.SourcePath,
		Title:    hunt.Title,
		Context:  hunt.Context,
	}

	passed, failed, err := rt.runCommands(ctx, hunt.Commands, result)
	result.TotalSteps = passed + failed
	result.Passed = passed
	result.Failed = failed
	result.Success = failed == 0
	return result, err
}

func (rt *Runtime) runCommands(ctx context.Context, commands []dsl.Command, huntRes *explain.HuntResult) (int, int, error) {
	passed, failed := 0, 0
	for _, cmd := range commands {
		if err := ctx.Err(); err != nil {
			return passed, failed, fmt.Errorf("runtime: context cancelled: %w", err)
		}

		stepResult, err := rt.executeCommand(ctx, cmd)
		if huntRes != nil {
			huntRes.Results = append(huntRes.Results, stepResult)
		}
		if err != nil {
			failed++
			rt.logger.Error("step failed: %v", err)
			return passed, failed, err
		} else {
			passed++
		}
	}
	return passed, failed, nil
}

// StepResult is the result of a single DSL step run via RunStep.
type StepResult struct {
	// Command is the raw DSL text that was executed.
	Command string `json:"command"`
	// Success is true when the command succeeded.
	Success bool `json:"success"`
	// Error is the error message if Success is false.
	Error string `json:"error,omitempty"`
}

// RunStep executes a single raw DSL command string and returns its result.
func (rt *Runtime) RunStep(ctx context.Context, rawStep string) (*StepResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Parse the single-line command.
	hunt, err := dsl.Parse(strings.NewReader(rawStep))
	if err != nil {
		return &StepResult{Command: rawStep, Error: err.Error()}, err
	}
	if len(hunt.Commands) == 0 {
		return &StepResult{Command: rawStep, Success: true}, nil
	}
	stepResult, execErr := rt.executeCommand(ctx, hunt.Commands[0])
	return &StepResult{
		Command: rawStep,
		Success: execErr == nil,
		Error:   stepResult.Error,
	}, execErr
}

// executeCommand runs a single DSL command and returns its execution result.
func (rt *Runtime) executeCommand(ctx context.Context, cmd dsl.Command) (explain.ExecutionResult, error) {
	res := explain.ExecutionResult{
		Step:        cmd.Raw,
		CommandType: string(cmd.Type),
	}
	var err error

	switch cmd.Type {
	case dsl.CmdNavigate:
		url := rt.resolveVariables(cmd.URL)
		err = rt.page.Navigate(ctx, url)

	case dsl.CmdWait:
		time.Sleep(time.Duration(cmd.WaitSeconds * float64(time.Second)))

	case dsl.CmdPrint:
		text := rt.resolveVariables(cmd.PrintText)
		rt.logger.Info("PRINT: %s", text)

	case dsl.CmdSet:
		if cmd.SetVar != "" {
			val := rt.resolveVariables(cmd.SetValue)
			rt.vars.Set(cmd.SetVar, val, LevelRow)
			break
		}
		fallthrough

	case dsl.CmdClick, dsl.CmdFill, dsl.CmdType, dsl.CmdHover:
		// Target resolution needed for interaction
		raw, errProbe := rt.page.CallProbe(ctx, heuristics.BuildSnapshotProbe(), nil)
		if errProbe != nil {
			err = fmt.Errorf("probe failed: %w", errProbe)
			break
		}
		elements, errParse := heuristics.ParseProbeResult(raw)
		if errParse != nil {
			err = fmt.Errorf("parse probe failed: %w", errParse)
			break
		}

		// Figure out interaction mode
		mode := dsl.ModeNone
		if cmd.Type == dsl.CmdFill || cmd.Type == dsl.CmdSet || cmd.Type == dsl.CmdType {
			mode = dsl.ModeInput
		} else if cmd.Type == dsl.CmdClick {
			mode = dsl.ModeClickable
		}

		targetPath := rt.resolveVariables(cmd.Target)
		if cmd.Type == dsl.CmdSet {
			targetPath = rt.resolveVariables(cmd.SetVar)
		}

		ranked := scorer.Rank(targetPath, cmd.TypeHint, string(mode), elements, 5, nil)
		if len(ranked) == 0 {
			err = fmt.Errorf("target not found: %q", targetPath)
			break
		}

		for _, r := range ranked {
			res.RankedCandidates = append(res.RankedCandidates, r.Explain)
		}

		winner := ranked[0].Element

		// Perform action
		switch cmd.Type {
		case dsl.CmdFill, dsl.CmdSet, dsl.CmdType:
			val := rt.resolveVariables(cmd.Value)
			if cmd.Type == dsl.CmdSet && cmd.SetValue != "" {
				// This case should be handled by the specialized CmdSet above,
				// but we keep it for robustness if fallthrough happened.
				val = rt.resolveVariables(cmd.SetValue)
			}
			err = rt.page.SetInputValue(ctx, winner.XPath, val)
		case dsl.CmdClick:
			x, y, e := rt.page.GetElementCenter(ctx, winner.XPath)
			if e != nil {
				err = fmt.Errorf("center calc: %w", e)
			} else {
				err = rt.page.Click(ctx, x, y)
			}
		case dsl.CmdDoubleClick:
			x, y, e := rt.page.GetElementCenter(ctx, winner.XPath)
			if e != nil {
				err = fmt.Errorf("center calc: %w", e)
			} else {
				err = rt.page.DoubleClick(ctx, x, y)
			}
		case dsl.CmdRightClick:
			x, y, e := rt.page.GetElementCenter(ctx, winner.XPath)
			if e != nil {
				err = fmt.Errorf("center calc: %w", e)
			} else {
				err = rt.page.RightClick(ctx, x, y)
			}
		case dsl.CmdHover:
			x, y, e := rt.page.GetElementCenter(ctx, winner.XPath)
			if e != nil {
				err = fmt.Errorf("center calc: %w", e)
			} else {
				err = rt.page.Hover(ctx, x, y)
			}
		case dsl.CmdCheck, dsl.CmdUncheck:
			checked := cmd.Type == dsl.CmdCheck
			js := fmt.Sprintf(`document.evaluate("%s", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue.checked = %v`,
				strings.ReplaceAll(winner.XPath, `"`, `\"`), checked)
			_, err = rt.page.EvalJS(ctx, js)
		case dsl.CmdSelect:
			val := rt.resolveVariables(cmd.Value)
			js := fmt.Sprintf(`(() => {
				const el = document.evaluate("%s", document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
				if (!el) return;
				for (let opt of el.options) {
					if (opt.text === "%s" || opt.value === "%s") {
						el.value = opt.value;
						el.dispatchEvent(new Event('change', {bubbles: true}));
						return;
					}
				}
			})()`, strings.ReplaceAll(winner.XPath, `"`, `\"`),
				strings.ReplaceAll(val, `"`, `\"`),
				strings.ReplaceAll(val, `"`, `\"`))
			_, err = rt.page.EvalJS(ctx, js)
		}

	case dsl.CmdExtract:
		// Use dedicated extraction probe which handles tables/text nodes
		target := rt.resolveVariables(cmd.Target)
		hint := "" // we could extract hint from cmd if needed
		params := []string{target, hint}
		
		val, errProbe := rt.page.CallProbe(ctx, heuristics.BuildExtractProbe(), params)
		if errProbe != nil {
			err = errProbe
			break
		}
		
		extracted := strings.Trim(string(val), "\"") // Unquote JSON string if needed

		if extracted == "" || extracted == "null" {
			err = fmt.Errorf("extract target not found or empty: %q", target)
			break
		}
		rt.vars.Set(cmd.ExtractVar, extracted, LevelRow)
		rt.logger.Info("Extracted '%s' into {%s}", extracted, cmd.ExtractVar)

	case dsl.CmdScroll:
		if scroller, ok := rt.page.(interface {
			ScrollPage(context.Context, string, string) error
		}); ok {
			err = scroller.ScrollPage(ctx, cmd.ScrollDirection, cmd.ScrollContainer)
		} else {
			// fallback via EvalJS
			amount := 500
			if cmd.ScrollDirection == "up" {
				amount = -500
			}
			_, err = rt.page.EvalJS(ctx, fmt.Sprintf("window.scrollBy(0, %d)", amount))
		}

	case dsl.CmdVerify:
		// Lightweight text presence check via dedicated probe
		target := rt.resolveVariables(cmd.VerifyText)
		raw, errProbe := rt.page.CallProbe(ctx, heuristics.BuildVisibleTextProbe(), nil)
		if errProbe != nil {
			err = errProbe
			break
		}
		
		pageText := strings.ToLower(string(raw))
		present := strings.Contains(pageText, strings.ToLower(target))
		
		if cmd.VerifyNegated {
			if present {
				err = fmt.Errorf("verification failed: '%s' is present, but expected NOT to be", target)
			}
		} else {
			if !present {
				err = fmt.Errorf("verification failed: '%s' is not present", target)
			}
		}

	case dsl.CmdVerifyField:
		// Full element resolution for attribute-specific verification
		raw, errProbe := rt.page.CallProbe(ctx, heuristics.BuildSnapshotProbe(), nil)
		if errProbe != nil {
			err = errProbe
			break
		}
		elements, _ := heuristics.ParseProbeResult(raw)
		target := rt.resolveVariables(cmd.VerifyText)
		ranked := scorer.Rank(target, "", "none", elements, 1, nil)
		present := len(ranked) > 0 && ranked[0].Explain.Score.Total > 0.3
		if !present {
			err = fmt.Errorf("verification failed: target field '%s' not found", target)
		} else if cmd.VerifyState != "" {
			// verify state (e.g., checked, enabled)
			// ... logic ...
		}

	case dsl.CmdIf:
		var bodyToRun []dsl.Command
		for _, b := range cmd.Branches {
			if b.Kind == "else" {
				bodyToRun = b.Body
				break
			}
			matched, cerr := rt.evaluateCondition(ctx, b.Condition)
			if cerr != nil {
				err = cerr
				break
			}
			if matched {
				bodyToRun = b.Body
				break
			}
		}
		if err == nil && len(bodyToRun) > 0 {
			_, _, err = rt.runCommands(ctx, bodyToRun, nil)
		}

	case dsl.CmdRepeat:
		count := cmd.RepeatCount
		for i := 0; i < count; i++ {
			if cmd.RepeatVar != "" {
				rt.vars.Set(cmd.RepeatVar, fmt.Sprintf("%d", i), LevelRow)
			}
			_, _, err = rt.runCommands(ctx, cmd.Body, nil)
			if err != nil {
				break
			}
		}

	case dsl.CmdWhile:
		limit := 100
		for i := 0; i < limit; i++ {
			matched, cerr := rt.evaluateCondition(ctx, cmd.WhileCondition)
			if cerr != nil {
				err = cerr
				break
			}
			if !matched {
				break
			}
			_, _, err = rt.runCommands(ctx, cmd.Body, nil)
			if err != nil {
				break
			}
			if i == limit-1 {
				rt.logger.Warn("WHILE loop reached limit (100)")
			}
		}

	case dsl.CmdForEach:
		v, _ := rt.vars.Resolve(cmd.ForEachCollection)
		coll := v
		items := strings.Split(coll, ",")
		for _, val := range items {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			rt.vars.Set(cmd.ForEachVar, val, LevelRow)
			_, _, err = rt.runCommands(ctx, cmd.Body, nil)
			if err != nil {
				break
			}
		}

	default:
		err = fmt.Errorf("runtime: command %q not yet implemented", cmd.Type)
	}

	if err != nil {
		res.Error = err.Error()
	} else {
		res.Success = true
	}
	return res, err
}

func (rt *Runtime) evaluateCondition(ctx context.Context, cond string) (bool, error) {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return false, nil
	}
	if cond == "true" {
		return true, nil
	}
	if cond == "false" {
		return false, nil
	}

	// 1. Handle element existence: (button|link|field|element|checkbox) 'Target' [not] exists
	if strings.Contains(cond, "exists") {
		neg := strings.Contains(cond, "not exists")
		// Simple parsing for now, actual implementation should use regex
		parts := strings.Fields(cond)
		if len(parts) >= 2 {
			target := ""
			// Extract quoted target
			start := strings.Index(cond, "'")
			end := strings.LastIndex(cond, "'")
			if start != -1 && end != -1 && start < end {
				target = cond[start+1 : end]
			}
			
			raw, err := rt.page.CallProbe(ctx, heuristics.BuildSnapshotProbe(), nil)
			if err != nil {
				return false, err
			}
			elements, _ := heuristics.ParseProbeResult(raw)
			ranked := scorer.Rank(target, "", "clickable", elements, 1, nil)
			found := len(ranked) > 0 && ranked[0].Explain.Score.Total > 0.2
			if neg {
				return !found, nil
			}
			return found, nil
		}
	}

	// 2. Handle text presence: text 'Target' is [not] present
	if strings.Contains(cond, "is present") || strings.Contains(cond, "is not present") {
		neg := strings.Contains(cond, "is not present")
		start := strings.Index(cond, "'")
		end := strings.LastIndex(cond, "'")
		target := ""
		if start != -1 && end != -1 && start < end {
			target = cond[start+1 : end]
		}

		raw, err := rt.page.CallProbe(ctx, heuristics.BuildSnapshotProbe(), nil)
		if err != nil {
			return false, err
		}
		elements, _ := heuristics.ParseProbeResult(raw)
		ranked := scorer.Rank(target, "", "none", elements, 1, nil)
		found := len(ranked) > 0 && ranked[0].Explain.Score.Total > 0.2
		if neg {
			return !found, nil
		}
		return found, nil
	}

	// 3. Handle variable comparisons: {var} == 'val', $var != 'val'
	if strings.HasPrefix(cond, "{") || strings.HasPrefix(cond, "$") {
		// Resolve variables first
		resolved := rt.resolveVariables(cond)
		if strings.Contains(resolved, " == ") {
			parts := strings.Split(resolved, " == ")
			v1 := strings.TrimSpace(parts[0])
			v2 := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			return v1 == v2, nil
		}
		if strings.Contains(resolved, " != ") {
			parts := strings.Split(resolved, " != ")
			v1 := strings.TrimSpace(parts[0])
			v2 := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			return v1 != v2, nil
		}
		if strings.Contains(resolved, " contains ") {
			parts := strings.Split(resolved, " contains ")
			v1 := strings.TrimSpace(parts[0])
			v2 := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
			return strings.Contains(v1, v2), nil
		}
		// Truthy check for {var}
		val := strings.TrimSpace(resolved)
		return val != "" && val != "false" && val != "0" && val != "null", nil
	}

	return false, fmt.Errorf("unknown condition format: %q", cond)
}

func (rt *Runtime) resolveVariables(s string) string {
	return rt.vars.Interpolate(s)
}

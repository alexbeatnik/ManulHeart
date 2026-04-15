// Package runtime provides DSL execution orchestration for ManulHeart.
//
// The Runtime executes parsed .hunt files command by command, routing each
// command through the appropriate handler. Target-based commands are routed
// through the engine-core targeting pipeline before any browser action is taken.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/core"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/explain"
	"github.com/manulengineer/manulheart/pkg/utils"
)

// maxLoopIterations caps WHILE loops to prevent infinite execution.
const maxLoopIterations = 100

// Runtime executes Hunt commands against a browser page.
type Runtime struct {
	cfg        config.Config
	page       browser.Page
	targeting  *core.Targeting
	logger     *utils.Logger
	vars       map[string]string   // runtime variables (SET, EXTRACT)
	softErrors []string            // accumulated VERIFY SOFTLY failures
}

// New constructs a Runtime for the given page and config.
func New(cfg config.Config, page browser.Page, logger *utils.Logger) *Runtime {
	return &Runtime{
		cfg:       cfg,
		page:      page,
		targeting: core.NewTargeting(cfg, logger),
		logger:    logger,
		vars:      make(map[string]string),
	}
}

// RunHunt executes all commands in the given Hunt and returns a HuntResult.
func (r *Runtime) RunHunt(ctx context.Context, hunt *dsl.Hunt) (*explain.HuntResult, error) {
	start := time.Now()
	result := &explain.HuntResult{
		HuntFile:   hunt.SourcePath,
		Title:      hunt.Title,
		Context:    hunt.Context,
		TotalSteps: len(hunt.Commands),
	}

	// Seed runtime vars from @var: declarations.
	for k, v := range hunt.Vars {
		r.vars[k] = v
	}

	results, stop := r.executeBlock(ctx, hunt.Commands, hunt.Vars)
	result.Results = results
	for _, er := range results {
		if er.Success {
			result.Passed++
		} else {
			result.Failed++
		}
	}
	_ = stop

	result.TotalDuration = time.Since(start)
	result.TotalDurationMS = result.TotalDuration.Milliseconds()
	result.Success = result.Failed == 0
	if len(r.softErrors) > 0 {
		result.SoftErrors = r.softErrors
	}
	return result, nil
}

// executeBlock runs a slice of commands, handling IF/WHILE/REPEAT blocks.
// Returns accumulated results and whether to stop (fail-fast).
func (r *Runtime) executeBlock(ctx context.Context, cmds []dsl.Command, fileVars map[string]string) ([]explain.ExecutionResult, bool) {
	var results []explain.ExecutionResult
	i := 0

	for i < len(cmds) {
		cmd := cmds[i]

		// Substitute runtime variables into command fields before execution.
		r.applyRuntimeVars(&cmd)

		switch cmd.Type {
		case dsl.CmdIf:
			blockResults, stop := r.executeIf(ctx, cmd, fileVars)
			results = append(results, blockResults...)
			if stop {
				return results, true
			}
			i++

		case dsl.CmdWhile:
			blockResults, stop := r.executeWhile(ctx, cmd, fileVars)
			results = append(results, blockResults...)
			if stop {
				return results, true
			}
			i++

		case dsl.CmdRepeat:
			blockResults, stop := r.executeRepeat(ctx, cmd, fileVars)
			results = append(results, blockResults...)
			if stop {
				return results, true
			}
			i++

		case dsl.CmdForEach:
			blockResults, stop := r.executeForEach(ctx, cmd, fileVars)
			results = append(results, blockResults...)
			if stop {
				return results, true
			}
			i++

		// ELIF/ELSE at top level should not happen (they're nested inside IF).
		case dsl.CmdElIf, dsl.CmdElse:
			i++

		default:
			idx := len(results)
			execResult := r.executeCommand(ctx, cmd, idx)
			results = append(results, execResult)
			if !execResult.Success {
				r.logger.Error("FAILED [%d] %s → %s", idx+1, cmd.Raw, execResult.Error)
				return results, true
			}
			i++
		}
	}
	return results, false
}

// RunStep executes a single raw DSL command string and returns the result.
// This is the entry point for the `driver run-step` CLI subcommand.
func (r *Runtime) RunStep(ctx context.Context, rawStep string) (*explain.ExecutionResult, error) {
	cmd, err := parseSingleCommand(rawStep)
	if err != nil {
		return nil, err
	}
	result := r.executeCommand(ctx, cmd, 0)
	return &result, nil
}

// ── Internal execution ────────────────────────────────────────────────────────

func (r *Runtime) executeCommand(ctx context.Context, cmd dsl.Command, idx int) explain.ExecutionResult {
	start := time.Now()
	result := explain.ExecutionResult{
		Step:        cmd.Raw,
		StepIndex:   idx,
		StepBlock:   cmd.StepBlock,
		CommandType: string(cmd.Type),
	}

	// Attach current URL (best-effort)
	if url, err := r.page.CurrentURL(ctx); err == nil {
		result.PageURL = url
	}

	r.logger.Info("[%d] %s", idx+1, cmd.Raw)

	// ── Debug mode: pause before execution ──
	if r.cfg.DebugMode {
		shouldBreak := len(r.cfg.BreakLines) == 0 // break on every step if no breakpoints set
		for _, bl := range r.cfg.BreakLines {
			if bl == cmd.LineNumber {
				shouldBreak = true
				break
			}
		}
		if shouldBreak {
			r.logger.Info("  ⏸  DEBUG: about to execute [line %d] — press Enter to continue", cmd.LineNumber)
			fmt.Scanln()
		}
	}

	var execErr error

	switch cmd.Type {
	case dsl.CmdNavigate:
		result.TargetRequired = false
		result.ActionPerformed = "navigate"
		execErr = r.doNavigate(ctx, cmd, &result)

	case dsl.CmdWait:
		result.TargetRequired = false
		result.ActionPerformed = "wait"
		execErr = r.doWait(ctx, cmd, &result)

	case dsl.CmdWaitFor:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.ActionPerformed = "wait_for"
		execErr = r.doWaitFor(ctx, cmd, &result)

	case dsl.CmdVerify:
		result.TargetRequired = false
		result.ActionPerformed = "verify"
		execErr = r.doVerify(ctx, cmd, &result)

	case dsl.CmdVerifySoft:
		result.TargetRequired = false
		result.ActionPerformed = "verify_softly"
		execErr = r.doVerifySoft(ctx, cmd, &result)

	case dsl.CmdVerifyField:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.ActionPerformed = "verify_field"
		execErr = r.doVerifyField(ctx, cmd, &result)

	case dsl.CmdClick:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "click"
		execErr = r.doClick(ctx, cmd, &result)

	case dsl.CmdDoubleClick:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "double_click"
		execErr = r.doDoubleClick(ctx, cmd, &result)

	case dsl.CmdFill, dsl.CmdType:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = string(cmd.Type)
		result.ActionValue = cmd.Value
		execErr = r.doFill(ctx, cmd, &result)

	case dsl.CmdSelect:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "select"
		result.ActionValue = cmd.Value
		execErr = r.doSelect(ctx, cmd, &result)

	case dsl.CmdCheck, dsl.CmdUncheck:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = string(cmd.Type)
		execErr = r.doCheck(ctx, cmd, &result)

	case dsl.CmdScroll:
		result.TargetRequired = false
		result.ActionPerformed = "scroll"
		execErr = r.doScroll(ctx, cmd, &result)

	case dsl.CmdPress:
		result.TargetRequired = false
		result.ActionPerformed = "press"
		execErr = r.doPress(ctx, cmd, &result)

	case dsl.CmdExtract:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.ActionPerformed = "extract"
		execErr = r.doExtract(ctx, cmd, &result)

	case dsl.CmdSet:
		result.TargetRequired = false
		result.ActionPerformed = "set"
		execErr = r.doSet(ctx, cmd, &result)

	case dsl.CmdHover:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "hover"
		execErr = r.doHover(ctx, cmd, &result)

	case dsl.CmdRightClick:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "right_click"
		execErr = r.doRightClick(ctx, cmd, &result)

	case dsl.CmdDrag:
		result.TargetRequired = false
		result.ActionPerformed = "drag"
		execErr = r.doDrag(ctx, cmd, &result)

	case dsl.CmdUpload:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.ActionPerformed = "upload"
		execErr = r.doUpload(ctx, cmd, &result)

	case dsl.CmdPrint:
		result.TargetRequired = false
		result.ActionPerformed = "print"
		execErr = r.doPrint(ctx, cmd, &result)

	case dsl.CmdWaitForResponse:
		result.TargetRequired = false
		result.ActionPerformed = "wait_for_response"
		execErr = r.doWaitForResponse(ctx, cmd, &result)

	case dsl.CmdPause:
		result.TargetRequired = false
		result.ActionPerformed = "pause"
		execErr = r.doPause(ctx, cmd, &result)

	case dsl.CmdDebugVars:
		result.TargetRequired = false
		result.ActionPerformed = "debug_vars"
		execErr = r.doDebugVars(ctx, cmd, &result)

	default:
		execErr = fmt.Errorf("unknown command type: %s", cmd.Raw)
	}

	result.Duration = time.Since(start)
	result.DurationMS = result.Duration.Milliseconds()
	if execErr != nil {
		result.Success = false
		result.Error = execErr.Error()
		r.logger.Warn("  ✗ %s", execErr)

		// Screenshot on failure
		if r.cfg.Screenshot == "on-fail" || r.cfg.Screenshot == "always" {
			r.captureScreenshot(ctx, &result, idx)
		}
	} else {
		result.Success = true
		r.logger.Info("  ✓ done (%.0fms)", float64(result.Duration.Milliseconds()))

		// Screenshot on every step
		if r.cfg.Screenshot == "always" {
			r.captureScreenshot(ctx, &result, idx)
		}

		// Debug mode: highlight the resolved element
		if r.cfg.DebugMode && result.WinnerXPath != "" {
			_ = r.page.HighlightElement(ctx, result.WinnerXPath, 800)
		}
	}

	// Explain mode: output candidate ranking details
	if r.cfg.ExplainMode && len(result.RankedCandidates) > 0 {
		r.logger.Info("  ╭── EXPLAIN: %d candidates considered", result.CandidatesConsidered)
		limit := 5
		if len(result.RankedCandidates) < limit {
			limit = len(result.RankedCandidates)
		}
		for ci := 0; ci < limit; ci++ {
			c := result.RankedCandidates[ci]
			marker := "  │"
			if ci == 0 {
				marker = "  │ ★"
			}
			r.logger.Info("%s  [%.2f] %s  tag=%s text=%q", marker, c.Score.Total, c.XPath, c.Tag, c.VisibleText)
		}
		r.logger.Info("  ╰──")
	}

	return result
}

// ── Command handlers ──────────────────────────────────────────────────────────

func (r *Runtime) doNavigate(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	if cmd.URL == "" {
		return fmt.Errorf("NAVIGATE: no URL specified in %q", cmd.Raw)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.NavigationTimeout)
	defer cancel()
	if err := r.page.Navigate(timeoutCtx, cmd.URL); err != nil {
		return fmt.Errorf("NAVIGATE to %q: %w", cmd.URL, err)
	}
	// Update URL after navigation
	if url, err := r.page.CurrentURL(ctx); err == nil {
		res.PageURL = url
	}
	return nil
}

func (r *Runtime) doWait(_ context.Context, cmd dsl.Command, _ *explain.ExecutionResult) error {
	if cmd.WaitSeconds <= 0 {
		return nil
	}
	d := time.Duration(cmd.WaitSeconds * float64(time.Second))
	time.Sleep(d)
	return nil
}

func (r *Runtime) doVerify(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	if cmd.VerifyText == "" {
		return fmt.Errorf("VERIFY: no text specified in %q", cmd.Raw)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.VerifyTimeout)
	defer cancel()

	needle := strings.ToLower(strings.TrimSpace(cmd.VerifyText))
	deadline := time.Now().Add(r.cfg.VerifyTimeout)
	pollInterval := 500 * time.Millisecond

	for {
		_, pageText, err := r.targeting.ProbeVisibleText(timeoutCtx, r.page)
		if err != nil {
			return fmt.Errorf("VERIFY: page probe failed: %w", err)
		}
		found := strings.Contains(pageText, needle)
		if cmd.VerifyNegated {
			if !found {
				return nil // NOT present — success
			}
		} else {
			if found {
				return nil // present — success
			}
		}

		if time.Now().After(deadline) {
			break
		}
		select {
		case <-timeoutCtx.Done():
			return timeoutCtx.Err()
		case <-time.After(pollInterval):
		}
	}

	if cmd.VerifyNegated {
		return fmt.Errorf("VERIFY: %q is still present (expected NOT present)", cmd.VerifyText)
	}
	return fmt.Errorf("VERIFY: %q not found on page", cmd.VerifyText)
}

func (r *Runtime) doClick(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	// Scroll into view before clicking
	if err := r.page.ScrollIntoView(timeoutCtx, el.XPath); err != nil {
		r.logger.Debug("scroll into view failed (non-fatal): %v", err)
	}

	// Capture current URL so we can detect post-click navigation.
	urlBefore, _ := r.page.CurrentURL(timeoutCtx)

	// Primary click: JS element.click() — dispatches a synthetic click event
	// that React and other SPA frameworks handle correctly. Falls back to
	// coordinate-based mouse events if the element cannot be resolved.
	jsClickExpr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return false;
		el.click();
		return true;
	})()`, el.XPath)
	raw, jsErr := r.page.EvalJS(timeoutCtx, jsClickExpr)
	jsOK := false
	if jsErr == nil {
		_ = json.Unmarshal(raw, &jsOK)
	}
	if !jsOK {
		// Fallback: coordinate-based mouse events.
		cx := el.Rect.Left + el.Rect.Width/2
		cy := el.Rect.Top + el.Rect.Height/2
		if err := r.page.Click(timeoutCtx, cx, cy); err != nil {
			return &utils.ActionError{Action: "click", Target: cmd.Target, Cause: err}
		}
	}

	// Poll for URL change for up to 1 s (50 ms intervals).
	navDetected := false
	for i := 0; i < 20; i++ {
		select {
		case <-timeoutCtx.Done():
			goto afterNav
		case <-time.After(50 * time.Millisecond):
		}
		urlAfter, _ := r.page.CurrentURL(timeoutCtx)
		if urlAfter != "" && urlAfter != urlBefore {
			navDetected = true
			break
		}
	}

	if navDetected {
		// Wait for document.readyState == 'complete' via JS polling.
		if err := r.page.WaitForLoad(timeoutCtx); err != nil {
			r.logger.Debug("click: WaitForLoad error (non-fatal): %v", err)
		}
	}

afterNav:

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doFill(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	if err := r.page.ScrollIntoView(timeoutCtx, el.XPath); err != nil {
		r.logger.Debug("scroll into view failed (non-fatal): %v", err)
	}

	if err := r.page.SetInputValue(timeoutCtx, el.XPath, cmd.Value); err != nil {
		return &utils.ActionError{Action: "fill", Target: cmd.Target, Cause: err}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doSelect(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	// Select by setting value via JS
	selectExpr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return false;
		const options = Array.from(el.options || []);
		const opt = options.find(o =>
			o.text.toLowerCase().trim() === %q ||
			o.value.toLowerCase().trim() === %q
		);
		if (opt) {
			el.value = opt.value;
			el.dispatchEvent(new Event('change', {bubbles:true}));
			el.dispatchEvent(new Event('input', {bubbles:true}));
			return true;
		}
		return false;
	})()`, el.XPath, strings.ToLower(cmd.Value), strings.ToLower(cmd.Value))

	raw, err := r.page.EvalJS(timeoutCtx, selectExpr)
	if err != nil {
		return &utils.ActionError{Action: "select", Target: cmd.Target, Cause: err}
	}
	var ok bool
	if jsonErr := json.Unmarshal(raw, &ok); jsonErr != nil || !ok {
		return &utils.ActionError{
			Action: "select",
			Target: cmd.Target,
			Cause:  fmt.Errorf("option %q not found in dropdown %q", cmd.Value, cmd.Target),
		}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doCheck(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	want := cmd.Type == dsl.CmdCheck // true = check, false = uncheck
	checkExpr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return false;
		if (el.checked !== %v) {
			el.click();
			el.dispatchEvent(new Event('change', {bubbles:true}));
		}
		return true;
	})()`, el.XPath, want)

	if _, err := r.page.EvalJS(timeoutCtx, checkExpr); err != nil {
		return &utils.ActionError{Action: string(cmd.Type), Target: cmd.Target, Cause: err}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

// ── New command handlers ──────────────────────────────────────────────────────

func (r *Runtime) doScroll(ctx context.Context, cmd dsl.Command, _ *explain.ExecutionResult) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	if err := r.page.ScrollPage(timeoutCtx, cmd.ScrollDirection, cmd.ScrollContainer); err != nil {
		return fmt.Errorf("SCROLL: %w", err)
	}
	// Wait for content to settle after scroll (matches ManulEngine's SCROLL_WAIT).
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (r *Runtime) doPress(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	// If PRESS Key ON 'Target', focus the target first.
	if cmd.PressTarget != "" {
		resolved, err := r.resolveTarget(ctx, dsl.Command{
			Target:          cmd.PressTarget,
			InteractionMode: dsl.ModeClickable,
		}, res)
		if err != nil {
			return fmt.Errorf("PRESS: resolve target %q: %w", cmd.PressTarget, err)
		}
		if err := r.page.FocusByXPath(timeoutCtx, resolved.Element.XPath); err != nil {
			r.logger.Debug("PRESS: focus failed (non-fatal): %v", err)
		}
	}

	key, modifiers := parseKeyCombo(cmd.PressKey)
	if err := r.page.DispatchKey(timeoutCtx, key, modifiers); err != nil {
		return fmt.Errorf("PRESS %q: %w", cmd.PressKey, err)
	}
	return nil
}

func (r *Runtime) doDoubleClick(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	if err := r.page.ScrollIntoView(timeoutCtx, el.XPath); err != nil {
		r.logger.Debug("scroll into view failed (non-fatal): %v", err)
	}

	cx := el.Rect.Left + el.Rect.Width/2
	cy := el.Rect.Top + el.Rect.Height/2
	if err := r.page.DoubleClick(timeoutCtx, cx, cy); err != nil {
		return &utils.ActionError{Action: "double_click", Target: cmd.Target, Cause: err}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doExtract(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	if cmd.ExtractVar == "" {
		return fmt.Errorf("EXTRACT: no variable name specified (use 'into {var}')")
	}

	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	// Extract element text via JS.
	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return "";
		if (el.value !== undefined && el.value !== "") return el.value;
		return (el.innerText || el.textContent || "").trim();
	})()`, el.XPath)

	raw, err := r.page.EvalJS(timeoutCtx, expr)
	if err != nil {
		return fmt.Errorf("EXTRACT: JS eval failed: %w", err)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return fmt.Errorf("EXTRACT: unmarshal result: %w", err)
	}

	r.vars[cmd.ExtractVar] = text
	r.logger.Info("  → {%s} = %q", cmd.ExtractVar, text)

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doSet(_ context.Context, cmd dsl.Command, _ *explain.ExecutionResult) error {
	r.vars[cmd.SetVar] = cmd.SetValue
	r.logger.Info("  → {%s} = %q", cmd.SetVar, cmd.SetValue)
	return nil
}

func (r *Runtime) doHover(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	if err := r.page.ScrollIntoView(timeoutCtx, el.XPath); err != nil {
		r.logger.Debug("scroll into view failed (non-fatal): %v", err)
	}

	cx, cy, err := r.page.GetElementCenter(timeoutCtx, el.XPath)
	if err != nil {
		return &utils.ActionError{Action: "hover", Target: cmd.Target, Cause: err}
	}
	if err := r.page.Hover(timeoutCtx, cx, cy); err != nil {
		return &utils.ActionError{Action: "hover", Target: cmd.Target, Cause: err}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doRightClick(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	if err := r.page.ScrollIntoView(timeoutCtx, el.XPath); err != nil {
		r.logger.Debug("scroll into view failed (non-fatal): %v", err)
	}

	cx, cy, err := r.page.GetElementCenter(timeoutCtx, el.XPath)
	if err != nil {
		return &utils.ActionError{Action: "right_click", Target: cmd.Target, Cause: err}
	}
	if err := r.page.RightClick(timeoutCtx, cx, cy); err != nil {
		return &utils.ActionError{Action: "right_click", Target: cmd.Target, Cause: err}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doDrag(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	// Resolve source element
	srcCmd := dsl.Command{
		Target:          cmd.DragSource,
		InteractionMode: dsl.ModeClickable,
	}
	srcResolved, err := r.resolveTarget(ctx, srcCmd, res)
	if err != nil {
		return fmt.Errorf("DRAG source %q: %w", cmd.DragSource, err)
	}

	// Resolve target element
	dstCmd := dsl.Command{
		Target:          cmd.DragTarget,
		InteractionMode: dsl.ModeClickable,
	}
	dstResolved, err := r.resolveTarget(ctx, dstCmd, res)
	if err != nil {
		return fmt.Errorf("DRAG target %q: %w", cmd.DragTarget, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	// Scroll source into view (both elements should be visible for drag)
	if err := r.page.ScrollIntoView(timeoutCtx, srcResolved.Element.XPath); err != nil {
		r.logger.Debug("scroll source into view failed (non-fatal): %v", err)
	}
	// Also scroll target into view if it's a different element
	if err := r.page.ScrollIntoView(timeoutCtx, dstResolved.Element.XPath); err != nil {
		r.logger.Debug("scroll target into view failed (non-fatal): %v", err)
	}
	// Scroll source once more so both are approximately in viewport
	if err := r.page.ScrollIntoView(timeoutCtx, srcResolved.Element.XPath); err != nil {
		r.logger.Debug("scroll source into view (2nd) failed (non-fatal): %v", err)
	}
	time.Sleep(50 * time.Millisecond) // let viewport settle

	fromX, fromY, err := r.page.GetElementCenter(timeoutCtx, srcResolved.Element.XPath)
	if err != nil {
		return fmt.Errorf("DRAG: get source centre: %w", err)
	}
	toX, toY, err := r.page.GetElementCenter(timeoutCtx, dstResolved.Element.XPath)
	if err != nil {
		return fmt.Errorf("DRAG: get target centre: %w", err)
	}

	r.logger.Debug("DRAG: src=(%v,%v) → dst=(%v,%v)", fromX, fromY, toX, toY)

	// Use CDP mouse events — works for jQuery UI and native HTML5 drag
	if err := r.page.DragAndDrop(timeoutCtx, fromX, fromY, toX, toY); err != nil {
		return fmt.Errorf("DRAG %q → %q: %w", cmd.DragSource, cmd.DragTarget, err)
	}

	time.Sleep(300 * time.Millisecond) // settle time for DOM update

	// If CDP drag didn't trigger the drop (e.g. native HTML5 droppable),
	// try firing HTML5 DragEvent as a fallback
	checkDropped := fmt.Sprintf(`(() => {
		function xp(p) {
			return document.evaluate(p, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		}
		const dst = xp(%q);
		if (!dst) return 'no_target';
		
		// Check if drop already happened by looking for state change
		const text = (dst.innerText || dst.textContent || '').trim();
		if (text !== 'Drop here') return 'already_dropped';
		
		// Not dropped yet — try HTML5 DragEvent fallback
		const src = xp(%q);
		if (!src) return 'no_source';
		const dt = new DataTransfer();
		src.dispatchEvent(new DragEvent('dragstart', {bubbles:true, cancelable:true, dataTransfer:dt}));
		dst.dispatchEvent(new DragEvent('dragenter', {bubbles:true, cancelable:true, dataTransfer:dt}));
		dst.dispatchEvent(new DragEvent('dragover',  {bubbles:true, cancelable:true, dataTransfer:dt}));
		dst.dispatchEvent(new DragEvent('drop',      {bubbles:true, cancelable:true, dataTransfer:dt}));
		src.dispatchEvent(new DragEvent('dragend',   {bubbles:true, cancelable:true, dataTransfer:dt}));
		return 'html5_fallback';
	})()`, dstResolved.Element.XPath, srcResolved.Element.XPath)

	raw, _ := r.page.EvalJS(timeoutCtx, checkDropped)
	var dropResult string
	json.Unmarshal(raw, &dropResult)
	r.logger.Debug("DRAG: drop status: %s", dropResult)

	res.WinnerXPath = srcResolved.Element.XPath
	res.WinnerScore = srcResolved.Score
	return nil
}

func (r *Runtime) doUpload(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	if err := r.page.SetFileInput(timeoutCtx, el.XPath, []string{cmd.UploadFile}); err != nil {
		return &utils.ActionError{Action: "upload", Target: cmd.Target, Cause: err}
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doPrint(_ context.Context, cmd dsl.Command, _ *explain.ExecutionResult) error {
	r.logger.Info("  PRINT: %s", cmd.PrintText)
	return nil
}

func (r *Runtime) doWaitForResponse(ctx context.Context, cmd dsl.Command, _ *explain.ExecutionResult) error {
	timeout := r.cfg.DefaultTimeout
	if err := r.page.WaitForResponse(ctx, cmd.WaitResponseURL, timeout); err != nil {
		return fmt.Errorf("WAIT FOR RESPONSE %q: %w", cmd.WaitResponseURL, err)
	}
	return nil
}

func (r *Runtime) doPause(_ context.Context, _ dsl.Command, _ *explain.ExecutionResult) error {
	if r.cfg.DebugMode {
		r.logger.Info("  ⏸  PAUSED — press Enter to continue")
		fmt.Scanln()
	}
	return nil
}

func (r *Runtime) doDebugVars(_ context.Context, _ dsl.Command, _ *explain.ExecutionResult) error {
	r.logger.Info("  ── Runtime Variables ──")
	if len(r.vars) == 0 {
		r.logger.Info("    (none)")
		return nil
	}
	for k, v := range r.vars {
		r.logger.Info("    {%s} = %q", k, v)
	}
	return nil
}

func (r *Runtime) doVerifySoft(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	err := r.doVerify(ctx, cmd, res)
	if err != nil {
		// Non-fatal: log warning, accumulate, but don't fail.
		r.softErrors = append(r.softErrors, fmt.Sprintf("line %d: %s", cmd.LineNumber, err.Error()))
		r.logger.Warn("  ⚠ SOFT VERIFY failed: %s (continuing)", err)
		res.Success = true // Override — soft verify doesn't fail the run.
		res.Error = fmt.Sprintf("SOFT: %s", err.Error())
		return nil
	}
	return nil
}

func (r *Runtime) doVerifyField(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	resolved, err := r.resolveTarget(ctx, cmd, res)
	if err != nil {
		return err
	}

	el := resolved.Element
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.VerifyTimeout)
	defer cancel()

	var jsField string
	switch cmd.VerifyFieldKind {
	case "value":
		jsField = `el.value || el.getAttribute("value") || ""`
	case "placeholder":
		jsField = `el.getAttribute("placeholder") || ""`
	default: // "text"
		jsField = `(el.innerText || el.textContent || "").trim()`
	}

	expr := fmt.Sprintf(`(() => {
		const r = document.evaluate(%q, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
		const el = r.singleNodeValue;
		if (!el) return null;
		return %s;
	})()`, el.XPath, jsField)

	raw, jsErr := r.page.EvalJS(timeoutCtx, expr)
	if jsErr != nil {
		return fmt.Errorf("VERIFY field: JS eval failed: %w", jsErr)
	}

	var actual string
	if err := json.Unmarshal(raw, &actual); err != nil {
		return fmt.Errorf("VERIFY field: expected %q, but element not found", cmd.Value)
	}

	expected := strings.ToLower(strings.TrimSpace(cmd.Value))
	got := strings.ToLower(strings.TrimSpace(actual))
	if got != expected {
		return fmt.Errorf("VERIFY: %q field HAS %s — expected %q, got %q",
			cmd.Target, strings.ToUpper(cmd.VerifyFieldKind), cmd.Value, actual)
	}

	res.WinnerXPath = el.XPath
	res.WinnerScore = resolved.Score
	return nil
}

func (r *Runtime) doWaitFor(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) error {
	if cmd.Target == "" {
		return fmt.Errorf("WAIT FOR: no target specified in %q", cmd.Raw)
	}

	deadline := time.Now().Add(r.cfg.VerifyTimeout)
	pollInterval := 500 * time.Millisecond

	for {
		_, pageText, err := r.targeting.ProbeVisibleText(ctx, r.page)
		if err != nil {
			return fmt.Errorf("WAIT FOR: probe failed: %w", err)
		}
		needle := strings.ToLower(strings.TrimSpace(cmd.Target))
		found := strings.Contains(pageText, needle)

		switch cmd.WaitForState {
		case "visible":
			if found {
				return nil
			}
		case "hidden", "disappear":
			if !found {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("WAIT FOR %q to be %s: timed out after %v",
				cmd.Target, cmd.WaitForState, r.cfg.VerifyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// ── Control flow ──────────────────────────────────────────────────────────────

// executeIf handles IF/ELIF/ELSE blocks using pre-nested Branches.
func (r *Runtime) executeIf(ctx context.Context, cmd dsl.Command, fileVars map[string]string) ([]explain.ExecutionResult, bool) {
	for _, branch := range cmd.Branches {
		if branch.Kind == "else" || r.evaluateCondition(ctx, branch.Condition) {
			r.logger.Debug("  [IF] taking %s branch (condition: %s)", branch.Kind, branch.Condition)
			return r.executeBlock(ctx, branch.Body, fileVars)
		}
	}
	// No branch taken.
	return nil, false
}

// executeWhile handles WHILE loops using pre-nested Body.
func (r *Runtime) executeWhile(ctx context.Context, cmd dsl.Command, fileVars map[string]string) ([]explain.ExecutionResult, bool) {
	condition := cmd.Condition
	var allResults []explain.ExecutionResult
	for iteration := 1; iteration <= maxLoopIterations; iteration++ {
		if !r.evaluateCondition(ctx, condition) {
			break
		}
		r.vars["i"] = strconv.Itoa(iteration)
		results, stop := r.executeBlock(ctx, cmd.Body, fileVars)
		allResults = append(allResults, results...)
		if stop {
			return allResults, true
		}
	}
	return allResults, false
}

// executeRepeat handles REPEAT N TIMES loops using pre-nested Body.
func (r *Runtime) executeRepeat(ctx context.Context, cmd dsl.Command, fileVars map[string]string) ([]explain.ExecutionResult, bool) {
	count := cmd.RepeatCount
	loopVar := cmd.RepeatVar

	var allResults []explain.ExecutionResult
	for iteration := 1; iteration <= count; iteration++ {
		r.vars[loopVar] = strconv.Itoa(iteration)
		r.vars["i"] = strconv.Itoa(iteration)
		results, stop := r.executeBlock(ctx, cmd.Body, fileVars)
		allResults = append(allResults, results...)
		if stop {
			return allResults, true
		}
	}
	return allResults, false
}

// executeForEach handles FOR EACH {var} IN {collection} loops using pre-nested Body.
// The collection variable is split by comma, pipe, or newline.
func (r *Runtime) executeForEach(ctx context.Context, cmd dsl.Command, fileVars map[string]string) ([]explain.ExecutionResult, bool) {
	loopVar := cmd.ForEachVar
	collection := cmd.ForEachCollection

	// Resolve the collection — could be a variable reference like {items}
	items := r.splitCollection(collection)

	var allResults []explain.ExecutionResult
	for idx, item := range items {
		r.vars[loopVar] = strings.TrimSpace(item)
		r.vars["i"] = strconv.Itoa(idx + 1)
		r.logger.Debug("  [FOR EACH] {%s} = %q  (iteration %d/%d)", loopVar, item, idx+1, len(items))
		results, stop := r.executeBlock(ctx, cmd.Body, fileVars)
		allResults = append(allResults, results...)
		if stop {
			return allResults, true
		}
	}
	return allResults, false
}

// splitCollection splits a collection string by comma, pipe, or newline.
func (r *Runtime) splitCollection(collection string) []string {
	// Try comma-separated first
	if strings.Contains(collection, ",") {
		return strings.Split(collection, ",")
	}
	// Pipe-separated
	if strings.Contains(collection, "|") {
		return strings.Split(collection, "|")
	}
	// Newline-separated
	if strings.Contains(collection, "\n") {
		return strings.Split(collection, "\n")
	}
	// Single item
	return []string{collection}
}

// evaluateCondition checks a condition string against runtime state.
// Supports:
//   - button/element/link/field 'X' exists / not exists
//   - text 'X' is present / text 'X' is not present
//   - {var} == 'value', {var} != 'value', {var} contains 'substring'
//   - {var} (truthy)
func (r *Runtime) evaluateCondition(ctx context.Context, cond string) bool {
	condLower := strings.ToLower(strings.TrimSpace(cond))

	// button/element/link/field 'X' [not] exists
	if reCondElementExists.MatchString(condLower) {
		needle := extractQuotedFromCond(cond)
		if needle == "" {
			return false
		}
		_, pageText, err := r.targeting.ProbeVisibleText(ctx, r.page)
		if err != nil {
			return false
		}
		found := strings.Contains(pageText, strings.ToLower(needle))
		if strings.Contains(condLower, "not exists") || strings.Contains(condLower, "not exist") {
			return !found
		}
		return found
	}

	// text 'X' is present / text 'X' is not present
	if strings.HasPrefix(condLower, "text ") || strings.Contains(condLower, "is present") || strings.Contains(condLower, "is not present") {
		needle := extractQuotedFromCond(cond)
		if needle == "" {
			return false
		}
		_, pageText, err := r.targeting.ProbeVisibleText(ctx, r.page)
		if err != nil {
			return false
		}
		found := strings.Contains(pageText, strings.ToLower(needle))
		if strings.Contains(condLower, "not present") {
			return !found
		}
		return found
	}

	// {var} == 'value' / {var} != 'value'
	if strings.Contains(cond, "==") || strings.Contains(cond, "!=") {
		var parts []string
		var isNeg bool
		if strings.Contains(cond, "!=") {
			parts = strings.SplitN(cond, "!=", 2)
			isNeg = true
		} else {
			parts = strings.SplitN(cond, "==", 2)
		}
		if len(parts) == 2 {
			varVal := r.resolveVar(strings.TrimSpace(parts[0]))
			expected := unquote(strings.TrimSpace(parts[1]))
			if isNeg {
				return !strings.EqualFold(varVal, expected)
			}
			return strings.EqualFold(varVal, expected)
		}
	}

	// {var} contains 'substring'
	if strings.Contains(condLower, " contains ") {
		parts := strings.SplitN(condLower, " contains ", 2)
		if len(parts) == 2 {
			varVal := strings.ToLower(r.resolveVar(strings.TrimSpace(parts[0])))
			sub := unquote(strings.TrimSpace(parts[1]))
			return strings.Contains(varVal, strings.ToLower(sub))
		}
	}

	// {var} — truthy check (non-empty and not 'false'/'0'/'none')
	v := r.resolveVar(strings.TrimSpace(cond))
	return v != "" && v != "false" && v != "0" && v != "none"
}

// reCondElementExists matches: button/element/link/field/input/checkbox/radio/dropdown 'X' [not] exists
var reCondElementExists = regexp.MustCompile(`(?i)^(?:button|element|link|field|input|checkbox|radio|dropdown)\s+`)

// reQuotedSimple is a simple single+double quote extractor for conditions.
var reQuotedSimple = regexp.MustCompile(`(?:"([^"]*)"|'([^']*)')`)

// extractQuotedFromCond extracts the first quoted string from a condition.
func extractQuotedFromCond(s string) string {
	m := reQuotedSimple.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// resolveVar resolves a {varName} reference or returns the string as-is.
func (r *Runtime) resolveVar(s string) string {
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if v, ok := r.vars[s]; ok {
		return v
	}
	return s
}

// unquote strips surrounding single or double quotes from a string.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// applyRuntimeVars substitutes {varName} from runtime vars into command fields.
func (r *Runtime) applyRuntimeVars(cmd *dsl.Command) {
	if len(r.vars) == 0 {
		return
	}
	sub := func(s string) string {
		for k, v := range r.vars {
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
	cmd.DragSource = sub(cmd.DragSource)
	cmd.DragTarget = sub(cmd.DragTarget)
	cmd.PrintText = sub(cmd.PrintText)
	cmd.UploadFile = sub(cmd.UploadFile)
	cmd.WaitResponseURL = sub(cmd.WaitResponseURL)
	cmd.ForEachCollection = sub(cmd.ForEachCollection)
}

// parseKeyCombo parses "Control+A" or "Enter" into key name and modifier bitmask.
func parseKeyCombo(combo string) (key string, modifiers int) {
	parts := strings.Split(combo, "+")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch strings.ToLower(p) {
		case "control", "ctrl":
			modifiers |= 2
		case "alt":
			modifiers |= 1
		case "meta", "command", "cmd":
			modifiers |= 4
		case "shift":
			modifiers |= 8
		default:
			key = p
		}
	}
	if key == "" {
		key = combo
	}
	return
}

func (r *Runtime) resolveTarget(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) (*core.ResolvedTarget, error) {
	if cmd.Target == "" {
		return nil, fmt.Errorf("command %q has no target", cmd.Raw)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	mode := string(cmd.InteractionMode)
	resolved, err := r.targeting.ResolveWithQualifiers(
		timeoutCtx, r.page, cmd.Target, cmd.TypeHint, mode, cmd.NearAnchor,
		cmd.OnRegion, cmd.InsideContainer, cmd.InsideRowText,
	)
	if err != nil {
		return nil, err
	}

	res.CandidatesConsidered = resolved.TotalConsidered
	res.RankedCandidates = core.BuildCandidateExplain(resolved.RankedCandidates)
	return resolved, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// captureScreenshot takes a screenshot and saves it to disk.
func (r *Runtime) captureScreenshot(ctx context.Context, res *explain.ExecutionResult, idx int) {
	pngData, err := r.page.Screenshot(ctx)
	if err != nil {
		r.logger.Debug("screenshot failed (non-fatal): %v", err)
		return
	}

	dir := "reports/screenshots"
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		r.logger.Debug("screenshot mkdir failed: %v", mkErr)
		return
	}

	filename := fmt.Sprintf("%s/step_%03d_%s.png", dir, idx+1, strings.ReplaceAll(res.ActionPerformed, " ", "_"))
	if err := os.WriteFile(filename, pngData, 0o644); err != nil {
		r.logger.Debug("screenshot write failed: %v", err)
		return
	}

	res.ScreenshotPath = filename
	r.logger.Info("  📸 screenshot saved: %s", filename)
}

func parseSingleCommand(raw string) (dsl.Command, error) {
	hunt, err := dsl.Parse(strings.NewReader(raw))
	if err != nil {
		return dsl.Command{}, err
	}
	if len(hunt.Commands) == 0 {
		return dsl.Command{}, fmt.Errorf("could not parse command: %q", raw)
	}
	return hunt.Commands[0], nil
}

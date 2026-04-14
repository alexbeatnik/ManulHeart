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
	"strings"
	"time"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/core"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/explain"
	"github.com/manulengineer/manulheart/pkg/utils"
)

// Runtime executes Hunt commands against a browser page.
type Runtime struct {
	cfg       config.Config
	page      browser.Page
	targeting *core.Targeting
	logger    *utils.Logger
}

// New constructs a Runtime for the given page and config.
func New(cfg config.Config, page browser.Page, logger *utils.Logger) *Runtime {
	return &Runtime{
		cfg:       cfg,
		page:      page,
		targeting: core.NewTargeting(cfg, logger),
		logger:    logger,
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

	for i, cmd := range hunt.Commands {
		execResult := r.executeCommand(ctx, cmd, i)
		result.Results = append(result.Results, execResult)
		if execResult.Success {
			result.Passed++
		} else {
			result.Failed++
			// Stop execution on first failure (fail-fast)
			r.logger.Error("FAILED [%d] %s → %s", i+1, cmd.Raw, execResult.Error)
			break
		}
	}

	result.TotalDuration = time.Since(start)
	result.Success = result.Failed == 0
	return result, nil
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

	case dsl.CmdVerify:
		result.TargetRequired = false
		result.ActionPerformed = "verify"
		execErr = r.doVerify(ctx, cmd, &result)

	case dsl.CmdClick:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "click"
		execErr = r.doClick(ctx, cmd, &result)

	case dsl.CmdFill, dsl.CmdType:
		result.TargetRequired = true
		result.TargetQuery = cmd.Target
		result.TypeHint = cmd.TypeHint
		result.ActionPerformed = "fill"
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

	default:
		execErr = fmt.Errorf("unknown command type: %s", cmd.Raw)
	}

	result.Duration = time.Since(start)
	if execErr != nil {
		result.Success = false
		result.Error = execErr.Error()
		r.logger.Warn("  ✗ %s", execErr)
	} else {
		result.Success = true
		r.logger.Info("  ✓ done (%.0fms)", float64(result.Duration.Milliseconds()))
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

// ── Shared targeting helper ───────────────────────────────────────────────────

func (r *Runtime) resolveTarget(ctx context.Context, cmd dsl.Command, res *explain.ExecutionResult) (*core.ResolvedTarget, error) {
	if cmd.Target == "" {
		return nil, fmt.Errorf("command %q has no target", cmd.Raw)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.DefaultTimeout)
	defer cancel()

	mode := string(cmd.InteractionMode)
	resolved, err := r.targeting.ResolveWithContext(timeoutCtx, r.page, cmd.Target, cmd.TypeHint, mode, cmd.NearAnchor)
	if err != nil {
		return nil, err
	}

	res.CandidatesConsidered = resolved.TotalConsidered
	res.RankedCandidates = core.BuildCandidateExplain(resolved.RankedCandidates)
	return resolved, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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

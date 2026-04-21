package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/scorer"
)

var ErrDebugStop = errors.New("debug: stop requested")

func (rt *Runtime) shouldPause(cmd dsl.Command, idx int) bool {
	if rt.debugContinue {
		return false
	}
	if len(rt.breakLines) == 0 && len(rt.breakSteps) == 0 {
		return true
	}
	if rt.breakLines[cmd.LineNum] {
		return true
	}
	if rt.breakSteps != nil && rt.breakSteps[idx] {
		return true
	}
	return false
}

func isTTY() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func (rt *Runtime) injectDebugModal(ctx context.Context, step string) error {
	stepJSON, _ := json.Marshal(step)
	js := fmt.Sprintf(`(function(){
var ex=document.getElementById('manul-debug-modal');if(ex)ex.remove();
var d=document.createElement('div');d.id='manul-debug-modal';
d.style.cssText='position:fixed;top:10px;right:10px;z-index:2147483647;background:#1a1a2e;color:#eee;padding:12px 16px;border-radius:8px;font-family:monospace;font-size:13px;max-width:420px;box-shadow:0 4px 20px rgba(0,0,0,.6);border:1px solid #444';
d.innerHTML='<div style="color:#ff79c6;margin-bottom:6px">⏸ ManulHeart Debug<\/div><div style="color:#aaa;margin-bottom:8px;word-break:break-all">'+%s+'<\/div><button id="manul-dbg-continue" style="background:#50fa7b;color:#000;border:none;padding:4px 10px;border-radius:4px;cursor:pointer;margin-right:6px;font-size:12px">Continue<\/button><button id="manul-dbg-abort" style="background:#ff5555;color:#fff;border:none;padding:4px 10px;border-radius:4px;cursor:pointer;font-size:12px">Abort<\/button>';
document.body.appendChild(d);
document.getElementById('manul-dbg-continue').onclick=function(){window.__manul_debug_action='continue';d.remove();};
document.getElementById('manul-dbg-abort').onclick=function(){window.__manul_debug_action='abort';d.remove();};
window.__manul_debug_action='';
})();`, string(stepJSON))
	_, err := rt.page.EvalJS(ctx, js)
	return err
}

func (rt *Runtime) removeDebugModal(ctx context.Context) error {
	js := `(function(){var d=document.getElementById('manul-debug-modal');if(d)d.remove();window.__manul_debug_action='';})();`
	_, err := rt.page.EvalJS(ctx, js)
	return err
}

func (rt *Runtime) debugHighlight(ctx context.Context, xpath string) error {
	xpathJSON, _ := json.Marshal(xpath)
	js := fmt.Sprintf(`(function(){
var s=document.getElementById('manul-debug-style');
if(!s){s=document.createElement('style');s.id='manul-debug-style';document.head.appendChild(s);}
s.textContent='[data-manul-debug-highlight]{outline:4px solid #ff00ff!important;box-shadow:0 0 15px #ff00ff!important;background:rgba(255,0,255,.12)!important;z-index:999999!important;}';
var prev=document.querySelector('[data-manul-debug-highlight]');if(prev)prev.removeAttribute('data-manul-debug-highlight');
var r=document.evaluate(%s,document,null,XPathResult.FIRST_ORDERED_NODE_TYPE,null);
var el=r.singleNodeValue;
if(el){el.setAttribute('data-manul-debug-highlight','true');el.scrollIntoView({block:'center'});}
})();`, string(xpathJSON))
	_, err := rt.page.EvalJS(ctx, js)
	return err
}

func (rt *Runtime) clearDebugHighlight(ctx context.Context) error {
	js := `(function(){
var el=document.querySelector('[data-manul-debug-highlight]');if(el)el.removeAttribute('data-manul-debug-highlight');
var s=document.getElementById('manul-debug-style');if(s)s.remove();
})();`
	_, err := rt.page.EvalJS(ctx, js)
	return err
}

func (rt *Runtime) pollForAbort(ctx context.Context, abortCh chan<- struct{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
		raw, err := rt.page.EvalJS(ctx, `window.__manul_debug_action||""`)
		if err != nil {
			return
		}
		var action string
		if json.Unmarshal(raw, &action) == nil && action == "abort" {
			select {
			case abortCh <- struct{}{}:
			default:
			}
			return
		}
	}
}

func scoreToConfidence(s float64) int {
	switch {
	case s >= 1.0:
		return 10
	case s >= 0.5:
		return 9
	case s >= 0.1:
		return 7
	case s >= 0.05:
		return 5
	case s >= 0.01:
		return 3
	case s > 0:
		return 1
	default:
		return 0
	}
}

func (rt *Runtime) explainStep(ctx context.Context, cmd dsl.Command) string {
	elements, err := rt.loadSnapshot(ctx)
	if err != nil {
		return fmt.Sprintf("explain: snapshot failed: %v", err)
	}

	query := cmd.Target
	if query == "" {
		query = cmd.Raw
	}
	mode := string(cmd.InteractionMode)
	if mode == "" {
		mode = string(dsl.ModeNone)
	}

	ranked := scorer.Rank(query, cmd.TypeHint, mode, elements, 5, nil)
	rt.lastExplainData = ranked

	if len(ranked) == 0 {
		return fmt.Sprintf("explain: no candidates found for %q", query)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "explain: top %d for %q\n", len(ranked), query)
	for i, c := range ranked {
		conf := scoreToConfidence(c.Explain.Score.Total)
		text := c.Element.VisibleText
		if len(text) > 60 {
			text = text[:57] + "..."
		}
		fmt.Fprintf(&sb, "  #%d score=%.3f conf=%d/10 <%s> %q\n      xpath=%s\n",
			i+1, c.Explain.Score.Total, conf, c.Element.Tag, text, c.Element.XPath)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (rt *Runtime) debugPrompt(ctx context.Context, cmd dsl.Command, idx int) error {
	if isTTY() {
		return rt.debugPromptTTY(ctx, cmd, idx)
	}
	return rt.debugPromptExtension(ctx, cmd, idx)
}

func (rt *Runtime) debugPromptTTY(ctx context.Context, cmd dsl.Command, idx int) error {
	if err := rt.injectDebugModal(ctx, cmd.Raw); err != nil {
		rt.logger.Warn("debug: modal inject failed: %v", err)
	}
	defer rt.removeDebugModal(ctx)

	abortCh := make(chan struct{}, 1)
	go rt.pollForAbort(ctx, abortCh)

	sc := bufio.NewScanner(os.Stdin)
	inputCh := make(chan string, 1)

	readNext := func() {
		go func() {
			rt.logger.Info("\n[DEBUG] paused at: %s", cmd.Raw)
			rt.logger.Info("  Commands: next | continue | debug-stop | highlight <xpath> | explain | abort")
			fmt.Fprint(os.Stdout, "  > ")
			if sc.Scan() {
				inputCh <- strings.TrimSpace(sc.Text())
			} else {
				inputCh <- "next"
			}
		}()
	}
	readNext()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-abortCh:
			rt.logger.Warn("debug: abort from browser")
			return ErrDebugStop
		case token := <-inputCh:
			switch {
			case token == "" || token == "next":
				rt.clearDebugHighlight(ctx)
				return nil
			case token == "continue":
				// Run to the next --break-lines breakpoint; clear one-shot step
				// advances but preserve user-set breakpoints.
				rt.breakSteps = make(map[int]bool)
				rt.clearDebugHighlight(ctx)
				return nil
			case token == "debug-stop":
				rt.debugContinue = true
				rt.breakLines = make(map[int]bool)
				rt.breakSteps = make(map[int]bool)
				rt.clearDebugHighlight(ctx)
				return nil
			case token == "abort":
				return ErrDebugStop
			case strings.HasPrefix(token, "highlight "):
				xpath := strings.TrimPrefix(token, "highlight ")
				if err := rt.debugHighlight(ctx, xpath); err != nil {
					rt.logger.Warn("debug: highlight failed: %v", err)
				}
				readNext()
			case token == "explain":
				rt.logger.Info(rt.explainStep(ctx, cmd))
				readNext()
			default:
				rt.logger.Warn("debug: unknown command %q — try: next, continue, debug-stop, highlight <xpath>, explain, abort", token)
				readNext()
			}
		}
	}
}

func confidenceLabel(score float64) string {
	switch {
	case score >= 0.5:
		return "high"
	case score >= 0.1:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "none"
	}
}

// explainNextPayload implements the VS Code extension's ExplainNextResult
// TypeScript interface (contracts/EXTENSION_ENGINE_CONTRACT.md §3.5).
// All 10 fields are serialized on every emission; null-typed fields use
// pointer types so `encoding/json` can write JSON null.
type explainNextPayload struct {
	Step            string   `json:"step"`
	Score           float64  `json:"score"`
	ConfidenceLabel string   `json:"confidence_label"`
	TargetFound     bool     `json:"target_found"`
	TargetElement   *string  `json:"target_element"`
	Explanation     string   `json:"explanation"`
	Risk            string   `json:"risk"`
	Suggestion      *string  `json:"suggestion"`
	HeuristicScore  *float64 `json:"heuristic_score"`
	HeuristicMatch  *string  `json:"heuristic_match"`
}

func (rt *Runtime) buildExplainNextResult(ctx context.Context, stepText string, cmd dsl.Command) explainNextPayload {
	elements, err := rt.loadSnapshot(ctx)
	if err != nil {
		return explainNextPayload{
			Step:            stepText,
			Score:           0,
			ConfidenceLabel: "none",
			TargetFound:     false,
			Explanation:     fmt.Sprintf("snapshot failed: %v", err),
		}
	}

	query := cmd.Target
	if query == "" {
		query = stepText
	}
	mode := string(cmd.InteractionMode)
	if mode == "" {
		mode = string(dsl.ModeNone)
	}
	ranked := scorer.Rank(query, cmd.TypeHint, mode, elements, 5, nil)
	rt.lastExplainData = ranked

	if len(ranked) == 0 {
		return explainNextPayload{
			Step:            stepText,
			Score:           0,
			ConfidenceLabel: "none",
			TargetFound:     false,
			Explanation:     fmt.Sprintf("no candidates found for %q", query),
		}
	}

	top := ranked[0]
	topXPath := top.Element.XPath
	topScore := top.Explain.Score.Total

	label := confidenceLabel(topScore)
	sb := top.Explain.Score
	textCh := sb.ExactTextMatch + sb.NormalizedTextMatch + sb.LabelMatch + sb.PlaceholderMatch + sb.AriaMatch + sb.DataQAMatch
	semanticCh := sb.TagSemantics + sb.TypeHintAlignment
	penaltyCh := sb.VisibilityScore * sb.InteractabilityScore
	explanation := fmt.Sprintf(
		"top candidate <%s> score=%.3f (text=%.3f id=%.3f semantic=%.3f penalty=%.3f)",
		top.Element.Tag,
		topScore,
		textCh,
		sb.IDMatch,
		semanticCh,
		penaltyCh,
	)

	risk := ""
	var suggestion *string
	if topScore < 0.1 {
		risk = "low confidence — target may be ambiguous or missing"
		if len(ranked) > 1 {
			s := fmt.Sprintf("next candidate <%s> xpath=%s score=%.3f",
				ranked[1].Element.Tag, ranked[1].Element.XPath, ranked[1].Explain.Score.Total)
			suggestion = &s
		}
	}

	match := top.Element.VisibleText
	if match == "" {
		match = top.Element.Tag
	}

	return explainNextPayload{
		Step:            stepText,
		Score:           topScore,
		ConfidenceLabel: label,
		TargetFound:     topScore > 0,
		TargetElement:   &topXPath,
		Explanation:     explanation,
		Risk:            risk,
		Suggestion:      suggestion,
		HeuristicScore:  &topScore,
		HeuristicMatch:  &match,
	}
}

func (rt *Runtime) debugPromptExtension(ctx context.Context, cmd dsl.Command, idx int) error {
	// Contract §3.4: payload idx is 1-based.
	pausePayload := fmt.Sprintf(`{"step":%q,"idx":%d}`, cmd.Raw, idx+1)

	emitPauseMarker := func() {
		fmt.Fprintf(os.Stdout, "\x00MANUL_DEBUG_PAUSE\x00%s\n", pausePayload)
		os.Stdout.Sync()
	}
	emitPauseMarker()

	sc := bufio.NewScanner(os.Stdin)
	sc.Split(bufio.ScanLines)

	inputCh := make(chan string, 1)
	readNext := func() {
		go func() {
			if sc.Scan() {
				inputCh <- sc.Text()
			} else {
				inputCh <- "next"
			}
		}()
	}
	readNext()

	emitExplain := func(stepText string) {
		payload := rt.buildExplainNextResult(ctx, stepText, cmd)
		ep, _ := json.Marshal(payload)
		fmt.Fprintf(os.Stdout, "\x00MANUL_EXPLAIN_NEXT\x00%s\n", ep)
		os.Stdout.Sync()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case token := <-inputCh:
			raw := strings.TrimRight(token, "\r\n")
			trimmed := strings.TrimSpace(raw)
			lower := strings.ToLower(trimmed)

			switch {
			case lower == "" || lower == "next":
				// Pause at the next step. Preserve existing breakLines; append
				// an index-based one-shot breakpoint at idx+1 (0-based internal).
				if rt.breakSteps == nil {
					rt.breakSteps = make(map[int]bool)
				}
				rt.breakSteps[idx+1] = true
				rt.clearDebugHighlight(ctx)
				return nil

			case lower == "continue":
				// Contract §4.3: remove all remaining breakpoints, run to end.
				rt.debugContinue = true
				rt.breakLines = make(map[int]bool)
				rt.breakSteps = make(map[int]bool)
				rt.clearDebugHighlight(ctx)
				return nil

			case lower == "debug-stop":
				// Contract §4.3: clear breakpoints, continue the run.
				rt.debugContinue = true
				rt.breakLines = make(map[int]bool)
				rt.breakSteps = make(map[int]bool)
				rt.clearDebugHighlight(ctx)
				return nil

			case lower == "abort":
				return ErrDebugStop

			case lower == "highlight":
				js := `(function(){var el=document.querySelector('[data-manul-debug-highlight]');if(el)el.scrollIntoView({behavior:'smooth',block:'center'});})();`
				rt.page.EvalJS(ctx, js)
				emitPauseMarker()
				readNext()

			case strings.HasPrefix(lower, "highlight "):
				xpath := strings.TrimPrefix(trimmed, "highlight ")
				xpath = strings.TrimPrefix(xpath, "HIGHLIGHT ")
				if err := rt.debugHighlight(ctx, xpath); err != nil {
					rt.logger.Warn("debug: highlight failed: %v", err)
				}
				emitPauseMarker()
				readNext()

			case lower == "explain-next" || lower == "explain":
				emitExplain(cmd.Raw)
				emitPauseMarker()
				readNext()

			case strings.HasPrefix(lower, "explain-next "):
				// Contract §4.3: `explain-next {"step":"<override>"}\n`.
				jsonPart := strings.TrimSpace(trimmed[len("explain-next"):])
				stepText := cmd.Raw
				var ov struct {
					Step string `json:"step"`
				}
				if err := json.Unmarshal([]byte(jsonPart), &ov); err == nil && ov.Step != "" {
					stepText = ov.Step
				}
				emitExplain(stepText)
				emitPauseMarker()
				readNext()

			default:
				emitPauseMarker()
				readNext()
			}
		}
	}
}
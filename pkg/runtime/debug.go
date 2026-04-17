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

// ErrDebugStop is returned by debugPrompt when the user or browser requests a halt.
var ErrDebugStop = errors.New("debug: stop requested")

// shouldPause reports whether execution should pause before cmd.
// Returns false immediately when debugContinue is set (user issued "continue").
// Pauses on every step when breakLines is empty; otherwise pauses only when
// cmd.LineNum is in the breakLines set.
func (rt *Runtime) shouldPause(cmd dsl.Command) bool {
	if rt.debugContinue {
		return false
	}
	if len(rt.breakLines) == 0 {
		return true
	}
	return rt.breakLines[cmd.LineNum]
}

// isTTY reports whether os.Stdin is connected to an interactive terminal.
func isTTY() bool {
	fileInfo, _ := os.Stdin.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// injectDebugModal injects a floating debug control panel into the live browser page.
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

// removeDebugModal removes the debug panel and clears the action signal.
func (rt *Runtime) removeDebugModal(ctx context.Context) error {
	js := `(function(){var d=document.getElementById('manul-debug-modal');if(d)d.remove();window.__manul_debug_action='';})();`
	_, err := rt.page.EvalJS(ctx, js)
	return err
}

// debugHighlight outlines the element matching xpath with a magenta highlight.
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

// clearDebugHighlight removes all debug highlight styles and attributes from the page.
func (rt *Runtime) clearDebugHighlight(ctx context.Context) error {
	js := `(function(){
var el=document.querySelector('[data-manul-debug-highlight]');if(el)el.removeAttribute('data-manul-debug-highlight');
var s=document.getElementById('manul-debug-style');if(s)s.remove();
})();`
	_, err := rt.page.EvalJS(ctx, js)
	return err
}

// pollForAbort polls window.__manul_debug_action every 200 ms and signals abortCh on "abort".
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

// scoreToConfidence maps a normalized [0,1] score to a 0–10 confidence level.
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

// explainStep runs the snapshot probe and scorer for cmd and returns a formatted summary.
// The top-5 candidates are cached in rt.lastExplainData for extension-mode serialization.
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

// debugPrompt dispatches to TTY or extension debug protocol based on stdin.
func (rt *Runtime) debugPrompt(ctx context.Context, cmd dsl.Command, idx int) error {
	if isTTY() {
		return rt.debugPromptTTY(ctx, cmd, idx)
	}
	return rt.debugPromptExtension(ctx, cmd, idx)
}

// debugPromptTTY handles the interactive terminal debug loop.
// Injects a browser modal and polls for abort while reading commands from stdin.
func (rt *Runtime) debugPromptTTY(ctx context.Context, cmd dsl.Command, idx int) error {
	if err := rt.injectDebugModal(ctx, cmd.Raw); err != nil {
		rt.logger.Warn("debug: modal inject failed: %v", err)
	}
	defer rt.removeDebugModal(ctx) //nolint:errcheck

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
				rt.clearDebugHighlight(ctx) //nolint:errcheck
				return nil
			case token == "continue":
				rt.debugContinue = true
				rt.clearDebugHighlight(ctx) //nolint:errcheck
				return nil
			case token == "debug-stop", token == "abort":
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

// debugPromptExtension handles the non-TTY (IDE extension) debug protocol.
// Emits NUL-delimited JSON markers directly to os.Stdout and reads NUL-delimited command tokens from stdin.
func (rt *Runtime) debugPromptExtension(ctx context.Context, cmd dsl.Command, idx int) error {
	payload := fmt.Sprintf(`{"step":%q,"idx":%d}`, cmd.Raw, idx)
	fmt.Fprintf(os.Stdout, "\x00MANUL_DEBUG_PAUSE\x00%s\n", payload)
	os.Stdout.Sync() //nolint:errcheck

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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case token := <-inputCh:
			cmdStr := strings.ToLower(strings.TrimSpace(token))
			switch {
			case cmdStr == "" || cmdStr == "next":
				rt.clearDebugHighlight(ctx) //nolint:errcheck
				return nil
			case cmdStr == "continue":
				rt.debugContinue = true
				rt.clearDebugHighlight(ctx) //nolint:errcheck
				return nil
			case cmdStr == "debug-stop" || cmdStr == "abort":
				return ErrDebugStop
			case strings.HasPrefix(cmdStr, "highlight "):
				xpath := strings.TrimPrefix(cmdStr, "highlight ")
				if err := rt.debugHighlight(ctx, xpath); err != nil {
					rt.logger.Warn("debug: highlight failed: %v", err)
				}
				readNext()
			case cmdStr == "explain":
				explainText := rt.explainStep(ctx, cmd)
				type explainPayload struct {
					Step       string                   `json:"step"`
					Candidates []scorer.RankedCandidate `json:"candidates"`
					Text       string                   `json:"text"`
				}
				ep, _ := json.Marshal(explainPayload{
					Step:       cmd.Raw,
					Candidates: rt.lastExplainData,
					Text:       explainText,
				})
				fmt.Fprintf(os.Stdout, "\x00MANUL_EXPLAIN_NEXT\x00%s\n", ep)
				os.Stdout.Sync() //nolint:errcheck
				readNext()
			default:
				readNext()
			}
		}
	}
}

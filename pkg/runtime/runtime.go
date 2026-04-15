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
}

// New creates a new Runtime bound to the given Config, Page, and Logger.
func New(cfg config.Config, page browser.Page, logger *utils.Logger) *Runtime {
	return &Runtime{cfg: cfg, page: page, logger: logger}
}

// RunHunt executes all commands in hunt against the bound page.
// It returns an explain.HuntResult summarising the execution.
func (rt *Runtime) RunHunt(ctx context.Context, hunt *dsl.Hunt) (*explain.HuntResult, error) {
	if hunt == nil {
		return nil, fmt.Errorf("runtime: nil hunt")
	}

	result := &explain.HuntResult{
		HuntFile: hunt.SourcePath,
		Title:    hunt.Title,
		Context:  hunt.Context,
	}

	for _, cmd := range hunt.Commands {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("runtime: context cancelled: %w", err)
		}

		stepResult, err := rt.executeCommand(ctx, cmd)
		result.TotalSteps++
		result.Results = append(result.Results, stepResult)
		if err != nil {
			result.Failed++
			rt.logger.Error("step %d failed: %v", result.TotalSteps, err)
		} else {
			result.Passed++
		}
	}

	result.Success = result.Failed == 0
	return result, nil
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
		err = rt.page.Navigate(ctx, cmd.URL)

	case dsl.CmdWait:
		time.Sleep(time.Duration(cmd.WaitSeconds * float64(time.Second)))

	case dsl.CmdClick, dsl.CmdFill, dsl.CmdSet, dsl.CmdType, dsl.CmdHover:
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

		targetPath := cmd.Target
		if cmd.Type == dsl.CmdSet {
			targetPath = cmd.SetVar
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
			val := cmd.Value
			if cmd.Type == dsl.CmdSet {
				val = cmd.SetValue
			}
			err = rt.page.SetInputValue(ctx, winner.XPath, val)
		case dsl.CmdClick:
			x, y, e := rt.page.GetElementCenter(ctx, winner.XPath)
			if e != nil {
				err = fmt.Errorf("center calc: %w", e)
			} else {
				err = rt.page.Click(ctx, x, y)
			}
		case dsl.CmdHover:
			x, y, e := rt.page.GetElementCenter(ctx, winner.XPath)
			if e != nil {
				err = fmt.Errorf("center calc: %w", e)
			} else {
				if hoverer, ok := rt.page.(interface {
					Hover(context.Context, float64, float64) error
				}); ok {
					err = hoverer.Hover(ctx, x, y)
				} else {
					err = fmt.Errorf("page does not support hover")
				}
			}
		}

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

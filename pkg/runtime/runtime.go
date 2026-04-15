// Package runtime implements the ManulHeart DSL execution engine.
//
// STATUS: Stub package. The full implementation will live here once the
// CDP backend (pkg/cdp) is production-ready.
package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/explain"
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
func (rt *Runtime) executeCommand(_ context.Context, cmd dsl.Command) (explain.ExecutionResult, error) {
	res := explain.ExecutionResult{
		Step:        cmd.Raw,
		CommandType: string(cmd.Type),
	}
	err := fmt.Errorf("runtime: command %q not yet implemented", cmd.Type)
	res.Error = err.Error()
	return res, err
}

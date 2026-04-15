// Package runtime implements the ManulHeart DSL execution engine.
//
// The runtime takes a parsed Hunt and a browser Page, then executes each
// command in the hunt's command list, performing element targeting via the
// heuristic scorer and browser interaction via the CDP backend.
//
// STATUS: Stub package. The full implementation will live here once the
// CDP backend (pkg/cdp) is production-ready.
package runtime

import (
	"context"
	"fmt"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
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

// HuntResult holds the outcome of a complete hunt execution.
type HuntResult struct {
	// HuntFile is the path to the .hunt file that was executed.
	HuntFile string `json:"hunt_file"`
	// TotalSteps is the total number of commands executed.
	TotalSteps int `json:"total_steps"`
	// Passed is the number of commands that succeeded.
	Passed int `json:"passed"`
	// Failed is the number of commands that failed.
	Failed int `json:"failed"`
	// Success is true when all commands passed.
	Success bool `json:"success"`
	// Steps holds the per-step result detail.
	Steps []StepResult `json:"steps,omitempty"`
}

// StepResult holds the outcome of a single DSL command.
type StepResult struct {
	// Command is the raw DSL command text.
	Command string `json:"command"`
	// Passed is true when the command succeeded.
	Passed bool `json:"passed"`
	// Error is the error message if the command failed.
	Error string `json:"error,omitempty"`
}

// RunHunt executes all commands in hunt against the bound page.
// It returns a HuntResult summarising the execution.
func (rt *Runtime) RunHunt(ctx context.Context, hunt *dsl.Hunt) (*HuntResult, error) {
	if hunt == nil {
		return nil, fmt.Errorf("runtime: nil hunt")
	}

	result := &HuntResult{
		HuntFile: hunt.SourcePath,
	}

	for _, cmd := range hunt.Commands {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("runtime: context cancelled: %w", err)
		}

		step, err := rt.executeCommand(ctx, cmd)
		result.TotalSteps++
		result.Steps = append(result.Steps, step)
		if err != nil {
			result.Failed++
			if !rt.cfg.Verbose {
				rt.logger.Error("step %d failed: %v", result.TotalSteps, err)
			}
		} else {
			result.Passed++
		}
	}

	result.Success = result.Failed == 0
	return result, nil
}

// executeCommand runs a single DSL command and returns its step result.
func (rt *Runtime) executeCommand(ctx context.Context, cmd dsl.Command) (StepResult, error) {
	step := StepResult{Command: cmd.Raw}
	err := fmt.Errorf("runtime: command %q not yet implemented", cmd.Verb)
	if err != nil {
		step.Error = err.Error()
		return step, err
	}
	step.Passed = true
	return step, nil
}

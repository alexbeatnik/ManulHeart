package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/manulengineer/manulheart/pkg/browser"
	"github.com/manulengineer/manulheart/pkg/config"
	"github.com/manulengineer/manulheart/pkg/dsl"
	"github.com/manulengineer/manulheart/pkg/explain"
	"github.com/manulengineer/manulheart/pkg/utils"
)

// PoolOptions configures a WorkerPool.
type PoolOptions struct {
	// Concurrency is the maximum number of Workers (and Chrome processes)
	// active at once. Required, must be >= 1.
	Concurrency int

	// Config is the engine-wide config inherited by every Worker.
	Config config.Config

	// Logger is the parent logger. Each Worker derives its own prefixed
	// child via utils.WithPrefix.
	Logger *utils.Logger

	// Allocator hands out CDP debug ports. Required.
	Allocator *PortAllocator

	// ChromeOptions are passed to each Worker's Chrome launch (Port is
	// always overridden by the Allocator).
	ChromeOptions browser.ChromeOptions

	// FailFast cancels the shared context as soon as any hunt errors,
	// causing other in-flight workers to abort their current step.
	// When false, all hunts run to completion regardless of failures.
	FailFast bool
}

// PoolResult bundles a hunt's outcome with the worker that ran it.
type PoolResult struct {
	WorkerID int
	Hunt     *dsl.Hunt
	Result   *explain.HuntResult
	Err      error
}

// WorkerPool dispatches hunts to a bounded set of Workers running in parallel.
type WorkerPool struct {
	opts PoolOptions
}

// NewPool returns a WorkerPool with the given options.
func NewPool(opts PoolOptions) (*WorkerPool, error) {
	if opts.Concurrency < 1 {
		return nil, errors.New("worker: PoolOptions.Concurrency must be >= 1")
	}
	if opts.Allocator == nil {
		return nil, errors.New("worker: PoolOptions.Allocator is required")
	}
	return &WorkerPool{opts: opts}, nil
}

// Run executes every hunt across the worker pool. It returns one PoolResult
// per input hunt, in input order. The error return is non-nil if any hunt
// failed (the first error encountered, errors-style); per-hunt errors are
// also embedded in their PoolResult.
//
// If PoolOptions.FailFast is true, the context shared with all workers is
// cancelled on the first failure, so in-flight hunts will abort.
func (p *WorkerPool) Run(ctx context.Context, hunts []*dsl.Hunt) ([]PoolResult, error) {
	if len(hunts) == 0 {
		return nil, nil
	}

	results := make([]PoolResult, len(hunts))
	jobs := make(chan int, len(hunts))
	for i := range hunts {
		jobs <- i
	}
	close(jobs)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		errMu    sync.Mutex
		firstErr error
	)
	recordErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
		if p.opts.FailFast {
			cancel()
		}
	}

	concurrency := p.opts.Concurrency
	if concurrency > len(hunts) {
		concurrency = len(hunts)
	}

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		workerSlot := w + 1
		go func() {
			defer wg.Done()
			worker, err := NewWorker(runCtx, Options{
				ID:            workerSlot,
				Config:        p.opts.Config,
				Logger:        p.opts.Logger,
				Allocator:     p.opts.Allocator,
				ChromeOptions: p.opts.ChromeOptions,
			})
			if err != nil {
				// Mark every job this worker would have run as failed so
				// the caller doesn't block waiting for them.
				err = fmt.Errorf("pool: spawn worker %d: %w", workerSlot, err)
				recordErr(err)
				for idx := range jobs {
					results[idx] = PoolResult{
						WorkerID: workerSlot,
						Hunt:     hunts[idx],
						Err:      err,
					}
				}
				return
			}
			defer worker.Close()

			for idx := range jobs {
				if runCtx.Err() != nil {
					results[idx] = PoolResult{
						WorkerID: worker.ID(),
						Hunt:     hunts[idx],
						Err:      runCtx.Err(),
					}
					recordErr(runCtx.Err())
					continue
				}
				res, runErr := worker.Run(runCtx, hunts[idx])
				results[idx] = PoolResult{
					WorkerID: worker.ID(),
					Hunt:     hunts[idx],
					Result:   res,
					Err:      runErr,
				}
				recordErr(runErr)
			}
		}()
	}

	wg.Wait()
	return results, firstErr
}

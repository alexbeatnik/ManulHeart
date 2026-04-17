# Copilot Instructions — ManulHeart

## GLOBAL DOC SYNC RULE

> **CRITICAL — Read this first.**
> Whenever the user asks to update documentation, a README, or a feature's description, you **MUST** automatically update the repo-local core files to keep the project's positioning and syntax rules perfectly synchronised:
> 1. `README.md` — public-facing feature docs and version footer
> 2. `.github/copilot-instructions.md` — AI training context and syntax rules
>
> A feature that appears in one file but not the other is a documentation bug.
> This `.github/copilot-instructions.md` file is the single canonical source of Copilot/LLM instructions for this repository.

## SOLO DEV ALPHA POSITIONING

> **CRITICAL — Read this first.**
> ManulHeart is a Go-based high-performance port of the ManulEngine interaction logic. It is currently in **active alpha**. 
> Never imply guarantees of stability, maturity, or production-readiness.
> Bugs are expected, APIs may change, and the project is meant for technical exploration.

## CLI INSTALL + VERSION

> **CRITICAL — Read this first.**
> Current documented ManulHeart CLI version is **0.0.0.3**.
> When documenting install or usage, prefer the Go binary as a PATH-visible system command named `manul`
> (for example `~/.local/bin/manul` or `/usr/local/bin/manul`) so editor extensions can invoke it directly.
> Do not document the repo-local binary as the only intended integration path when the request is about running from tools or extensions.

## AI Identity Directive

**CRITICAL — Read this first.**
ManulHeart is a **deterministic, DSL-first Web Automation Runtime** written in Go. It is NOT an AI-first tool.

1. **Prioritise deterministic actions.** Always default to the plain-English DSL (`CLICK`, `FILL`, `VERIFY`, `NAVIGATE`, `EXTRACT`, `SELECT`, `CHECK`, etc.) and the built-in `Scorer` heuristics.
2. **Three-Pass Targeting Strategy.** ManulHeart uses a robust multi-pass resolution for restrictive interaction modes (checkboxes, radios, selects):
   - **Pass 1 (Strict Match):** Finds elements of the requested type (e.g., input[type=checkbox]) matching the text directly.
   - **Pass 2 (Anchor Anchor):** Finds a non-interactive element (e.g., a <td> containing "7") to use as a proximity anchor.
   - **Pass 3 (Refined Target):** Searches for the desired interactive element near the identified anchor. If found within proximity limits, it targets that; otherwise, it targets the anchor and lets the action handler perform local refinement.
3. **Execution Robustness.** Interactions with checkboxes and radios use a full sequence of simulated mouse events (`mousedown`, `mouseup`, `click`) and native `el.click()` to ensure compatibility with modern frameworks (React, Vue, jQuery).
4. **Smart Scrolling.** The `SCROLL` command intelligently detects scrollable containers. If the primary target isn't scrollable, it recursively searches for the most deeply nested scrollable child with vertical overflow.

## What is this project?

ManulHeart is a high-performance Go port of the Manul interaction engine.
It acts as a standalone interpreter for the `.hunt` DSL, driving Chromium via CDP.
It resolves DOM elements using a weighted heuristic `Scorer` and a JavaScript `TreeWalker` snapshot probe that handles Shadow DOM boundaries.
It is specifically designed to handle "Frontend Hell": zero-size inputs, hidden labels, custom div-based dropdowns, and paginated dynamic tables.

**Stack:** Go 1.21+ · Chrome DevTools Protocol (CDP) · JavaScript (TreeWalker)
**Dependencies:** exactly one — `github.com/gorilla/websocket`. Do NOT add new
third-party deps (including `golang.org/x/sync`); implement equivalents inline.

## Repository layout

```text
cmd/manul/                 CLI entry point (main.go)
pkg/
  cdp/                     CDP WebSocket transport, Conn lifecycle, Subscription handles
  browser/                 Abstract browser/page interfaces + CDP backend + Chrome lifecycle
  dom/                     Element snapshotting and XPath resolution
  heuristics/              Scoring logic (Scorer), keyword analysis, and embedded JS probes
    snapshot_probe.js      TreeWalker DOM traversal (Shadow DOM aware)
    extract_data.js        Data extraction JS logic
    visible_text_probe.js  Deep text collection
  runtime/                 Interpretation of .hunt files, execution state, variable memory
                           (SINGLE-GOROUTINE — see "Concurrency contract" below)
  worker/                  Worker, WorkerPool, PortAllocator (parallel execution substrate)
  explain/                 Score breakdown and debugging visualization
  report/                  Per-hunt HTML report + aggregate index.html
  utils/                   Logger (dual-output: stdout+ANSI, file+stripped) + error types
examples/                  Reference .hunt files (mega.hunt, sampler.hunt)
```

## Concurrency contract (`0.0.0.3`+)

> **CRITICAL — Read this before writing any code that touches `Runtime`, `Page`, or CDP.**

1. **`runtime.Runtime` is single-goroutine.** The DOM snapshot cache, variable
   store, and sticky checkbox states are unguarded by design. Sharing a
   `Runtime` between goroutines is a data race, caught by `go test -race`.
2. **To run in parallel, use `pkg/worker`.** A `Worker` owns exactly one
   Chrome process, one `Page`, and one `Runtime`. Use `worker.NewWorker` for
   real Chrome; `worker.AdoptWorker` for tests/embedding with a pre-built
   `Page`.
3. **`WorkerPool` dispatches hunts over a bounded jobs channel.** Options:
   `Concurrency`, `Config`, `Allocator` (required), `ChromeOptions`,
   `FailFast`. Result ordering matches input order; per-hunt errors live on
   `PoolResult.Err`; the outer error is the first failure seen.
4. **`PortAllocator` hands out CDP debug ports.** Call `Acquire()` / `Release()`
   per worker. Two workers must never share a port — the allocator also
   best-effort-checks the port is free at the OS level.
5. **`cdp.Conn` is safe for concurrent use.** Writes are serialised by
   `writeMu`; request IDs use `atomic.Int64`; `Close()` is idempotent via
   `sync.Once`.
6. **`cdp.Conn.Subscribe()` returns a `*Subscription`.** Callers MUST invoke
   `sub.Close()` (typically `defer`). Do NOT use the legacy raw-channel form
   — it no longer exists.
7. **Extension registries (`RegisterCustomControl`, `RegisterGoCall`) are
   package-global.** Register at process init, BEFORE spawning the pool.
   Handlers must themselves be safe for concurrent invocation — every worker
   may call the same handler simultaneously.
8. **CI runs `go test -race` on every package.** Any new goroutine spawn,
   shared map access, or channel plumbing must pass the race detector.

## Step format

Steps are atomic browser instructions. **STEP-grouped (unnumbered) is the standard format.**

**Canonical format:**

```text
STEP 1: Navigate to the page
    NAVIGATE to https://example.com
    VERIFY that 'Login' is present

STEP 2: Enter credentials
    FILL 'Username' with 'admin'
    FILL 'Password' with 'secret'
    CLICK the 'Login' button
    VERIFY that 'Welcome' is present
```

**ABSOLUTE RULES for `.hunt` files:**
1. **Unnumbered DSL Syntax:** NEVER prepend numbers (`1.`, `2.`) to action lines.
2. **Logical `STEP` Grouping:** Use `STEP [Optional Number]: [Description]` for structure.
3. **4-space Indentation:** Action lines under `STEP` headers MUST be indented by 4 spaces.
4. **Static Data (@var):** NEVER hardcode test data. Use `@var: {key} = value` at the top and reference via `{key}`.
5. **Post-Input Guard:** Always follow a `FILL` or `TYPE` step with a `VERIFY ... has value "..."` assertion.

## System Keywords (parser-detected)

* `NAVIGATE to [url]`
* `WAIT [seconds]`
* `PRESS [Key]` (e.g., `PRESS Enter`)
* `CLICK [Target]`
* `DOUBLE CLICK [Target]`
* `RIGHT CLICK [Target]`
* `SELECT [Value] from [Dropdown]`
* `CHECK the checkbox for [Target]`
* `UNCHECK the checkbox for [Target]`
* `SCROLL DOWN` or `SCROLL DOWN inside the list`
* `EXTRACT [target] into {variable_name}`
* `VERIFY that [target] is [present|not present|checked|enabled|disabled]`
* `VERIFY [Target] has [text|placeholder|value] "[Expected]"`
* `USE [BlockName]` (Inlines imported step block)
* `CALL [BlockName]` (Functional alias for USE)
* `DONE.`

## Heuristic Scoring (Normalised 0.0–1.0)

The `Scorer` ranks candidates using weighted channels:
1. **Text (0.45):** Direct innerText, aria-label, and placeholder matches.
2. **Semantics (0.60):** Element type alignment (e.g., preference for `<input>` when filling).
3. **Attributes (0.25):** Matches on `id`, `class`, `name`, `data-qa`.
4. **Proximity (1.5 when contextual):** Euclidean distance to an anchor (used in `NEAR`, `INSIDE`, `STEP`).

## Common Pitfalls & Learnings

* **Shadow DOM:** Standard XPaths fail inside shadow roots. The Go engine uses a custom `ShadowHostPath` and JS-based resolution to bridge shadow boundaries.
* **Invisible Inputs:** React/Vue often hide the real `<input type="checkbox">` behind a styled `<div>`. The engine collects these hidden inputs and uses `Pass 2` anchors to find them.
* **Scroll Lag:** Dynamic dropdowns and lists often need a `WAIT 1` after clicking to allow the DOM to populate.
* **Pagination:** After clicking table pagination links, use `WAIT 1` in the `.hunt` file to let AJAX updates settle before the next targeting probe. Do not use `time.Sleep` in Go production code for this.

## Interaction Robustness

When generating automation logic:
* Use **quoted strings** for target labels (`'Login'`) to ensure high scoring priority.
* For tables, use **text identifiers** (`CHECK the checkbox for 'Item ID'`) – let the 3-pass targeting handle the proximity to the actual checkbox input.
* For custom dropdowns, the engine automatically falls back from `select_option` to `click()` on the resolved target.

## Parallel execution (Go API)

The CLI is still single-threaded; the worker pool is a Go API. Typical use:

```go
alloc := worker.NewPortAllocator(9222, 9321)
pool, _ := worker.NewPool(worker.PoolOptions{
    Concurrency: 4,
    Config:      config.Default(),
    Allocator:   alloc,
    FailFast:    false,
})
results, firstErr := pool.Run(ctx, hunts)
```

- Order preserved: `results[i]` corresponds to `hunts[i]`.
- Use `report.GenerateIndex(summaries, outDir)` for an aggregate `index.html`.
- Per-worker logs are prefixed `[wN] ` via `utils.WithPrefix(parent, "[wN] ")`.
- Logger API: `utils.NewLogger(logFile)` (stdout + optional ANSI-stripped file); `l.WithLevel(level)` for verbose mode; semantic methods `BlockStart/Pass/Fail`, `ActionStart/Pass/Fail/Warn`, `HeuristicDetail`, `ActionDetail`.
- Per-hunt report filenames carry an atomic sequence counter — never collide.

## Testing expectations

- **Default to `-race`:** `go test -race ./...`. CI runs race on every package.
- **Worker tests use `AdoptWorker`:** `pkg/worker/worker_test.go` dispatches 16
  hunts across 16 adopted workers with `MockPage`. That is the canonical
  pattern for verifying "no state bleed" when adding new Runtime state.
- **CDP tests use an in-process `httptest` WebSocket echo server:** see
  `pkg/cdp/conn_test.go`. Any new CDP transport feature must be tested there
  before shipping.
- **Do not introduce `time.Sleep` in production paths.** Prefer context-aware
  waits such as `select { case <-ctx.Done(): ... case <-time.After(...): ... }`
  or explicit readiness checks. Test-only `time.Sleep` is acceptable where
  necessary, but runtime/production code must not depend on fixed sleeps.

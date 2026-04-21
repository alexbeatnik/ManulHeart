# ManulHeart

A deterministic, DSL-first browser automation runtime in Go.

Current alpha version: `0.0.0.6`.

ManulHeart executes `.hunt` files using plain-English commands, DOM intelligence,
heuristic element resolution, and structured explainability.
It connects to system-installed Chrome via the Chrome DevTools Protocol (CDP).

**No Playwright. No CSS/XPath selectors as a public API. No LLM in the loop.**

Single dependency: `gorilla/websocket`. Pure Go. ~476 tests.

---

## Core Philosophy

| Principle | What it means |
|-----------|--------------|
| **DSL-first** | `.hunt` files are the primary automation artifact. You express intent in plain English, not CSS/XPath. |
| **Deterministic by default** | Element resolution uses explicit scoring, reproducible heuristics, and ranked candidates — the same input always produces the same resolution path. |
| **Heuristics at the first query** | The engine does not fetch a raw DOM and then apply heuristics as an afterthought. On the very first meaningful page query, JS probing, candidate extraction, visibility checks, accessible-name inference, and scoring all run together in one pipeline pass. |
| **Explainable execution** | Every command execution returns a structured result with scored candidates, signal breakdowns, the winning element, and the action performed. |
| **Backend independence** | Chrome/CDP is the first backend. The `browser.Page` interface is clean and can accommodate WebKit, Firefox, or desktop backends. |

---

## Quick Start

### 1. Build

You can build the `manul` binary using the provided `Makefile`:

```bash
make build
```

This creates a `manul` executable in the current directory.

### 2. Install

To use `manul` as a system-wide command, install it to your `PATH`.

**User-local install** (installs to `~/.local/bin`):
```bash
make install
```

**System-wide install** (installs to `/usr/local/bin`, requires `sudo`):
```bash
make install-system
```

Verify the installation:
```bash
manul --help
```

### 3. Other Commands

- `make test` — Run all tests.
- `make clean` — Remove the compiled binary.
- `make uninstall` — Remove the binary from both local and system paths.

### 4. Run a hunt file

```bash
manul examples/saucedemo.hunt
```

Chrome is launched automatically with remote debugging, the hunt is executed,
and Chrome is closed when done. No manual browser setup required.

### 5. Run all hunt files in a directory

```bash
manul examples/
manul .
```

### 6. Run headless

```bash
manul examples/saucedemo.hunt --headless
```

### 7. Connect to existing Chrome

If you already have Chrome running with `--remote-debugging-port=9222`:

```bash
manul examples/saucedemo.hunt --cdp http://127.0.0.1:9222
```

When `--cdp` is set, the driver skips auto-launch and connects to the running instance.

### 8. Run a single step (requires running Chrome)

```bash
manul run-step "Click the 'Login' button" --cdp http://127.0.0.1:9222
```

### 9. Verbose / JSON output

```bash
manul examples/saucedemo.hunt --verbose
manul examples/saucedemo.hunt --json 2>/dev/null | jq .
```

> **Note:** The `manul` command name is shared with the Python ManulEngine.
> Whichever you install last takes priority. To switch back to Python: `pipx install manul-engine`.
> For ManulHeart `0.0.0.6`, prefer a PATH install so extensions can execute `manul` directly.

### CLI Flags

| Flag | Default | Description |
|------|---------|------------|
| `--cdp` | *(auto-launch)* | Connect to existing Chrome (skip auto-launch) |
| `--user-data-dir` | `/tmp/manulheart-chrome` | Chrome profile directory |
| `--headless` | `false` | Run Chrome without a visible window |
| `--verbose` | `false` | Enable debug-level logging |
| `--json` | `false` | Print JSON execution result to stdout |
| `--timeout` | `30s` | Default per-command timeout |
| `--debug` | `false` | Interactive step-by-step execution with breakpoints |
| `--explain` | `false` | Show targeting candidate rankings and scores |
| `--screenshot` | `none` | Capture screenshots: `none`, `on-fail`, `always` |
| `--html-report` | `false` | Generate a styled HTML report |
| `--tags` | *(none)* | Comma-separated tag filter (match `@tags:` in hunt files) |
| `--retries` | `0` | Retry failed steps N times |
| `--disable-cache` | `false` | Disable DOM snapshot caching |

### Environment Variables

ManulHeart respects the following `MANUL_` prefix environment variables (CLI flags always take precedence):

| Variable | Type | Description |
|----------|------|-------------|
| `MANUL_HEADLESS` | `bool` | Run Chrome in headless mode if `true` |
| `MANUL_TIMEOUT` | `dur` | Default per-command timeout (e.g. `5s` or `5000` ms) |
| `MANUL_EXPLAIN` | `bool` | Enable explain mode scores if `true` |
| `MANUL_SCREENSHOT` | `str` | Screenshot mode: `none`, `on-fail`, `always` |

---

## DSL Syntax

ManulHeart supports 32 command types. Commands are case-insensitive.

### Navigation

```
NAVIGATE to 'https://example.com'
SCROLL DOWN
SCROLL UP
SCROLL DOWN inside the 'Container'
```

### Interaction

```
Click the 'Login' button
Click the 'Sign up' link
DOUBLE CLICK the 'Cell' element
RIGHT CLICK 'Item'
Fill 'Email' field with 'user@example.com'
Type 'hello' into the 'Search' field
Select 'Option A' from the 'Category' dropdown
Check the checkbox for 'Remember me'
Uncheck the checkbox for 'Subscribe'
HOVER over 'Menu Item'
DRAG 'Card A' and drop onto 'Column B'
UPLOAD '/path/to/file.png' to 'Avatar Upload'
```

### Keyboard

```
PRESS Enter
PRESS Escape
PRESS Control+A
PRESS Tab ON 'Username'
```

### Contextual Qualifiers

```
Click 'Add to cart' button NEAR 'Sauce Labs Bolt T-Shirt'
Click the 'Logo' link ON HEADER
Click the 'Terms' link ON FOOTER
Click the 'Delete' button INSIDE 'Actions' row with 'John'
```

`NEAR` resolves the anchor text first, then prefers the matching element
using Euclidean pixel proximity blended with DOM ancestry affinity.

`ON HEADER`/`ON FOOTER` restricts candidates to header/footer regions.

`INSIDE` restricts candidates to a named container, optionally filtered
by a row containing specific text.

### Assertions

```
VERIFY that 'Welcome' is present
VERIFY that 'Error' is NOT present
VERIFY SOFTLY that 'Optional text' is present
Verify 'Email' field has value 'user@example.com'
Verify 'Heading' field has text 'Dashboard'
```

`VERIFY SOFTLY` is non-fatal — the step is logged as a warning but
execution continues.

### Data

```
EXTRACT the 'Price' into {total}
SET {username} = admin
PRINT 'Current user: {username}'
```

### Waiting

```
WAIT 2
Wait for 'Spinner' to be hidden
Wait for 'Results' to be visible
WAIT FOR RESPONSE "api/users"
```

### Control Flow

```
IF 'Dashboard' is present:
    Click the 'Logout' button
ELIF {role} == 'admin':
    Click the 'Admin Panel' link
ELSE:
    Click the 'Login' button

WHILE 'Next' is present:
    Click the 'Next' button

REPEAT 3 TIMES as {i}:
    Click the 'Add Item' button

FOR EACH {item} IN {items}:
    Fill 'Search' field with '{item}'
```

Conditions support: element present/not present, `{var} == 'value'`,
`{var} != 'value'`, `{var} contains 'text'`, truthy variable checks.

### Interactive Debugger

ManulHeart includes a powerful TTY-based interactive debugger. It can be triggered in three ways:
1. **Global Debug Mode**: Run `manul` with the `--debug` flag to pause before every step.
2. **Breakpoints**: Use `--break-lines 12,45` to pause only at specific line numbers.
3. **DSL Pause**: Insert the `PAUSE` command anywhere in your `.hunt` file.

When paused, you have access to the following commands in your terminal:

- `next` (or Enter): Execute the current step and pause at the next one.
- `continue`: Resume execution and skip all future pauses.
- `debug-stop`: Alias for `continue`.
- `explain-next`: Score candidates for the current step and print a full `ExplainNextResult` breakdown, then re-pause.
- `explain-next {"step":"<text>"}`: Same as above but scores the overridden step text.
- `highlight <xpath>`: Outline a specific element in the browser with a magenta highlight.
- `debug-vars` (or `DEBUG VARS` in DSL): Dump all currently set variables and their scopes.
- `abort`: Stop the hunt execution immediately.

While paused, a **debug control panel** is also injected into the browser page, allowing you to `Continue` or `Abort` directly from the UI.

#### Debugging Commands in DSL

```
PAUSE
DEBUG VARS
```

`PAUSE` enters the interactive debugger. `DEBUG VARS` prints a formatted table of all variables in the `ROW`, `STEP`, `MISSION`, and `GLOBAL` scopes to the console.

### File structure

```
@context: suite description
@title: short suite name
@var: {username} = standard_user
@var: {password} = secret_sauce
@tags: smoke, login
@import: Checkout from 'shared/checkout.hunt'

STEP 1: Login
    NAVIGATE to https://www.saucedemo.com/
    USE LoginBlock
    VERIFY that 'Products' is present

* `VERIFY [Target] has [text|placeholder|value] "[Expected]"`
* `USE [BlockName]` (Inlines imported step block)
* `CALL [BlockName]` (Functional alias for USE)
* `DONE.`
```

**Structural Commands:**

| Command | Purpose |
|---------|---------|
| `USE [Block]` | Inlines a `STEP` block from an imported `.hunt` file at parse time. |
| `CALL [Block]` | Alias for `USE` (functional/modular call). |

**File-level directives:**

| Directive | Purpose |
|-----------|---------|
| `@context:` | Suite description for reporting |
| `@title:` | Short suite name |
| `@var: {key} = value` | Variable, substituted at parse time |
| `@tags:` | Comma-separated tags (filterable with `--tags`) |
| `@import:` | Import steps from another hunt file |
| `@data:` | External data reference |
| `@schedule:` | Scheduling metadata |
| `@export:` | Export metadata |

**Import variants:**

```
@import: Login from 'shared/auth.hunt'
@import: Login as QuickLogin from 'shared/auth.hunt'
@import: * from 'shared/setup.hunt'
```

---

## Architecture

```
cmd/manul           CLI entry point (produces `manul` binary)
pkg/cdp             Low-level CDP WebSocket transport and domain wrappers
pkg/browser         Abstract browser/page interfaces + CDP backend + Chrome lifecycle
pkg/runtime         Targeting pipeline: probe → filter → score → resolve;
                    DSL execution, control flow, variable management
pkg/worker          Worker / WorkerPool / PortAllocator for parallel execution
pkg/dom             Normalized DOM element model (ElementSnapshot with 37 fields)
pkg/heuristics      In-page JS probes (SnapshotProbe, VisibleTextProbe, ExtractDataProbe)
pkg/scorer          Deterministic 4-channel [0.0–1.0] scoring and ranking
pkg/dsl             .hunt file parser, import resolver, command AST with block nesting
pkg/explain         Structured execution results and explainability types
pkg/report          Styled HTML report generation + aggregate index.html
pkg/config          Runtime configuration (20 fields)
pkg/core            Shared enums (e.g. ScrollStrategy: window vs generic-list containers)
pkg/utils           Semantic logging (Block/Action/Detail), ANSI stripping, error types
examples/           7 sample .hunt files
docs/               Documentation
```

See [docs/overview.md](docs/overview.md) for a detailed architecture walkthrough.

---

## Parallel Execution (API, `0.0.0.6`)

As of `0.0.0.3` ManulHeart ships a Go-level worker pool for running hunts in
parallel. The `manul` CLI is still single-threaded; embed the pool directly
to fan out. Each `Worker` owns its own isolated `Runtime`, `Page`, and
`ChromeProcess` — sharing any of these across workers is a data race caught
by `go test -race`.

```go
import (
    "context"
    "github.com/manulengineer/manulheart/pkg/browser"
    "github.com/manulengineer/manulheart/pkg/config"
    "github.com/manulengineer/manulheart/pkg/dsl"
    "github.com/manulengineer/manulheart/pkg/report"
    "github.com/manulengineer/manulheart/pkg/worker"
)

func runInParallel(ctx context.Context, hunts []*dsl.Hunt) error {
    // 1. Setup a shared logger (optional: pass a file writer for dual logging)
    logger := utils.NewLogger(nil).WithLevel(utils.LogLevelDebug)

    // 2. Use the convenience wrapper for zero-config parallel execution
    cfg := config.Default()
    results, err := worker.RunHuntsInParallel(ctx, cfg, hunts, 4, logger)
    if err != nil {
        // err is the first hunt failure encountered
    }

    // 3. Generate an aggregate report
    summaries := make([]report.RunSummary, len(results))
    for i, r := range results {
        summaries[i] = report.RunSummary{Result: r.Result, WorkerID: r.WorkerID}
    }
    _, _ = report.GenerateIndex(summaries, "reports")
    return err
}
```

**Rules of engagement:**

- One `Runtime`, `Page`, and `ChromeProcess` per worker. Sharing them across
  goroutines is a data race — verified by `go test -race` in CI.
- Register custom controls and `CALL GO` handlers **before** the pool spawns.
  The handler maps themselves are mutex-guarded, but handlers must be safe
  for concurrent invocation (every worker may invoke the same handler
  simultaneously).
- Each worker logs with a `[wN]` prefix via `utils.WithPrefix`.
- Per-hunt reports include a monotonic sequence suffix so two workers
  finishing the same hunt title in the same second do not collide.

---

## Configuration

ManulHeart resolves runtime configuration from four sources in strict priority order:

```
CLI Flags  >  MANUL_* environment variables  >  manul_engine_configuration.json  >  Defaults
```

If a `manul_engine_configuration.json` file exists in the current working directory, its values
are merged with defaults before environment variables and flags are applied. The `config.Default()`
function always returns a safe, zero-configuration baseline — no file on disk is required.

```go
cfg, _ := config.Load()  // applies all layers: defaults → JSON → env vars
// CLI flag parsing then overrides cfg fields directly
```

The `pkg/config` package exposes 20 fields covering headless mode, timeouts, screenshot
policy, debug breakpoints, scoring thresholds, and more.

---

## Development Guides

For developers working on the ManulHeart engine, we provide detailed "Skill Guides" covering core systems:

- [**Scoring & Heuristics**](.claude/skills/scoring-heuristics/SKILL.md) — How element targeting works.
- [**Concurrency Rules**](.claude/skills/concurrency-rules/SKILL.md) — Thread-safety and the worker pool.
- [**Adding DSL Commands**](.claude/skills/adding-dsl-commands/SKILL.md) — How to extend the natural language syntax.
- [**Go Calls & Extensions**](.claude/skills/extensions-and-go-calls/SKILL.md) — Implementing custom logic in Go.
- [**Testing ManulHeart**](.claude/skills/testing-manulheart/SKILL.md) — Best practices for unit and integration tests.
- [**Hunt Authoring**](.claude/skills/hunt-authoring/SKILL.md) — Writing effective `.hunt` files.

---

## Project Status

Alpha. The core engine covers:
32 DSL commands, full control flow (IF/ELIF/ELSE, WHILE, REPEAT, FOR EACH),
import system (including USE/CALL expansion), 4-channel scoring, contextual
qualifiers (NEAR, ON HEADER/FOOTER, INSIDE), Shadow DOM support, 3-pass
proximity resolution, HTML reporting, screenshots, debug mode, explain mode.

As of `0.0.0.6` the engine also exposes a **parallel-execution substrate**:
a goroutine-safe CDP transport, a `pkg/worker` package with `Worker`,
`WorkerPool`, and `PortAllocator`, per-worker log prefixes, and collision-proof
report filenames. Every test (CDP, runtime, scorer, worker) runs under
`go test -race` in CI.

Not yet implemented: a CLI flag to expose the worker pool end-to-end (the API
is there, the CLI is still single-threaded), LLM-based fallback,
scan/record subcommands.

**Documented CLI version:** `0.0.0.6`.

**Recommended install target:** expose the binary as a PATH command named `manul`
for editor extensions and automation tooling.

---

## What's New

### `0.0.0.6` — snapshot expansion, config growth, convenience parallelism & extension contract

- **37-field `ElementSnapshot`** — Expanded from 27 fields to 37; the JS `SnapshotProbe` (Shadow-DOM-aware, single-pass `TreeWalker`) now collects richer identity, text, state, and geometry signals in one round-trip for the scorer.
- **20-field `Config`** — Grew from 18 to 20 fields; new knobs surface via the same four-layer priority chain (CLI > `MANUL_*` env > `manul_engine_configuration.json` > `config.Default()`).
- **`worker.RunHuntsInParallel`** — Zero-config convenience wrapper over `WorkerPool` that creates a pool, runs hunts, and returns per-hunt results in input order. Use `NewPool` directly when you need `FailFast` or custom `ChromeOptions`.
- **`pkg/core` shared enums** — New package centralising cross-cutting enums (e.g. `ScrollStrategy`: window vs generic-list containers) that previously lived inline in `pkg/runtime`.
- **VS Code extension contract** — Engine version string is `manul-heart v0.0.9.29 (core 0.0.0.6)`, satisfying the extension `MIN_MANUL_ENGINE_VERSION` gate. BLOCK log markers are now ANSI-free at the bracket prefix so the extension regex matches cleanly. `os.Stdout.Sync()` is called after every output line.
- **Debug protocol v2** — Pause marker now carries a 1-based `idx` field (`\x00MANUL_DEBUG_PAUSE\x00{"step":"...","idx":N}\n`). New stdin tokens: `explain-next` (and `explain-next {"step":"<override>"}`) cause the engine to score candidates and emit a 10-field `ExplainNextResult` JSON via `\x00MANUL_EXPLAIN_NEXT\x00<json>\n` before re-pausing. The `debug-stop` token is an alias for `continue`.
- **`run_history.json` artifact** — After every hunt run, `pkg/report.AppendRunHistory` appends a JSONL record to `<cwd>/reports/run_history.json`: `{file, name, timestamp (RFC3339 UTC), status ∈ {"pass","fail"}, duration_ms}`. Directory is created automatically; file is append-only.
- **Skill guides refreshed** — `concurrency-rules`, `scoring-heuristics`, and the repo-level `CLAUDE.md` / `.github/copilot-instructions.md` updated to reflect the `0.0.0.6` field counts, new convenience APIs, `pkg/core`, and the VS Code extension contract.

### `0.0.0.5` — configuration system, debug protocol & test coverage

- **Configuration System** — `pkg/config` resolves settings from four layers in priority order: CLI Flags > `MANUL_*` env vars > `manul_engine_configuration.json` > `config.Default()`. An 18-field `Config` struct covers headless, timeouts, screenshot policy, debug breakpoints, scoring thresholds, and more. No config file is required for zero-configuration use.
- **VS Code Debug Protocol** — `pkg/runtime/debug.go` formalised with `\x00MANUL_DEBUG_PAUSE\x00` JSON markers on stdout and stdin polling so VS Code extensions can drive the interactive debugger over a simple pipe. `scoreToConfidence()` maps a `[0.0–1.0]` heuristic score to a 0–10 confidence integer for display.
- **Expanded test coverage** — Added white-box test suites for `pkg/report` (`sanitizeFilename`, `GenerateHTML`, `GenerateIndex`), `pkg/runtime/debug` (`scoreToConfidence`, `shouldPause`), `pkg/core` (scroll strategy constants), `pkg/explain` (JSON serialisation round-trips for `ScoreBreakdown`, `ExecutionResult`, `HuntResult`, `Candidate`), and `pkg/utils` (ANSI writer, logger levels, `WithPrefix`, race safety).
- **Worker isolation clarified** — Each `Worker` owns its own isolated `Runtime`, `Page`, and `ChromeProcess`; sharing them is a data race caught by `go test -race`.

### `0.0.0.4` — interactive debugger & skill guides

- **Interactive Runtime Debugger** — Added a TTY-based interactive debugger that can be triggered via `--debug` or the `PAUSE` command. Features include step-by-step execution, browser modal controls, element highlighting (`highlight <xpath>`), and scoring explanations (`explain`).
- **Execution Breakpoints** — Added the `--break-lines` flag to pause execution only at specific lines in the `.hunt` file.
- **Expanded scoring-heuristics skill** — Documented the dual-mode proximity signal: base weight `0.10` (XPath-depth DOM ancestry, always active) vs. contextual override `1.50` (Euclidean pixel distance from anchor, active under `NEAR`/`INSIDE`). Added inline comments and a 3-pass targeting pipeline explanation for the `ThresholdPass3*` constants.
- **Expanded testing skill** — Added an explicit callout for the two `MockPage` fields most commonly left at zero: `IsVisible` (silently drops elements from the visibility pre-filter) and `Rect` (required by the proximity scorer for `NEAR`/`INSIDE` path; zero value flattens the proximity channel).
- **Expanded hunt-authoring skill** — Added an `## Advanced / less common commands` section covering keyboard input (`PRESS`), `WAIT FOR RESPONSE`, `DRAG`/`UPLOAD`, `EXTRACT`/`PRINT`, and `DEBUG VARS`/`PAUSE`.
- **Expanded concurrency-rules skill** — Added `## Per-worker logging` section documenting `utils.WithPrefix` for `[wN]`-prefixed child loggers; extended `## Key files` to include `pkg/utils/logger.go`.

### `0.0.0.3` — logging & pool refactor

- **Simplified Semantic Logger** — Refactored `pkg/utils` to use a leaner, hierarchy-first logging model (Block > Action > Detail). Removed legacy timestamping in favor of cleaner terminal output.
- **Improved Dual-Logging** — `NewLogger` now supports optional ANSI-stripped file logging via `StripANSIWriter` without requiring separate cleanup functions.
- **Parallel Substrate Convenience** — Added `RunHuntsInParallel` convenience wrapper in `pkg/worker` for zero-config fan-out.
- **CLI Renaming** — Formally standardized the CLI binary name as `manul` across all documentation and build scripts.

### `0.0.0.2` — concurrency substrate

- **Hardened CDP transport** — `readLoop` now honors parent-context cancellation
  via a watchdog that tears down the WebSocket on cancel. Request IDs use
  `atomic.Int64` instead of a mutex. `Conn.Close()` is idempotent via
  `sync.Once`.
- **Subscription handles** — `Conn.Subscribe()` returns a `*Subscription` with
  `C()` / `Close()` instead of a raw channel. Channels are closed on connection
  teardown, so subscribers unblock cleanly. Prevents the old "orphaned
  channel in a slice" leak path.
- **`pkg/worker`** — new package with:
  - `PortAllocator` — round-robin CDP debug-port allocation with an OS-level
    free-check, safe for concurrent `Acquire` / `Release`.
  - `Worker` — owns exactly one Chrome + Page + Runtime; launches its own
    Chrome in `NewWorker`, or wraps an existing page via `AdoptWorker` (for
    tests/embedding).
  - `WorkerPool` — bounded jobs channel with first-error tracking and optional
    `FailFast`. Implemented without adding a dependency (no `x/sync/errgroup`).
- **Runtime concurrency contract** — `pkg/runtime.Runtime` is now explicitly
  documented as single-goroutine. Use `pkg/worker` for parallel execution.
- **Extension-registry policy** — `RegisterCustomControl` / `RegisterGoCall`
  are intended to be called at process init, before the worker pool spawns.
  Documented inline in [pkg/runtime/extensions.go](pkg/runtime/extensions.go).
- **Per-worker log prefixes** — `utils.WithPrefix(parent, "[w3] ")` derives
  child loggers that share the parent's writer/level but prepend the prefix.
- **Collision-proof report filenames** — `report_{title}_{ts}_{seq}.html`
  with a process-wide atomic counter; two workers finishing in the same
  second no longer overwrite each other.
- **Aggregate reporter** — `report.GenerateIndex(summaries, outDir)` writes
  an `index.html` linking to every per-hunt report for a parallel run.
- **`-race` in CI** — every test invocation in the `synthetic-tests` workflow
  now runs with the race detector, with dedicated CDP and worker steps.

### Earlier (`0.0.0.1`)

- **32 DSL commands** — full interaction set including double-click, right-click,
  hover, drag-and-drop, file upload, keyboard shortcuts, scroll with containers.
- **Control flow** — IF/ELIF/ELSE conditionals, WHILE loops (capped at 100),
  REPEAT N TIMES with loop variable, FOR EACH over collections.
- **Import system** — `@import: Name from 'file.hunt'`, wildcard `*`, aliases.
- **INSIDE qualifier** — `INSIDE 'Container' row with 'Text'` restricts candidates
  to a named container filtered by row content.
- **ON HEADER/ON FOOTER** — region-based candidate filtering.
- **Soft verification** — `VERIFY SOFTLY` continues on failure, reports warnings.
- **Field verification** — `Verify 'Field' has value/text/placeholder 'Expected'`.
- **EXTRACT** — table-aware data extraction with column-header resolution.
- **WAIT FOR** — poll for element visibility/hidden state; network response matching.
- **HTML reports** — styled dark-themed pass/fail report with embedded screenshots.
- **Screenshot modes** — `--screenshot none|on-fail|always`.
- **Debug mode** — `--debug` for interactive step-by-step; `PAUSE` command; breakpoints.
- **Explain mode** — `--explain` prints top-5 candidates with full score breakdowns.
- **Tags** — `@tags:` directive + `--tags` CLI filter.
- **NEAR qualifier** — spatial proximity + DOM ancestry + anchor entity affinity.
- **Variables** — `@var:` at parse time, `SET`/`EXTRACT` at runtime.
- **JS-based click** — `element.click()` for React/SPA compat; coordinate fallback.
- **Drag-and-drop** — CDP mouse events + HTML5 DragEvent fallback.
- **`manul` CLI** — `manul test.hunt`, `manul examples/`, `manul .`, `manul run-step`.
- **Browser cleanup** — Chrome is always killed on exit, including SIGINT/SIGTERM.
- **476 synthetic tests** — 35 test files covering 15 domain-specific DOMs
  (e-commerce, social media, fintech, cybersecurity, healthcare, etc.).

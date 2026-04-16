# ManulHeart

A deterministic, DSL-first browser automation runtime in Go.

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

```bash
cd ManulHeart
go build -o manul ./cmd/manul
```

### 2. Run a hunt file

```bash
manul examples/saucedemo.hunt
```

Chrome is launched automatically with remote debugging, the hunt is executed,
and Chrome is closed when done. No manual browser setup required.

### 3. Run all hunt files in a directory

```bash
manul examples/
manul .
```

### 4. Run headless

```bash
manul examples/saucedemo.hunt --headless
```

### 5. Connect to existing Chrome

If you already have Chrome running with `--remote-debugging-port=9222`:

```bash
manul examples/saucedemo.hunt --cdp http://127.0.0.1:9222
```

When `--cdp` is set, the driver skips auto-launch and connects to the running instance.

### 6. Run a single step (requires running Chrome)

```bash
manul run-step "Click the 'Login' button" --cdp http://127.0.0.1:9222
```

### 7. Verbose / JSON output

```bash
manul examples/saucedemo.hunt --verbose
manul examples/saucedemo.hunt --json 2>/dev/null | jq .
```

### Install globally

```bash
go build -o manul ./cmd/manul
cp manul ~/.local/bin/manul
```

> **Note:** The `manul` command name is shared with the Python ManulEngine.
> Whichever you install last takes priority. To switch back to Python: `pipx install manul-engine`.

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

### Debugging

```
PAUSE
DEBUG VARS
```

`PAUSE` enters interactive mode (requires `--debug`). `DEBUG VARS` dumps
all runtime variables.

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
pkg/runtime            Targeting pipeline: probe → filter → score → resolve
pkg/dom             Normalized DOM element model (ElementSnapshot with 27 fields)
pkg/heuristics      In-page JS probes (SnapshotProbe, VisibleTextProbe, ExtractDataProbe)
pkg/scorer          Deterministic 4-channel [0.0–1.0] scoring and ranking
pkg/runtime         DSL execution, control flow, variable management
pkg/dsl             .hunt file parser, import resolver, command AST with block nesting
pkg/explain         Structured execution results and explainability types
pkg/report          Styled HTML report generation
pkg/config          Runtime configuration (18 fields)
pkg/utils           Logging, error types
examples/           7 sample .hunt files
docs/               Documentation
```

See [docs/overview.md](docs/overview.md) for a detailed architecture walkthrough.

---

## Project Status

Alpha. The core engine is feature-complete for single-threaded execution:
32 DSL commands, full control flow (IF/ELIF/ELSE, WHILE, REPEAT, FOR EACH),
import system (including USE/CALL expansion), 4-channel scoring, contextual 
qualifiers (NEAR, ON HEADER/FOOTER, INSIDE), Shadow DOM support, 3-pass 
proximity resolution, HTML reporting, screenshots, debug mode, explain mode, 
and 476 synthetic unit tests across 35 test files.

Not yet implemented: parallel execution (workers > 1), LLM-based fallback,
scan/record subcommands.

---

## What's New

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

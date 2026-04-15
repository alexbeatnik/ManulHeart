# ManulHeart

A deterministic, DSL-first browser automation runtime in Go — a parallel Manul-family product.

ManulHeart executes `.hunt` files using plain-English commands, DOM intelligence,
heuristic element resolution, and structured explainability.
It connects to system-installed Chrome via the Chrome DevTools Protocol (CDP).

**No Playwright. No selectors as a public API. No LLM in the loop.**

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

---

## DSL Syntax

ManulHeart supports a Manul-style DSL subset. Commands are case-insensitive.

### Navigation

```
NAVIGATE to 'https://example.com'
```

### Interaction

```
Click the 'Login' button
Click the 'Sign up' link
Fill 'Email' field with 'user@example.com'
Type 'hello' into the 'Search' field
Select 'Option A' from the 'Category' dropdown
Check the checkbox for 'Remember me'
Uncheck the checkbox for 'Subscribe'
```

### Contextual Qualifiers

```
Click 'Add to cart' button NEAR 'Sauce Labs Bolt T-Shirt'
```

`NEAR` resolves the anchor text first, then prefers the matching element
using Euclidean pixel proximity blended with DOM ancestry affinity.
Product card layouts encode the item label in button IDs/data-test attributes
(e.g. `add-to-cart-sauce-labs-bolt-t-shirt`), so anchor entity affinity
further disambiguates identical-text buttons.

### Assertions

```
VERIFY that 'Welcome' is present
VERIFY that 'Error' is NOT present
```

### Control

```
WAIT 2
```

### File structure

```
@context: suite description
@title: short suite name
@var: {username} = standard_user
@var: {password} = secret_sauce

STEP 1: Login
    NAVIGATE to https://www.saucedemo.com/
    FILL 'Username' field with '{username}'
    FILL 'Password' field with '{password}'
    CLICK 'Login' button

STEP 2: Add item to cart
    VERIFY that 'Products' is present
    CLICK 'Add to cart' button NEAR 'Sauce Labs Bolt T-Shirt'

STEP 3: Review cart
    CLICK the 'Shopping cart' link
    VERIFY that 'Sauce Labs Bolt T-Shirt' is present

DONE.
```

- `@var:` declares variables substituted at parse time (e.g. `{username}` → `standard_user`).
- `NEAR 'anchor'` biases element resolution by contextual proximity to the anchor text.
- `#` lines are comments (ignored by the parser).

---

## Architecture

```
cmd/manul           CLI entry point (produces `manul` binary)
pkg/cdp             Low-level CDP WebSocket transport and domain wrappers
pkg/browser         Abstract browser/page interfaces + CDP backend
pkg/core            Engine-core targeting pipeline (owns all DOM intelligence)
pkg/dom             Normalized DOM element model (ElementSnapshot, PageSnapshot)
pkg/heuristics      In-page JS probes (the primary DOM interrogation layer)
pkg/scorer          Deterministic [0.0–1.0] candidate scoring and ranking
pkg/runtime         DSL execution orchestration
pkg/dsl             .hunt file parser and command AST
pkg/explain         Structured execution results and explainability types
pkg/config          Runtime configuration
pkg/utils           Logging, error types
examples/           Sample .hunt files
docs/               Documentation
```

See [docs/overview.md](docs/overview.md) for a detailed architecture walkthrough.

---

## Project Status

Alpha. Core execution loop, CDP transport, heuristic targeting pipeline, DSL
parser, variables, and NEAR contextual qualifier are implemented.
Not yet implemented: parallel execution, screenshots,
LLM-based fallback, ON HEADER/ON FOOTER/INSIDE qualifiers, scan/record subcommands.

---

## What's New

- **`manul` CLI** — run hunt files directly: `manul test.hunt`, `manul examples/`, `manul .`.
  Replaces the old `driver run` subcommand. The `run` subcommand still works for backward compat.
- **Directory execution** — `manul <dir>` collects and runs all `.hunt` files in the directory.
- **Browser cleanup** — Chrome is always killed on exit, including SIGINT/SIGTERM.
  No more orphaned browser processes.
- **Automation-safe Chrome profile** — password manager, autofill, credential prompts,
  and breach detection dialogs are all suppressed via flags and profile preferences.
- **NEAR qualifier** — `CLICK 'Add to cart' NEAR 'Product Name'` resolves the
  anchor text first, then ranks candidates by spatial proximity blended with
  DOM ancestry affinity and anchor entity affinity in dev attributes.
- **Variables** — `@var: {key} = value` in hunt files, substituted at parse time.
- **JS-based click** — primary click uses `element.click()` for React/SPA compat;
  coordinate mouse events are a fallback.
- **Reliable navigation** — `document.readyState` JS polling instead of CDP
  `Page.loadEventFired` (which misses events on cached/fast pages).
- **`data-test` attribute** — probes now capture both `data-testid` and `data-test`.
- **RawScore sorting** — candidates are ranked by unclipped weighted score so
  NEAR/attr signals differentiate buttons that saturate the `[0,1]` clamp.

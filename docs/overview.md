# ManulHeart — Architecture Overview

## Execution Model

```
.hunt file
    │
    ▼
[pkg/dsl] Parse
    │  Reads .hunt file into Hunt{Commands[]Command}
    │  Each Command has: Type, Target, TypeHint, Value, URL, …
    ▼
[pkg/runtime] RunHunt
    │  Iterates commands, routes each to its handler
    │  For target-based commands → Targeting.Resolve()
    ▼
[pkg/runtime] target-resolution pipeline  ← ENGINE CORE
    │
    ├─ 1. CallProbe(SnapshotProbe, [mode, queries])
    │       → [pkg/heuristics] SnapshotProbe() JS runs IN PAGE
    │         Collects: visibleText, ariaLabel, placeholder, labelText,
    │                   dataQA, id, role, rect, isVisible, isDisabled,
    │                   isEditable, xpath, … for every candidate element.
    │         This is the FIRST and ONLY DOM query for targeting.
    │
    ├─ 2. deserializeSnapshot()
    │       → []dom.ElementSnapshot (normalized Go structs)
    │
    ├─ 3. scorer.Rank(query, typeHint, mode, elements)
    │       → pkg/scorer computes per-channel scores:
    │           text:      exact/substr text, aria, placeholder, label, dataQA
    │           id:        html id variants
    │           semantic:  tag/role alignment with interaction mode
    │           penalty:   disabled ×0.0, hidden ×0.1
    │           proximity: XPath depth
    │         Returns []RankedCandidate sorted by Total score
    │
    └─ 4. Threshold check → return ResolvedTarget{Element, Score, RankedCandidates}
              │
              ▼
    [pkg/runtime] Action execution
        click  → page.ScrollIntoView + page.Click(cx, cy)
        fill   → page.ScrollIntoView + page.SetInputValue(xpath, value)
        select → page.EvalJS (option match by text or value)
        verify → ProbeVisibleText (lightweight text probe, polled)
        wait   → time.Sleep
              │
              ▼
    [pkg/explain] ExecutionResult
        step, commandType, pageURL, candidates, ranked, winnerXPath,
        winnerScore, actionPerformed, success, error, duration
```

---

## Package Responsibilities

### `pkg/runtime`

The engine core. Owners of the target-resolution pipeline and hunt execution.
1. Calls the heuristic probe via `Page.CallProbe()`.
2. Deserializes the probe result into `[]dom.ElementSnapshot`.
3. Delegates scoring to `pkg/scorer`.
4. Enforces the scoring threshold.
5. Performs actions (Click, Fill) via `pkg/browser`.
6. Preserves execution state (variables, loops).

Nothing in `pkg/browser` returns "the element to click" — that is `pkg/runtime`'s job.

### `pkg/heuristics`

Provides `SnapshotProbe()` — a self-contained JavaScript arrow-function expression
that runs a full TreeWalker pass over the live DOM and returns every signal for
every candidate element in one evaluation round-trip. This cost is paid once per
targeting call, not incrementally.

The probe is mode-aware (clickable / input / checkbox / select) so that the
candidate set is already filtered to elements that are relevant for the action.

### `pkg/dsl`

Pure parser. No browser access. Reads `.hunt` files into `Hunt{Commands[]Command}`.
Each `Command` carries the raw source text (preserved for explainability), the
classified `CommandType`, the quoted target, optional element type hint, and the
fill value or URL.

### `pkg/scorer`

Deterministic, stateless scoring. Given a query string, type hint, mode, and a
`[]dom.ElementSnapshot`, returns them ranked by a normalized `[0.0, 1.0]` score.

Scoring channels and weights:

| Channel   | Weight | Signals |
|-----------|--------|---------|
| text      | 0.45   | exact text, normalized text, label, placeholder, aria-label, data-qa |
| id        | 0.25   | html id (with space → dash/underscore variants) |
| semantic  | 0.60   | tag/role alignment, type hint match, cross-mode penalty |
| penalty   | ×mult  | disabled ×0.0, hidden ×0.1 |
| proximity | 0.10   | XPath depth (shallower = small bonus) |

### `pkg/browser`

Defines `Page` and `Browser` interfaces. The CDP backend (`CDPPage`) implements
them via `pkg/cdp`.

The `Page` interface exposes only:
- `Navigate`, `CurrentURL`, `Wait`
- `EvalJS`, `CallProbe` (JS evaluation)
- `Click`, `FocusByXPath`, `SetInputValue`, `ScrollIntoView` (input dispatch)

Nothing in `Page` returns "the element to click" — that is `pkg/runtime`'s job.

### `pkg/cdp`

Raw WebSocket transport for the Chrome DevTools Protocol. Handles:
- Target discovery via `/json` HTTP endpoint
- WebSocket connection lifecycle
- Request/response pipelining with per-call channels
- Event subscription
- Domain helpers: `Navigate`, `Evaluate`, `CallFunctionOn`, `Click`,
  `TypeText`, `SetInputValue`, etc.

### `pkg/explain`

Pure data types: `ExecutionResult`, `HuntResult`, `Candidate`, `ScoreBreakdown`,
`CandidateSignal`. No logic, no browser access. Consumed by the CLI for JSON
output and by the runtime for structured logging.

---

## In-Page JS Heuristics — Design Rationale

A common pattern in thin automation frameworks is:

```
1. fetch(document.querySelectorAll('button'))   ← raw DOM
2. filter by text                               ← basic matching
3. maybe check aria-label                       ← ad hoc enrichment
```

ManulHeart does not do this. Instead, the heuristic probe:

- Runs a **single** TreeWalker pass.
- At each interactive element, collects **all** text signals simultaneously:
  aria-label, placeholder, title, data-qa, data-testid, labelText (resolved via
  `for=`, wrapping `<label>`, or `aria-labelledby`), visibleText, nameAttr, value.
- Computes the element's **accessible name** in-page (where the DOM context is
  available at zero extra cost).
- Builds a **deterministic XPath** (with `[@id=]` anchors where possible).
- Assigns the element a numeric ID via `window.__manulIdCounter`.
- Stores the live DOM node in `window.__manulReg[id]` for later action dispatch
  and debugging of targeting/dispatch.
- Returns **all of this in one JSON payload** — the engine never goes back to the
  DOM to "check one more thing."

The scoring happens entirely in Go on the deserialized snapshot: no additional
round-trips to the page.

---

## Extensibility Points

| Feature | Where to add |
|---------|-------------|
| More browsers | New `browser.Page` implementation in `pkg/browser/` |
| Variables in DSL | `pkg/dsl` — interpolate `{vars}` at parse time |
| Control flow (IF/LOOP) | `pkg/dsl` + `pkg/runtime` |
| Setup/teardown hooks | `pkg/runtime` — `@before:` / `@after:` headers |
| Page abstractions | `pkg/dsl` — `@page:` declarations |
| Custom controls | `pkg/runtime` — pluggable resolver hooks |
| Screenshots | `pkg/cdp` — `Page.captureScreenshot` |
| Scan-page | New subcommand + `pkg/heuristics` scan probe |
| Contextual qualifiers | `pkg/runtime` — NEAR/ON HEADER/INSIDE/USE/CALL expansion |
| Semantic cache      | `pkg/runtime` — XPath reuse from previous steps |
| Shadow DOM          | `pkg/heuristics` — TreeWalker crossing shadow-root boundaries |
| Proximity Targeting | `pkg/runtime` — 3-pass resolution for restrictive inputs |

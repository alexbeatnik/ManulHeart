# Getting Started with ManulHeart

## Prerequisites

- Go 1.21+
- Google Chrome (or Chromium) installed
- A terminal

---

## 1. Build

```bash
git clone <repo>
cd ManulHeart
go build -o driver ./cmd/driver
```

---

## 2. Launch Chrome with remote debugging

ManulHeart attaches to a **running** Chrome instance via CDP.
You must start Chrome yourself with the remote debugging port enabled.

**Linux:**
```bash
google-chrome \
  --remote-debugging-port=9222 \
  --no-first-run \
  --no-default-browser-check \
  --user-data-dir=/tmp/manulheart-profile
```

**macOS:**
```bash
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome \
  --remote-debugging-port=9222 \
  --user-data-dir=/tmp/manulheart-profile
```

**Headless (no window):**
```bash
google-chrome \
  --remote-debugging-port=9222 \
  --headless=new \
  --no-sandbox \
  --user-data-dir=/tmp/manulheart-profile
```

Verify Chrome is listening:
```bash
curl -s http://127.0.0.1:9222/json | head -20
```

---

## 3. Write a .hunt file

Create `tests/my_flow.hunt`:

```
@context: Login to the demo application and verify the secure area
@title: Demo Login

STEP 1: Navigate and verify start page
    NAVIGATE to 'https://the-internet.herokuapp.com/login'
    VERIFY that 'Login Page' is present

STEP 2: Submit credentials
    Fill 'Username' field with 'tomsmith'
    Fill 'Password' field with 'SuperSecretPassword!'
    Click the 'Login' button

STEP 3: Verify success
    VERIFY that 'You logged into a secure area!' is present

DONE.
```

**DSL rules:**
- Commands are case-insensitive (`NAVIGATE`, `navigate`, and `Navigate` all work).
- Target text goes inside single quotes: `Click the 'Login' button`.
- The element type hint (`button`, `link`, `field`, etc.) after the target is optional
  but improves scoring accuracy.
- `STEP N:` blocks are structural labels and do not affect execution order.
- `DONE.` terminates the file; lines after it are ignored.

---

## 4. Run the hunt file

```bash
./driver run tests/my_flow.hunt --cdp http://127.0.0.1:9222
```

Expected output:
```
[12:34:56.789] [INFO] ManulHeart — tests/my_flow.hunt
[12:34:56.790] [INFO] Title: Demo Login
[12:34:56.791] [INFO] Commands: 6
[12:34:56.792] [INFO] CDP: http://127.0.0.1:9222
[12:34:56.800] [INFO] [1] NAVIGATE to 'https://the-internet.herokuapp.com/login'
[12:34:57.543] [INFO]   ✓ done (743ms)
[12:34:57.544] [INFO] [2] VERIFY that 'Login Page' is present
[12:34:57.603] [INFO]   ✓ done (59ms)
...
[12:34:59.120] [INFO] ✓ All 6 steps passed (2330ms)
```

---

## 5. Run a single step interactively

```bash
./driver run-step "NAVIGATE to 'https://example.com'" --cdp http://127.0.0.1:9222
./driver run-step "Click the 'More information...' link" --cdp http://127.0.0.1:9222
./driver run-step "VERIFY that 'IANA' is present"
```

---

## 6. JSON output

Both `run` and `run-step` support `--json` for machine-readable output:

```bash
./driver run examples/login.hunt --json 2>/dev/null | jq '.results[] | {step:.step, success:.success, winner:.winner_xpath, score:.winner_score}'
```

The JSON result includes:
- Per-command `ExecutionResult` with ranked candidates and score breakdowns
- `winner_xpath`, `winner_score` for the resolved element
- `candidates_considered`, `ranked_candidates` with full `ScoreBreakdown`
- `duration_ms`, `success`, `error`

---

## 7. Verbose / debug mode

```bash
./driver run examples/login.hunt --verbose
```

Verbose mode logs:
- Number of candidates discovered per targeting call
- The best candidate's visible text, XPath, and score
- Scroll and action dispatch details

---

## Troubleshooting

| Problem | Fix |
|---------|-----|
| `connect to browser: dial …: connection refused` | Chrome is not running with `--remote-debugging-port=9222` |
| `no page target found` | The Chrome instance has no open tab; open one manually |
| `best score 0.xxx below threshold 0.15` | The target text doesn't match any element well. Use `--verbose` and `--json` to inspect candidates. |
| `VERIFY: "X" not found on page` | The text is not in the visible DOM within the verify timeout. Check the page state. |

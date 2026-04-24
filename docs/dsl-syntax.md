# DSL Syntax Reference

> *The complete `.hunt` language. Plain English. No selectors.*

A `.hunt` file is a sequence of commands, metadata headers, and structural blocks. Commands are **case-insensitive**. Indentation (4 spaces) denotes block nesting under control-flow headers.

---

## File Structure

```hunt
@context: Suite description for reports
@title: short-suite-name
@var: {username} = standard_user
@var: {password} = secret_sauce
@tags: smoke, login
@script: {helpers} = mypackage.helpers
@import: Login from 'shared/auth.hunt'

[SETUP]
    CALL GO {helpers}.seed_db
    SET {token} = placeholder
[END SETUP]

STEP 1: Navigate and login
    NAVIGATE to https://www.saucedemo.com/
    USE Login
    VERIFY that 'Products' is present

[TEARDOWN]
    CALL GO {helpers}.cleanup_db
[END TEARDOWN]

DONE.
```

### Directives

| Directive | Syntax | Scope |
|-----------|--------|-------|
| `@context:` | `@context: description text` | Report metadata |
| `@title:` | `@title: suite_name` | Report title, default step block label |
| `@var:` | `@var: {name} = value` | Mission-level variable (LevelMission) |
| `@tags:` | `@tags: tag1, tag2` | File-level tags (filterable with `--tags`) |
| `@script:` | `@script: {alias} = dotted.go.path` | Alias for `CALL GO` rewrite |
| `@import:` | `@import: Name from 'file.hunt'` | Import reusable step blocks |
| `@data:` | `@data: data.json` | External data file reference |
| `@schedule:` | `@schedule: daily` | Scheduling metadata |
| `@export:` | `@export: var1, var2` | Export variables across hunts |

### Import Variants

```hunt
@import: Login from 'shared/auth.hunt'
@import: Login as QuickLogin from 'shared/auth.hunt'
@import: * from 'shared/setup.hunt'
```

Imported blocks are referenced via `USE BlockName` or `CALL BlockName`.

---

## Navigation

```hunt
NAVIGATE to 'https://example.com'
NAVIGATE to https://example.com/path
SCROLL DOWN
SCROLL UP
SCROLL DOWN inside the 'Product List'
```

| Command | Description |
|---------|-------------|
| `NAVIGATE to <url>` | Load URL and wait for readyState complete |
| `SCROLL DOWN` | Scroll page down by ~500px |
| `SCROLL UP` | Scroll page up by ~500px |
| `SCROLL DOWN inside the '<container>'` | Scroll within a container element |

---

## Interaction

### Click

```hunt
Click the 'Login' button
Click the 'Sign up' link
Click 'Submit' element
DOUBLE CLICK the 'Copy Text' button
RIGHT CLICK the 'Context Menu' element
```

### Fill & Type

```hunt
Fill 'Email' field with 'user@example.com'
Fill 'Password' field with '{password}'
Type 'hello world' into the 'Search' field
```

- `FILL` sets the value directly via CDP `DOM.setInputValue`
- `TYPE` simulates keystrokes via `Input.dispatchKeyEvent`

### Select

```hunt
Select 'Option A' from the 'Category' dropdown
Select 'United States' from the 'Country' select
```

Handles both native `<select>` elements and custom dropdowns (clicks container, finds option by text).

### Check / Uncheck

```hunt
Check the checkbox for 'Remember me'
Uncheck the checkbox for 'Subscribe'
Check the 'Terms' checkbox
```

Compound hints like `radio button` and `toggle switch` are resolved to the correct semantic type.

### Hover

```hunt
Hover over the 'Profile' menu
Hover the 'Tooltip' element
```

### Drag & Drop

```hunt
Drag the element 'Card A' and drop it into 'Column B'
Drag 'Source' and drop it into 'Target'
```

### Upload

```hunt
Upload '/path/to/file.png' to 'Avatar Upload'
Upload '/path/to.pdf' to 'Attachment' field
```

### Keyboard

```hunt
PRESS Enter
PRESS Escape
PRESS Control+A
PRESS Tab ON 'Username'
```

| Key | Support |
|-----|---------|
| `Enter`, `Tab`, `Escape`, `Backspace`, `Delete` | Full |
| `ArrowUp`, `ArrowDown`, `ArrowLeft`, `ArrowRight` | Full |
| `Home`, `End`, `PageUp`, `PageDown`, `Space` | Full |
| `Control+A`, `Control+C`, `Control+V` | Modifier combos |
| `F1`–`F12` | Full |

The `ON '<target>'` qualifier focuses the element before dispatching the key.

---

## Contextual Qualifiers

Qualifiers restrict the candidate set **before** scoring runs. They are composable.

### NEAR

```hunt
Click 'Add to cart' button NEAR 'Sauce Labs Fleece Jacket'
Fill 'Quantity' field with '2' NEAR 'Product Name'
```

`NEAR` resolves the anchor text first, then boosts candidates within Euclidean pixel distance of the anchor. The proximity channel weight is `1.50` (vs `0.10` base DOM ancestry).

### ON HEADER / ON FOOTER

```hunt
Click the 'Logo' link ON HEADER
Click the 'Terms' link ON FOOTER
```

Restricts candidates to elements geometrically within the header/footer region or structurally inside `<header>` / `<footer>` / `<nav>` ancestors.

### INSIDE

```hunt
Click the 'Delete' button INSIDE 'Actions'
Click the 'Edit' button INSIDE 'Users' row with 'John Doe'
```

- `INSIDE 'Container'` — restricts to descendants of the container element
- `INSIDE 'Container' row with 'Text'` — finds the container, then the row containing the text, then descendants of that row

---

## Assertions

### VERIFY

```hunt
VERIFY that 'Welcome' is present
VERIFY that 'Error' is NOT present
VERIFY that 'Spinner' is hidden
VERIFY that 'Modal' is visible
```

Hard assertion. Failure stops the hunt. Uses a polled visible-text probe with `DefaultTimeout` deadline.

### VERIFY SOFTLY

```hunt
VERIFY SOFTLY that 'Optional Banner' is present
VERIFY SOFTLY that 'Legacy Text' is NOT present
```

Non-fatal. Logs a warning but execution continues. Single-shot (no retry loop).

### VERIFY FIELD

```hunt
Verify 'Email' field has value 'user@example.com'
Verify 'Heading' field has text 'Dashboard'
Verify 'Search' field has placeholder 'Type to search...'
```

Full element resolution + state verification. Supports `text`, `value`, `placeholder` kinds.

### Element State Verification

```hunt
VERIFY that 'Terms' is checked
VERIFY that 'Newsletter' is unchecked
VERIFY that 'Submit' is enabled
VERIFY that 'Submit' is disabled
VERIFY that 'Modal' is visible
VERIFY that 'Toast' is hidden
VERIFY that 'Option B' is selected
```

States: `checked`, `unchecked`, `enabled`, `disabled`, `visible`, `hidden`, `selected`.

---

## Data & Variables

### @var Declaration

```hunt
@var: {base_url} = https://example.com
@var: {timeout} = 30
```

`@var:` values are stored at **LevelMission** scope. They shadow globals and are shadowed by step/row variables.

### SET

```hunt
SET {discount} = 10%
SET {full_name} = '{first_name} {last_name}'
```

`SET` writes to **LevelStep** scope.

### EXTRACT

```hunt
EXTRACT the 'Price' into {total}
EXTRACT the 'Username' into {current_user}
```

Reads the element's text/value and stores it in **LevelRow** scope.

### PRINT

```hunt
PRINT 'Current total: {total}'
PRINT 'Debug: user={current_user}, role={role}'
```

Outputs to the semantic logger at `ActionDetail` level.

### Variable Substitution

Variables are interpolated at **runtime** using the precedence chain:

```
Row > Step > Mission > Global > Import
```

Syntax: `{var_name}`, `${var_name}`, or `$var_name`.

---

## Waiting

```hunt
WAIT 2
WAIT 1.5
Wait for 'Spinner' to be hidden
Wait for 'Results' to be visible
Wait for 'Toast' to disappear
WAIT FOR RESPONSE "api/users"
```

| Command | Behavior |
|---------|----------|
| `WAIT <seconds>` | Pause execution |
| `WAIT FOR '<target>' to be <state>` | Poll element state until deadline |
| `WAIT FOR RESPONSE "<pattern>"` | Block until network response matches URL suffix |

States for `WAIT FOR`: `visible`, `hidden`, `disappear`.

---

## Control Flow

### IF / ELIF / ELSE

```hunt
IF 'Dashboard' is present:
    Click the 'Logout' button
ELIF {role} == 'admin':
    Click the 'Admin Panel' link
ELSE:
    Click the 'Login' button
```

Conditions supported:
- Element presence: `'<target>' is present`, `'<target>' is NOT present`
- Element existence: `button '<target>' exists`, `field '<target>' not exists`
- Variable equality: `{var} == 'value'`, `{var} != 'value'`
- Variable containment: `{var} contains 'substring'`
- Truthy check: `{var}` (bare variable, true if non-empty and not `"false"`/`"0"`/`"null"`)

### WHILE

```hunt
WHILE 'Next' is present:
    Click the 'Next' button
```

Capped at 100 iterations to prevent infinite loops.

### REPEAT

```hunt
REPEAT 3 TIMES:
    Click the 'Add Item' button

REPEAT 5 TIMES as {i}:
    PRINT 'Iteration {i}'
```

Default loop variable is `{i}`.

### FOR EACH

```hunt
FOR EACH {item} IN {items}:
    Fill 'Search' field with '{item}'
    Click the 'Search' button
```

The collection variable is split on commas. `{items}` might be set to `"apple,banana,cherry"`.

---

## Hooks

### SETUP

```hunt
[SETUP]
    CALL GO helpers.seed_db
    SET {token} = abc123
[END SETUP]
```

Runs **before** the mission body. If setup fails, the mission aborts (fail-fast). Variables set during setup persist into the mission.

### TEARDOWN

```hunt
[TEARDOWN]
    CALL GO helpers.cleanup_db
    PRINT 'Teardown complete'
[END TEARDOWN]
```

Runs **always**, even if setup or the mission fails. Variables set during teardown persist until the hunt ends.

---

## Extensions

### CALL GO

```hunt
CALL GO mypackage.helpers.generate_token "admin" into {token}
CALL GO math.add 5 10 to {result}
CALL GO {helpers}.echo "hello" with args: 'a' 'b' into {out}
```

Invokes a registered Go handler. Supports `@script:` aliases, positional arguments, and `into` / `to` result binding.

### USE / CALL (Step Blocks)

```hunt
USE LoginBlock
CALL LoginBlock
```

Inlines an imported step block at parse time. `USE` and `CALL` are aliases.

---

## Debugging

### PAUSE

```hunt
STEP 5:
    PAUSE
    Click the 'Confirm' button
```

Enters the interactive debugger. In TTY mode, presents a terminal prompt. In pipe mode (VS Code extension), emits `\x00MANUL_DEBUG_PAUSE\x00` markers.

### DEBUG VARS

```hunt
DEBUG VARS
```

Prints all variables across all scopes (ROW, STEP, MISSION, GLOBAL, IMPORT).

### Interactive Debugger Commands

When paused (`--debug` or `PAUSE`):

| Command | Action |
|---------|--------|
| `next` / Enter | Execute current step, pause at next |
| `continue` | Free-run until next breakpoint |
| `debug-stop` | Clear all breakpoints, free-run to end |
| `explain` / `explain-next` | Score candidates and print breakdown |
| `explain-next {"step":"..."}` | Score an overridden step text |
| `highlight <xpath>` | Magenta outline on specific element |
| `abort` | Stop hunt execution |

---

## Structural Commands

| Command | Purpose |
|---------|---------|
| `STEP N: Label` | Logical grouping label (no execution effect) |
| `USE <Block>` | Inline imported step block |
| `CALL <Block>` | Alias for `USE` |
| `DONE.` | Terminates file; subsequent lines ignored |

---

## Complete Command Reference

| Command | Category | Example |
|---------|----------|---------|
| `NAVIGATE` | Navigation | `NAVIGATE to 'https://example.com'` |
| `CLICK` | Interaction | `Click the 'Submit' button` |
| `DOUBLE CLICK` | Interaction | `DOUBLE CLICK the 'Copy' button` |
| `RIGHT CLICK` | Interaction | `RIGHT CLICK the 'Menu' element` |
| `FILL` | Interaction | `Fill 'Email' field with 'user@example.com'` |
| `TYPE` | Interaction | `Type 'hello' into the 'Search' field` |
| `SELECT` | Interaction | `Select 'Option A' from the 'Category' dropdown` |
| `CHECK` | Interaction | `Check the checkbox for 'Terms'` |
| `UNCHECK` | Interaction | `Uncheck the checkbox for 'Subscribe'` |
| `HOVER` | Interaction | `Hover over the 'Profile' menu` |
| `DRAG` | Interaction | `Drag 'Source' and drop it into 'Target'` |
| `UPLOAD` | Interaction | `Upload '/path.png' to 'Avatar'` |
| `PRESS` | Keyboard | `PRESS Control+A` |
| `SCROLL` | Navigation | `SCROLL DOWN inside the 'List'` |
| `WAIT` | Timing | `WAIT 2` |
| `WAIT FOR` | Timing | `Wait for 'Spinner' to be hidden` |
| `WAIT FOR RESPONSE` | Network | `WAIT FOR RESPONSE "api/users"` |
| `VERIFY` | Assertion | `VERIFY that 'Welcome' is present` |
| `VERIFY SOFTLY` | Assertion | `VERIFY SOFTLY that 'Banner' is present` |
| `VERIFY FIELD` | Assertion | `Verify 'Email' has value 'user@example.com'` |
| `EXTRACT` | Data | `EXTRACT the 'Price' into {total}` |
| `SET` | Data | `SET {name} = Alice` |
| `PRINT` | Data | `PRINT 'Done: {name}'` |
| `CALL GO` | Extension | `CALL GO helpers.auth into {token}` |
| `USE` / `CALL` | Modular | `USE LoginBlock` |
| `IF` / `ELIF` / `ELSE` | Control | `IF 'X' is present: ...` |
| `WHILE` | Control | `WHILE 'Next' is present: ...` |
| `REPEAT` | Control | `REPEAT 3 TIMES: ...` |
| `FOR EACH` | Control | `FOR EACH {i} IN {items}: ...` |
| `PAUSE` | Debug | `PAUSE` |
| `DEBUG VARS` | Debug | `DEBUG VARS` |

---

## DSL Rules

1. **Case-insensitive.** `NAVIGATE`, `navigate`, and `Navigate` are identical.
2. **Quotes matter.** Target labels go in single quotes: `'Login'`. Values can use single or double quotes.
3. **Indentation is 4 spaces.** Block bodies under `IF`, `WHILE`, `REPEAT`, `FOR EACH`, `STEP` must be indented.
4. **Comments start with `#`.** Inline and line-leading comments are ignored.
5. **`DONE.` terminates parsing.** Nothing after it is read.
6. **Variable interpolation is runtime.** `{var}` is resolved when the command executes, not when it parses (except `@var:` which is parsed into scope).
7. **Setup runs first, teardown runs last.** Teardown executes even if setup or mission fails.

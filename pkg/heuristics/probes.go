// Package heuristics provides the in-page JavaScript probes that ManulHeart
// injects into the browser to collect normalized candidate data.
//
// These probes are first-class components of the engine targeting pipeline.
// They are NOT ad hoc helper scripts — they are invoked on every target-based
// DSL command as the primary DOM interrogation step.
//
// The JS returned by SnapshotProbe collects every meaningful text signal,
// accessibility attribute, geometry, and interactability hint for each
// candidate element in a single page evaluation pass.
package heuristics

import _ "embed"

// SnapshotProbe returns the JavaScript expression (a self-invoking arrow
// function) that collects normalized element candidates from the live page.
//
// Arguments passed to the function (as a JSON array serialized to string):
//
//	[mode string, expectedTexts []string]
//
// mode is "input" for fill/type commands, "checkbox" for check/uncheck,
// "select" for select commands, and "clickable" for everything else.
//
// Returns a JSON-serializable object:
//
//	{
//	  "url":         string,
//	  "title":       string,
//	  "visibleText": string,
//	  "elements":    ElementSnapshot[]
//	}
func SnapshotProbe() string {
	return snapshotProbeJS
}

// VisibleTextProbe returns JavaScript that collects only the full visible
// text of the page. Used for VERIFY commands that don't require element resolution.
func VisibleTextProbe() string {
	return visibleTextProbeJS
}

// XPathResolveProbe returns JavaScript that resolves an XPath to a DOM node
// and returns its bounding rect and current visibility state.
func XPathResolveProbe() string {
	return xpathResolveProbeJS
}

// ExtractDataProbe returns JavaScript that extracts data from the page using
// table-row word matching and column-header resolution. This bypasses the
// standard DOM scoring pipeline (matching ManulEngine's EXTRACT_DATA_JS).
//
// Arguments (JSON array): [target string, hint string]
//   - target: the full query, e.g. "cpu of chrome"
//   - hint:   context words extracted from the query minus stop words
//
// Returns a string with the extracted value, or empty string if not found.
func ExtractDataProbe() string {
	return extractDataProbeJS
}

// ── JavaScript probe implementations ─────────────────────────────────────────

// snapshotProbeJS is the primary heuristic probe.
// It runs a full TreeWalker pass over the DOM, extracting every element that
// could plausibly be the target of a Manul DSL command.
// For each candidate it collects:
//   - tag, id, class, role, inputType
//   - visibleText (innerText, trimmed)
//   - ariaLabel, placeholder, title, dataQA, dataTestId, labelText, nameAttr, value
//   - boundingRect (top/left/bottom/right/width/height)
//   - isVisible, isDisabled, isHidden, isEditable, isInShadow
//   - a deterministic XPath
//   - an engine-assigned numeric id stored in window.manulElements[id]
const snapshotProbeJS = `([mode, expectedTexts]) => {
    // ── Element registry (idempotent across calls) ────────────────────
    if (!window.__manulElements) {
        window.__manulElements = {};
        window.__manulIdCounter = 0;
    }
    const reg = window.__manulElements;

    const isInputMode    = mode === 'input';
    const isCheckMode    = mode === 'checkbox';
    const isSelectMode   = mode === 'select';
    const isLocateMode   = mode === 'none' || mode === 'locate';
    const hasCheckVis    = typeof Element.prototype.checkVisibility === 'function';

    // ── Pruned tags (subtrees entirely skipped) ───────────────────────
    const PRUNE = new Set(['SCRIPT','STYLE','NOSCRIPT','META','PATH','G','BR','HR','TEMPLATE','SVG']);

    // ── Clickable role set ────────────────────────────────────────────
    const CLICK_ROLES = new Set([
        'button','checkbox','radio','tab','option','menuitem','switch',
        'slider','application','link','textbox','spinbutton','menuitemcheckbox',
        'menuitemradio','treeitem','gridcell','columnheader','rowheader'
    ]);
    const RE_CLS = /btn|button|swatch|card|tab|option|ui-drag|ui-drop/i;

    // ── Interactivity predicate (mode-aware) ──────────────────────────
    const isInteractive = (el) => {
        const t = el.tagName;
        // Locate/none mode: broad search — include any element with text content.
        // Used by EXTRACT, WAIT FOR, and similar text-finding commands.
        if (isLocateMode) {
            if (t === 'BUTTON' || t === 'A' || t === 'INPUT' || t === 'SELECT' ||
                t === 'TEXTAREA' || t === 'LABEL' || t === 'SUMMARY') return true;
            if (t === 'TD' || t === 'TH' || t === 'LI' || t === 'SPAN' || t === 'P' ||
                t === 'H1' || t === 'H2' || t === 'H3' || t === 'H4' || t === 'H5' || t === 'H6' ||
                t === 'DIV' || t === 'SECTION' || t === 'ARTICLE' || t === 'FIGCAPTION' ||
                t === 'CAPTION' || t === 'DD' || t === 'DT' || t === 'OPTION') return true;
            if (el.hasAttribute('data-qa') || el.hasAttribute('data-testid') || el.hasAttribute('data-test')) return true;
            if (el.hasAttribute('aria-label') || el.hasAttribute('title')) return true;
            const txt = (el.innerText || el.textContent || '').trim();
            if (txt.length > 0 && txt.length < 300) {
                // Any element with meaningful text content
                if (el.children.length === 0 || t === 'TD' || t === 'TH') return true;
            }
            return false;
        }
        if (isInputMode) {
            if (t === 'INPUT' && el.type !== 'submit' && el.type !== 'button'
                && el.type !== 'file' && el.type !== 'image' && el.type !== 'reset') return true;
            if (t === 'TEXTAREA') return true;
            if (el.getAttribute('contenteditable') === 'true') return true;
            const r = (el.getAttribute('role') || '').toLowerCase();
            return r === 'textbox' || r === 'slider' || r === 'spinbutton' || r === 'combobox';
        }
        if (isCheckMode) {
            if (t === 'INPUT' && (el.type === 'checkbox' || el.type === 'radio')) return true;
            if (t === 'LABEL') return true;
            const r = (el.getAttribute('role') || '').toLowerCase();
            return r === 'checkbox' || r === 'radio' || r === 'switch';
        }
        if (isSelectMode) {
            if (t === 'SELECT') return true;
            const r = (el.getAttribute('role') || '').toLowerCase();
            return r === 'listbox' || r === 'combobox' || r === 'option';
        }
        // clickable (default)
        if (t === 'BUTTON' || t === 'A' || t === 'INPUT' || t === 'SELECT' ||
            t === 'TEXTAREA' || t === 'SUMMARY' || t === 'LABEL') return true;
        if (t === 'IMG') return !!(el.getAttribute('alt') || el.getAttribute('aria-label'));
        if (t.includes('-')) return true; // custom elements
        const r = (el.getAttribute('role') || '').toLowerCase();
        if (r && CLICK_ROLES.has(r)) return true;
        if (el.hasAttribute('data-qa') || el.hasAttribute('data-testid') || el.hasAttribute('data-test')) return true;
        if (el.hasAttribute('aria-label') || el.hasAttribute('title')) return true;
        if (el.hasAttribute('onclick') || el.hasAttribute('tabindex')) return true;
        if (el.id && (t === 'DIV' || t === 'SPAN')) return true;
        // <li> elements are common dropdown/menu items; include if they have brief text
        if (t === 'LI') {
            const txt = (el.innerText || el.textContent || '').trim();
            if (txt.length > 0 && txt.length < 200) return true;
        }
        // <option> elements inside custom dropdowns (non-native)
        if (t === 'OPTION') return true;
        const cn = typeof el.className === 'string' ? el.className : '';
        if (cn && RE_CLS.test(cn)) return true;
        return false;
    };

    // ── Visibility check ──────────────────────────────────────────────
    const isVisible = (el) => {
        if (hasCheckVis) {
            return el.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true });
        }
        const cs = window.getComputedStyle(el);
        const hasLayout = el.offsetWidth > 0 && el.offsetHeight > 0;
        const hidden = cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) < 0.01;
        return hasLayout && !hidden;
    };

    // ── XPath builder (deterministic, full absolute path) ─────────────
    const buildXPath = (el) => {
        const parts = [];
        let n = el;
        while (n && n.nodeType === Node.ELEMENT_NODE) {
            let idx = 1;
            let sib = n.previousElementSibling;
            while (sib) {
                if (sib.tagName === n.tagName) idx++;
                sib = sib.previousElementSibling;
            }
            const tag = n.tagName.toLowerCase();
            // Optimize with id if unique
            if (n.id && document.querySelectorAll('#' + CSS.escape(n.id)).length === 1) {
                parts.unshift(tag + '[@id="' + n.id + '"]');
                n = null; // absolute anchor found
                break;
            }
            parts.unshift(tag + '[' + idx + ']');
            n = n.parentElement;
        }
        return (n === null ? '//' : '/') + parts.join('/');
    };

    // ── Label text resolver ───────────────────────────────────────────
    const getLabelText = (el) => {
        // 1. Explicitly linked <label for="id">
        if (el.id) {
            const lbl = document.querySelector('label[for="' + CSS.escape(el.id) + '"]');
            if (lbl) return (lbl.innerText || '').trim();
        }
        // 2. Wrapping <label>
        const parent = el.closest('label');
        if (parent) {
            // Get label text minus the input's own text
            return (parent.innerText || '').replace((el.innerText || ''), '').trim();
        }
        // 3. aria-labelledby
        const labelledBy = el.getAttribute('aria-labelledby');
        if (labelledBy) {
            return labelledBy.split(/\s+/).map(id => {
                const ref = document.getElementById(id);
                return ref ? (ref.innerText || '').trim() : '';
            }).filter(Boolean).join(' ');
        }
        // 4. Preceding sibling or parent text node that looks like a label
        const prev = el.previousElementSibling;
        if (prev && (prev.tagName === 'LABEL' || prev.tagName === 'SPAN' || prev.tagName === 'P')) {
            const t = (prev.innerText || '').trim();
            if (t.length > 0 && t.length < 80) return t;
        }
        // 5. Table row context: for inputs inside <td> (or <td> elements
        //    themselves), collect text from sibling <td> cells in the same
        //    <tr>, plus the column header from <th>. This handles paginated
        //    tables where checkboxes are in one column and the row label
        //    (e.g. row number, product name) is in adjacent columns.
        //    The column header text enables EXTRACT 'CPU of Chrome' to
        //    match the correct cell by combining row+column context.
        const td = el.closest('td');
        if (td) {
            const tr = td.closest('tr');
            if (tr) {
                const cells = Array.from(tr.cells || tr.querySelectorAll('td'));
                const texts = [];
                for (const c of cells) {
                    if (c === td) continue;
                    const ct = (c.innerText || '').trim();
                    if (ct.length > 0 && ct.length < 100) texts.push(ct);
                }
                // Also include column header from <th>
                const table = tr.closest('table');
                if (table) {
                    const colIdx = Array.from(tr.cells || tr.children).indexOf(td);
                    if (colIdx >= 0) {
                        const hrow = table.querySelector('thead tr')
                            || (() => { const fr = table.querySelector('tr'); return (fr && fr !== tr && fr.querySelector('th')) ? fr : null; })();
                        if (hrow) {
                            const hCells = Array.from(hrow.cells || hrow.children);
                            if (colIdx < hCells.length) {
                                const ht = (hCells[colIdx].innerText || '').trim();
                                if (ht && ht.length < 100) texts.unshift(ht);
                            }
                        }
                    }
                }
                if (texts.length > 0) return texts.join(' ');
            }
        }
        return '';
    };

    // ── Accessible name derivation ────────────────────────────────────
    const getAccessibleName = (el) => {
        const ariaLabel = el.getAttribute('aria-label');
        if (ariaLabel) return ariaLabel.trim();
        const labelText = getLabelText(el);
        if (labelText) return labelText;
        const title = el.getAttribute('title');
        if (title) return title.trim();
        const alt = el.getAttribute('alt');
        if (alt) return alt.trim();
        return '';
    };

    // ── Collection ────────────────────────────────────────────────────
    const seen = new Set();
    const elements = [];

    const processElement = (el, inShadow) => {
        if (seen.has(el)) return;
        seen.add(el);
        if (!isInteractive(el)) return;

        const tag = el.tagName.toLowerCase();
        const vis = isVisible(el);

        // Special inputs (checkbox, radio, file) get collected even when hidden
        const isSpecial = tag === 'input' && (el.type === 'checkbox' || el.type === 'radio' || el.type === 'file');
        if (!vis && !isSpecial) return;

        const rect = el.getBoundingClientRect();
        if (!isSpecial && rect.width < 2 && rect.height < 2) return;

        // Register element in global registry
        let eid = el.__manulId;
        if (eid === undefined) {
            eid = ++window.__manulIdCounter;
            el.__manulId = eid;
        }
        reg[eid] = el;

        const visibleText = (el.innerText || el.textContent || '').trim().slice(0, 300);
        const accessibleName = getAccessibleName(el);
        const labelText = getLabelText(el);

        elements.push({
            id:          eid,
            xpath:       buildXPath(el),
            tag:         tag,
            inputType:   el.type || '',
            visibleText: visibleText,
            ariaLabel:   el.getAttribute('aria-label') || '',
            placeholder: el.getAttribute('placeholder') || '',
            title:       el.getAttribute('title') || '',
            dataQA:      el.getAttribute('data-qa') || '',
            dataTestId:  el.getAttribute('data-testid') || el.getAttribute('data-test') || '',
            labelText:   labelText,
            nameAttr:    el.getAttribute('name') || '',
            htmlId:      el.id || '',
            className:   (typeof el.className === 'string' ? el.className : '') || '',
            role:        el.getAttribute('role') || '',
            value:       (el.value !== undefined ? el.value : '') || '',
            accessibleName: accessibleName,
            isVisible:   vis,
            isDisabled:  el.disabled || el.getAttribute('aria-disabled') === 'true',
            isHidden:    !vis,
            isEditable:  !el.disabled && !el.readOnly &&
                         (tag === 'input' || tag === 'textarea' ||
                          el.getAttribute('contenteditable') === 'true' ||
                          (el.getAttribute('role') || '') === 'textbox'),
            isInShadow:  inShadow,
            rect: {
                top:    rect.top,
                left:   rect.left,
                bottom: rect.bottom,
                right:  rect.right,
                width:  rect.width,
                height: rect.height,
            },
        });
    };

    // ── TreeWalker DOM traversal ──────────────────────────────────────
    const walk = (root, inShadow) => {
        const tw = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT, {
            acceptNode(n) {
                if (PRUNE.has(n.tagName)) return NodeFilter.FILTER_REJECT;
                if (n.hasAttribute && n.hasAttribute('data-manul-debug')) return NodeFilter.FILTER_REJECT;
                return NodeFilter.FILTER_ACCEPT;
            }
        });
        let n;
        while ((n = tw.nextNode())) {
            if (n.shadowRoot) walk(n.shadowRoot, true);
            processElement(n, inShadow);
        }
    };
    walk(document.body || document.documentElement, false);

    // ── Full visible page text (for VERIFY) ───────────────────────────
    const pageText = (() => {
        let t = (document.body ? document.body.innerText : '') + ' ';
        document.querySelectorAll('[aria-label],[placeholder],[title],[aria-valuetext]').forEach(el => {
            t += ' ' + (el.getAttribute('aria-label') || '');
            t += ' ' + (el.getAttribute('placeholder') || '');
            t += ' ' + (el.getAttribute('title') || '');
            t += ' ' + (el.getAttribute('aria-valuetext') || '');
        });
        return t.toLowerCase();
    })();

    return {
        url:         window.location.href,
        title:       document.title || '',
        visibleText: pageText,
        elements:    elements
    };
}`

// visibleTextProbeJS quickly collects visible page text without full element traversal.
const visibleTextProbeJS = `() => {
    let t = (document.body ? document.body.innerText : '') + ' ';
    document.querySelectorAll('[aria-label],[placeholder],[title],[value]').forEach(el => {
        const cs = window.getComputedStyle(el);
        if (cs.display === 'none' || cs.visibility === 'hidden') return;
        t += ' ' + (el.getAttribute('aria-label') || '');
        t += ' ' + (el.getAttribute('placeholder') || '');
        t += ' ' + (el.getAttribute('title') || '');
        if (el.value) t += ' ' + el.value;
    });
    return { url: window.location.href, text: t.toLowerCase() };
}`

// xpathResolveProbeJS resolves an XPath expression and returns its current state.
const xpathResolveProbeJS = `(xpath) => {
    const result = document.evaluate(
        xpath, document, null,
        XPathResult.FIRST_ORDERED_NODE_TYPE, null
    );
    const el = result.singleNodeValue;
    if (!el) return null;
    const rect = el.getBoundingClientRect();
    const cs = window.getComputedStyle(el);
    const vis = !(cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) < 0.01)
                && rect.width > 0 && rect.height > 0;
    return {
        found:      true,
        tag:        el.tagName.toLowerCase(),
        isVisible:  vis,
        isDisabled: el.disabled || el.getAttribute('aria-disabled') === 'true',
        rect:       { top: rect.top, left: rect.left, bottom: rect.bottom, right: rect.right,
                      width: rect.width, height: rect.height }
    };
}`

// extractDataProbeJS is loaded from extract_data.js via go:embed.
// It is a dedicated data-extraction probe for EXTRACT commands.
//
//go:embed extract_data.js
var extractDataProbeJS string

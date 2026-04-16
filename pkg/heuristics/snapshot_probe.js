(() => {
    // ── Constants ──────────────────────────────────────────────────────────────

    // Tags to prune entirely (their subtrees carry no targeting signal).
    const PRUNE = new Set(['SCRIPT', 'STYLE', 'NOSCRIPT', 'SVG', 'TEMPLATE', 'HEAD']);

    // ── Utilities ──────────────────────────────────────────────────────────────

    const normalize = (v) => (v || '').replace(/\s+/g, ' ').trim();

    const isVisible = (el) => {
        const cs = window.getComputedStyle(el);
        const rect = el.getBoundingClientRect();
        return cs.display !== 'none'
            && cs.visibility !== 'hidden'
            && parseFloat(cs.opacity) >= 0.01
            && rect.width > 0
            && rect.height > 0;
    };

    const isInteractive = (el) => {
        const tag = el.tagName.toLowerCase();
        if (['button', 'a', 'input', 'select', 'textarea', 'option', 'label',
             'summary', 'details', 'img'].includes(tag)) return true;
        const role = (el.getAttribute('role') || '').toLowerCase();
        if (['button', 'link', 'menuitem', 'tab', 'checkbox', 'radio',
             'switch', 'slider', 'spinbutton', 'textbox', 'combobox',
             'listbox', 'option', 'treeitem', 'gridcell', 'columnheader',
             'rowheader', 'row', 'cell'].includes(role)) return true;
        if (el.getAttribute('contenteditable') === 'true') return true;
        // Table data cells — needed for EXTRACT and disambiguation.
        if (tag === 'td' || tag === 'th' || tag === 'li' || tag === 'span' ||
            tag === 'p' || tag === 'div' || tag === 'section') return true;
        if (el.tagName.includes('-')) return true; // custom elements
        return false;
    };

    // ── Label resolution ───────────────────────────────────────────────────────

    const getLabelText = (el) => {
        // 1. Explicit <label for="id">
        if (el.id) {
            const lbl = document.querySelector(`label[for="${CSS.escape(el.id)}"]`);
            if (lbl) return normalize(lbl.innerText || lbl.textContent);
        }
        // 2. Wrapping <label>
        const parent = el.closest('label');
        if (parent) {
            const clone = parent.cloneNode(true);
            for (const child of clone.querySelectorAll('input,select,textarea')) child.remove();
            return normalize(clone.innerText || clone.textContent);
        }
        // 3. aria-labelledby
        const lblIds = el.getAttribute('aria-labelledby');
        if (lblIds) {
            const text = lblIds.split(/\s+/)
                .map(id => document.getElementById(id))
                .filter(Boolean)
                .map(e => normalize(e.innerText || e.textContent))
                .join(' ');
            if (text) return text;
        }
        // 4. Table column header context for <td>
        if (el.tagName === 'TD' || el.tagName === 'TH') {
            const tr = el.closest('tr');
            if (tr) {
                const cells = Array.from(tr.cells || tr.querySelectorAll('td,th'));
                const texts = [];
                for (const c of cells) {
                    if (c === el) continue;
                    const ct = normalize(c.innerText || c.textContent);
                    if (ct.length > 0 && ct.length < 100) texts.push(ct);
                }
                const table = tr.closest('table');
                if (table) {
                    const colIdx = Array.from(tr.cells || tr.children).indexOf(el);
                    if (colIdx >= 0) {
                        const hrow = table.querySelector('thead tr')
                            || (() => {
                                const fr = table.querySelector('tr');
                                return (fr && fr !== tr && fr.querySelector('th')) ? fr : null;
                            })();
                        if (hrow) {
                            const hCells = Array.from(hrow.cells || hrow.children);
                            if (colIdx < hCells.length) {
                                const ht = normalize(hCells[colIdx].innerText || hCells[colIdx].textContent);
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

    // ── Accessible name ────────────────────────────────────────────────────────

    const getAccessibleName = (el) => {
        const ariaLabel = el.getAttribute('aria-label');
        if (ariaLabel) return normalize(ariaLabel);
        const labelText = getLabelText(el);
        if (labelText) return labelText;
        const title = el.getAttribute('title');
        if (title) return normalize(title);
        const alt = el.getAttribute('alt');
        if (alt) return normalize(alt);
        return '';
    };

    // ── XPath builder ──────────────────────────────────────────────────────────

    const buildXPath = (el) => {
        const parts = [];
        let node = el;
        while (node && node.nodeType === Node.ELEMENT_NODE) {
            const tag = node.tagName.toLowerCase();
            let idx = 1;
            let sibling = node.previousElementSibling;
            while (sibling) {
                if (sibling.tagName.toLowerCase() === tag) idx++;
                sibling = sibling.previousElementSibling;
            }
            parts.unshift(`${tag}[${idx}]`);
            node = node.parentElement;
        }
        return '/' + parts.join('/');
    };

    // ── Element ID registry ────────────────────────────────────────────────────

    // Use expando property to avoid mutating the live DOM with attributes.
    if (window.__manulIdCounter === undefined) window.__manulIdCounter = 0;
    const reg = window.__manulReg = window.__manulReg || {};

    // ── Collection ─────────────────────────────────────────────────────────────

    const seen = new Set();
    const elements = [];

    const processElement = (el, inShadow) => {
        if (seen.has(el)) return;
        seen.add(el);
        if (!isInteractive(el)) return;

        const tag = el.tagName.toLowerCase();
        const vis = isVisible(el);

        // Special inputs (checkbox, radio, file) collected even when hidden.
        const isSpecial = tag === 'input' && (el.type === 'checkbox' || el.type === 'radio' || el.type === 'file');
        if (!vis && !isSpecial) return;

        const rect = el.getBoundingClientRect();
        if (!isSpecial && rect.width < 2 && rect.height < 2) return;

        let eid = el.__manulId;
        if (eid === undefined) {
            eid = ++window.__manulIdCounter;
            el.__manulId = eid;
        }
        reg[eid] = el;

        const visibleText = normalize((el.innerText || el.textContent || '').slice(0, 300));
        const labelText   = getLabelText(el);
        const accessibleName = getAccessibleName(el);

        elements.push({
            id:             eid,
            xpath:          buildXPath(el),
            tag:            tag,
            input_type:     el.type || '',
            visible_text:   visibleText,
            aria_label:     el.getAttribute('aria-label') || '',
            placeholder:    el.getAttribute('placeholder') || '',
            title:          el.getAttribute('title') || '',
            data_qa:        el.getAttribute('data-qa') || '',
            data_testid:    el.getAttribute('data-testid') || el.getAttribute('data-test') || '',
            label_text:     labelText,
            name_attr:      el.getAttribute('name') || '',
            html_id:        el.id || '',
            class_name:     (typeof el.className === 'string' ? el.className : '') || '',
            role:           el.getAttribute('role') || '',
            value:          (el.value !== undefined ? el.value : '') || '',
            accessible_name: accessibleName,
            is_visible:     vis,
            is_disabled:    el.disabled || el.getAttribute('aria-disabled') === 'true',
            is_hidden:      !vis,
            is_editable:    !el.disabled && !el.readOnly &&
                            (tag === 'input' || tag === 'textarea' ||
                             el.getAttribute('contenteditable') === 'true' ||
                             (el.getAttribute('role') || '') === 'textbox'),
            is_checked:     el.checked || el.getAttribute('aria-checked') === 'true',
            is_in_shadow:   inShadow,
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

    // ── TreeWalker DOM traversal (with shadow DOM support) ─────────────────────

    const walk = (root, inShadow) => {
        const tw = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT, {
            acceptNode(n) {
                if (PRUNE.has(n.tagName)) return NodeFilter.FILTER_REJECT;
                return NodeFilter.FILTER_ACCEPT;
            }
        });
        let n;
        while ((n = tw.nextNode())) {
            // Pierce shadow roots — handles Web Components.
            if (n.shadowRoot) walk(n.shadowRoot, true);
            processElement(n, inShadow);
        }
    };
    walk(document.body || document.documentElement, false);

    // ── Full visible page text (for VERIFY) ────────────────────────────────────

    const pageText = (() => {
        let t = (document.body ? document.body.innerText : '') + ' ';
        document.querySelectorAll('[aria-label],[placeholder],[title],[aria-valuetext],input,textarea,select').forEach(el => {
            const cs = window.getComputedStyle(el);
            if (cs.display === 'none' || cs.visibility === 'hidden') return;
            t += ' ' + (el.getAttribute('aria-label') || '');
            t += ' ' + (el.getAttribute('placeholder') || '');
            t += ' ' + (el.getAttribute('title') || '');
            t += ' ' + (el.getAttribute('aria-valuetext') || '');
            if (typeof el.value === 'string') t += ' ' + el.value;
        });
        return t.toLowerCase();
    })();

    return {
        url:         window.location.href,
        title:       document.title || '',
        visible_text: pageText,
        elements:    elements
    };
})()

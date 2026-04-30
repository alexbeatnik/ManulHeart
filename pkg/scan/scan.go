// Package scan implements the `manul scan <URL>` subcommand for ManulHeart.
//
// It opens a URL in Chrome, runs a DOM scanner JS, and writes a draft .hunt file.
package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manulengineer/manulheart/pkg/browser"
)

// SCAN_JS is the JavaScript payload executed in the browser to discover
// interactive elements. Mirrors ManulEngine's SCAN_JS.
const SCAN_JS = `() => {
    function isHidden(el) {
        if (el.getAttribute('aria-hidden') === 'true') return true;
        const r = el.getBoundingClientRect();
        if (r.width === 0 && r.height === 0) return true;
        try {
            const st = window.getComputedStyle(el);
            if (st.display === 'none' || st.visibility === 'hidden' || parseFloat(st.opacity) === 0) return true;
        } catch (_) {}
        return false;
    }
    function bestLabel(el) {
        const tag  = el.tagName ? el.tagName.toUpperCase() : '';
        const type = (el.getAttribute('type') || '').toLowerCase();
        if (tag === 'INPUT' && (type === 'radio' || type === 'checkbox')) {
            if (el.id) {
                const root = el.getRootNode();
                const lbl = root.querySelector('label[for="' + CSS.escape(el.id) + '"]');
                if (lbl) return lbl.innerText.trim();
            }
            const closestLbl = el.closest('label');
            if (closestLbl) return closestLbl.innerText.trim();
            const nextSib = el.nextElementSibling;
            if (nextSib && nextSib.tagName === 'LABEL') return nextSib.innerText.trim();
        }
        const text = (el.innerText || el.textContent || '').replace(/\s+/g, ' ').trim();
        if (text && text.length <= 80) return text;
        const aria = el.getAttribute('aria-label') || '';
        if (aria.trim()) return aria.trim();
        const ph = el.getAttribute('placeholder') || '';
        if (ph.trim()) return ph.trim();
        const title = el.getAttribute('title') || '';
        if (title.trim()) return title.trim();
        const name = el.getAttribute('name') || '';
        if (name.trim()) return name.trim();
        const id = el.getAttribute('id') || '';
        if (id.trim()) return id.trim();
        return '';
    }
    function classify(el) {
        const tag = el.tagName ? el.tagName.toUpperCase() : '';
        const type = (el.getAttribute('type') || '').toLowerCase();
        const role = (el.getAttribute('role') || '').toLowerCase();
        if (tag === 'SELECT') return 'select';
        if (tag === 'INPUT' && type === 'checkbox') return 'checkbox';
        if (tag === 'INPUT' && type === 'radio') return 'radio';
        if (tag === 'INPUT' && !['submit', 'reset', 'image', 'hidden', 'button'].includes(type)) return 'input';
        if (tag === 'TEXTAREA') return 'input';
        if (tag === 'BUTTON') return 'button';
        if (tag === 'A' && el.getAttribute('href') !== null) return 'link';
        if (role === 'button') return 'button';
        if (role === 'link') return 'link';
        if (role === 'checkbox') return 'checkbox';
        if (role === 'radio') return 'radio';
        if (role === 'combobox') return 'select';
        if (role === 'switch') return 'checkbox';
        if (tag === 'INPUT' && type === 'submit') return 'button';
        if (tag === 'INPUT' && type === 'button') return 'button';
        return null;
    }
    function scanRoot(root, results, seen) {
        const candidates = root.querySelectorAll(
            'button, a[href], input, select, textarea, ' +
            '[role="button"], [role="link"], [role="checkbox"], [role="radio"], ' +
            '[role="combobox"], [role="switch"]'
        );
        for (const el of candidates) {
            if (seen.has(el)) continue;
            seen.add(el);
            if (isHidden(el)) continue;
            const kind = classify(el);
            if (!kind) continue;
            const label = bestLabel(el);
            if (!label) continue;
            const entry = { type: kind, identifier: label };
            if ((kind === 'input' || kind === 'select') && el.value !== undefined && el.value !== '') {
                entry.value = el.value;
            }
            results.push(entry);
            if (el.shadowRoot) scanRoot(el.shadowRoot, results, seen);
        }
    }
    const results = [];
    const seen = new Set();
    scanRoot(document, results, seen);
    return JSON.stringify(results);
}`

var skipLabels = map[string]bool{
	"": true, "click": true, "button": true, "submit": true, "link": true,
	"go": true, "close": true, "×": true, "✕": true, "✖": true,
	"menu": true, "toggle": true, "show": true, "hide": true,
}

// Element represents a scanned interactive element.
type Element struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
	Value      string `json:"value,omitempty"`
}

func isUseful(identifier, kind string) bool {
	label := strings.TrimSpace(strings.ToLower(identifier))
	if label == "" || skipLabels[label] {
		return false
	}
	if len(label) > 80 {
		return false
	}
	if strings.HasPrefix(label, "http://") || strings.HasPrefix(label, "https://") {
		return false
	}
	return true
}

func mapToStep(kind, identifier string) string {
	i := strings.TrimSpace(identifier)
	switch kind {
	case "input":
		return fmt.Sprintf("Fill '%s' with ''", i)
	case "select":
		return fmt.Sprintf("Select 'Option' from the '%s' dropdown", i)
	case "checkbox":
		return fmt.Sprintf("Check the checkbox for '%s'", i)
	case "radio":
		return fmt.Sprintf("Click the radio button for '%s'", i)
	case "link":
		return fmt.Sprintf("Click the '%s' link", i)
	default:
		return fmt.Sprintf("Click the '%s' button", i)
	}
}

// BuildHunt generates a draft .hunt file from a URL and scanned elements.
func BuildHunt(url string, elements []Element) string {
	lines := []string{
		fmt.Sprintf("@context: Auto-generated scan for %s", url),
		"@title: scan-draft",
		"",
		fmt.Sprintf("STEP 1:\n    NAVIGATE to %s", url),
		"",
		"STEP 2:\n    WAIT 2",
		"",
	}

	step := 3
	seen := make(map[string]bool)

	for _, el := range elements {
		if !isUseful(el.Identifier, el.Type) {
			continue
		}
		key := el.Type + "|" + strings.ToLower(el.Identifier)
		if seen[key] {
			continue
		}
		seen[key] = true

		action := mapToStep(el.Type, el.Identifier)
		lines = append(lines, fmt.Sprintf("STEP %d:\n    %s", step, action))
		lines = append(lines, "")
		step++
	}

	lines = append(lines, "DONE.")
	return strings.Join(lines, "\n") + "\n"
}

// ScanPage opens url in a headless Chrome, runs the DOM scanner, and returns
// the scanned elements.
func ScanPage(ctx context.Context, url string, headless bool) ([]Element, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	opts := browser.DefaultChromeOptions()
	opts.Headless = headless
	chrome, err := browser.LaunchChrome(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("launch chrome: %w", err)
	}
	defer chrome.Close()

	b := browser.NewCDPBrowser(chrome.Endpoint())
	page, err := b.FirstPage(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to page: %w", err)
	}
	defer page.Close()

	if err := page.Navigate(ctx, url); err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}
	_ = page.Wait(ctx, 2*time.Second)
	_ = page.WaitForLoad(ctx)

	raw, err := page.EvalJS(ctx, SCAN_JS)
	if err != nil {
		return nil, fmt.Errorf("scan js: %w", err)
	}

	var elements []Element
	if err := json.Unmarshal(raw, &elements); err != nil {
		// SCAN_JS returns JSON string, try unmarshal again
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			_ = json.Unmarshal([]byte(s), &elements)
		}
	}
	return elements, nil
}

// Run is the entry point for `manul scan <URL>`.
func Run(ctx context.Context, url, outputFile string, headless bool) error {
	fmt.Printf("\n🔍 Manul Scanner — scanning %s\n", url)
	fmt.Printf("   Headless: %v\n", headless)

	elements, err := ScanPage(ctx, url, headless)
	if err != nil {
		return err
	}
	fmt.Printf("   Found %d interactive element(s) before dedup/filter.\n", len(elements))

	huntText := BuildHunt(url, elements)

	absOut, _ := filepath.Abs(outputFile)
	_ = os.MkdirAll(filepath.Dir(absOut), 0755)
	if err := os.WriteFile(absOut, []byte(huntText), 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Printf("\n✅ Draft saved → %s\n", absOut)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println(huntText)
	fmt.Println(strings.Repeat("─", 60))
	return nil
}

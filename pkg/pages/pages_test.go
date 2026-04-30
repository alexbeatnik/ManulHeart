package pages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistry_LookupExactMatch(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{
    "site": "https://example.com/",
    "Domain": "Example",
    "https://example.com/login": "Login Page",
    "https://example.com/dashboard": "Dashboard"
}`)
	_ = os.WriteFile(filepath.Join(dir, "example.com.json"), data, 0644)

	r := NewRegistry(dir)

	if got := r.LookupPageName("https://example.com/login"); got != "Login Page" {
		t.Errorf("login = %q, want Login Page", got)
	}
	if got := r.LookupPageName("https://example.com/dashboard"); got != "Dashboard" {
		t.Errorf("dashboard = %q, want Dashboard", got)
	}
}

func TestRegistry_LookupRegexMatch(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{
    "site": "https://shop.com/",
    "Domain": "Shop",
    ".*/products/\\d+": "Product Detail Page"
}`)
	_ = os.WriteFile(filepath.Join(dir, "shop.com.json"), data, 0644)

	r := NewRegistry(dir)

	got := r.LookupPageName("https://shop.com/products/42")
	if got != "Product Detail Page" {
		t.Errorf("regex match = %q, want Product Detail Page", got)
	}
}

func TestRegistry_LookupDomainFallback(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{
    "site": "https://example.com/",
    "Domain": "Example Domain"
}`)
	_ = os.WriteFile(filepath.Join(dir, "example.com.json"), data, 0644)

	r := NewRegistry(dir)

	got := r.LookupPageName("https://example.com/unknown-page")
	if got != "Example Domain" {
		t.Errorf("domain fallback = %q, want Example Domain", got)
	}
}

func TestRegistry_AutoPopulate(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	got := r.LookupPageName("https://brand-new-site.io/dashboard")
	if !strings.HasPrefix(got, "Auto:") {
		t.Errorf("expected Auto: prefix, got %q", got)
	}

	// File should have been created.
	frag := filepath.Join(dir, "brand-new-site.io.json")
	if _, err := os.Stat(frag); os.IsNotExist(err) {
		t.Fatalf("auto-populate did not create fragment %s", frag)
	}

	// Second lookup should return the same placeholder without creating duplicates.
	got2 := r.LookupPageName("https://brand-new-site.io/dashboard")
	if got2 != got {
		t.Errorf("second lookup = %q, want %q", got2, got)
	}
}

func TestRegistry_WrappedForm(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{
    "https://wrapped.com/": {
        "Domain": "Wrapped",
        "/admin": "Admin Page"
    }
}`)
	_ = os.WriteFile(filepath.Join(dir, "wrapped.com.json"), data, 0644)

	r := NewRegistry(dir)
	if got := r.LookupPageName("https://wrapped.com/admin"); got != "Admin Page" {
		t.Errorf("wrapped form = %q, want Admin Page", got)
	}
}

func TestRegistry_LongestPrefixWins(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "example.com.json"), []byte(`{
    "site": "https://example.com/",
    "Domain": "Root"
}`), 0644)
	_ = os.WriteFile(filepath.Join(dir, "app.example.com.json"), []byte(`{
    "site": "https://app.example.com/",
    "Domain": "App"
}`), 0644)

	r := NewRegistry(dir)
	if got := r.LookupPageName("https://app.example.com/dashboard"); got != "App" {
		t.Errorf("longest prefix = %q, want App", got)
	}
	if got := r.LookupPageName("https://example.com/about"); got != "Root" {
		t.Errorf("root prefix = %q, want Root", got)
	}
}

func TestSafeSiteFilename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://www.saucedemo.com/", "www.saucedemo.com.json"},
		{"https://example.com/", "example.com.json"},
		{"https://sub.domain.co.uk/", "sub.domain.co.uk.json"},
	}
	for _, tc := range tests {
		if got := safeSiteFilename(tc.in); got != tc.want {
			t.Errorf("safeSiteFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBelongsToSite(t *testing.T) {
	tests := []struct {
		url  string
		site string
		want bool
	}{
		{"https://example.com/login", "https://example.com/", true},
		{"https://example.com/login", "https://other.com/", false},
		{"https://example.com/api/v1", "https://example.com/api/", true},
		{"https://example.com/", "https://example.com/api/", false},
	}
	for _, tc := range tests {
		if got := belongsToSite(tc.url, tc.site); got != tc.want {
			t.Errorf("belongsToSite(%q, %q) = %v, want %v", tc.url, tc.site, got, tc.want)
		}
	}
}

func TestNormalizeFragment_Lean(t *testing.T) {
	raw := map[string]any{
		"site":    "https://example.com/",
		"Domain":  "Example",
		"/login":  "Login",
	}
	got := normalizeFragment(raw)
	if len(got) != 1 {
		t.Fatalf("expected 1 site, got %d", len(got))
	}
	if got["https://example.com/"]["/login"] != "Login" {
		t.Errorf("login = %q", got["https://example.com/"]["/login"])
	}
}

func TestNormalizeFragment_Wrapped(t *testing.T) {
	raw := map[string]any{
		"https://example.com/": map[string]any{
			"Domain": "Example",
			"/login": "Login",
		},
	}
	got := normalizeFragment(raw)
	if len(got) != 1 {
		t.Fatalf("expected 1 site, got %d", len(got))
	}
	if got["https://example.com/"]["/login"] != "Login" {
		t.Errorf("login = %q", got["https://example.com/"]["/login"])
	}
}

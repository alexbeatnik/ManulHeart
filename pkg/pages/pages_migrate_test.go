package pages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyJSON_Lean(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "pages.json")
	_ = os.WriteFile(legacy, []byte(`{
    "site": "https://example.com/",
    "Domain": "Example",
    "/login": "Login Page",
    "/dashboard": "Dashboard"
}`), 0644)

	outDir := filepath.Join(dir, "pages")
	if err := MigrateLegacyJSON(legacy, outDir); err != nil {
		t.Fatalf("MigrateLegacyJSON failed: %v", err)
	}

	frag := filepath.Join(outDir, "example.com.json")
	data, err := os.ReadFile(frag)
	if err != nil {
		t.Fatalf("fragment not created: %v", err)
	}
	if !contains(string(data), "Login Page") {
		t.Fatal("fragment missing Login Page")
	}
	if !contains(string(data), "Dashboard") {
		t.Fatal("fragment missing Dashboard")
	}
}

func TestMigrateLegacyJSON_Wrapped(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "pages.json")
	_ = os.WriteFile(legacy, []byte(`{
    "https://shop.com/": {
        "Domain": "Shop",
        "/products": "Products"
    }
}`), 0644)

	outDir := filepath.Join(dir, "pages")
	if err := MigrateLegacyJSON(legacy, outDir); err != nil {
		t.Fatalf("MigrateLegacyJSON failed: %v", err)
	}

	frag := filepath.Join(outDir, "shop.com.json")
	data, err := os.ReadFile(frag)
	if err != nil {
		t.Fatalf("fragment not created: %v", err)
	}
	if !contains(string(data), "Products") {
		t.Fatal("fragment missing Products")
	}
}

func TestMigrateLegacyJSON_MultipleSites(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "pages.json")
	_ = os.WriteFile(legacy, []byte(`{
    "https://example.com/": {
        "Domain": "Example",
        "/login": "Login"
    },
    "https://app.example.com/": {
        "Domain": "App",
        "/dashboard": "Dashboard"
    }
}`), 0644)

	outDir := filepath.Join(dir, "pages")
	if err := MigrateLegacyJSON(legacy, outDir); err != nil {
		t.Fatalf("MigrateLegacyJSON failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "example.com.json")); err != nil {
		t.Fatalf("example.com.json not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "app.example.com.json")); err != nil {
		t.Fatalf("app.example.com.json not created: %v", err)
	}
}

func TestMigrateLegacyJSON_MissingFile(t *testing.T) {
	err := MigrateLegacyJSON("/nonexistent/pages.json", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMigrateLegacyJSON_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(legacy, []byte("not json"), 0644)

	err := MigrateLegacyJSON(legacy, dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMigrateLegacyJSON_EmptyObject(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "empty.json")
	_ = os.WriteFile(legacy, []byte("{}"), 0644)

	err := MigrateLegacyJSON(legacy, dir)
	if err == nil {
		t.Fatal("expected error for empty object (no site blocks)")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

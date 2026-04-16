package dsl

// ─────────────────────────────────────────────────────────────────────────────
// TAGS PARSING TEST SUITE
//
// Port of ManulEngine test_22_tags.py — @tags: header parsing and filtering.
//
// Validates:
// 1. Parser extracts @tags: into Hunt.Tags
// 2. Comma-separated tags are trimmed
// 3. Single tag (no comma) works
// 4. Empty @tags: produces nil/empty
// 5. Tags coexist with other headers (@context:, @title:)
// 6. Tag filtering via set intersection (OR logic)
// ─────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"testing"
)

// parseTags is a helper that parses a hunt string and returns the Tags field.
func parseTags(t *testing.T, content string) []string {
	t.Helper()
	h, err := Parse(strings.NewReader(content))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return h.Tags
}

// ── Section 1: Parser (@tags: extraction) ────────────────────────────────────

func TestTags_BasicCommaSeparated(t *testing.T) {
	tags := parseTags(t, "@context: Login flow\n@title: auth\n@tags: smoke, auth, regression\n\nNAVIGATE to 'https://example.com'\nDONE.\n")
	want := []string{"smoke", "auth", "regression"}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %d: %v", len(want), len(tags), tags)
	}
	for i, w := range want {
		if tags[i] != w {
			t.Errorf("tag[%d] = %q, want %q", i, tags[i], w)
		}
	}
}

func TestTags_NotInCommands(t *testing.T) {
	h, err := Parse(strings.NewReader("@tags: smoke\nNAVIGATE to 'https://example.com'\nDONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	for _, cmd := range h.Commands {
		if strings.Contains(cmd.Raw, "@tags:") {
			t.Error("@tags: should not appear as a command")
		}
	}
}

func TestTags_CoexistsWithOtherHeaders(t *testing.T) {
	h, err := Parse(strings.NewReader("@context: Login flow\n@title: auth\n@tags: smoke, auth, regression\nDONE.\n"))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if h.Context != "Login flow" {
		t.Errorf("context = %q, want %q", h.Context, "Login flow")
	}
	if h.Title != "auth" {
		t.Errorf("title = %q, want %q", h.Title, "auth")
	}
}

func TestTags_MissingTagsReturnsEmpty(t *testing.T) {
	tags := parseTags(t, "@context: No tags here\nDONE.\n")
	if len(tags) != 0 {
		t.Errorf("expected empty tags, got %v", tags)
	}
}

func TestTags_WhitespaceStripped(t *testing.T) {
	tags := parseTags(t, "@tags:  critical ,  slow ,  nightly  \nDONE.\n")
	want := []string{"critical", "slow", "nightly"}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %d: %v", len(want), len(tags), tags)
	}
	for i, w := range want {
		if tags[i] != w {
			t.Errorf("tag[%d] = %q, want %q", i, tags[i], w)
		}
	}
}

func TestTags_SingleTag(t *testing.T) {
	tags := parseTags(t, "@tags: smoke\nDONE.\n")
	if len(tags) != 1 || tags[0] != "smoke" {
		t.Errorf("expected [smoke], got %v", tags)
	}
}

func TestTags_EmptyValueReturnsEmpty(t *testing.T) {
	tags := parseTags(t, "@tags:\nDONE.\n")
	if len(tags) != 0 {
		t.Errorf("expected empty tags for empty @tags:, got %v", tags)
	}
}

// ── Section 2: Tag filtering (intersection rule) ─────────────────────────────

// filterByTags mimics the CLI tag filter: a hunt matches if it shares at least
// one tag with the requested set (OR logic / set intersection).
func filterByTags(hunts map[string][]string, requested map[string]bool) []string {
	var matched []string
	for name, tags := range hunts {
		for _, t := range tags {
			if requested[t] {
				matched = append(matched, name)
				break
			}
		}
	}
	return matched
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func TestTagFilter_SmokeOnly(t *testing.T) {
	hunts := map[string][]string{
		"smoke_auth":      {"smoke", "auth"},
		"regression_only": {"regression"},
		"no_tags":         {},
		"combo":           {"smoke", "regression"},
	}
	result := filterByTags(hunts, map[string]bool{"smoke": true})
	if !contains(result, "smoke_auth") || !contains(result, "combo") {
		t.Errorf("expected smoke_auth and combo, got %v", result)
	}
	if contains(result, "regression_only") || contains(result, "no_tags") {
		t.Errorf("regression_only and no_tags should be excluded, got %v", result)
	}
}

func TestTagFilter_RegressionOnly(t *testing.T) {
	hunts := map[string][]string{
		"smoke_auth":      {"smoke", "auth"},
		"regression_only": {"regression"},
		"no_tags":         {},
		"combo":           {"smoke", "regression"},
	}
	result := filterByTags(hunts, map[string]bool{"regression": true})
	if !contains(result, "regression_only") || !contains(result, "combo") {
		t.Errorf("expected regression_only and combo, got %v", result)
	}
	if contains(result, "smoke_auth") {
		t.Errorf("smoke_auth should be excluded, got %v", result)
	}
}

func TestTagFilter_MultipleTagsOR(t *testing.T) {
	hunts := map[string][]string{
		"smoke_auth":      {"smoke", "auth"},
		"regression_only": {"regression"},
		"no_tags":         {},
		"combo":           {"smoke", "regression"},
	}
	result := filterByTags(hunts, map[string]bool{"smoke": true, "regression": true})
	if !contains(result, "smoke_auth") || !contains(result, "regression_only") || !contains(result, "combo") {
		t.Errorf("expected all tagged files, got %v", result)
	}
	if contains(result, "no_tags") {
		t.Errorf("no_tags should be excluded, got %v", result)
	}
}

func TestTagFilter_NonexistentTag(t *testing.T) {
	hunts := map[string][]string{
		"smoke_auth": {"smoke", "auth"},
		"combo":      {"smoke", "regression"},
	}
	result := filterByTags(hunts, map[string]bool{"nonexistent": true})
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestTagFilter_UntaggedExcluded(t *testing.T) {
	hunts := map[string][]string{
		"no_tags": {},
	}
	result := filterByTags(hunts, map[string]bool{"smoke": true})
	if len(result) != 0 {
		t.Errorf("untagged file should be excluded, got %v", result)
	}
}

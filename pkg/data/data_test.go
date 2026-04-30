package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile_JSON(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "test.json"), []byte(`[
		{"username": "alice", "password": "secret1"},
		{"username": "bob", "password": "secret2"}
	]`), 0644)

	rows, err := LoadFile("test.json", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["username"] != "alice" {
		t.Errorf("row[0].username = %q", rows[0]["username"])
	}
	if rows[1]["password"] != "secret2" {
		t.Errorf("row[1].password = %q", rows[1]["password"])
	}
}

func TestLoadFile_CSV(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "users.csv"), []byte("username,password\nalice,secret1\nbob,secret2\n"), 0644)

	rows, err := LoadFile("users.csv", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["username"] != "alice" {
		t.Errorf("row[0].username = %q", rows[0]["username"])
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("missing.json", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_UnsupportedType(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "data.xml"), []byte("<root/>"), 0644)
	_, err := LoadFile("data.xml", dir)
	if err == nil {
		t.Fatal("expected error for unsupported file type")
	}
}

func TestLoadFile_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "empty.json"), []byte("[]"), 0644)
	rows, err := LoadFile("empty.json", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for empty JSON array, got %d", len(rows))
	}
}

func TestLoadFile_EmptyCSV(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "empty.csv"), []byte("username,password\n"), 0644)
	rows, err := LoadFile("empty.csv", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for CSV with only header, got %d", len(rows))
	}
}

func TestLoadFile_CSVWithSpaces(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "spaces.csv"), []byte(" username , password \n alice , secret1 \n"), 0644)
	rows, err := LoadFile("spaces.csv", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["username"] != "alice" {
		t.Errorf("username = %q, want alice", rows[0]["username"])
	}
	if rows[0]["password"] != "secret1" {
		t.Errorf("password = %q, want secret1", rows[0]["password"])
	}
}

func TestLoadFile_JSONObjectInsteadOfArray(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "object.json"), []byte(`{"username": "alice", "password": "secret1"}`), 0644)
	_, err := LoadFile("object.json", dir)
	if err == nil {
		t.Fatal("expected error for JSON object instead of array")
	}
}

func TestLoadFile_CRLFLineEndings(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "crlf.csv"), []byte("username,password\r\nalice,secret1\r\nbob,secret2\r\n"), 0644)
	rows, err := LoadFile("crlf.csv", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["username"] != "alice" {
		t.Errorf("row[0].username = %q", rows[0]["username"])
	}
}

func TestLoadFile_SingleRowCSV(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "single.csv"), []byte("username,password\nalice,secret1\n"), 0644)
	rows, err := LoadFile("single.csv", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["username"] != "alice" {
		t.Errorf("row[0].username = %q", rows[0]["username"])
	}
}

func TestLoadFile_JSONWithNulls(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "nulls.json"), []byte(`[
		{"username": "alice", "password": null},
		{"username": null, "password": "secret2"}
	]`), 0644)
	rows, err := LoadFile("nulls.json", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["password"] != "" {
		t.Errorf("null value should become empty string, got %q", rows[0]["password"])
	}
	if rows[1]["username"] != "" {
		t.Errorf("null value should become empty string, got %q", rows[1]["username"])
	}
}

func TestLoadFile_JSONWithNumbers(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "numbers.json"), []byte(`[
		{"id": 42, "name": "test"}
	]`), 0644)
	rows, err := LoadFile("numbers.json", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["id"] != "42" {
		t.Errorf("number should become string, got %q", rows[0]["id"])
	}
}

func TestLoadFile_CSVWithEmptyFields(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "emptyfields.csv"), []byte("a,b,c\n1,,3\n"), 0644)
	rows, err := LoadFile("emptyfields.csv", dir)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0]["a"] != "1" || rows[0]["b"] != "" || rows[0]["c"] != "3" {
		t.Errorf("unexpected values: %v", rows[0])
	}
}

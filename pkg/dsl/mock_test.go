package dsl

import (
	"testing"
)

func TestParseMock_GET(t *testing.T) {
	cmd := parseOne(t, `MOCK GET "/api/users" with 'mocks/users.json'`)
	if cmd.Type != CmdMock {
		t.Fatalf("expected CmdMock, got %v", cmd.Type)
	}
	if cmd.MockMethod != "GET" {
		t.Errorf("MockMethod = %q, want GET", cmd.MockMethod)
	}
	if cmd.MockPattern != "/api/users" {
		t.Errorf("MockPattern = %q, want /api/users", cmd.MockPattern)
	}
	if cmd.MockFile != "mocks/users.json" {
		t.Errorf("MockFile = %q, want mocks/users.json", cmd.MockFile)
	}
}

func TestParseMock_POST(t *testing.T) {
	cmd := parseOne(t, `MOCK POST "/api/login" with 'mocks/login.json'`)
	if cmd.Type != CmdMock {
		t.Fatalf("expected CmdMock, got %v", cmd.Type)
	}
	if cmd.MockMethod != "POST" {
		t.Errorf("MockMethod = %q, want POST", cmd.MockMethod)
	}
	if cmd.MockPattern != "/api/login" {
		t.Errorf("MockPattern = %q, want /api/login", cmd.MockPattern)
	}
}

func TestParseMock_PUT(t *testing.T) {
	cmd := parseOne(t, `MOCK PUT "/api/users/1" with 'mocks/update.json'`)
	if cmd.MockMethod != "PUT" {
		t.Errorf("MockMethod = %q, want PUT", cmd.MockMethod)
	}
}

func TestParseMock_PATCH(t *testing.T) {
	cmd := parseOne(t, `MOCK PATCH "/api/users/1" with 'mocks/patch.json'`)
	if cmd.MockMethod != "PATCH" {
		t.Errorf("MockMethod = %q, want PATCH", cmd.MockMethod)
	}
}

func TestParseMock_DELETE(t *testing.T) {
	cmd := parseOne(t, `MOCK DELETE "/api/users/1" with 'mocks/delete.json'`)
	if cmd.MockMethod != "DELETE" {
		t.Errorf("MockMethod = %q, want DELETE", cmd.MockMethod)
	}
}

func TestParseMock_NoFile(t *testing.T) {
	cmd := parseOne(t, `MOCK GET "/api/users"`)
	if cmd.Type != CmdMock {
		t.Fatalf("expected CmdMock, got %v", cmd.Type)
	}
	if cmd.MockMethod != "GET" {
		t.Errorf("MockMethod = %q, want GET", cmd.MockMethod)
	}
	if cmd.MockPattern != "/api/users" {
		t.Errorf("MockPattern = %q, want /api/users", cmd.MockPattern)
	}
	if cmd.MockFile != "" {
		t.Errorf("MockFile = %q, want empty", cmd.MockFile)
	}
}

func TestParseMock_InvalidMethod(t *testing.T) {
	cmd, err := parseCommand("MOCK TRACE /api/users", 1)
	if err != nil {
		// Parser may or may not error; either way it should not be a valid mock
		t.Skip("parser rejected invalid method")
	}
	if cmd.Type == CmdMock && cmd.MockMethod != "" {
		t.Fatal("expected invalid method to be rejected")
	}
}

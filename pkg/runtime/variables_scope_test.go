package runtime

import (
	"testing"
)

func TestScopedVariables_Precedence(t *testing.T) {
	sv := NewScopedVariables()
	sv.Set("email", "global@test.com", LevelGlobal)
	sv.Set("email", "mission@test.com", LevelMission)
	sv.Set("email", "step@test.com", LevelStep)
	sv.Set("email", "row@test.com", LevelRow)

	if val, _ := sv.Resolve("email"); val != "row@test.com" {
		t.Errorf("expected row@test.com, got %s", val)
	}

	sv.ClearLevel(LevelRow)
	if val, _ := sv.Resolve("email"); val != "step@test.com" {
		t.Errorf("expected step@test.com, got %s", val)
	}

	sv.ClearLevel(LevelStep)
	if val, _ := sv.Resolve("email"); val != "mission@test.com" {
		t.Errorf("expected mission@test.com, got %s", val)
	}

	sv.ClearLevel(LevelMission)
	if val, _ := sv.Resolve("email"); val != "global@test.com" {
		t.Errorf("expected global@test.com, got %s", val)
	}

	sv.ClearLevel(LevelGlobal)
	if _, ok := sv.Resolve("email"); ok {
		t.Error("expected variable to be missing")
	}
}

func TestScopedVariables_ResolveLevel(t *testing.T) {
	sv := NewScopedVariables()
	sv.Set("x", "1", LevelGlobal)
	sv.Set("x", "2", LevelStep)

	val, level, ok := sv.ResolveLevel("x")
	if !ok {
		t.Fatal("expected x to be resolved")
	}
	if val != "2" {
		t.Errorf("expected 2, got %s", val)
	}
	if level != LevelStep {
		t.Errorf("expected level STEP (2), got %v", level)
	}

	_, _, ok = sv.ResolveLevel("nonexistent")
	if ok {
		t.Error("expected nonexistent to return false")
	}
}

func TestScopedVariables_Flatten(t *testing.T) {
	sv := NewScopedVariables()
	sv.Set("a", "global_a", LevelGlobal)
	sv.Set("b", "mission_b", LevelMission)
	sv.Set("a", "step_a", LevelStep)

	flat := sv.Flatten()
	if flat["a"] != "step_a" {
		t.Errorf("expected step_a for 'a', got %s", flat["a"])
	}
	if flat["b"] != "mission_b" {
		t.Errorf("expected mission_b for 'b', got %s", flat["b"])
	}
}

func TestScopedVariables_Interpolate(t *testing.T) {
	sv := NewScopedVariables()
	sv.Set("project", "Manul", LevelGlobal)
	sv.Set("user", "Alex", LevelStep)

	input := "Hello $user, welcome to $project or ${project}!"
	output := sv.Interpolate(input)

	expected := "Hello Alex, welcome to Manul or Manul!"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

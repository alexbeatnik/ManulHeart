package runtime

import (
	"fmt"
	"strings"
)

// Level represents the scope level of a variable.
type Level int

const (
	LevelRow     Level = 1
	LevelStep    Level = 2
	LevelMission Level = 3
	LevelGlobal  Level = 4
)

// ScopedVariables manages variables across multiple precedence levels.
type ScopedVariables struct {
	levels map[Level]map[string]string
}

// NewScopedVariables initializes a new variable scope manager.
func NewScopedVariables() *ScopedVariables {
	return &ScopedVariables{
		levels: map[Level]map[string]string{
			LevelRow:     make(map[string]string),
			LevelStep:    make(map[string]string),
			LevelMission: make(map[string]string),
			LevelGlobal:  make(map[string]string),
		},
	}
}

// Set stores a value at the specified precedence level.
func (sv *ScopedVariables) Set(name, value string, level Level) {
	if sv.levels[level] == nil {
		sv.levels[level] = make(map[string]string)
	}
	sv.levels[level][name] = value
}

// Resolve returns the value of a variable, respecting precedence (Row > Step > Mission > Global).
func (sv *ScopedVariables) Resolve(name string) (string, bool) {
	// Priority order: 1, 2, 3, 4
	for i := Level(1); i <= 4; i++ {
		if val, ok := sv.levels[i][name]; ok {
			return val, true
		}
	}
	return "", false
}

// ResolveLevel returns the value and the level it was found at.
func (sv *ScopedVariables) ResolveLevel(name string) (string, Level, bool) {
	for i := Level(1); i <= 4; i++ {
		if val, ok := sv.levels[i][name]; ok {
			return val, i, true
		}
	}
	return "", 0, false
}

// ClearLevel wipes all variables at a specific level.
func (sv *ScopedVariables) ClearLevel(level Level) {
	sv.levels[level] = make(map[string]string)
}

// ClearAll wipes all variables across all levels.
func (sv *ScopedVariables) ClearAll() {
	for i := Level(1); i <= 4; i++ {
		sv.ClearLevel(i)
	}
}

// Flatten merges all levels into a single map, following precedence rules.
func (sv *ScopedVariables) Flatten() map[string]string {
	flat := make(map[string]string)
	// Iterate backwards (Global to Row) so higher priority overwrites.
	for i := Level(4); i >= 1; i-- {
		for k, v := range sv.levels[i] {
			flat[k] = v
		}
	}
	return flat
}

// Interpolate replaces $var, ${var}, or {var} placeholders in a string.
func (sv *ScopedVariables) Interpolate(s string) string {
	flat := sv.Flatten()
	for k, v := range flat {
		s = strings.ReplaceAll(s, "$"+k, v)
		s = strings.ReplaceAll(s, "${"+k+"}", v)
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// String returns a debug representation of all variables.
func (sv *ScopedVariables) String() string {
	var sb strings.Builder
	sb.WriteString("DEBUG VARS:\n")
	for i := Level(1); i <= 4; i++ {
		lvlName := ""
		switch i {
		case LevelRow: lvlName = "ROW"
		case LevelStep: lvlName = "STEP"
		case LevelMission: lvlName = "MISSION"
		case LevelGlobal: lvlName = "GLOBAL"
		}
		sb.WriteString(fmt.Sprintf("  [%s]:\n", lvlName))
		for k, v := range sv.levels[i] {
			sb.WriteString(fmt.Sprintf("    %s = %s\n", k, v))
		}
	}
	return sb.String()
}

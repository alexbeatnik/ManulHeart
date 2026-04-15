package dsl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveImports loads and resolves all @import directives for a Hunt.
// It reads the imported .hunt files relative to the source file's directory,
// and merges imported STEP blocks (or all commands for wildcard imports)
// into the Hunt's available blueprints.
//
// Blueprint blocks are stored in hunt.Blueprints keyed by their STEP name.
// If aliases are specified, the alias name is used as the key.
func ResolveImports(hunt *Hunt) error {
	if len(hunt.Imports) == 0 {
		return nil
	}

	if hunt.Blueprints == nil {
		hunt.Blueprints = make(map[string][]Command)
	}

	baseDir := filepath.Dir(hunt.SourcePath)
	if baseDir == "" {
		baseDir = "."
	}

	for _, imp := range hunt.Imports {
		importPath := filepath.Join(baseDir, imp.Source)
		if _, err := os.Stat(importPath); err != nil {
			return fmt.Errorf("import %q: file not found: %s", imp.Source, importPath)
		}

		imported, err := ParseFile(importPath)
		if err != nil {
			return fmt.Errorf("import %q: %w", imp.Source, err)
		}

		// Extract STEP blocks by their label.
		stepBlocks := extractStepBlocks(imported.Commands)

		if len(imp.Names) == 1 && imp.Names[0] == "*" {
			// Wildcard import: import all step blocks.
			for name, cmds := range stepBlocks {
				hunt.Blueprints[name] = cmds
			}
			// Also import file-level vars.
			for k, v := range imported.Vars {
				if _, exists := hunt.Vars[k]; !exists {
					hunt.Vars[k] = v
				}
			}
		} else {
			// Named imports: only import specified blocks.
			for _, name := range imp.Names {
				cmds, found := stepBlocks[name]
				if !found {
					// Try case-insensitive match.
					for k, v := range stepBlocks {
						if strings.EqualFold(k, name) {
							cmds = v
							found = true
							break
						}
					}
				}
				if !found {
					return fmt.Errorf("import %q: step block %q not found in %s",
						imp.Source, name, importPath)
				}

				key := name
				if alias, ok := imp.Aliases[name]; ok {
					key = alias
				}
				hunt.Blueprints[key] = cmds
			}
		}
	}
	return nil
}

// extractStepBlocks groups commands by their StepBlock label.
// Commands without a StepBlock are grouped under "".
func extractStepBlocks(cmds []Command) map[string][]Command {
	blocks := make(map[string][]Command)
	for _, cmd := range cmds {
		label := cmd.StepBlock
		blocks[label] = append(blocks[label], cmd)
	}
	return blocks
}

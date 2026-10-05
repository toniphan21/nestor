package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func collectSpec(api nestor.API, filteredName string) (*nestor.SandboxSpec, error) {
	allSpecs := api.Runtime().Registry.SandboxSpecs()
	if len(allSpecs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to prompt"))
		return nil, nil
	}

	if filteredName != "" {
		for _, v := range allSpecs {
			if v.Name != filteredName {
				continue
			}
			return &v, nil
		}
		return nil, fmt.Errorf("%w: sandbox spec %q", nestor.ErrNotFound, filteredName)
	}

	slices.SortFunc(allSpecs, func(a nestor.SandboxSpec, b nestor.SandboxSpec) int {
		return strings.Compare(a.Name, b.Name)
	})

	var err error
	var spec nestor.SandboxSpec
	if len(allSpecs) > 1 {
		spec, err = SelectSandboxSpec(allSpecs)
		if err != nil {
			return nil, err
		}
	} else {
		spec = allSpecs[0]
		fmt.Printf("use sandbox spec %s - harness %s - profile %s\n", pterm.Green(spec.Name), pterm.Magenta(spec.Harness), pterm.Red(spec.Profile))
	}
	return &spec, nil
}

func collectPath(spec nestor.SandboxSpec, filteredPath string) (*string, error) {
	var paths []string
	for _, v := range spec.Mounts {
		paths = append(paths, v.Path)
	}

	if filteredPath != "" {
		for _, v := range paths {
			if v != filteredPath {
				continue
			}
			return &v, nil
		}
		return nil, fmt.Errorf("%w: path %q", nestor.ErrNotFound, filteredPath)
	}

	if len(paths) == 0 {
		fmt.Println(pterm.Red("no sandbox mounts found"))
		return nil, nil
	}

	var selectedPath string
	if len(paths) > 1 {
		r, err := Select(paths, "Select path", func(i int, s string) string {
			return fmt.Sprintf("%d. %s", i+1, s)
		})
		if err != nil {
			return nil, err
		}
		selectedPath = r.Value
	} else {
		selectedPath = paths[0]
		fmt.Printf("use path %s\n", pterm.Cyan(selectedPath))
	}
	return &selectedPath, nil
}

// escapeCLIFlagValue quotes val so a POSIX shell (sh/bash/zsh) reads it as
// exactly one word with no expansion.
func escapeCLIFlagValue(val string) string {
	if val == "" {
		return "''"
	}
	if !strings.ContainsFunc(val, needsQuoting) {
		return val
	}
	// Single quotes are fully literal in POSIX shells. Nothing inside them
	// needs escaping, but a single quote can't appear inside them either.
	if !strings.Contains(val, "'") {
		return "'" + val + "'"
	}
	// Value contains ' → use double quotes and escape the characters
	// that stay special inside them: \ " $ `
	var b strings.Builder
	b.Grow(len(val) + 2)
	b.WriteByte('"')
	for _, r := range val {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// needsQuoting reports whether r is outside the set of characters that are
// always literal in an unquoted shell word.
func needsQuoting(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	switch r {
	case '-', '_', '.', '/', ':', '=', '@', '%', '+', ',':
		return false
	}
	return true
}

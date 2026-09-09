package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func collectSpec(api nestor.API) (*nestor.SandboxSpec, error) {
	allSpecs := api.Runtime().Registry.SandboxSpecs()
	if len(allSpecs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to prompt"))
		return nil, nil
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

func collectPath(spec nestor.SandboxSpec) (*string, error) {
	var paths []string
	for _, v := range spec.Mounts {
		paths = append(paths, v.Path)
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

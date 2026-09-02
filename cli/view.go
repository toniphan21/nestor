package cli

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func View(api nestor.API) (ViewData, error) {
	var result ViewData
	ctx := context.Background()
	runtime := api.Runtime()

	result.NestorDir = runtime.Platform.NestorDir()
	result.SpecFilePath = runtime.Platform.SandboxYmlFile()
	result.ProfileFilePath = runtime.Platform.ProfileYmlFile()
	result.Template = runtime.Template
	result.Platform = runtime.Platform
	result.Specs = runtime.Registry.SandboxSpecs()
	result.Profiles = runtime.Registry.Profiles()
	result.Harnesses = runtime.Registry.Harnesses()
	sbs, err := api.ListSandboxes(ctx)
	if err != nil {
		return result, err
	}
	result.Sandboxes = sbs

	return result, nil
}

const leftSize = 26

type ViewData struct {
	NestorDir       string
	SpecFilePath    string
	ProfileFilePath string
	Template        nestor.Template
	Platform        nestor.Platform
	Specs           []nestor.SandboxSpec
	Profiles        []nestor.Profile
	Sandboxes       []nestor.Sandbox
	Harnesses       []nestor.Harness
}

func (d *ViewData) PrintWithSetupMessage() {
	fmt.Printf("%*s: %s\n", leftSize, pterm.Green("setup"), pterm.Green("done"))
	d.Print()
}

func (d *ViewData) Print() {
	w := leftSize
	nd := d.NestorDir + "/"
	fmt.Println()

	fmt.Printf("%*s: %s\n", w, pterm.Blue("nestor dir"), d.NestorDir)
	fmt.Printf("%*s: %s%s\n", w, pterm.Blue("spec file"), pterm.Gray(nd), strings.TrimPrefix(d.SpecFilePath, nd))
	fmt.Printf("%*s: %s%s\n", w, pterm.Blue("profile file"), pterm.Gray(nd), strings.TrimPrefix(d.ProfileFilePath, nd))
	fmt.Printf("%*s: %s\n", w, pterm.Blue("platform os"), d.Platform.OS())

	fmt.Printf("%*s: %s = %q\n", w, pterm.Blue("templates"), "SandboxID       ", d.Template.SandboxID)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "SandboxTag      ", d.Template.SandboxTag)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "SandboxContainer", d.Template.SandboxContainer)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "WorktreeID      ", d.Template.WorktreeID)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "InitialBranch   ", d.Template.InitialBranch)

	fmt.Println()

	for _, harness := range d.Harnesses {
		fmt.Printf("%*s: %s\n", w, pterm.Magenta("harness"), harness.Name())
		fmt.Println()
	}

	for _, profile := range d.Profiles {
		fmt.Printf("%*s: %s\n", w, pterm.Red("profile"), profile.Name)

		var models = slices.Collect(maps.Keys(profile.Models))
		sort.Strings(models)
		for i, v := range models {
			if v == profile.DefaultModel {
				models[i] = pterm.Yellow(v + " (default)")
				continue
			}
		}
		fmt.Printf("%*s: %s\n", w, pterm.Red("models"), strings.Join(models, ", "))

		var targets []string
		for _, v := range profile.Targets {
			if v == profile.DefaultTarget {
				targets = append(targets, pterm.Yellow(v+" (default)"))
				continue
			}
			targets = append(targets, v)
		}

		fmt.Printf("%*s: %s\n", w, pterm.Red("targets"), strings.Join(targets, ", "))
		fmt.Println()
	}

	fmt.Println()
}

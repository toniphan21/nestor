package cli

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

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

	stat, err := CollectStat(ctx, "/", time.Second)
	if err != nil {
		return result, err
	}
	result.Stat = stat

	return result, nil
}

const leftSize = 32

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
	Stat            *Stat
}

func (d *ViewData) PrintWithSetupMessage() {
	fmt.Printf("%*s: %s\n", leftSize, pterm.Green("setup"), pterm.Green("done"))
	d.Print()
}

func (d *ViewData) Print() {
	w := leftSize
	nd := d.NestorDir + "/"
	fmt.Println()

	if d.Stat != nil {
		fmt.Printf("%*s: %s\n", w, pterm.Blue("system"), d.statLine())
	}
	fmt.Printf("%*s: %s\n", w, pterm.Blue("nestor dir"), d.NestorDir)
	fmt.Printf("%*s: %s%s\n", w, pterm.Blue("spec file"), pterm.Gray(nd), strings.TrimPrefix(d.SpecFilePath, nd))
	fmt.Printf("%*s: %s%s\n", w, pterm.Blue("profile file"), pterm.Gray(nd), strings.TrimPrefix(d.ProfileFilePath, nd))
	fmt.Printf("%*s: %s\n", w, pterm.Blue("platform os"), d.Platform.OS())

	var harnesses []string
	for _, v := range d.Harnesses {
		harnesses = append(harnesses, pterm.Magenta(v.Name()))
	}
	fmt.Printf("%*s: %s\n", w, pterm.Blue("supported harnesses"), strings.Join(harnesses, pterm.Gray(" · ")))

	fmt.Printf("%*s: %s = %q\n", w, pterm.Blue("templates"), "SandboxID       ", d.Template.SandboxID)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "SandboxTag      ", d.Template.SandboxTag)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "SandboxContainer", d.Template.SandboxContainer)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "WorktreeID      ", d.Template.WorktreeID)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "InitialBranch   ", d.Template.InitialBranch)

	fmt.Println()

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
		fmt.Printf("%*s: %s\n", w, pterm.Red("models"), strings.Join(models, pterm.Gray(" · ")))

		var targets []string
		for _, v := range profile.Targets {
			if v == profile.DefaultTarget {
				targets = append(targets, pterm.Yellow(v+" (default)"))
				continue
			}
			targets = append(targets, v)
		}

		fmt.Printf("%*s: %s\n", w, pterm.Red("targets"), strings.Join(targets, pterm.Gray(" · ")))
		fmt.Println()
	}

	fmt.Println()
}

func (d *ViewData) statLine() string {
	if d.Stat == nil {
		return ""
	}

	cpu := fmt.Sprintf("cpu: %s", pterm.Cyan(fmt.Sprintf("%.2f%%", d.Stat.CPUPercent)))
	mem := fmt.Sprintf(
		"mem: %s - %s/%s",
		pterm.Cyan(fmt.Sprintf("%.0f%%", d.Stat.MemPercent)),
		d.humanBytes(d.Stat.MemUsed, false),
		d.humanBytes(d.Stat.MemTotal, true),
	)
	disk := fmt.Sprintf(
		"disk : %s - %s/%s",
		pterm.Cyan(fmt.Sprintf("%.0f%%", d.Stat.DiskPercent)),
		d.humanBytes(d.Stat.DiskUsed, false),
		d.humanBytes(d.Stat.DiskTotal, true),
	)

	var values = []string{cpu, mem, disk}

	if d.Stat.TempC != nil {
		temp := fmt.Sprintf("temp: %s", pterm.Cyan(fmt.Sprintf("%.1f°C", *d.Stat.TempC)))
		values = append(values, temp)
	}

	return strings.Join(values, pterm.Gray(" · "))
}

func (d *ViewData) humanBytes(b uint64, showUnit bool) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	if showUnit {
		return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
	}
	return fmt.Sprintf("%.1f", float64(b)/float64(div))
}

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

type Config struct {
	Dir                         string            `yaml:"dir"`
	Root                        string            `yaml:"root"`
	PowerOverhead               float64           `yaml:"power_overhead"`
	SectorSize                  uint64            `yaml:"sector_size"`
	TotalBytesWrittenOverhead   uint64            `yaml:"total_bytes_written_overhead"`
	TotalBytesWrittenFormat     string            `yaml:"total_bytes_written_format"`
	TotalBytesWrittenThresholds map[uint64]string `yaml:"total_bytes_written_thresholds"`
	PathReplacements            map[string]string `yaml:"paths,omitempty"`
	Redacted                    redacted          `yaml:"redacted,omitempty"`
}

type redacted struct {
	Options  []string `yaml:"options"`
	Settings []string `yaml:"settings"`
	Envs     []string `yaml:"envs"`
}

const defaultTotalBytesWrittenFormat = "TBW=%.3f"

func View(api nestor.API, cf *Config) error {
	result, err := collectViewData(api, cf)
	if err == nil {
		result.Print()
	}
	return err
}

func Explain(api nestor.API, cf *Config) error {
	result, err := collectViewData(api, cf)
	if err == nil {
		result.PrintArch(10, 2)
	}
	return err
}

func collectViewData(api nestor.API, config *Config) (viewData, error) {
	if config == nil {
		config = &Config{
			PowerOverhead:             1.2,
			SectorSize:                512,
			TotalBytesWrittenOverhead: 0,
			TotalBytesWrittenFormat:   defaultTotalBytesWrittenFormat,
		}
	}

	var result viewData
	ctx := context.Background()
	runtime := api.Runtime()

	result.NestorDir = runtime.Platform.NestorDir()
	result.SpecFilePath = runtime.Platform.SandboxYmlFile()
	result.ProfileFilePath = runtime.Platform.ProfileYmlFile()
	result.Runtime = runtime
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

	stat, err := CollectStat(ctx, "/", 100*time.Millisecond)
	if err != nil {
		return result, err
	}
	result.Stat = stat
	result.Config = *config

	return result, nil
}

const leftSize = 32

type viewData struct {
	NestorDir       string
	SpecFilePath    string
	ProfileFilePath string
	Runtime         nestor.Runtime
	Template        nestor.Template
	Platform        nestor.Platform
	Specs           []nestor.SandboxSpec
	Profiles        []nestor.Profile
	Sandboxes       []nestor.Sandbox
	Harnesses       []nestor.Harness
	Stat            *Stat
	Config          Config
}

func (d *viewData) PrintArch(drawPadding int, textPadding int) {
	f, err := nestor.Embed.ReadFile("assets/arch")
	if err != nil {
		return
	}

	coloring := map[string]func(a ...any) string{
		"Harness":                          pterm.Magenta,
		"Profile":                          pterm.Red,
		"SandboxSpec ":                     pterm.Cyan,
		"Sandbox ":                         pterm.Green,
		"Lease":                            pterm.Blue,
		"RunResult":                        pterm.Green,
		"Acquire()":                        pterm.Yellow,
		"API.":                             pterm.Gray,
		"(auto) docker build · docker run": pterm.Gray,
		"instantiate":                      pterm.Gray,
		"binds 1 harness + 1 profile":      pterm.Gray,
		"1                 1":              pterm.Gray,
		"sleep infinity":                   pterm.Gray,
		"docker exec":                      pterm.Gray,
		"parse":                            pterm.Gray,
		"profile.yml":                      pterm.LightRed,
		"sandbox.yml":                      pterm.LightCyan,
	}

	var pad = strings.Repeat(" ", drawPadding)
	var out []string
	lines := strings.Split(string(f), "\n")
	for _, line := range lines {
		for t, fn := range coloring {
			line = strings.ReplaceAll(line, t, fn(t))
		}

		out = append(out, pad+line)
	}

	fmt.Println(strings.Join(out, "\n"))

	pad = strings.Repeat(" ", textPadding)
	text := `
<pad><profile.yml> stores the auth method, token, and dockerfile path, plus profile metadata: models/aliases, targets, and the default model and target.
<pad><sandbox.yml> stores the <SandboxSpec> configuration, which binds a <Harness> (an interface; built-ins provided, extendable) and a <Profile>. SandboxSpec also holds the mounts/work path configuration. 
<pad>Specs need not come from disk; pass them directly with <WithSandboxSpecs()>, and profiles with <WithProfiles()>.

<pad>From a <SandboxSpec> creates one or more Sandboxes, up to its max_instance limit. Each <Sandbox> is a container running 'sleep infinity'.

<pad>Use <API>.<Acquire(ctx, spec, path)> to get a <Lease>. Nestor picks an available sandbox based on path, creating a new one if needed (Acquire automatically builds the docker image and starts the sandbox).

<pad>A <Lease> expires shortly after it is acquired. Use <Extend()> to extend it, <Run()> to execute a prompt, and <Release()> to release it; this is where you put your logic (each run triggers a docker exec on the running Sandbox container).
`
	replaces := map[string]string{
		"<pad>":                      pad,
		"<profile.yml>":              pterm.Red(d.ReplacePath(d.ProfileFilePath)),
		"<sandbox.yml>":              pterm.Cyan(d.ReplacePath(d.SpecFilePath)),
		"<Harness>":                  pterm.Magenta("Harness"),
		"<Profile>":                  pterm.Red("Profile"),
		"<Sandbox>":                  pterm.Green("Sandbox"),
		"<SandboxSpec>":              pterm.Cyan("SandboxSpec"),
		"<Lease>":                    pterm.Blue("Lease"),
		"<API>":                      pterm.Gray("API"),
		"<Acquire(ctx, spec, path)>": pterm.Yellow("Acquire(ctx, spec, path)"),
		"<Extend()>":                 pterm.Yellow("Extend()"),
		"<Run()>":                    pterm.Yellow("Run()"),
		"<Release()>":                pterm.Yellow("Release()"),
		"<WithSandboxSpecs()>":       pterm.Yellow("WithSandboxSpecs()"),
		"<WithProfiles()>":           pterm.Yellow("WithProfiles()"),
	}
	for k, v := range replaces {
		text = strings.ReplaceAll(text, k, v)
	}
	fmt.Println(text)
}

func (d *viewData) Print() {
	ctx := context.Background()

	w := leftSize
	nd := d.ReplacePath(d.NestorDir) + "/"
	fmt.Println()

	if d.Stat != nil {
		fmt.Printf("%*s: %s\n", w, pterm.Blue("time"), d.Stat.Time.Format("02.01.2006 15:04:05 MST"))
		fmt.Printf("%*s: %s\n", w, pterm.Blue("system"), d.statLine())
	}
	fmt.Printf("%*s: %s\n", w, pterm.Blue("nestor dir"), d.ReplacePath(d.NestorDir))
	fmt.Printf("%*s: %s%s\n", w, pterm.Blue("spec file"), pterm.Gray(nd), strings.TrimPrefix(d.ReplacePath(d.SpecFilePath), nd))
	fmt.Printf("%*s: %s%s\n", w, pterm.Blue("profile file"), pterm.Gray(nd), strings.TrimPrefix(d.ReplacePath(d.ProfileFilePath), nd))
	fmt.Printf("%*s: %s\n", w, pterm.Blue("platform os"), d.Platform.OS())

	fmt.Printf("%*s: %s = %q\n", w, pterm.Blue("templates"), "SandboxID       ", d.Template.SandboxID)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "SandboxTag      ", d.Template.SandboxTag)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "SandboxContainer", d.Template.SandboxContainer)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "WorktreeID      ", d.Template.WorktreeID)
	fmt.Printf("%*s  %s = %q\n", w, pterm.Blue(""), "InitialBranch   ", d.Template.InitialBranch)

	fmt.Println()

	for _, harness := range d.Harnesses {
		fmt.Printf("%*s: %s · %s \n", w, pterm.Magenta("harness"), harness.Name(), harness.DisplayName())
		d.printKeyValues(w, harness.DefaultOptions(d.Runtime), "option", pterm.Magenta("options"), pterm.Magenta(""))
		fmt.Println()
	}

	for _, profile := range d.Profiles {
		fmt.Printf("%*s: %s\n", w, pterm.Red("profile"), profile.Name)
		fmt.Printf("%*s: %s\n", w, pterm.Red("auth"), profile.Auth)
		proxy := "off" + pterm.Yellow(" (the API token is mounted into the container)")
		if profile.Proxy {
			proxy = "on" + pterm.Green(" (the container never sees an API token)")
		}
		fmt.Printf("%*s: %s\n", w, pterm.Red("proxy"), proxy)

		d.printKeyValues(w, profile.Settings, "setting", pterm.Red("settings"), pterm.Red(""))
		d.printKeyValues(w, profile.Options, "option", pterm.Red("options"), pterm.Red(""))

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

	for _, spec := range d.Specs {
		fmt.Printf("%*s: %s %s %s \n", w, pterm.Cyan("sandbox spec"), spec.Name, pterm.Gray("· docker image ="), pterm.Green(d.Template.MakeSandboxTag(spec.Name)))
		fmt.Printf("%*s: %d\n", w, pterm.Cyan("max instances"), spec.MaxInstances)

		fmt.Printf("%*s: %s\n", w, pterm.Cyan("harness"), spec.Harness)

		var profile = ""
		piy := spec.ProfileInYaml()
		if piy != nil && *piy != spec.Profile {
			profile = "<not-set> " + pterm.Gray("(defaults to harness name: "+spec.Profile+")")
		} else {
			profile = spec.Profile
		}
		fmt.Printf("%*s: %s\n", w, pterm.Cyan("profile"), profile)

		var target = ""
		tiy := spec.TargetInYaml()
		if tiy != nil && *tiy != spec.Target {
			target = "<not-set> " + pterm.Gray("(defaults to harness default target: "+spec.Target+")")
		} else {
			target = spec.Target
		}
		fmt.Printf("%*s: %s\n", w, pterm.Cyan("target"), target)

		// lease duration
		lease := ""
		if spec.Lease == nil {
			lease = "<not-set> " + pterm.Gray("(defaults to init 1m, extend 15m)")
		} else {
			lease = fmt.Sprintf("init %s, extend %s", spec.LeaseInitDuration(), spec.LeaseExtendDuration())
		}
		fmt.Printf("%*s: %s\n", w, pterm.Cyan("lease config"), lease)

		d.printKeyValues(w, spec.Env, "env", pterm.Cyan("env"), pterm.Cyan(""))

		if len(spec.Mounts) == 0 {
			fmt.Printf("%*s: %s\n", w, pterm.Cyan("mounts"), pterm.Red("<not-set> (required — no lease can be acquired until you set this)"))
		} else {
			for i, v := range spec.Mounts {
				var keys = []string{"type", "path"}

				mounts := make(map[string]string)
				mounts["type"] = string(v.Type)
				mounts["path"] = v.Path
				if v.At != "" {
					mounts["at"] = v.At
					keys = append(keys, "at")
				}
				if v.ReadOnly {
					mounts["readonly"] = "true"
					keys = append(keys, "readonly")
				}

				mt := fmt.Sprintf("mounts[%d]", i+1)
				d.printKeyValuesWithKeysOrder(w, mounts, "-", pterm.Cyan(mt), pterm.Magenta(""), keys)
			}
		}

		sandboxes := d.sandboxes(spec)
		for i, v := range sandboxes {
			sb := fmt.Sprintf("sandbox[%d]", i+1)
			status := pterm.Gray("docker container = " + d.Template.MakeSandboxContainer(v.ID()) + " · stopped")
			if v.IsRunning(ctx) {
				status = pterm.Gray("docker container = ")
				status += pterm.Green(d.Template.MakeSandboxContainer(v.ID()))
				status += pterm.Gray(" · ")
				status += pterm.Green("running")
			}
			fmt.Printf("%*s: id       = %s %s %s\n", w, pterm.Green(sb), v.ID(), pterm.Gray("·"), status)

			leases := v.Leases()
			if len(leases) == 0 {
				fmt.Printf("%*s  %s\n", w, pterm.Green(""), pterm.Blue("<no-lease>"))
			} else {
				for j, l := range leases {
					lt := pterm.Blue(fmt.Sprintf("lease[%d]", j+1))
					fmt.Printf("%*s  %s = %s %s %s %s %s\n", w, pterm.Cyan(""),
						lt,
						l.ID(), pterm.Gray("·"),
						pterm.Cyan(l.ExpiresAt().Format(time.RFC3339)), pterm.Gray("·"),
						d.ReplacePath(l.HostPath()),
					)
				}
			}
		}

		fmt.Println()
	}
}

func (d *viewData) printKeyValues(w int, kv map[string]string, typ string, firstLineLabel, label string) {
	keys := slices.Collect(maps.Keys(kv))
	sort.Strings(keys)
	d.printKeyValuesWithKeysOrder(w, kv, typ, firstLineLabel, label, keys)
}

func (d *viewData) printKeyValuesWithKeysOrder(w int, kv map[string]string, typ string, firstLineLabel, label string, keys []string) {
	if kv == nil {
		return
	}

	ml := 0
	for _, key := range keys {
		if len(key) > ml {
			ml = len(key)
		}
	}

	for i, key := range keys {
		v, ok := d.Redact(typ, key, d.ReplacePath(kv[key]))
		if ok {
			v = pterm.Gray(v)
		}

		if i == 0 {
			fmt.Printf("%*s: %*s = %s\n", w, firstLineLabel, 0-ml, key, v)
			continue
		}
		fmt.Printf("%*s  %*s = %s\n", w, label, 0-ml, key, v)
	}
}

func (d *viewData) ReplacePath(path string) string {
	if d.Config.PathReplacements == nil {
		return path
	}
	keys := slices.Collect(maps.Keys(d.Config.PathReplacements))
	slices.SortFunc(keys, func(a, b string) int {
		return strings.Compare(b, a)
	})
	for _, k := range keys {
		if strings.HasPrefix(path, k) {
			return d.Config.PathReplacements[k] + strings.TrimPrefix(path, k)
		}
	}
	return path
}

func (d *viewData) Redact(typ, key, value string) (string, bool) {
	list := make(map[string]bool)
	switch typ {
	case "option":
		for _, v := range d.Config.Redacted.Options {
			list[v] = true
		}
	case "setting":
		for _, v := range d.Config.Redacted.Settings {
			list[v] = true
		}
	case "env":
		for _, v := range d.Config.Redacted.Envs {
			list[v] = true
		}
	}

	_, have := list[key]
	if !have {
		return value, false
	}
	return "----- redacted -----", true
}

func (d *viewData) statLine() string {
	if d.Stat == nil {
		return ""
	}

	var values []string

	switch {
	case d.Stat.Power != nil && d.Stat.TempC != nil:
		pt := fmt.Sprintf(
			"power: %s - %s",
			pterm.Red(fmt.Sprintf("~%.1fW", *d.Stat.Power*d.Config.PowerOverhead)),
			pterm.Red(fmt.Sprintf("%.1f°C", *d.Stat.TempC)),
		)
		values = append(values, pt)

	case d.Stat.Power != nil:
		pt := fmt.Sprintf(
			"power: %s",
			pterm.Red(fmt.Sprintf("~%.1fW", *d.Stat.Power*d.Config.PowerOverhead)),
		)
		values = append(values, pt)

	case d.Stat.TempC != nil:
		pt := fmt.Sprintf(
			"temp: %s",
			pterm.Cyan(fmt.Sprintf("%.1f°C", *d.Stat.TempC)),
		)
		values = append(values, pt)
	}

	cpu := fmt.Sprintf("cpu: %s", pterm.Cyan(fmt.Sprintf("%.2f%%", d.Stat.CPUPercent)))
	mem := fmt.Sprintf(
		"mem: %s - %s",
		pterm.Cyan(fmt.Sprintf("%.0f%%", d.Stat.MemPercent)),
		bytePairAuto(d.Stat.MemUsed, d.Stat.MemTotal),
	)

	disk := fmt.Sprintf(
		"disk: %s - %s",
		pterm.Cyan(fmt.Sprintf("%.0f%%", d.Stat.DiskPercent)),
		bytePairAuto(d.Stat.DiskUsed, d.Stat.DiskTotal),
	)

	if d.Stat.DiskTotalBytesWritten != nil {
		tbw := *d.Stat.DiskTotalBytesWritten
		tbw *= d.Config.SectorSize
		tbw += d.Config.TotalBytesWrittenOverhead * gigabyte
		value := float64(tbw) / float64(gigabyte)

		disk = fmt.Sprintf(
			"disk: %s - %s %s",
			pterm.Cyan(fmt.Sprintf("%.0f%%", d.Stat.DiskPercent)),
			bytePairAuto(d.Stat.DiskUsed, d.Stat.DiskTotal),
			formatTBW(d.Config.TotalBytesWrittenFormat, value, d.Config.TotalBytesWrittenThresholds),
		)
	}

	values = append(values, cpu, mem, disk)

	return strings.Join(values, pterm.Gray(" · "))
}

func (d *viewData) sandboxes(spec nestor.SandboxSpec) []nestor.Sandbox {
	var out []nestor.Sandbox
	for _, v := range d.Sandboxes {
		if v.Spec().Name != spec.Name {
			continue
		}
		out = append(out, v)
	}
	return out
}

type byteUnit struct {
	Name string
	Div  float64
}

const gigabyte = 1024 * 1024 * 1024

var (
	miB = byteUnit{"MB", 1 << 20}
	giB = byteUnit{"GB", 1 << 30}
)

// Pair formats used and total in a shared unit.
func bytePair(used, total uint64, u byteUnit) string {
	return fmt.Sprintf("%.1f/%.1f %s", float64(used)/u.Div, float64(total)/u.Div, u.Name)
}

func bytePairAuto(used, total uint64) string {
	u := miB
	if total >= 1<<30 {
		u = giB
	}
	return bytePair(used, total, u)
}

func formatTBW(format string, value float64, thresholds map[uint64]string) string {
	if format == "" {
		format = defaultTotalBytesWrittenFormat
	}
	color := findTBWThresholdColor(value, thresholds)
	text := fmt.Sprintf(format, value/1024)
	switch color {
	case "gray":
		return pterm.Gray(text)
	case "cyan":
		return pterm.Cyan(text)
	case "red":
		return pterm.Red(text)
	case "green":
		return pterm.Green(text)
	case "blue":
		return pterm.Blue(text)
	case "yellow":
		return pterm.Yellow(text)
	default:
		return text
	}
}

type tbwThreshold struct {
	value float64
	color string
}

func findTBWThresholdColor(value float64, thresholds map[uint64]string) string {
	var points []tbwThreshold
	for k, v := range thresholds {
		points = append(points, tbwThreshold{float64(k), v})
	}
	slices.SortFunc(points, func(a, b tbwThreshold) int {
		return int(a.value - b.value)
	})
	for _, v := range points {
		if value < v.value {
			return v.color
		}
	}
	return points[len(points)-1].color
}

package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/infra/fs"
)

type SetupWorkspaceArgs struct {
	Path          string
	DisplayedPath string
	DryRun        bool
}

type setupWorkspaceData struct {
	spec         *nestor.SandboxSpec
	profile      nestor.Profile
	harness      nestor.Harness
	path         string
	maxInstances int
	useWorktree  bool
	target       string
	specName     string
}

func SetupWorkspace(api nestor.API, args SetupWorkspaceArgs) error {
	specs := api.Runtime().Registry.SandboxSpecs()
	seen := make(map[string]bool)
	for _, v := range specs {
		seen[v.Name] = true
	}

	var err error
	data := &setupWorkspaceData{
		path: args.Path,
	}

	ws, have := hasWorkspace(api, args.Path)
	if have {
		seen := make(map[string]bool)
		for _, v := range ws.Mounts() {
			seen[v.Spec().Name] = true
		}

		var availableSpecs []nestor.SandboxSpec
		for _, v := range specs {
			if !seen[v.Name] {
				availableSpecs = append(availableSpecs, v)
			}
		}

		if len(availableSpecs) == 0 {
			fmt.Println(pterm.Yellow(fmt.Sprintf("Path %s is already mounted to all sandboxes, nothing to do", args.DisplayedPath)))
			fmt.Println(pterm.Green("done"))
			return nil
		}

		if data.spec, err = chooseSpecToSetup(availableSpecs); err != nil {
			return err
		}
	} else {
		if data.spec, err = chooseSpecToSetup(specs); err != nil {
			return err
		}
	}

	if data.spec == nil { // only ask about profile and harness if it is a new spec
		pah, err := chooseProfileAndHarnessToSetup(api.Runtime())
		if err != nil {
			return err
		}

		if pah == nil {
			fmt.Println(pterm.Yellow("Nothing to do"))
			fmt.Println(pterm.Green("done"))
			return nil
		}

		data.harness = pah.harness
		data.profile = pah.profile
		data.target = chooseDockerTargetToSetup(pah.profile, pah.resolvedProfile)
	}

	if fs.HasDir(filepath.Join(args.Path, ".git")) {
		wtq := "The current directory is a git repository. Use a separate worktree per sandbox?"
		if data.useWorktree, err = pterm.DefaultInteractiveConfirm.WithDefaultText(wtq).WithDefaultValue(true).Show(); err != nil {
			return err
		}
	}

	if data.spec == nil {
		miq := "Max number of sandbox instances that can run in parallel"
		if data.maxInstances, err = askInt(miq, 3, 1, 1000); err != nil {
			return err
		}

		// name of the new spec
		var suggestedName string
		i := 0
		for {
			suggestedName = data.harness.Name() + "-" + data.target
			if i != 0 {
				suggestedName += "-" + strconv.Itoa(i)
			}

			if !seen[suggestedName] {
				break
			}
			i++
		}

		if data.specName, err = askSpecNameForSetup(suggestedName, seen); err != nil {
			return err
		}
	}

	return runSetupWorkspace(api, args, data)
}

func runSetupWorkspace(api nestor.API, args SetupWorkspaceArgs, data *setupWorkspaceData) error {
	var before, after []byte
	var err error

	if args.DryRun {
		before, err = api.Runtime().Registry.SandboxSpecYMLContent()
		if err != nil {
			return err
		}
	}

	if data.spec == nil { // create new sandbox spec
		var mounts []nestor.SandboxSpecMount
		if data.useWorktree {
			mounts = append(mounts, nestor.SandboxSpecMount{Type: nestor.MountTypeGitWorktree, Path: data.path})
		} else {
			mounts = append(mounts, nestor.SandboxSpecMount{Type: nestor.MountTypeDirect, Path: data.path})
		}

		newSpec := nestor.SandboxSpec{
			Name:         data.specName,
			MaxInstances: data.maxInstances,
			Harness:      data.harness.ID(),
			Profile:      data.profile.Name,
			Target:       data.target,
			Mounts:       mounts,
		}

		api.Runtime().Registry.RegisterSandboxSpec(newSpec)
	} else { // apply to current spec
		if data.useWorktree {
			data.spec.Mounts = append(data.spec.Mounts, nestor.SandboxSpecMount{Type: nestor.MountTypeGitWorktree, Path: data.path})
		} else {
			data.spec.Mounts = append(data.spec.Mounts, nestor.SandboxSpecMount{Type: nestor.MountTypeDirect, Path: data.path})
		}

		api.Runtime().Registry.RegisterSandboxSpec(*data.spec)
	}

	after, err = api.Runtime().Registry.SandboxSpecYMLContent()
	if err != nil {
		return nil
	}

	if args.DryRun {
		removed := pterm.NewStyle(pterm.FgRed)
		added := pterm.NewStyle(pterm.FgGreen)
		a := difflib.SplitLines(string(before))
		b := difflib.SplitLines(string(after))

		w := len(strconv.Itoa(max(len(a), len(b))))
		w = max(w, 4) // 4 = len("file")

		p := max(4-w, 0)
		fmt.Println()
		fmt.Println("--- diff")
		fmt.Printf("%*sfile: %s\n", p, "", api.Runtime().Platform.SandboxYmlFile())

		line := func(n int, mark, s string) string {
			return fmt.Sprintf("%*d. %s %s", w, n, mark, s)
		}

		for _, op := range difflib.NewMatcher(a, b).GetOpCodes() {
			switch op.Tag {
			case 'e':
				for i, l := range b[op.J1:op.J2] {
					fmt.Print(line(op.J1+i+1, " ", l))
				}
			case 'd', 'r':
				for i, l := range a[op.I1:op.I2] {
					fmt.Print(removed.Sprint(line(op.I1+i+1, "-", l)))
				}
			}
			if op.Tag == 'i' || op.Tag == 'r' {
				for i, l := range b[op.J1:op.J2] {
					fmt.Print(added.Sprint(line(op.J1+i+1, "+", l)))
				}
			}
		}

		fmt.Println()
		fmt.Println("--- new content; file: ", api.Runtime().Platform.SandboxYmlFile())
		fmt.Println(string(after))
	} else {
		err = fs.AtomicWriteFile(api.Runtime().Platform.SandboxYmlFile(), after, 0o644)
		if err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println(pterm.Green("done"))
	return nil
}

func hasWorkspace(api nestor.API, path string) (nestor.Workspace, bool) {
	ctx := context.Background()

	workspaces := api.Workspaces(ctx)
	for _, v := range workspaces {
		if v.Path() == path {
			return v, true
		}
	}
	return nestor.Workspace{}, false
}

type setupSpecOption struct {
	spec  *nestor.SandboxSpec
	text  string
	order int
}

func chooseSpecToSetup(specs []nestor.SandboxSpec) (*nestor.SandboxSpec, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	var options []*setupSpecOption
	options = append(options, &setupSpecOption{text: pterm.Blue("Create new sandbox spec"), order: 0})
	for _, v := range specs {
		options = append(options, &setupSpecOption{
			spec:  &v,
			text:  fmt.Sprintf("%s - harness %s - profile %s", pterm.Green(v.Name), pterm.Magenta(v.Harness), pterm.Red(v.Profile)),
			order: 1,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].order != options[j].order {
			return options[i].order < options[j].order
		}
		return pterm.RemoveColorFromString(options[i].text) < pterm.RemoveColorFromString(options[j].text)
	})

	s, err := Select(options, "Choose spec", func(i int, t *setupSpecOption) string {
		return fmt.Sprintf("%d. %s", i+1, t.text)
	})
	if err != nil {
		return nil, err
	}
	return s.Value.spec, nil
}

type setupProfileHarnessOption struct {
	profile         nestor.Profile
	harness         nestor.Harness
	resolvedProfile nestor.ResolvedProfile
	text            string
}

func chooseProfileAndHarnessToSetup(runtime nestor.Runtime) (*setupProfileHarnessOption, error) {
	nestor.SetTheme(&ptermTheme{})
	var options []*setupProfileHarnessOption
	for _, profile := range runtime.Registry.Profiles() {
		for _, harness := range runtime.Registry.Harnesses() {
			rp, err := harness.ResolveProfile(runtime, profile)
			if rp == nil || err != nil {
				continue
			}

			text := fmt.Sprintf("harness %s with profile %s", pterm.Magenta(harness.DisplayName()), rp.SetupString())
			options = append(options, &setupProfileHarnessOption{
				profile:         profile,
				harness:         harness,
				resolvedProfile: rp,
				text:            text,
			})
		}
	}
	slices.SortFunc(options, func(a, b *setupProfileHarnessOption) int {
		return strings.Compare(pterm.RemoveColorFromString(a.text), pterm.RemoveColorFromString(b.text))
	})

	if len(options) == 0 {
		return nil, nil
	}

	s, err := Select(options, "Choose harness and profile for the sandbox", func(i int, opt *setupProfileHarnessOption) string {
		return fmt.Sprintf("%d. %s", i+1, opt.text)
	})
	if err != nil {
		return nil, err
	}
	return s.Value, nil
}

func chooseDockerTargetToSetup(profile nestor.Profile, resolvedProfile nestor.ResolvedProfile) string {
	q := fmt.Sprintf("Choose target (from %s)", resolvedProfile.Dockerfile())
	s, err := Select(profile.Targets, q, func(i int, opt string) string {
		return fmt.Sprintf("%d. %s", i+1, opt)
	})
	if err != nil {
		return profile.DefaultTarget
	}
	return s.Value
}

func askInt(prompt string, def, lo, hi int) (int, error) {
	input := pterm.DefaultInteractiveTextInput.WithDefaultValue(strconv.Itoa(def))
	for {
		s, err := input.Show(prompt)
		if err != nil {
			return 0, err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return def, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			pterm.Error.Printfln("%q is not an integer", s)
			continue
		}
		if n < lo || n > hi {
			pterm.Error.Printfln("must be between %d and %d", lo, hi)
			continue
		}
		return n, nil
	}
}

func askSpecNameForSetup(suggestedName string, unavailables map[string]bool) (string, error) {
	input := pterm.DefaultInteractiveTextInput.WithDefaultValue(suggestedName)
	for {
		s, err := input.Show("Sandbox spec name")
		if err != nil {
			return "", err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			pterm.Error.Printfln("Please enter valid name")
			continue
		}

		if unavailables[s] {
			pterm.Error.Printfln("%q is taken, please enter other name", s)
			continue
		}
		return s, nil
	}
}

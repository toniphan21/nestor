package cli

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func LaunchSandbox(api nestor.API, args LaunchArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	sandboxes, err := collectFreeSandboxes(api, spec, path, args.Sandbox)
	if err != nil {
		return err
	}
	if len(sandboxes) != 1 {
		return fmt.Errorf("this command is supposed to used by the TUI, don't run it manually")
	}
	sandbox := sandboxes[0]

	w := 25
	label := func(txt string) string {
		return fmt.Sprintf("%*s", w, txt)
	}

	fmt.Printf(
		"%s: %s - harness %s - profile %s\n",
		label("Spec"), pterm.Blue(spec.Name), pterm.Blue(spec.Harness), pterm.Blue(spec.Profile),
	)
	fmt.Printf("%s: %s\n", label("Sandbox"), pterm.Green(sandbox.ID()))
	fmt.Printf("%s: %s\n", label("Path"), pterm.Magenta(path))
	wd, hwd, _ := sandbox.ResolveWorkDir(path)
	if wd != hwd {
		fmt.Printf("%s: %s\n", label("Container Path"), sandbox.ID())
		fmt.Printf("%s: %s\n", label("Host Work Path"), hwd)
	} else {
		fmt.Printf("%s: %s\n", label("Work Path"), hwd)
	}

	sc := sessionCollector{
		sandboxes:          sandboxes,
		path:               path,
		displaySandboxName: false,
		timeout:            30 * time.Second,
		selectLabel:        " Session",
		titleInputLabel:    "Session (optional)",
	}
	_, sessionID, sessionTitle, err := sc.collect()
	if err != nil {
		return err
	}
	if args.DryRun != "" {
		return launchDryRun(sandbox, path, sessionID, sessionTitle, args.DryRun)
	}
	return launch(api, sandbox, path, sessionID, sessionTitle)
}

func LaunchSpec(api nestor.API, args LaunchArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	sandboxes, err := freeSandboxes(api, spec, path)
	if err != nil {
		return err
	}

	w := 25
	label := func(txt string) string {
		return fmt.Sprintf("%*s", w, txt)
	}

	fmt.Printf(
		"%s: %s - harness %s - profile %s\n",
		label("Spec"), pterm.Blue(spec.Name), pterm.Blue(spec.Harness), pterm.Blue(spec.Profile),
	)
	fmt.Printf("%s: %s\n", label("Path"), pterm.Magenta(path))

	sc := sessionCollector{
		sandboxes:          sandboxes,
		path:               path,
		displaySandboxName: true,
		timeout:            30 * time.Second,
		selectLabel:        " Session",
		titleInputLabel:    "Session (optional)",
	}
	sandbox, sessionID, sessionTitle, err := sc.collect()
	if err != nil {
		return err
	}
	if args.DryRun != "" {
		return launchDryRun(sandbox, path, sessionID, sessionTitle, args.DryRun)
	}
	return launch(api, sandbox, path, sessionID, sessionTitle)
}

func PromptSandbox(api nestor.API, args PromptArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	sandboxes, err := collectFreeSandboxes(api, spec, path, args.Sandbox)
	if err != nil {
		return err
	}
	if len(sandboxes) != 1 {
		return fmt.Errorf("this command is supposed to used by the TUI, don't run it manually")
	}
	sandbox := sandboxes[0]

	w := 25
	label := func(txt string) string {
		return fmt.Sprintf("%*s", w, txt)
	}

	fmt.Printf(
		"%s: %s - harness %s - profile %s\n",
		label("Spec"), pterm.Blue(spec.Name), pterm.Blue(spec.Harness), pterm.Blue(spec.Profile),
	)
	fmt.Printf("%s: %s\n", label("Sandbox"), pterm.Green(sandbox.ID()))
	fmt.Printf("%s: %s\n", label("Path"), pterm.Magenta(path))
	wd, hwd, _ := sandbox.ResolveWorkDir(path)
	if wd != hwd {
		fmt.Printf("%s: %s\n", label("Container Path"), sandbox.ID())
		fmt.Printf("%s: %s\n", label("Host Work Path"), hwd)
	} else {
		fmt.Printf("%s: %s\n", label("Work Path"), hwd)
	}

	model, err := collectModel(api, spec, "   Model", args.Model)
	if err != nil {
		return err
	}

	sc := sessionCollector{
		sandboxes:          sandboxes,
		path:               path,
		displaySandboxName: false,
		timeout:            30 * time.Second,
		selectLabel:        " Session",
		titleInputLabel:    "Session (optional)",
	}
	sandbox, sessionID, sessionTitle, err := sc.collect()
	if err != nil {
		return err
	}

	content, err := collectPrompt(sandbox, path, args.PromptFile)
	if err != nil {
		return err
	}

	if args.DryRun != "" {
		return promptDryRun(sandbox, path, model, sessionID, sessionTitle, content, args.DryRun)
	}
	return prompt(api, sandbox, path, model, sessionID, sessionTitle, content)
}

func PromptSpec(api nestor.API, args PromptArgs) error {
	spec, path, err := collectSpecAndPath(api, args.SandboxSpec, args.Path)
	if err != nil {
		return err
	}

	sandboxes, err := freeSandboxes(api, spec, path)
	if err != nil {
		return err
	}

	w := 25
	label := func(txt string) string {
		return fmt.Sprintf("%*s", w, txt)
	}

	fmt.Printf(
		"%s: %s - harness %s - profile %s\n",
		label("Spec"), pterm.Blue(spec.Name), pterm.Blue(spec.Harness), pterm.Blue(spec.Profile),
	)
	fmt.Printf("%s: %s\n", label("Path"), pterm.Magenta(path))
	model, err := collectModel(api, spec, "   Model", args.Model)
	if err != nil {
		return err
	}

	sc := sessionCollector{
		sandboxes:          sandboxes,
		path:               path,
		displaySandboxName: true,
		timeout:            30 * time.Second,
		selectLabel:        " Session",
		titleInputLabel:    "Session (optional)",
	}
	sandbox, sessionID, sessionTitle, err := sc.collect()
	if err != nil {
		return err
	}

	content, err := collectPrompt(sandbox, path, args.PromptFile)
	if err != nil {
		return err
	}

	if args.DryRun != "" {
		return promptDryRun(sandbox, path, model, sessionID, sessionTitle, content, args.DryRun)
	}
	return prompt(api, sandbox, path, model, sessionID, sessionTitle, content)
}

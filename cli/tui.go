package cli

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type LaunchSandboxArgs struct {
	Spec    string
	Sandbox string
	Path    string
	DryRun  bool
}

func LaunchSandbox(api nestor.API, args LaunchSandboxArgs) error {
	spec, path, err := collectSpecAndPath(api, args.Spec, args.Path)
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
	if args.DryRun {
		return launchDryRun(sandbox, path, sessionID, sessionTitle)
	}
	return launch(api, sandbox, path, sessionID, sessionTitle)
}

type LaunchSpecArgs struct {
	Spec   string
	Path   string
	DryRun bool
}

func LaunchSpec(api nestor.API, args LaunchSpecArgs) error {
	spec, path, err := collectSpecAndPath(api, args.Spec, args.Path)
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
	if args.DryRun {
		return launchDryRun(sandbox, path, sessionID, sessionTitle)
	}
	return launch(api, sandbox, path, sessionID, sessionTitle)
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/infra/fs"
)

const welcome = `
Thank you for using <nestor> — a Go library with a built-in binary that helps you run <Claude Code> or <OpenCode> inside a <sandboxed> container, with <git worktree support>.

Please note that nestor comes with no warranty or guarantee of any kind; use it at your own risk.
`

const alreadySetUp = `
nestor is already set up at %s. Common tasks:

1. Update the sandbox Dockerfile:
   - edit ./.nestor/claude/Dockerfile or ./.nestor/opencode/Dockerfile
   - run  nestor down && nestor build
2. Add credentials or keys: edit ./.nestor/profile.yml
3. Edit the sandbox — add paths, raise the instance count: edit ./.nestor/sandbox.yml
4. Launch your coding agent: nestor launch
5. Release a stuck lease: nestor release
6. Destroy the sandboxes and start fresh: nestor destroy

`

const profileClaudeOAuth = "claude-subscription"
const profileClaudeAPI = "claude-api"
const profileOpencode = "opencode"

func setup(cmd *cobra.Command, args []string) error {
	colors := map[string]string{
		"<nestor>":               pterm.Green("nestor"),
		"<Claude Code>":          pterm.Red("Claude Code"),
		"<OpenCode>":             pterm.Blue("OpenCode"),
		"<git worktree support>": pterm.Yellow("git worktree support"),
		"<sandboxed>":            pterm.Cyan("sandboxed"),
	}
	msg := welcome
	for k, v := range colors {
		msg = strings.ReplaceAll(msg, k, v)
	}
	fmt.Println(msg)

	var accept bool
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			fmt.Println(pterm.Green("done"))
		}
	}()

	config := readConfig()
	if fs.HasDir(config.Dir) {
		fmt.Printf(alreadySetUp, filepath.Join(wd, config.Dir))
		return nil
	}

	text := fmt.Sprintf(
		"Setting up nestor in the current directory: %s. Continue?",
		wd,
	)

	accept, err = pterm.DefaultInteractiveConfirm.WithDefaultText(text).WithDefaultValue(true).Show()
	if err != nil {
		return err
	}

	if !accept {
		return nil
	}

	dir := filepath.Join(wd, ".nestor")
	maxInstances := 1
	useWT := false
	if fs.HasDir(filepath.Join(wd, ".git")) {
		wtq := "The current directory is a git repository. Use a separate worktree per sandbox?"
		useWT, err = pterm.DefaultInteractiveConfirm.WithDefaultText(wtq).WithDefaultValue(true).Show()
		if err != nil {
			return err
		}

		if useWT {
			miq := "Max number of sandbox instances that can run in parallel, each on its own worktree"
			maxInstances, err = askInt(miq, 3, 1, 1000)
		}
	}

	shareHarnessState := true
	shsq := "Share harness state across all sandbox instances? Sessions are then shared too (recommended)"
	shareHarnessState, err = pterm.DefaultInteractiveConfirm.WithDefaultText(shsq).WithDefaultValue(shareHarnessState).Show()
	if err != nil {
		return err
	}

	scf := sandboxCfg{
		maxInstances:      maxInstances,
		worktree:          useWT,
		shareHarnessState: shareHarnessState,
		path:              wd,
	}

	var h *cli.SelectResult[string]
	h, err = cli.Select([]string{profileClaudeOAuth, profileClaudeAPI, profileOpencode}, "Choose a harness for the sandbox", func(i int, harness string) string {
		switch harness {
		case profileClaudeOAuth:
			return fmt.Sprintf(
				"%d. %s - %s %s", i+1, "Claude subscription",
				pterm.Green("personal, cheap"),
				pterm.Yellow("(OAuth token is set inside the sandbox; Anthropic is reported to ban accounts that use it as an API key)"),
			)

		case profileClaudeAPI:
			return fmt.Sprintf(
				"%d. %s - %s %s", i+1, "Claude API Key",
				pterm.Red("expensive"),
				pterm.Green("(key stays on the host; the sandbox never sees it)"),
			)

		case profileOpencode:
			return fmt.Sprintf(
				"%d. %s - %s %s", i+1, "OpenCode",
				pterm.Green("any provider"),
				pterm.Green("(key stays on the host; the sandbox never sees it)"),
			)

		default:
			return fmt.Sprintf("%d. %s", i+1, harness)
		}
	})
	if err != nil {
		return err
	}

	fmt.Println()
	if err = fs.MkdirAll(dir); err != nil {
		return nil
	}
	fmt.Printf("created %s directory\n", dir)

	specs := scf.ToSandboxSpecs(h.Value)

	var b []byte
	b, err = nestor.MarshallSandboxSpecs(specs)
	if err != nil {
		return err
	}
	if err = fs.WriteFile(filepath.Join(dir, "sandbox.yml"), b, 0644); err != nil {
		return err
	}
	fmt.Printf("saved %s\n", filepath.Join(dir, "sandbox.yml"))

	config = cli.DefaultConfig(wd)
	switch h.Value {
	case profileClaudeOAuth:
		config.Hidden.Profiles = []string{profileClaudeAPI, profileOpencode}
		config.Hidden.Harnesses = []string{string(nestor.HarnessOpenCode)}
	case profileClaudeAPI:
		config.Hidden.Profiles = []string{profileClaudeOAuth, profileOpencode}
		config.Hidden.Harnesses = []string{string(nestor.HarnessOpenCode)}
	case profileOpencode:
		config.Hidden.Profiles = []string{profileClaudeOAuth, profileClaudeAPI}
		config.Hidden.Harnesses = []string{string(nestor.HarnessClaudeCode)}
	}
	b, err = yaml.Marshal(config)
	if err = fs.WriteFile(filepath.Join(wd, ".nestor.yml"), b, 0644); err != nil {
		return err
	}
	fmt.Printf("saved %s\n", filepath.Join(wd, ".nestor.yml"))

	if _, err = nestor.New(nestor.WithDir(dir)); err != nil {
		return err
	}
	fmt.Println("initialized nestor")
	fmt.Println()

	profileYmlPath := filepath.Join(dir, "profile.yml")
	fmt.Printf("Setup complete. %s\n", pterm.Yellow("Next: add your API key or OAuth token to "+profileYmlPath))
	fmt.Println()

	fmt.Printf("After that, start your coding agent in a sandbox with: %s\n", pterm.Cyan("nestor launch"))
	fmt.Println()
	fmt.Println(pterm.White("Add .nestor/ and .nestor.yml to your .gitignore; they hold local config and secrets, and don't belong in the repo."))
	fmt.Println()
	fmt.Println("For persistent sessions, consider " + pterm.Blue("tmux") + " or " + pterm.Blue("shpool") + ".")
	fmt.Println()
	return nil
}

type sandboxCfg struct {
	maxInstances      int
	worktree          bool
	shareHarnessState bool
	path              string
}

func (s *sandboxCfg) ToSandboxSpecs(profile string) []nestor.SandboxSpec {
	return []nestor.SandboxSpec{s.toSandboxSpec(profile)}
}

func (s *sandboxCfg) toSandboxSpec(profile string) nestor.SandboxSpec {
	stateScope := nestor.StateScopeInstance
	if s.shareHarnessState {
		stateScope = nestor.StateScopeShared
	}
	switch profile {
	case profileOpencode:
		return nestor.SandboxSpec{
			Name:         profile,
			Harness:      nestor.HarnessOpenCode,
			StateScope:   stateScope,
			Profile:      profile,
			Target:       "base",
			MaxInstances: s.maxInstances,
			Mounts:       s.Mounts(),
		}

	default:
		return nestor.SandboxSpec{
			Name:         profile,
			Harness:      nestor.HarnessClaudeCode,
			StateScope:   stateScope,
			Profile:      profile,
			Target:       "base",
			MaxInstances: s.maxInstances,
			Mounts:       s.Mounts(),
		}

	}
}

func (s *sandboxCfg) Mounts() []nestor.SandboxSpecMount {
	if !s.worktree {
		return []nestor.SandboxSpecMount{{Type: nestor.MountTypeDirect, Path: s.path}}
	}
	return []nestor.SandboxSpecMount{{Type: nestor.MountTypeGitWorktree, Path: s.path}}
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

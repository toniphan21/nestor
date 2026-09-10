package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
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

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	config := readConfig()
	if fs.HasDir(config.Dir) {
		fmt.Printf(alreadySetUp, filepath.Join(wd, config.Dir))

		fmt.Println(pterm.Green("done"))
		return nil
	}

	text := fmt.Sprintf(
		"Setting up nestor in the current directory: %s. Continue?",
		wd,
	)
	result, err := pterm.DefaultInteractiveConfirm.WithDefaultText(text).Show()
	if err != nil {
		return err
	}
	fmt.Println(result)

	return nil
}

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type promptInfo struct {
	spec   string
	path   string
	prompt string
	run    bool
}

func collectPromptInfo(api nestor.API, args []string) (*promptInfo, error) {
	// TODO: handle args
	allSpecs := api.Runtime().Registry.SandboxSpecs()
	if len(allSpecs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to prompt"))
		fmt.Println(pterm.Green("done"))
		return &promptInfo{run: false}, nil
	}

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

	var paths []string
	for _, v := range spec.Mounts {
		paths = append(paths, v.Path)
	}

	if len(paths) == 0 {
		fmt.Println(pterm.Red("no sandbox mounts found"))
		fmt.Println(pterm.Green("done"))
		return &promptInfo{run: false}, nil
	}

	var selectedPath string
	if len(paths) > 1 {
		r, err := Select(paths, "select path", func(i int, s string) string {
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

	p, err := Input("prompt", "")
	if err != nil {
		return nil, err
	}

	return &promptInfo{spec: spec.Name, path: selectedPath, prompt: p, run: true}, nil
}

func Prompt(api nestor.API, args []string) error {
	info, err := collectPromptInfo(api, args)
	if err != nil {
		return err
	}
	if !info.run {
		return nil
	}

	return DoPrompt(api, info.spec, info.path, info.prompt)
}

func DoPrompt(api nestor.API, spec, path, prompt string) error {
	ctx := context.Background()

	lease, err := api.Acquire(ctx, spec, path)
	if err != nil {
		return fmt.Errorf("acquire lease: %w", err)
	}
	if err = lease.Extend(ctx); err != nil {
		return fmt.Errorf("extend lease: %w", err)
	}

	_, err = lease.Run(ctx, prompt, nestor.RunOption{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})

	_ = lease.Release(ctx)
	if err != nil {
		return fmt.Errorf("run lease: %w", err)
	}
	return nil
}

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli/claude"
	"nhatp.com/go/nestor/cli/opencode"
)

type promptInfo struct {
	spec  string
	path  string
	model string
	run   bool
}

func collectPromptInfo(api nestor.API, args []string) (*promptInfo, error) {
	allSpecs := api.Runtime().Registry.SandboxSpecs()
	if len(allSpecs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to prompt"))
		return &promptInfo{run: false}, nil
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

	var paths []string
	for _, v := range spec.Mounts {
		paths = append(paths, v.Path)
	}

	if len(paths) == 0 {
		fmt.Println(pterm.Red("no sandbox mounts found"))
		return &promptInfo{run: false}, nil
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

	profile, have := api.Runtime().Registry.Profile(spec.Profile)
	if !have {
		return &promptInfo{run: false}, err
	}

	models := profile.SupportedModelsWithoutDefault()
	var selectedModel string
	if len(models) == 0 {
		selectedModel = profile.DefaultModel
		fmt.Printf("use model %s\n", pterm.Cyan(selectedModel))
	} else {
		models = slices.Insert(models, 0, profile.DefaultModel)
		dt := fmt.Sprintf("Select model (%d available)", len(models))
		r, err := Select(models, dt, func(i int, s string) string {
			return fmt.Sprintf("%d. %s", i+1, s)
		})
		if err != nil {
			return nil, err
		}
		selectedModel = r.Value
	}

	return &promptInfo{spec: spec.Name, path: selectedPath, model: selectedModel, run: true}, nil
}

func Prompt(api nestor.API, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	info, err := collectPromptInfo(api, args)
	if err != nil {
		return err
	}
	if !info.run {
		fmt.Println(pterm.Green("done"))
		return nil
	}

	return DoPrompt(ctx, api, info.spec, info.path, info.model)
}

func DoPrompt(ctx context.Context, api nestor.API, spec, path, model string) error {
	lease, err := api.Acquire(ctx, spec, path)
	if err != nil {
		return fmt.Errorf("acquire lease: %w", err)
	}
	if err = lease.Extend(ctx); err != nil {
		return fmt.Errorf("extend lease: %w", err)
	}

	var stdout io.Writer
	switch lease.Sandbox().Harness().Name() {
	case string(nestor.HarnessClaudeCode):
		stdout = claude.NewWriter()
	case string(nestor.HarnessOpenCode):
		stdout = opencode.NewWriter()
	default:
		stdout = os.Stdout
	}

	var returnErr error
	defer func() {
		if err := lease.Release(ctx); err != nil {
			returnErr = fmt.Errorf("run lease: %w", err)
			return
		}

		fmt.Println(pterm.Green("done"))
		returnErr = nil
	}()

	for {
		select {
		case <-ctx.Done():
			return returnErr

		default:
			prompt, err := Input("Prompt", "")
			if err != nil {
				if errors.Is(err, ErrInterrupted) {
					return nil
				}
				return returnErr
			}

			switch prompt {
			case "":
				continue
			case "exit":
				return returnErr
			default:
				fmt.Println(pterm.Blue(fmt.Sprintf("running in container %s...", lease.Sandbox().Container())))

				if err = lease.Extend(ctx); err != nil {
					return fmt.Errorf("extend lease: %w", err)
				}
				_, err = lease.Run(ctx, prompt, nestor.RunOption{Stdout: stdout, Model: model})
				fmt.Println("")
			}
		}
	}
}

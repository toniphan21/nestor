package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
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
	var err error
	var spec nestor.SandboxSpec
	if v, err := collectSpec(api); err != nil || v == nil {
		return &promptInfo{run: false}, err
	} else {
		spec = *v
	}

	var selectedPath string
	if v, err := collectPath(spec); err != nil || v == nil {
		return &promptInfo{run: false}, err
	} else {
		selectedPath = *v
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
	fmt.Printf("\n%s %s\n\n", pterm.Yellow("Note: nestor prompt is only a demo of a headless call. Headless mode isn't practical from a standalone binary — the real use is"), pterm.Blue("nestor launch"))

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

	defer func() {
		if rErr := lease.Release(ctx); err != nil {
			err = errors.Join(err, fmt.Errorf("release lease: %w", rErr))
		} else if err == nil {
			fmt.Println(pterm.Green("done"))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		default:
			prompt, err := Input("Prompt", "")
			if err != nil {
				if errors.Is(err, ErrInterrupted) {
					return nil
				}
				return err
			}

			switch prompt {
			case "":
				continue
			case "exit":
				return nil
			default:
				fmt.Println(pterm.Blue(fmt.Sprintf("running in container %s...", lease.Sandbox().Container())))

				if err = lease.Extend(ctx); err != nil {
					return fmt.Errorf("extend lease: %w", err)
				}

				_, err = lease.Run(ctx, nestor.Headless{Prompt: prompt, Stdout: stdout, Model: model})
				fmt.Println("")
			}
		}
	}
}

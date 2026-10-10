package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type BuildArgs struct {
	SandboxSpecs []string
	NoCache      bool
}

func Build(api nestor.API, args BuildArgs) error {
	runtime := api.Runtime()
	allSpecs := runtime.Registry.SandboxSpecs()

	runs := make(map[string]bool)
	exists := make(map[string]bool)
	for _, v := range allSpecs {
		exists[v.Name] = true
	}

	if len(args.SandboxSpecs) > 0 {
		for _, v := range args.SandboxSpecs {
			if !exists[v] {
				continue
			}
			runs[v] = true
		}
	} else {
		for k := range exists {
			runs[k] = true
		}
	}

	if len(runs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to build"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	ctx := context.Background()
	for v := range runs {
		spec, have := runtime.Registry.SandboxSpec(v)
		if !have {
			continue
		}

		tag := spec.DockerImageName(runtime.Template)
		fmt.Printf("building docker image for spec %s with tag %s...\n", pterm.Blue(v), pterm.Cyan(tag))
		opt := nestor.DockerBuildOption{
			NoCache: args.NoCache,
			Stdout:  os.Stdout,
			Stderr:  os.Stderr,
			OnStart: func(cmdArgs []string) {
				fmt.Fprintln(os.Stderr, pterm.Magenta("docker "+strings.Join(cmdArgs, " ")))
			},
		}
		if err := spec.Build(ctx, runtime, runtime.Docker(), opt); err != nil {
			return err
		}
	}
	fmt.Println(pterm.Green("done"))
	return nil
}

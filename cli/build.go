package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

type BuildArgs struct {
	SandboxSpecs []string
}

func Build(api nestor.API, args BuildArgs) error {
	allSpecs := api.Runtime().Registry.SandboxSpecs()

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
		spec, have := api.Runtime().Registry.SandboxSpec(v)
		if !have {
			continue
		}

		tag := spec.DockerImageName(api.Runtime().Template)
		fmt.Printf("building docker image for spec %s with tag %s...\n", pterm.Blue(v), pterm.Cyan(tag))
		err := api.Build(ctx, v)
		if err != nil {
			return err
		}
	}
	fmt.Println(pterm.Green("done"))
	return nil
}

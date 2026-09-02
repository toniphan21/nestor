package cli

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func Build(api nestor.API, specs []string) error {
	allSpecs := api.Runtime().Registry.SandboxSpecs()

	runs := make(map[string]bool)
	exists := make(map[string]bool)
	for _, v := range allSpecs {
		exists[v.Name] = true
	}

	if len(specs) > 0 {
		for _, v := range specs {
			if !exists[v] {
				continue
			}
			runs[v] = true
		}
	} else {
		for k, _ := range exists {
			runs[k] = true
		}
	}

	if len(runs) == 0 {
		fmt.Println(pterm.Yellow("no sandbox specs to build"))
		fmt.Println(pterm.Green("done"))
		return nil
	}

	ctx := context.Background()
	template := api.Runtime().Template
	for spec, _ := range runs {
		tag := template.MakeSandboxTag(spec)
		fmt.Printf("building docker image for spec %s with tag %s...\n", pterm.Blue(spec), pterm.Cyan(tag))
		err := api.Build(ctx, spec)
		if err != nil {
			return err
		}
	}
	fmt.Println(pterm.Green("done"))
	return nil

}

package main

import (
	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/internal/nx"
)

func down() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: shortDesc["down"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			names := nx.Dedup(argv)

			return cli.Down(api, cli.DownArgs{Sandboxes: names})
		}),
	}
	return withGlobalFlags(cmd)
}

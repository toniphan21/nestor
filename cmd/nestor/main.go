package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/infra/fs"
)

var shortDesc = map[string]string{
	"root":    "Inspect and manage the nestor directory",
	"setup":   "Initialize the nestor directory",
	"destroy": "Remove all sandboxes, images",
	"build":   "Build a sandbox image from a spec",
	"launch":  "Start an interactive harness session in a sandbox",
	"prompt":  "Run a prompt in a sandbox",
	"down":    "Stop all running sandboxes so the next run picks up config changes",
	"release": "Release the lease held on a sandbox",
	"view":    "Show the current nestor state",
	"explain": "Show the nestor architecture and how it works",
	"version": "Print the nestor version",
}

func main() {
	root := &cobra.Command{
		Use: "nestor", Short: shortDesc["root"],
		RunE: func(cmd *cobra.Command, args []string) error {
			config := readConfig()
			switch config.Root {
			case "view":
				fn := runWithAPI(view)
				return fn(cmd, args)
			case "explain":
				fn := runWithAPI(explain)
				return fn(cmd, args)
			case "launch":
				fn := runWithAPI(launch)
				return fn(cmd, args)
			default:
				return cmd.Usage()
			}
		},
	}

	root.AddCommand(
		&cobra.Command{Use: "version", Short: shortDesc["version"], Run: printVersion},

		command("setup", setup),
		command("destroy", destroy),
		command("build", build),
		commandWithAlias("launch", []string{"open", "start"}, launch),
		command("prompt", prompt),
		command("down", down),
		command("release", release),
		command("view", view),
		command("explain", explain),
	)

	if err := withDirFlag(root).Execute(); err != nil {
		os.Exit(1)
	}
}

func withDirFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP("dir", "d", "", "nestor directory; use NESTOR_DIR if not specified")
	return cmd
}

func readConfig() *cli.Config {
	var config *cli.Config
	if wd, err := os.Getwd(); err == nil {
		if c, err := fs.AtomicReadFile(filepath.Join(wd, ".nestor.yml")); err == nil {
			_ = yaml.Unmarshal(c, &config)
		}
	}
	return config
}

func runWithAPI(fn func(nestor.API, *cli.Config, ...string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		config := readConfig()

		dir, err := cmd.Flags().GetString("dir")
		if err != nil {
			fmt.Println(pterm.Red("cannot read the --dir flag ", err.Error()))
			return nil
		}

		if config != nil && strings.TrimSpace(config.Dir) != "" {
			dir = config.Dir
		}

		if strings.TrimSpace(dir) != "" {
			abs, err := filepath.Abs(dir)
			if err != nil {
				fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot resolve absolute path %q", err.Error(), dir)))
				return nil
			}
			dir = abs
		}

		options := []nestor.Option{nestor.WithDir(dir)}
		logger, closer, err := nestor.DefaultLogger(slog.LevelDebug, options...)
		if err != nil {
			fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot create a logger %q", err.Error(), dir)))
			return nil
		}
		defer closer.Close()

		api, err := nestor.New(append(options, nestor.WithLogger(logger))...)
		if err != nil {
			fmt.Println(pterm.Red("cannot create nestor API: ", err.Error()))
			return nil
		}

		err = fn(api, config, args...)
		if err != nil {
			fmt.Println(pterm.Red("Error: ", err.Error()))
		}
		return nil
	}
}

func command(name string, fn func(nestor.API, *cli.Config, ...string) error) *cobra.Command {
	return withDirFlag(&cobra.Command{
		Use: name, Short: shortDesc[name],
		RunE: runWithAPI(fn),
	})
}

func commandWithAlias(name string, aliases []string, fn func(nestor.API, *cli.Config, ...string) error) *cobra.Command {
	return withDirFlag(&cobra.Command{
		Use: name, Short: shortDesc[name],
		Aliases: aliases,
		RunE:    runWithAPI(fn),
	})
}

func printVersion(cmd *cobra.Command, args []string) {
	fmt.Println(nestor.Version)
}

func setup(api nestor.API, cf *cli.Config, args ...string) error {
	fmt.Println("setup is not implemented yet - TODO", api.Runtime().Platform.NestorDir())
	return nil
}

func destroy(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Destroy(api)
}

func build(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Build(api, args)
}

func launch(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Launch(api, args)
}

func prompt(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Prompt(api, args)
}

func down(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Down(api)
}

func release(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Release(api, args)
}

func view(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.View(api, cf)
}

func explain(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Explain(api, cf)
}

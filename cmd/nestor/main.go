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
	"root":    "Run coding agents in sandboxed containers",
	"setup":   "Initialize the nestor directory",
	"destroy": "Remove all sandboxes, images",
	"build":   "Build a sandbox image from a spec",
	"prompt":  "Run a headless prompt in a sandbox (demo of library usage)",

	"launch": "Start an interactive harness session in a sandbox", // migrated

	"spec":         "Manage sandbox specs",
	"spec-down":    "Stop running sandboxes (all if none given)", // migrated
	"spec-release": "Release the lease held on a sandbox",        // migrated

	"sandbox":        "Manage sandboxes",
	"sandbox-rename": "Give a sandbox a memorable name", // migrated
	"sandbox-delete": "Remove a single sandbox",         // migrated

	"version": "Print the nestor version",
}

func main() {
	root := &cobra.Command{
		Use: "nestor", Short: shortDesc["root"],
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	spec := newSpecCmd()
	sandbox := newSandboxCmd()
	launch := newLaunchCmd()

	root.AddCommand(
		&cobra.Command{Use: "version", Short: shortDesc["version"], Run: printVersion},
		spec.root,
		spec.down,
		spec.release,

		sandbox.root,

		cmdSetup(),

		command("destroy", destroy),
		command("build", build),
		launch,
		command("prompt", prompt),
	)

	if err := withDirFlag(root).Execute(); err != nil {
		os.Exit(1)
	}
}

func run(fn func(nestor.API, *cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, argv []string) error {
		var verbose bool
		if v, err := cmd.Flags().GetBool("verbose"); err == nil {
			verbose = v
		}

		ll := slog.LevelInfo
		if verbose {
			ll = slog.LevelDebug
		}

		logger, closer, err := nestor.DefaultLogger(ll)
		if err != nil {
			fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot create a logger", err.Error())))
			return nil
		}
		defer func() {
			if cErr := closer.Close(); cErr != nil {
				fmt.Println(pterm.Red(cErr.Error()))
			}
		}()

		api, err := nestor.New(nestor.WithLogger(logger))
		if err != nil {
			fmt.Println(pterm.Red("cannot create nestor API: ", err.Error()))
			return nil
		}

		err = fn(api, cmd, argv)
		if err != nil {
			fmt.Println(pterm.Red("Error: ", err.Error()))
		}
		return nil
	}
}

func withGlobalFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP("verbose", "v", false, "set log level to debug")
	return cmd
}

func withDirFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP("dir", "d", "", "nestor directory")
	return cmd
}

func readConfig() *cli.Config {
	var config *cli.Config
	wd, err := os.Getwd()
	if err != nil {
		return cli.DefaultConfig("")
	}

	if c, err := fs.AtomicReadFile(filepath.Join(wd, ".nestor.yml")); err == nil {
		_ = yaml.Unmarshal(c, &config)
	}
	if config == nil {
		config = cli.DefaultConfig(wd)
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

		options := localDirOptions(dir)
		if err = fs.MkdirAll(dir); err != nil {
			fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot create work dir %q", err.Error(), dir)))
			return nil
		}

		var ll slog.Level
		switch config.LogLevel {
		case "debug":
			ll = slog.LevelDebug
		case "error":
			ll = slog.LevelError
		case "warn":
			ll = slog.LevelWarn
		default:
			ll = slog.LevelInfo
		}
		logger, closer, err := nestor.DefaultLogger(ll, options...)
		if err != nil {
			fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot create a logger %q", err.Error(), dir)))
			return nil
		}
		defer func() {
			if cErr := closer.Close(); cErr != nil {
				fmt.Println(pterm.Red(cErr.Error()))
			}
		}()

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

func printVersion(cmd *cobra.Command, args []string) {
	fmt.Println(nestor.Version)
}

func destroy(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Destroy(api)
}

func build(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Build(api, args)
}

func prompt(api nestor.API, cf *cli.Config, args ...string) error {
	return cli.Prompt(api, args)
}

func localDirOptions(dir string) []nestor.Option {
	return []nestor.Option{
		nestor.WithConfigDir(filepath.Join(dir, "config")),
		nestor.WithDataDir(filepath.Join(dir, "data")),
		nestor.WithStateDir(filepath.Join(dir, "state")),
	}
}

package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cmd/nestor/tui"
)

var shortDesc = map[string]string{
	"root":  "Run coding agents in sandboxed containers",
	"setup": "Initialize the nestor directory",

	"spec":         "Manage sandbox specs",
	"spec-build":   "Build a sandbox image from a spec",
	"spec-launch":  "Start an interactive harness session in a sandbox",
	"spec-prompt":  "Run a headless prompt in a sandbox (demo of library usage)",
	"spec-release": "Release the lease held on a sandbox",

	"sandbox":        "Manage sandboxes",
	"sandbox-delete": "Remove a single sandbox",
	"sandbox-down":   "Stop running sandboxes (all if none given)",
	"sandbox-rename": "Give a sandbox a memorable name",
	"sandbox-up":     "Start sandboxes (all if none given)",

	"version": "Print the nestor version",
}

func main() {
	root := &cobra.Command{
		Use:   "nestor",
		Short: shortDesc["root"],
		RunE:  runTUI,
	}

	spec := newSpecCmd()
	sandbox := newSandboxCmd()
	setup := setupCmd()

	root.AddCommand(
		spec.root,
		spec.build,
		spec.launch,
		spec.prompt,
		spec.release,

		sandbox.root,
		sandbox.down,

		setup,

		&cobra.Command{
			Use:   "version",
			Short: shortDesc["version"],
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println(nestor.Version)
			},
		},
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(fn func(nestor.API, *cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, argv []string) error {
		logger, closer, err := newLogger(cmd)
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

func runTUI(cmd *cobra.Command, argv []string) error {
	logger, closer, err := newLogger(cmd)
	if err != nil {
		fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot create a logger", err.Error())))
		return nil
	}
	defer func() {
		if cErr := closer.Close(); cErr != nil {
			fmt.Println(pterm.Red(cErr.Error()))
		}
	}()

	opts := []nestor.Option{nestor.WithLogger(logger)}
	provider := tui.NewAPIProvider(func() (nestor.API, error) {
		return nestor.New(opts...)
	})

	if err := tui.Run(provider); err != nil {
		fmt.Println(pterm.Red("Error: ", err.Error()))
	}
	return nil
}

func newLogger(cmd *cobra.Command) (*slog.Logger, io.Closer, error) {
	var verbose bool
	if v, err := cmd.Flags().GetBool("verbose"); err == nil {
		verbose = v
	}

	ll := slog.LevelInfo
	if verbose {
		ll = slog.LevelDebug
	}
	return nestor.DefaultLogger(ll)
}

func withGlobalFlags(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().BoolP("verbose", "v", false, "set log level to debug")
	return cmd
}

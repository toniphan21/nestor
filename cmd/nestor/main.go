package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
)

// nestor [view] -d|--dir
// nestor setup -d|--dir
// nestor destroy -d|--dir
// nestor prompt -d|--dir -s|--spec
// nestor build -d|--dir -s|--spec
// nestor down -d|--dir
// nestor proxy -d|--dir
// nestor view -d|--dir
// nestor version

var shortDesc = map[string]string{
	"root":    "Inspect and manage the nestor directory",
	"setup":   "Initialize the nestor directory",
	"destroy": "Remove all sandboxes, images, and the nestor directory",
	"build":   "Build image from a spec",
	"prompt":  "Run a prompt in the sandbox",
	"down":    "Stop all sandboxes so the next run picks up config changes",
	"proxy":   "Run built-in reverse proxy for host",
	"view":    "Show all nestor state",
	"version": "Print the nestor version",
}

func main() {
	root := &cobra.Command{
		Use: "nestor", Short: shortDesc["root"],
		RunE: runWithAPI(view),
	}

	root.AddCommand(
		&cobra.Command{Use: "version", Short: shortDesc["version"], Run: printVersion},

		command("setup", setup),
		command("destroy", destroy),
		command("build", build),
		command("prompt", prompt),
		command("down", down),
		command("proxy", proxy),
		command("view", view),
	)

	if err := withDirFlag(root).Execute(); err != nil {
		os.Exit(1)
	}
}

func withDirFlag(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().StringP("dir", "d", "", "nestor directory; use NESTOR_DIR if not specified")
	return cmd
}

func runWithAPI(fn func(nestor.API, ...string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		dir, err := cmd.Flags().GetString("dir")
		if err != nil {
			return err
		}

		api, err := nestor.New(nestor.WithDir(dir))
		if err != nil {
			return err
		}
		return fn(api, args...)
	}
}

func command(name string, fn func(nestor.API, ...string) error) *cobra.Command {
	return withDirFlag(&cobra.Command{
		Use: name, Short: shortDesc[name],
		RunE: runWithAPI(fn),
	})
}

func printVersion(cmd *cobra.Command, args []string) {
	fmt.Println(nestor.Version)
}

func setup(api nestor.API, args ...string) error {
	fmt.Println("setup", api.Runtime().Platform.NestorDir())
	return nil
}

func destroy(api nestor.API, args ...string) error {
	fmt.Println("setup", api.Runtime().Platform.NestorDir())
	return nil
}

func build(api nestor.API, args ...string) error {
	fmt.Println("build", api.Runtime().Platform.NestorDir())
	return nil
}

func prompt(api nestor.API, args ...string) error {
	fmt.Println("prompt", api.Runtime().Platform.NestorDir())
	return nil
}

func down(api nestor.API, args ...string) error {
	fmt.Println("down", api.Runtime().Platform.NestorDir())
	return nil
}

func proxy(api nestor.API, args ...string) error {
	fmt.Println("proxy", api.Runtime().Platform.NestorDir())
	return nil
}

func view(api nestor.API, args ...string) error {
	fmt.Println("view", api.Runtime().Platform.NestorDir())
	return nil
}

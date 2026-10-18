package main

import (
	"strings"

	"github.com/spf13/cobra"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
	"nhatp.com/go/nestor/internal/nx"
)

type specCmd struct {
	root    *cobra.Command
	build   *cobra.Command
	launch  *cobra.Command
	prompt  *cobra.Command
	release *cobra.Command
}

func newSpecCmd() *specCmd {
	root := &cobra.Command{
		Use:   "spec",
		Short: shortDesc["spec"],
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	build := &cobra.Command{
		Use:   "build",
		Short: shortDesc["spec-build"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			noCache, err := cmd.Flags().GetBool("no-cache")
			if err != nil {
				return err
			}
			names := nx.Dedup(argv)
			return cli.Build(api, cli.BuildArgs{SandboxSpecs: names, NoCache: noCache})
		}),
	}
	build.Flags().Bool("no-cache", false, "do not use cache when building the image")

	launch := &cobra.Command{
		Use:     "launch",
		Short:   shortDesc["spec-launch"],
		Aliases: []string{"open", "start"},
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}
			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			session, err := cmd.Flags().GetString("session")
			if err != nil {
				return err
			}
			dryRun, err := cmd.Flags().GetString("dry-run")
			if err != nil {
				return err
			}

			return cli.Launch(api, cli.LaunchArgs{
				SandboxSpec: strings.TrimSpace(spec),
				Path:        strings.TrimSpace(path),
				Sandbox:     strings.TrimSpace(name),
				Session:     strings.TrimSpace(session),
				DryRun:      strings.TrimSpace(dryRun),
			})
		}),
	}
	launch.Flags().StringP("spec", "s", "", "name of the sandbox spec")
	launch.Flags().StringP("path", "p", "", "path")
	launch.Flags().StringP("name", "n", "", "sandbox name")
	launch.Flags().StringP("session", "r", "", "session id to resume")
	launch.Flags().String("dry-run", "", "dry run, writing the result as JSON to this file")

	prompt := &cobra.Command{
		Use:   "prompt",
		Short: shortDesc["spec-prompt"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}
			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}
			model, err := cmd.Flags().GetString("model")
			if err != nil {
				return err
			}
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			session, err := cmd.Flags().GetString("session")
			if err != nil {
				return err
			}
			title, err := cmd.Flags().GetString("title")
			if err != nil {
				return err
			}
			promptFile, err := cmd.Flags().GetString("prompt-file")
			if err != nil {
				return err
			}
			dryRun, err := cmd.Flags().GetString("dry-run")
			if err != nil {
				return err
			}

			return cli.Prompt(api, cli.PromptArgs{
				SandboxSpec: strings.TrimSpace(spec),
				Path:        strings.TrimSpace(path),
				Model:       strings.TrimSpace(model),
				Sandbox:     strings.TrimSpace(name),
				Session:     strings.TrimSpace(session),
				Title:       strings.TrimSpace(title),
				PromptFile:  strings.TrimSpace(promptFile),
				DryRun:      strings.TrimSpace(dryRun),
			})
		}),
	}
	prompt.Flags().StringP("spec", "s", "", "name of the sandbox spec")
	prompt.Flags().StringP("path", "p", "", "path")
	prompt.Flags().StringP("model", "m", "", "model")
	prompt.Flags().StringP("name", "n", "", "sandbox name")
	prompt.Flags().StringP("session", "r", "", "session id to resume")
	prompt.Flags().StringP("title", "t", "", "session title (for new session)")
	prompt.Flags().StringP("prompt-file", "f", "", "prompt file")
	prompt.Flags().String("dry-run", "", "dry run, writing the result as JSON to this file")

	release := &cobra.Command{
		Use:   "release",
		Short: shortDesc["spec-release"],
		RunE: run(func(api nestor.API, cmd *cobra.Command, argv []string) error {
			spec, err := cmd.Flags().GetString("spec")
			if err != nil {
				return err
			}
			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return err
			}
			return cli.Release(api, cli.ReleaseArgs{
				SandboxSpec: strings.TrimSpace(spec),
				Path:        strings.TrimSpace(path),
			})
		}),
	}
	release.Flags().StringP("spec", "s", "", "name of the sandbox spec")
	release.Flags().StringP("path", "p", "", "path")

	root.AddCommand(build, launch, prompt, release)

	return &specCmd{
		root:    root,
		build:   withGlobalFlags(build),
		launch:  withGlobalFlags(launch),
		prompt:  withGlobalFlags(prompt),
		release: withGlobalFlags(release),
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"nhatp.com/go/nestor"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	dir := filepath.Join(home, "github", "toniphan21", "nestor", "work", ".nestor")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("cannot make dir: %e", err)
	}

	ctx := context.Background()

	logger, closer, err := nestor.NewCLILogger(
		filepath.Join(dir, nestor.DefaultLogFile), os.Stdout, slog.LevelDebug,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer closer.Close()

	api, err := nestor.New(
		nestor.WithDir(dir),
		nestor.WithLogger(logger),
		nestor.WithGit(nestor.NoopGit),
		nestor.WithDocker(nestor.NoopDocker),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := api.Build(ctx, "claude-go"); err != nil {
		log.Fatal(err)
	}

	sb, err := api.Create(ctx, "claude-go")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(sb)

	sandboxes, err := api.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, sandbox := range sandboxes {
		fmt.Println(sandbox)
		if err := sandbox.Delete(ctx); err != nil {
			logger.Error(err.Error())
			log.Fatal(err)
		}
	}
}

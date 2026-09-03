package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	dir := filepath.Join(home, "github", "toniphan21", "nestor", "work", ".nestor")

	options := []nestor.Option{nestor.WithDir(dir)}
	logger, closer, err := nestor.DefaultLogger(slog.LevelDebug, options...)
	if err != nil {
		fmt.Println(pterm.Red(fmt.Sprintf("%s, cannot create a logger %q", err.Error(), dir)))
		log.Fatal(err)
	}
	defer closer.Close()

	api, err := nestor.New(append(options, nestor.WithLogger(logger))...)
	if err != nil {
		fmt.Println(pterm.Red("cannot create nestor API: ", err.Error()))
		log.Fatal(err)
	}

	ctx := context.Background()
	spec := "claude-go"
	path := "/Users/nhatp/github/toniphan21/nestor/work/chats"
	prompt := "what is golang?"

	lease, err := api.Acquire(ctx, spec, path)
	if err != nil {
		log.Fatal(fmt.Errorf("acquire lease: %w", err))
	}
	if err = lease.Extend(ctx); err != nil {
		log.Fatal(fmt.Errorf("extend lease: %w", err))
	}

	// read the output via Stdout
	reader := exec.Command("claude-chat-reader")
	w, err := reader.StdinPipe()

	if err != nil {
		log.Fatal(err)
	}
	reader.Stdout = os.Stdout
	reader.Stderr = os.Stderr

	if err := reader.Start(); err != nil {
		log.Fatal(err)
	}

	_, runErr := lease.Run(ctx, prompt, nestor.RunOption{
		Model:  "haiku",
		Stdout: w,
		Stderr: os.Stderr,
	})

	w.Close()
	readerErr := reader.Wait()

	err = lease.Release(ctx)
	if err != nil {
		log.Fatal(fmt.Errorf("release lease: %w", err))
	}
	if runErr != nil {
		log.Fatal(fmt.Errorf("run lease: %w", err))
	}
	if readerErr != nil {
		log.Fatal(fmt.Errorf("reader error: %w", readerErr))
	}
	fmt.Println("done")
}

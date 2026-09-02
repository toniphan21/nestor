package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"
	"nhatp.com/go/nestor"
	"nhatp.com/go/nestor/cli"
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

	spec := "claude-go"
	path := "/Users/nhatp/github/toniphan21/nestor/work/chats"
	prompt := "test"
	err = cli.DoPrompt(api, spec, path, prompt)
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"nhatp.com/go/nestor"
)

func main() {
	dir := "/Users/nhatp/github/toniphan21/nestor/work/.nestor"
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("cannot make dir: %e", err)
	}

	logger, closer, err := nestor.NewCLILogger(filepath.Join(dir, nestor.DefaultLogFile), os.Stdout, slog.LevelDebug)
	if err != nil {
		log.Fatal(err)
	}
	defer closer.Close()
	api, err := nestor.New(nestor.WithDir(dir), nestor.WithLogger(logger))
	if err != nil {
		log.Fatal(err)
	}

	err = api.Init()
	if err != nil {
		log.Fatal(err)
	}

	err = api.Build()
	if err != nil {
		log.Fatal(err)
	}
}

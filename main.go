package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"maragu.dev/redo/internal/redo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			return runInit()
		default:
			return fmt.Errorf("unknown command: %s", os.Args[1])
		}
	}

	return runWatch()
}

func runInit() error {
	const configFile = "redo.yaml"

	if _, err := os.Stat(configFile); err == nil {
		return errors.New(configFile + " already exists")
	}

	if err := os.WriteFile(configFile, []byte(sampleConfig), 0644); err != nil {
		return err
	}

	fmt.Println("Created", configFile)
	return nil
}

const sampleConfig = `commands:
  - name: "app"
    run: "go run ."
    watch:
      - "**/*.go"
      - "go.mod"
      - "go.sum"
      - ".env"
`

func runWatch() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r := redo.New(dir, cfg, os.Stdout)
	return r.Run(ctx)
}

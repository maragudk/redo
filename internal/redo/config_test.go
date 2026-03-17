package redo_test

import (
	"os"
	"path/filepath"
	"testing"

	"maragu.dev/is"

	"maragu.dev/redo/internal/redo"
)

func TestLoadConfig(t *testing.T) {
	t.Run("loads a valid config with multiple commands", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `
commands:
  - name: server
    run: go run .
    watch:
      - "**/*.go"
      - go.mod
      - .env

  - name: tailwind
    run: npx tailwindcss -i input.css -o static/styles.css
    watch:
      - "**/*.css"
      - "**/*.html"
`)

		cfg, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.NotError(t, err)
		is.Equal(t, 2, len(cfg.Commands))

		is.Equal(t, "server", cfg.Commands[0].Name)
		is.Equal(t, "go run .", cfg.Commands[0].Run)
		is.Equal(t, 3, len(cfg.Commands[0].Watch))
		is.Equal(t, "**/*.go", cfg.Commands[0].Watch[0])

		is.Equal(t, "tailwind", cfg.Commands[1].Name)
	})

	t.Run("returns an error for a missing file", func(t *testing.T) {
		_, err := redo.LoadConfig("/nope/not/here/redo.yaml")
		is.True(t, err != nil)
	})

	t.Run("returns an error for invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `][not yaml`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})

	t.Run("returns an error when no commands are defined", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `commands: []`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})

	t.Run("returns an error when a command has no name", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `
commands:
  - run: go run .
    watch: ["**/*.go"]
`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})

	t.Run("returns an error when a command has no run", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `
commands:
  - name: server
    watch: ["**/*.go"]
`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})

	t.Run("returns an error when a command has no watch patterns", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `
commands:
  - name: server
    run: go run .
`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})

	t.Run("returns an error for duplicate command names", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `
commands:
  - name: server
    run: go run .
    watch: ["**/*.go"]
  - name: server
    run: go run ./other
    watch: ["**/*.go"]
`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})

	t.Run("returns an error for an invalid glob pattern", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "redo.yaml"), `
commands:
  - name: server
    run: go run .
    watch: ["[unterminated"]
`)

		_, err := redo.LoadConfig(filepath.Join(dir, "redo.yaml"))
		is.True(t, err != nil)
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	is.NotError(t, err)
}

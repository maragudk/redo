# redo

[![Docs](https://pkg.go.dev/badge/maragu.dev/redo)](https://pkg.go.dev/maragu.dev/redo)
[![CI](https://github.com/maragudk/redo/actions/workflows/ci.yml/badge.svg)](https://github.com/maragudk/redo/actions/workflows/ci.yml)

A simple, robust file watcher that runs commands on changes. Like Air, but simpler and more reliable.

Made with sparkles by [maragu](https://www.maragu.dev/): independent software consulting for cloud-native Go apps & AI engineering.

[Contact me at markus@maragu.dk](mailto:markus@maragu.dk) for consulting work, or perhaps an invoice to support this project?

## Install

```shell
go install maragu.dev/redo@latest
```

## Usage

Generate a sample config file:

```shell
redo init
```

This creates a `redo.yaml` in the current directory:

```yaml
commands:
  - name: "app"
    run: "go run ."
    watch:
      - "**/*.go"
      - "go.mod"
      - "go.sum"
      - ".env"
```

Edit it to match your project, then run:

```shell
redo
```

All commands start immediately. When a file matching a command's watch patterns changes, that command is killed and restarted.

Output is prefixed with the command name in the terminal:

```
[redo] Starting app: go run .
[app] Listening on :8080
[redo] Restarting app (main.go changed)
[app] Listening on :8080
```

Each command's raw output (no prefix) is also written to `[name].log` in the project root. The log file is truncated on each restart.

### Multiple commands

```yaml
commands:
  - name: "server"
    run: "go run ."
    watch:
      - "**/*.go"
      - "go.mod"
      - "go.sum"
      - ".env"

  - name: "tailwind"
    run: "npx tailwindcss -i input.css -o static/styles.css"
    watch:
      - "**/*.css"
      - "**/*.html"
```

## Features

- Run different commands on different file type changes
- Glob patterns for watch paths (including `**` for recursive matching)
- Kills the entire process group on restart (no orphan processes)
- 50ms debounce to coalesce rapid file changes
- Prefixed output for grep-friendly logs
- Per-command `[name].log` files with raw output, truncated on restart

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

Create a `redo.yaml` in your project root:

```yaml
commands:
  - name: server
    run: go run .
    watch:
      - "**/*.go"
      - go.mod
      - go.sum
      - .env

  - name: tailwind
    run: npx tailwindcss -i input.css -o static/styles.css
    watch:
      - "**/*.css"
      - "**/*.html"
```

Then run:

```shell
redo
```

All commands start immediately. When a file matching a command's watch patterns changes, that command is killed and restarted.

Output is prefixed with the command name:

```
[redo] Starting server: go run .
[redo] Starting tailwind: npx tailwindcss -i input.css -o static/styles.css
[server] Listening on :8080
[tailwind] Done in 120ms
[redo] Restarting server (main.go changed)
[server] Listening on :8080
```

## Features

- Run different commands on different file type changes
- Glob patterns for watch paths (including `**` for recursive matching)
- Kills the entire process group on restart (no orphan processes)
- 50ms debounce to coalesce rapid file changes
- Prefixed output for grep-friendly logs

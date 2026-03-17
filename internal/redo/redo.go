package redo

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Runner watches for file changes and restarts commands.
type Runner struct {
	dir      string
	config   Config
	out      io.Writer
	commands []*command
	watcher  *fsnotify.Watcher
	stopped  atomic.Bool
}

// New creates a Runner for the given directory and config.
func New(dir string, config Config, out io.Writer) *Runner {
	return &Runner{
		dir:    dir,
		config: config,
		out:    &syncWriter{w: out},
	}
}

// Run starts all commands and watches for file changes until the context is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	r.commands = nil
	r.stopped.Store(false)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()
	r.watcher = watcher

	if err := r.addWatchDirs(); err != nil {
		return err
	}

	for _, cfg := range r.config.Commands {
		cmd := newCommand(cfg.Name, cfg.Run, r.dir, r.out)
		r.commands = append(r.commands, cmd)
		r.log("Starting %s: %s", cfg.Name, cfg.Run)
		if err := cmd.start(); err != nil {
			r.log("Error starting %s: %v", cfg.Name, err)
		}
	}

	timers := make([]*time.Timer, len(r.commands))

	for {
		select {
		case <-ctx.Done():
			r.stopped.Store(true)
			for _, timer := range timers {
				if timer != nil {
					timer.Stop()
				}
			}
			r.stopAll()
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}

			// Watch newly created directories.
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					relPath, relErr := filepath.Rel(r.dir, event.Name)
					if relErr == nil && !containsSkippedDir(relPath) {
						_ = watcher.Add(event.Name)
					}
				}
			}

			relPath, err := filepath.Rel(r.dir, event.Name)
			if err != nil {
				continue
			}

			for i, cfg := range r.config.Commands {
				if matchesAny(relPath, cfg.Watch) {
					if timers[i] != nil {
						timers[i].Stop()
					}
					idx := i
					name := cfg.Name
					file := relPath
					timers[idx] = time.AfterFunc(50*time.Millisecond, func() {
						if r.stopped.Load() {
							return
						}
						r.log("Restarting %s (%s changed)", name, file)
						if err := r.commands[idx].restart(); err != nil {
							r.log("Error restarting %s: %v", name, err)
						}
					})
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			r.log("Watcher error: %v", err)
		}
	}
}

func (r *Runner) log(format string, args ...any) {
	line := fmt.Sprintf("[redo] "+format+"\n", args...)
	_, _ = r.out.Write([]byte(line))
}

func (r *Runner) stopAll() {
	for _, cmd := range r.commands {
		cmd.stop()
		cmd.closeLog()
	}
}

func (r *Runner) addWatchDirs() error {
	return filepath.WalkDir(r.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			r.log("Warning: cannot access %s: %v", path, err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			if err := r.watcher.Add(path); err != nil {
				r.log("Warning: cannot watch %s: %v", path, err)
				return nil
			}
		}
		return nil
	})
}

func shouldSkipDir(name string) bool {
	return name == ".git" || name == "node_modules"
}

func containsSkippedDir(relPath string) bool {
	dir := relPath
	for dir != "." && dir != "" {
		if shouldSkipDir(filepath.Base(dir)) {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

// syncWriter wraps an io.Writer with a mutex to make concurrent writes safe.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *syncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

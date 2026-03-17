package redo_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"maragu.dev/is"

	"maragu.dev/redo/internal/redo"
)

func TestRunner(t *testing.T) {
	t.Run("starts all commands on startup and prefixes output", func(t *testing.T) {
		dir := t.TempDir()

		var buf syncBuf
		r := redo.New(dir, redo.Config{
			Commands: []redo.CommandConfig{
				{Name: "greeter", Run: "echo hello from greeter", Watch: []string{"**/*.go"}},
				{Name: "other", Run: "echo hello from other", Watch: []string{"**/*.txt"}},
			},
		}, &buf)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			done <- r.Run(ctx)
		}()

		// Wait for startup output
		waitFor(t, &buf, "[greeter] hello from greeter", 2*time.Second)
		waitFor(t, &buf, "[other] hello from other", 2*time.Second)
		waitFor(t, &buf, "[redo] Starting greeter:", 2*time.Second)
		waitFor(t, &buf, "[redo] Starting other:", 2*time.Second)

		cancel()
		is.NotError(t, <-done)
	})

	t.Run("restarts a command when a matching file changes", func(t *testing.T) {
		dir := t.TempDir()

		// Create a file so the watcher has something to watch
		writeFile(t, filepath.Join(dir, "main.go"), "package main")

		var buf syncBuf
		r := redo.New(dir, redo.Config{
			Commands: []redo.CommandConfig{
				{Name: "builder", Run: "echo building", Watch: []string{"**/*.go"}},
			},
		}, &buf)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			done <- r.Run(ctx)
		}()

		// Wait for initial startup
		waitFor(t, &buf, "[builder] building", 2*time.Second)

		// Modify a watched file
		writeFile(t, filepath.Join(dir, "main.go"), "package main // changed")

		// Wait for restart
		waitFor(t, &buf, "[redo] Restarting builder", 2*time.Second)

		// Should have two "building" outputs (initial + restart)
		waitForCount(t, &buf, "[builder] building", 2, 2*time.Second)

		cancel()
		is.NotError(t, <-done)
	})

	t.Run("does not restart when a non-matching file changes", func(t *testing.T) {
		dir := t.TempDir()

		var buf syncBuf
		r := redo.New(dir, redo.Config{
			Commands: []redo.CommandConfig{
				{Name: "goonly", Run: "echo go-build", Watch: []string{"**/*.go"}},
			},
		}, &buf)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			done <- r.Run(ctx)
		}()

		waitFor(t, &buf, "[goonly] go-build", 2*time.Second)

		// Write a CSS file (should not trigger restart)
		writeFile(t, filepath.Join(dir, "style.css"), "body{}")

		// Give some time for a potential false restart
		time.Sleep(200 * time.Millisecond)

		output := buf.String()
		is.Equal(t, 1, strings.Count(output, "[goonly] go-build"))

		cancel()
		is.NotError(t, <-done)
	})

	t.Run("only restarts the command whose watch patterns match", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "main.go"), "package main")

		var buf syncBuf
		r := redo.New(dir, redo.Config{
			Commands: []redo.CommandConfig{
				{Name: "server", Run: "echo server-start", Watch: []string{"**/*.go"}},
				{Name: "css", Run: "echo css-build", Watch: []string{"**/*.css"}},
			},
		}, &buf)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			done <- r.Run(ctx)
		}()

		waitFor(t, &buf, "[server] server-start", 2*time.Second)
		waitFor(t, &buf, "[css] css-build", 2*time.Second)

		// Change a Go file - only server should restart
		writeFile(t, filepath.Join(dir, "main.go"), "package main // v2")

		waitForCount(t, &buf, "[server] server-start", 2, 2*time.Second)

		// Give time to ensure CSS didn't restart
		time.Sleep(200 * time.Millisecond)
		is.Equal(t, 1, strings.Count(buf.String(), "[css] css-build"))

		cancel()
		is.NotError(t, <-done)
	})

	t.Run("captures stderr from a command that fails", func(t *testing.T) {
		dir := t.TempDir()

		var buf syncBuf
		r := redo.New(dir, redo.Config{
			Commands: []redo.CommandConfig{
				{Name: "broken", Run: "/nonexistent/binary/that/does/not/exist", Watch: []string{"**/*.go"}},
			},
		}, &buf)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			done <- r.Run(ctx)
		}()

		// sh -c starts fine, but the command inside fails and outputs to stderr.
		// The error message varies by OS, so just check the prefix is there.
		waitFor(t, &buf, "[broken] sh:", 2*time.Second)

		cancel()
		is.NotError(t, <-done)
	})
}

// syncBuf is a concurrency-safe buffer for test output.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// waitFor polls the buffer until it contains the expected string or times out.
func waitFor(t *testing.T, buf *syncBuf, expected string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), expected) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in output:\n%s", expected, buf.String())
}

// waitForCount polls until the expected string appears at least count times.
func waitForCount(t *testing.T, buf *syncBuf, expected string, count int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Count(buf.String(), expected) >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d occurrences of %q in output:\n%s", count, expected, buf.String())
}

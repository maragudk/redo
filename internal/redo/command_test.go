package redo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"maragu.dev/is"
)

func TestCommand(t *testing.T) {
	t.Run("starts a command and captures prefixed output", func(t *testing.T) {
		var buf safeBuffer
		cmd := newCommand("test", "echo hello world", t.TempDir(), &buf)

		err := cmd.start()
		is.NotError(t, err)

		// Wait for output
		time.Sleep(100 * time.Millisecond)

		is.True(t, strings.Contains(buf.String(), "[test] hello world"))
	})

	t.Run("stops a long-running command", func(t *testing.T) {
		var buf safeBuffer
		cmd := newCommand("sleeper", "sleep 60", t.TempDir(), &buf)

		err := cmd.start()
		is.NotError(t, err)

		// Give it a moment to start
		time.Sleep(50 * time.Millisecond)

		cmd.stop()

		// Process should be dead now - if stop didn't work, this test hangs
	})

	t.Run("stops a command that spawns children via process group kill", func(t *testing.T) {
		var buf safeBuffer
		// sh -c spawns a child sleep process
		cmd := newCommand("parent", "sh -c 'sleep 60'", t.TempDir(), &buf)

		err := cmd.start()
		is.NotError(t, err)

		time.Sleep(50 * time.Millisecond)

		cmd.stop()

		// If process group kill didn't work, the sleep child would be orphaned
	})

	t.Run("restarts a command", func(t *testing.T) {
		var buf safeBuffer
		cmd := newCommand("counter", "echo restarted", t.TempDir(), &buf)

		err := cmd.start()
		is.NotError(t, err)
		time.Sleep(100 * time.Millisecond)

		err = cmd.restart()
		is.NotError(t, err)
		time.Sleep(100 * time.Millisecond)

		output := buf.String()
		is.Equal(t, 2, strings.Count(output, "[counter] restarted"))
	})

	t.Run("stopping an already-exited command is a no-op", func(t *testing.T) {
		var buf safeBuffer
		cmd := newCommand("quick", "echo done", t.TempDir(), &buf)

		err := cmd.start()
		is.NotError(t, err)
		time.Sleep(100 * time.Millisecond)

		// Command already exited, stop should not panic or hang
		cmd.stop()
	})

	t.Run("stopping a never-started command is a no-op", func(t *testing.T) {
		var buf safeBuffer
		cmd := newCommand("idle", "echo nope", t.TempDir(), &buf)

		// Should not panic or hang
		cmd.stop()
	})

	t.Run("writes raw output to a log file", func(t *testing.T) {
		dir := t.TempDir()
		var buf safeBuffer
		cmd := newCommand("logger", "echo one && echo two", dir, &buf)

		err := cmd.start()
		is.NotError(t, err)
		time.Sleep(100 * time.Millisecond)

		logContent, err := os.ReadFile(filepath.Join(dir, "logger.log"))
		is.NotError(t, err)
		is.True(t, strings.Contains(string(logContent), "one"))
		is.True(t, strings.Contains(string(logContent), "two"))
		// Log file should not have prefixes
		is.True(t, !strings.Contains(string(logContent), "[logger]"))
	})

	t.Run("truncates the log file on restart", func(t *testing.T) {
		dir := t.TempDir()
		var buf safeBuffer
		cmd := newCommand("truncator", "echo before-restart", dir, &buf)

		err := cmd.start()
		is.NotError(t, err)
		time.Sleep(100 * time.Millisecond)

		logPath := filepath.Join(dir, "truncator.log")
		content, err := os.ReadFile(logPath)
		is.NotError(t, err)
		is.True(t, strings.Contains(string(content), "before-restart"))

		// Restart with different output
		cmd.run = "echo after-restart"
		err = cmd.restart()
		is.NotError(t, err)
		time.Sleep(100 * time.Millisecond)

		content, err = os.ReadFile(logPath)
		is.NotError(t, err)
		is.True(t, strings.Contains(string(content), "after-restart"))
		// Old content should be gone
		is.True(t, !strings.Contains(string(content), "before-restart"))
	})
}

// safeBuffer is a concurrency-safe bytes.Buffer for tests.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

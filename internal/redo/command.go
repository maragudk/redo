package redo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// sentinel is a shell wrapper that ensures children are killed when redo dies.
// It runs the user command in the background, starts a watcher that reads from
// a pipe (fd 3) held open by redo, and waits for the command to finish.
// When redo dies (or closes the pipe), read returns and the watcher kills the
// command. When the command finishes normally, kill 0 cleans up the watcher.
const sentinel = `eval "$REDO_CMD" & CMD_PID=$!; (read <&3 || true; kill $CMD_PID 2>/dev/null) & wait $CMD_PID 2>/dev/null; kill 0 2>/dev/null`

type command struct {
	name    string
	run     string
	dir     string
	out     io.Writer
	logFile *os.File
	pipe    *os.File

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

func newCommand(name, run, dir string, out io.Writer) *command {
	return &command{
		name: name,
		run:  run,
		dir:  dir,
		out:  out,
	}
}

func (c *command) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.startLocked()
}

func (c *command) startLocked() error {
	// Close previous log file if any.
	if c.logFile != nil {
		_ = c.logFile.Close()
		c.logFile = nil
	}

	// Close previous sentinel pipe if any.
	if c.pipe != nil {
		_ = c.pipe.Close()
		c.pipe = nil
	}

	// Open log file, truncating any previous content.
	logFile, err := os.Create(filepath.Join(c.dir, c.name+".log"))
	if err != nil {
		return err
	}
	c.logFile = logFile

	// Create sentinel pipe. The write end stays open in redo; the read end
	// is passed to the child as fd 3. When redo exits (for any reason),
	// the OS closes the write end, which unblocks the child's read and
	// triggers cleanup of the process group.
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return err
	}

	cmd := exec.Command("sh", "-c", sentinel)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(), "REDO_CMD="+c.run)
	cmd.ExtraFiles = []*os.File{pipeR}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	c.done = make(chan struct{})

	if err := cmd.Start(); err != nil {
		_ = pipeR.Close()
		_ = pipeW.Close()
		return err
	}

	// Close read end in parent; the child inherited its own copy.
	_ = pipeR.Close()
	c.pipe = pipeW

	c.cmd = cmd

	// Capture logFile reference for goroutines so they don't race with restart.
	lf := c.logFile
	go c.prefixLines(stdout, lf)
	go c.prefixLines(stderr, lf)

	go func() {
		_ = cmd.Wait()
		close(c.done)
	}()

	return nil
}

func (c *command) prefixLines(r io.Reader, logFile *os.File) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		prefixed := fmt.Sprintf("[%s] %s\n", c.name, text)
		_, _ = c.out.Write([]byte(prefixed))
		_, _ = logFile.Write([]byte(text + "\n"))
	}
}

func (c *command) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()
}

func (c *command) stopLocked() {
	if c.cmd == nil || c.cmd.Process == nil {
		return
	}

	// Check if already exited before sending signals.
	select {
	case <-c.done:
		c.cmd = nil
		return
	default:
	}

	// Close sentinel pipe to trigger child cleanup.
	if c.pipe != nil {
		_ = c.pipe.Close()
		c.pipe = nil
	}

	pid := c.cmd.Process.Pid

	// Send SIGTERM to the entire process group.
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	select {
	case <-c.done:
	case <-time.After(500 * time.Millisecond):
		// Check again before SIGKILL to avoid PID reuse.
		select {
		case <-c.done:
		default:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			<-c.done
		}
	}

	c.cmd = nil
}

func (c *command) closeLog() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.logFile != nil {
		_ = c.logFile.Close()
		c.logFile = nil
	}
}

func (c *command) restart() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()
	return c.startLocked()
}

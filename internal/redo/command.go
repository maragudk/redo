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

type command struct {
	name    string
	run     string
	dir     string
	out     io.Writer
	logFile *os.File

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

	// Open log file, truncating any previous content.
	logFile, err := os.Create(filepath.Join(c.dir, c.name+".log"))
	if err != nil {
		return err
	}
	c.logFile = logFile

	cmd := exec.Command("sh", "-c", c.run)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
		return err
	}

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

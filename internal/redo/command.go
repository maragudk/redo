package redo

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type command struct {
	name string
	run  string
	out  io.Writer

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

func newCommand(name, run string, out io.Writer) *command {
	return &command{
		name: name,
		run:  run,
		out:  out,
	}
}

func (c *command) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.startLocked()
}

func (c *command) startLocked() error {
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

	go c.prefixLines(stdout)
	go c.prefixLines(stderr)

	go func() {
		_ = cmd.Wait()
		close(c.done)
	}()

	return nil
}

func (c *command) prefixLines(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := fmt.Sprintf("[%s] %s\n", c.name, scanner.Text())
		_, _ = c.out.Write([]byte(line))
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

func (c *command) restart() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLocked()
	return c.startLocked()
}

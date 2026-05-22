package agent

import (
	"bufio"
	"fmt"
	"os/exec"
)

// Runner manages a single agent child process.
type Runner struct {
	buf *RingBuffer
	cmd *exec.Cmd
}

func NewRunner() *Runner {
	return &Runner{buf: NewRingBuffer(1000)}
}

// Start launches the agent command in dir. done receives nil on clean exit, error otherwise.
func (r *Runner) Start(command string, args []string, dir string, done chan<- error) error {
	r.cmd = exec.Command(command, args...)
	r.cmd.Dir = dir

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("start agent %q: %w", command, err)
	}

	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			r.buf.Add(sc.Text())
		}
	}()

	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			r.buf.Add("stderr: " + sc.Text())
		}
	}()

	go func() {
		done <- r.cmd.Wait()
	}()

	return nil
}

// Stop kills the agent process.
func (r *Runner) Stop() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	return r.cmd.Process.Kill()
}

// Lines returns the buffered output lines.
func (r *Runner) Lines() []string {
	return r.buf.Lines()
}

// LastError returns the last stderr line, or empty string if none.
func (r *Runner) LastError() string {
	lines := r.buf.Lines()
	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) > 8 && lines[i][:8] == "stderr: " {
			return lines[i][8:]
		}
	}
	return ""
}

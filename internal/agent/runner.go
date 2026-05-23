package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Runner manages a single agent child process.
type Runner struct {
	buf *RingBuffer
	cmd *exec.Cmd
}

func NewRunner() *Runner {
	return &Runner{buf: NewRingBuffer(5000)}
}

// Start launches the agent command in dir. Output is parsed from stream-json
// into human-readable lines. done receives nil on clean exit, error otherwise.
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

	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if lines := parseStreamJSON(sc.Text()); len(lines) > 0 {
				for _, l := range lines {
					r.buf.Add(l)
				}
			}
		}
	}()

	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			r.buf.Add(sc.Text())
		}
	}()

	go func() {
		err := r.cmd.Wait()
		<-scanDone // ensure all output is captured before signalling done
		done <- err
	}()

	return nil
}

// parseStreamJSON converts one stream-json line into zero or more display lines.
func parseStreamJSON(raw string) []string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		// Not JSON (e.g. plain text fallback) — emit as-is if non-empty.
		if s := strings.TrimSpace(raw); s != "" {
			return []string{s}
		}
		return nil
	}

	var typ string
	_ = json.Unmarshal(obj["type"], &typ)

	switch typ {
	case "assistant":
		return parseAssistantEvent(obj)
	case "user":
		return parseUserEvent(obj)
	case "result":
		// final result line — skip, already shown via assistant events
		return nil
	default:
		// system, hook events etc — skip
		return nil
	}
}

type msgBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
}

func parseAssistantEvent(obj map[string]json.RawMessage) []string {
	var msg struct {
		Content []msgBlock `json:"content"`
	}
	if err := json.Unmarshal(obj["message"], &msg); err != nil {
		return nil
	}
	var out []string
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			if t := strings.TrimSpace(b.Text); t != "" {
				for _, line := range strings.Split(t, "\n") {
					out = append(out, line)
				}
			}
		case "tool_use":
			inputStr := ""
			var m map[string]interface{}
			if err := json.Unmarshal(b.Input, &m); err == nil {
				if cmd, ok := m["command"].(string); ok {
					inputStr = cmd
				} else if skill, ok := m["skill"].(string); ok {
					inputStr = skill
				} else {
					raw, _ := json.Marshal(m)
					inputStr = string(raw)
					if len(inputStr) > 80 {
						inputStr = inputStr[:80] + "…"
					}
				}
			}
			out = append(out, fmt.Sprintf("⚙ %s  %s", b.Name, inputStr))
		}
	}
	return out
}

func parseUserEvent(obj map[string]json.RawMessage) []string {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(obj["message"], &msg); err != nil {
		return nil
	}

	// content is either a string or []block
	var blocks []msgBlock
	if err := json.Unmarshal(msg.Content, &blocks); err != nil {
		return nil
	}
	var out []string
	for _, b := range blocks {
		if b.Type != "tool_result" {
			continue
		}
		// content inside tool_result can be string or []block
		var text string
		if err := json.Unmarshal(b.Content, &text); err == nil {
			text = strings.TrimSpace(text)
		} else {
			var inner []msgBlock
			if err := json.Unmarshal(b.Content, &inner); err == nil {
				var parts []string
				for _, ib := range inner {
					if ib.Type == "text" {
						parts = append(parts, ib.Text)
					}
				}
				text = strings.TrimSpace(strings.Join(parts, "\n"))
			}
		}
		if text != "" {
			out = append(out, "  └ "+strings.ReplaceAll(text, "\n", "\n    "))
		}
	}
	return out
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

// LastError returns the last non-empty line from the buffer.
func (r *Runner) LastError() string {
	lines := r.buf.Lines()
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != "" {
			return lines[i]
		}
	}
	return ""
}

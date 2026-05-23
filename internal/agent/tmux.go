package agent

import (
	"os/exec"
	"strings"

	"github.com/agrajgarg/orka/internal/state"
)

// TmuxSessionName returns a stable tmux session name for a task.
func TmuxSessionName(task *state.Task) string {
	// tmux session names can't contain dots or colons
	id := task.ID
	if len(id) > 8 {
		id = id[:8]
	}
	return "orka-" + id
}

// TmuxSessionExists returns true if a tmux session with the given name is running.
func TmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

// TmuxLaunch starts a new tmux session running the agent, then returns the
// command to attach to it. The session runs detached first, then we attach —
// this way detaching with ctrl+b d returns to orka cleanly.
func TmuxLaunch(task *state.Task, prompt, dir string) (string, []string) {
	name := TmuxSessionName(task)
	var agentCmd string
	switch task.Agent {
	case "codex":
		agentCmd = "codex --full-auto " + shellQuote(prompt)
	default:
		agentCmd = "claude --dangerously-skip-permissions " + shellQuote(prompt)
	}
	// Create detached session with custom key bindings, then attach.
	// We override the prefix to ctrl+x and bind ctrl+x ctrl+x to detach
	// so the user can detach with ctrl+x d (or just ctrl+x ctrl+x).
	exec.Command("tmux", "new-session", "-d", "-s", name, "-c", dir,
		"-x", "220", "-y", "50",
		agentCmd,
	).Run() //nolint:errcheck
	// Disable default prefix and bind ctrl+q to detach — single keypress to return to orka.
	exec.Command("tmux", "set-option", "-t", name, "prefix", "None").Run()       //nolint:errcheck
	exec.Command("tmux", "bind-key", "-T", "root", "C-q", "detach-client").Run() //nolint:errcheck
	return "tmux", []string{"attach-session", "-t", name}
}

// TmuxResume attaches to an existing session or creates a new one with resume command.
func TmuxResume(task *state.Task, worktreeDir string) (string, []string) {
	name := TmuxSessionName(task)
	if TmuxSessionExists(name) {
		return "tmux", []string{"attach-session", "-t", name}
	}
	// Session gone — start a fresh resume session
	var agentCmd string
	switch task.Agent {
	case "codex":
		agentCmd = "codex --continue"
	default:
		if id := latestClaudeSessionID(worktreeDir); id != "" {
			agentCmd = "claude --dangerously-skip-permissions --resume " + id
		} else {
			agentCmd = "claude --dangerously-skip-permissions --continue"
		}
	}
	exec.Command("tmux", "new-session", "-d", "-s", name, "-c", worktreeDir, agentCmd).Run() //nolint:errcheck
	exec.Command("tmux", "set-option", "-t", name, "prefix", "None").Run()                   //nolint:errcheck
	exec.Command("tmux", "bind-key", "-T", "root", "C-q", "detach-client").Run()             //nolint:errcheck
	return "tmux", []string{"attach-session", "-t", name}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

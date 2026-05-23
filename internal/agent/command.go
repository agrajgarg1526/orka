package agent

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agrajgarg/orka/internal/state"
)

// BuildCommand returns the CLI binary and args for the given task and prompt.
// The process is handed the terminal directly via tea.ExecProcess.
func BuildCommand(task *state.Task, prompt string) (string, []string) {
	switch task.Agent {
	case "codex":
		return "codex", []string{"--full-auto", prompt}
	default: // claude-code
		return "claude", []string{"--dangerously-skip-permissions", prompt}
	}
}

// ResumeCommand returns the CLI invocation to resume the most recent session
// for the task's worktree. Uses --resume <id> for claude to avoid the slow
// --continue search; falls back to --continue if no session ID is found.
func ResumeCommand(task *state.Task, worktreeDir string) (string, []string) {
	switch task.Agent {
	case "codex":
		return "codex", []string{"--continue"}
	default: // claude-code
		if id := latestClaudeSessionID(worktreeDir); id != "" {
			return "claude", []string{"--dangerously-skip-permissions", "--resume", id}
		}
		return "claude", []string{"--dangerously-skip-permissions", "--continue"}
	}
}

// latestClaudeSessionID finds the most recently modified session file in
// ~/.claude/projects/<encoded-worktree-path>/ and returns its UUID.
func latestClaudeSessionID(worktreeDir string) string {
	encoded := encodePath(worktreeDir)
	dir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", encoded)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var latestName string
	var latestTime time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestName = e.Name()
		}
	}
	if latestName == "" {
		return ""
	}
	return strings.TrimSuffix(latestName, ".jsonl")
}

// encodePath converts an absolute path to claude's project directory encoding.
// Claude replaces each '/' and '.' with '-'.
func encodePath(absPath string) string {
	r := strings.NewReplacer("/", "-", ".", "-")
	return r.Replace(absPath)
}

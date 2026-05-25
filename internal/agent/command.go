package agent

import (
	"bufio"
	"encoding/json"
	"errors"
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

// ResumeCommand returns the CLI invocation to resume a saved or discoverable
// session for the task's worktree, or opens the agent's native resume picker.
func ResumeCommand(task *state.Task, worktreeDir string) (string, []string) {
	switch task.Agent {
	case "codex":
		if id, ok := ResolveSessionID(task, worktreeDir); ok {
			return "codex", []string{"resume", id}
		}
		return "codex", []string{"resume"}
	default: // claude-code
		if id, ok := ResolveSessionID(task, worktreeDir); ok {
			return "claude", []string{"--dangerously-skip-permissions", "--resume", id}
		}
		return "claude", []string{"--dangerously-skip-permissions", "--resume"}
	}
}

// ResolveSessionID returns a valid persisted or discoverable session ID for the task.
func ResolveSessionID(task *state.Task, worktreeDir string) (string, bool) {
	if task.AgentSessionID != "" && sessionIDExists(task.Agent, worktreeDir, task.AgentSessionID) {
		return task.AgentSessionID, true
	}
	id := LatestSessionID(task.Agent, worktreeDir)
	return id, id != ""
}

// LatestSessionID returns the most recent session id for the given agent and worktree.
func LatestSessionID(agentName, worktreeDir string) string {
	switch agentName {
	case "codex":
		return latestCodexSessionID(worktreeDir)
	default:
		return latestClaudeSessionID(worktreeDir)
	}
}

// ManualResumeCommand returns a user-facing resume command for when automatic
// recovery cannot safely determine a session id.
func ManualResumeCommand(task *state.Task, worktreeDir string) string {
	switch task.Agent {
	case "codex":
		if id, ok := ResolveSessionID(task, worktreeDir); ok {
			return "cd " + shellQuote(worktreeDir) + " && codex resume " + shellQuote(id)
		}
		return "cd " + shellQuote(worktreeDir) + " && codex resume"
	default:
		if id, ok := ResolveSessionID(task, worktreeDir); ok {
			return "cd " + shellQuote(worktreeDir) + " && claude --dangerously-skip-permissions --resume " + shellQuote(id)
		}
		return "cd " + shellQuote(worktreeDir) + " && claude --dangerously-skip-permissions --resume"
	}
}

func sessionIDExists(agentName, worktreeDir, id string) bool {
	if id == "" {
		return false
	}
	switch agentName {
	case "codex":
		_, err := codexSessionPathByID(id)
		return err == nil
	default:
		_, err := os.Stat(claudeSessionPath(worktreeDir, id))
		return err == nil
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

func claudeSessionPath(worktreeDir, id string) string {
	encoded := encodePath(worktreeDir)
	return filepath.Join(os.Getenv("HOME"), ".claude", "projects", encoded, id+".jsonl")
}

// latestCodexSessionID finds the newest Codex session whose session_meta cwd
// matches the task worktree path.
func latestCodexSessionID(worktreeDir string) string {
	root := filepath.Join(os.Getenv("HOME"), ".codex", "sessions")
	var latestID string
	var latestTime time.Time
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".jsonl" {
			return nil
		}
		id, cwd, err := codexSessionMeta(p)
		if err != nil || cwd != worktreeDir || id == "" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestID = id
		}
		return nil
	})
	return latestID
}

func codexSessionPathByID(id string) (string, error) {
	root := filepath.Join(os.Getenv("HOME"), ".codex", "sessions")
	pattern := filepath.Join(root, "*", "*", "*", "*"+id+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", errors.New("session not found")
	}
	return matches[0], nil
}

func codexSessionMeta(sessionPath string) (string, string, error) {
	f, err := os.Open(sessionPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	type metaPayload struct {
		ID  string `json:"id"`
		Cwd string `json:"cwd"`
	}
	type line struct {
		Type    string      `json:"type"`
		Payload metaPayload `json:"payload"`
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry line
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.Type == "session_meta" {
			return entry.Payload.ID, entry.Payload.Cwd, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	return "", "", errors.New("session_meta not found")
}

// encodePath converts an absolute path to claude's project directory encoding.
// Claude replaces each '/' and '.' with '-'.
func encodePath(absPath string) string {
	r := strings.NewReplacer("/", "-", ".", "-")
	return r.Replace(absPath)
}

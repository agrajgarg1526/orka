package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agrajgarg/orka/internal/state"
	"github.com/agrajgarg/orka/internal/worktree"
)

func EnsureTaskPR(projectDir string, task *state.Task, finalize bool) (string, error) {
	if task == nil || task.Branch == "" || strings.TrimSpace(task.PRBaseBranch) == "" {
		return "", nil
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "", fmt.Errorf("gh is not installed")
	}

	dir := projectDir
	wtDir := worktree.WorktreePath(projectDir, task.Branch)
	if info, err := os.Stat(wtDir); err == nil && info.IsDir() {
		dir = wtDir
	}

	baseRef, err := resolveRef(dir, task.PRBaseBranch)
	if err != nil {
		return "", fmt.Errorf("resolve base branch %q: %w", task.PRBaseBranch, err)
	}
	ahead, err := revListCount(dir, baseRef+".."+task.Branch)
	if err != nil {
		return "", fmt.Errorf("compare %s..%s: %w", baseRef, task.Branch, err)
	}
	if ahead == 0 {
		return "", nil
	}

	if err := pushBranch(dir, task.Branch); err != nil {
		return "", err
	}

	number, url, err := findOpenPR(dir, task.Branch)
	if err != nil {
		return "", err
	}
	title := prTitle(task.Title, finalize)
	body := prBody(task)

	if number == 0 {
		args := []string{"pr", "create", "--base", task.PRBaseBranch, "--head", task.Branch, "--title", title, "--body", body}
		if assignee, err := currentUser(dir); err == nil && assignee != "" {
			args = append(args, "--assignee", assignee)
		}
		out, err := runOutput(dir, "gh", args...)
		if err != nil {
			return "", fmt.Errorf("create pr: %w", err)
		}
		return strings.TrimSpace(out), nil
	}

	args := []string{"pr", "edit", strconv.Itoa(number), "--title", title, "--base", task.PRBaseBranch}
	if _, err := runOutput(dir, "gh", args...); err != nil {
		return "", fmt.Errorf("edit pr #%d: %w", number, err)
	}
	if url != "" {
		return url, nil
	}
	return prURL(dir, number)
}

func prTitle(title string, finalize bool) string {
	title = strings.TrimSpace(title)
	if finalize {
		return strings.TrimSpace(strings.TrimPrefix(title, "[WIP]"))
	}
	if strings.HasPrefix(title, "[WIP]") {
		return title
	}
	return "[WIP] " + title
}

func prBody(task *state.Task) string {
	if task.Description != "" {
		return task.Description
	}
	return "Automated PR created by orka."
}

func currentUser(dir string) (string, error) {
	out, err := runOutput(dir, "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func findOpenPR(dir, branch string) (int, string, error) {
	out, err := runOutput(dir, "gh", "pr", "list", "--head", branch, "--state", "open", "--json", "number,url")
	if err != nil {
		return 0, "", fmt.Errorf("list prs for %s: %w", branch, err)
	}
	var prs []struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return 0, "", fmt.Errorf("decode pr list: %w", err)
	}
	if len(prs) == 0 {
		return 0, "", nil
	}
	return prs[0].Number, prs[0].URL, nil
}

func prURL(dir string, number int) (string, error) {
	out, err := runOutput(dir, "gh", "pr", "view", strconv.Itoa(number), "--json", "url", "--jq", ".url")
	if err != nil {
		return "", fmt.Errorf("view pr #%d: %w", number, err)
	}
	return strings.TrimSpace(out), nil
}

func pushBranch(dir, branch string) error {
	if _, err := runOutput(dir, "git", "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push branch %q: %w", branch, err)
	}
	return nil
}

func resolveRef(dir, branch string) (string, error) {
	candidates := []string{
		branch,
		filepath.Join("refs", "heads", branch),
		filepath.Join("refs", "remotes", "origin", branch),
	}
	for _, candidate := range candidates {
		if err := exec.Command("git", "-C", dir, "rev-parse", "--verify", candidate).Run(); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("branch %q not found locally or on origin", branch)
}

func revListCount(dir, expr string) (int, error) {
	out, err := runOutput(dir, "git", "rev-list", "--count", expr)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, err
	}
	return n, nil
}

func runOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

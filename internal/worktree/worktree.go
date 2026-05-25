package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Setup ensures a git branch and worktree exist for the given branch name,
// rooted at projectDir. Returns the worktree path.
// If the worktree already exists it is returned as-is.
func Setup(projectDir, branchName string) (string, error) {
	return SetupFromBase(projectDir, branchName, "")
}

// SetupFromBase ensures a git branch and worktree exist for the given branch
// name, creating new branches from baseBranch when one is provided.
func SetupFromBase(projectDir, branchName, baseBranch string) (string, error) {
	wtPath := WorktreePath(projectDir, branchName)

	if _, err := os.Stat(wtPath); err == nil {
		if err := verifyWorktreeBranch(wtPath, branchName); err != nil {
			return "", err
		}
		return wtPath, nil
	}

	// Check if the branch already exists.
	branchExists := branchExistsInRepo(projectDir, branchName)

	if branchExists {
		// Branch exists — just add the worktree pointing at it.
		if err := run(projectDir, "git", "worktree", "add", wtPath, branchName); err != nil {
			return "", fmt.Errorf("add worktree for existing branch %q: %w", branchName, err)
		}
	} else {
		// Create branch and worktree together.
		args := []string{"git", "worktree", "add", "-b", branchName, wtPath}
		if startPoint := resolveStartPoint(projectDir, baseBranch); startPoint != "" {
			args = append(args, startPoint)
		}
		if err := run(projectDir, args...); err != nil {
			return "", fmt.Errorf("create worktree+branch %q: %w", branchName, err)
		}
	}

	if err := verifyWorktreeBranch(wtPath, branchName); err != nil {
		return "", err
	}
	return wtPath, nil
}

// Remove removes the worktree and deletes the branch.
func Remove(projectDir, branchName string) error {
	wtPath := WorktreePath(projectDir, branchName)
	if err := run(projectDir, "git", "worktree", "remove", "--force", wtPath); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	_ = run(projectDir, "git", "branch", "-D", branchName)
	return nil
}

func WorktreePath(projectDir, branchName string) string {
	// Sanitise branch name for use as a directory component.
	safe := branchName
	for i, c := range branchName {
		if c == '/' || c == '\\' {
			safe = branchName[i+1:]
		}
	}
	return filepath.Join(projectDir, ".worktrees", safe)
}

func branchExistsInRepo(dir, branch string) bool {
	return refExists(dir, "refs/heads/"+branch)
}

func resolveStartPoint(dir, baseBranch string) string {
	baseBranch = strings.TrimSpace(baseBranch)
	if baseBranch == "" {
		return ""
	}
	candidates := []string{
		baseBranch,
		"refs/heads/" + baseBranch,
		"refs/remotes/origin/" + baseBranch,
	}
	for _, candidate := range candidates {
		if refExists(dir, candidate) {
			return candidate
		}
	}
	return baseBranch
}

func verifyWorktreeBranch(dir, branch string) error {
	actual, err := currentBranch(dir)
	if err != nil {
		return fmt.Errorf("inspect worktree %q: %w", dir, err)
	}
	if actual != branch {
		return fmt.Errorf("worktree %q is on branch %q, expected %q", dir, actual, branch)
	}
	return nil
}

func currentBranch(dir string) (string, error) {
	out, err := output(dir, "git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func refExists(dir, ref string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", ref)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func output(dir string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w\n%s", err, string(out))
	}
	return string(out), nil
}

func run(dir string, args ...string) error {
	_, err := output(dir, args...)
	return err
}

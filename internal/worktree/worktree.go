package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Setup ensures a git branch and worktree exist for the given branch name,
// rooted at projectDir. Returns the worktree path.
// If the worktree already exists it is returned as-is.
func Setup(projectDir, branchName string) (string, error) {
	wtPath := WorktreePath(projectDir, branchName)

	// If the worktree directory already exists, assume it's set up correctly.
	if _, err := os.Stat(wtPath); err == nil {
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
		if err := run(projectDir, "git", "worktree", "add", "-b", branchName, wtPath); err != nil {
			return "", fmt.Errorf("create worktree+branch %q: %w", branchName, err)
		}
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
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func run(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(out))
	}
	return nil
}

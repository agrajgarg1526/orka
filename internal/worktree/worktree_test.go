package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupFromBaseCreatesBranchFromExplicitBase(t *testing.T) {
	repo := initRepo(t)

	writeFile(t, filepath.Join(repo, "file.txt"), "main\n")
	git(t, repo, "add", "file.txt")
	git(t, repo, "commit", "-m", "main")
	git(t, repo, "checkout", "-b", "other")
	writeFile(t, filepath.Join(repo, "file.txt"), "other\n")
	git(t, repo, "commit", "-am", "other")

	wtPath, err := SetupFromBase(repo, "task", "main")
	if err != nil {
		t.Fatalf("SetupFromBase: %v", err)
	}

	if got := strings.TrimSpace(gitOutput(t, wtPath, "branch", "--show-current")); got != "task" {
		t.Fatalf("worktree branch = %q, want task", got)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(wtPath, "file.txt"))); got != "main" {
		t.Fatalf("worktree file = %q, want main", got)
	}
}

func TestSetupFromBaseRejectsExistingWorktreeOnDifferentBranch(t *testing.T) {
	repo := initRepo(t)

	writeFile(t, filepath.Join(repo, "file.txt"), "main\n")
	git(t, repo, "add", "file.txt")
	git(t, repo, "commit", "-m", "main")
	if _, err := SetupFromBase(repo, "existing", "main"); err != nil {
		t.Fatalf("initial SetupFromBase: %v", err)
	}

	_, err := SetupFromBase(repo, "feature/existing", "main")
	if err == nil {
		t.Fatal("expected branch mismatch error")
	}
	if !strings.Contains(err.Error(), `expected "feature/existing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	git(t, "", "init", "-b", "main", repo)
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test User")
	return repo
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

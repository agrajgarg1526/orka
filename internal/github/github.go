package github

import (
	"encoding/json"
	"fmt"
	"net/url"
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

	dir, err := worktree.SetupFromBase(projectDir, task.Branch, task.PRBaseBranch)
	if err != nil {
		return "", fmt.Errorf("setup worktree for %q: %w", task.Branch, err)
	}

	baseRef, err := resolveRef(dir, task.PRBaseBranch)
	if err != nil {
		return "", fmt.Errorf("resolve base branch %q: %w", task.PRBaseBranch, err)
	}
	ahead, err := revListCount(dir, baseRef+"..HEAD")
	if err != nil {
		return "", fmt.Errorf("compare %s..%s: %w", baseRef, task.Branch, err)
	}
	if ahead == 0 {
		return "", nil
	}

	if err := pushHead(dir, task.Branch); err != nil {
		return "", err
	}

	repo, err := remoteRepo(dir)
	if err != nil {
		return "", err
	}
	number, prURL, err := findOpenPR(dir, repo, task.Branch)
	if err != nil {
		return "", err
	}
	title := prTitle(task.Title, finalize)
	body := prBody(task)

	if number == 0 {
		url, err := createPR(dir, repo, task, title, body)
		if err != nil {
			return "", fmt.Errorf("create pr: %w", err)
		}
		return url, nil
	}

	updatedURL, err := updatePR(dir, repo, number, title, task.PRBaseBranch)
	if err != nil {
		return "", fmt.Errorf("edit pr #%d: %w", number, err)
	}
	if updatedURL != "" {
		return updatedURL, nil
	}
	return prURL, nil
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

type repoRef struct {
	Owner string
	Name  string
}

func remoteRepo(dir string) (repoRef, error) {
	out, err := runOutput(dir, "git", "remote", "get-url", "origin")
	if err != nil {
		return repoRef{}, fmt.Errorf("get origin remote: %w", err)
	}
	repo, err := parseRemoteRepo(strings.TrimSpace(out))
	if err != nil {
		return repoRef{}, fmt.Errorf("parse origin remote: %w", err)
	}
	return repo, nil
}

func parseRemoteRepo(raw string) (repoRef, error) {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if raw == "" {
		return repoRef{}, fmt.Errorf("empty remote URL")
	}

	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		path := strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/")
		return parseOwnerRepoPath(path)
	}

	if i := strings.LastIndex(raw, ":"); i >= 0 {
		return parseOwnerRepoPath(raw[i+1:])
	}
	return parseOwnerRepoPath(raw)
}

func parseOwnerRepoPath(path string) (repoRef, error) {
	path = strings.Trim(strings.TrimSuffix(path, ".git"), "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return repoRef{}, fmt.Errorf("expected owner/repo, got %q", path)
	}
	owner := parts[len(parts)-2]
	name := parts[len(parts)-1]
	if owner == "" || name == "" {
		return repoRef{}, fmt.Errorf("expected owner/repo, got %q", path)
	}
	return repoRef{Owner: owner, Name: name}, nil
}

func findOpenPR(dir string, repo repoRef, branch string) (int, string, error) {
	out, err := runOutput(
		dir,
		"gh",
		"api",
		repo.apiPath("pulls"),
		"-X",
		"GET",
		"-f",
		"head="+repo.Owner+":"+branch,
		"-f",
		"state=open",
	)
	if err != nil {
		return 0, "", fmt.Errorf("list prs for %s: %w", branch, err)
	}
	var prs []struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return 0, "", fmt.Errorf("decode pr list: %w", err)
	}
	if len(prs) == 0 {
		return 0, "", nil
	}
	return prs[0].Number, prs[0].HTMLURL, nil
}

func createPR(dir string, repo repoRef, task *state.Task, title, body string) (string, error) {
	out, err := runOutput(
		dir,
		"gh",
		"api",
		repo.apiPath("pulls"),
		"-X",
		"POST",
		"-f",
		"title="+title,
		"-f",
		"head="+task.Branch,
		"-f",
		"base="+task.PRBaseBranch,
		"-f",
		"body="+body,
	)
	if err != nil {
		return "", err
	}
	return decodeHTMLURL(out)
}

func updatePR(dir string, repo repoRef, number int, title, baseBranch string) (string, error) {
	out, err := runOutput(
		dir,
		"gh",
		"api",
		repo.apiPath("pulls", strconv.Itoa(number)),
		"-X",
		"PATCH",
		"-f",
		"title="+title,
		"-f",
		"base="+baseBranch,
	)
	if err != nil {
		return "", err
	}
	return decodeHTMLURL(out)
}

func decodeHTMLURL(raw string) (string, error) {
	var res struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return "", fmt.Errorf("decode pr response: %w", err)
	}
	return res.HTMLURL, nil
}

func (r repoRef) apiPath(parts ...string) string {
	all := append([]string{"repos", r.Owner, r.Name}, parts...)
	return strings.Join(all, "/")
}

func pushHead(dir, branch string) error {
	if _, err := runOutput(dir, "git", "push", "-u", "origin", "HEAD:refs/heads/"+branch); err != nil {
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

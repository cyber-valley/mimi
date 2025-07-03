package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

const gitPath = "/usr/bin/git"

func Clone(baseRepoPath, owner, name string) error {
	_, err := Git(baseRepoPath, "clone", AsUrl(owner, name), AsPath(owner, name))
	return err
}

func Pull(baseRepoPath, owner, name string) error {
	cwd := filepath.Join(baseRepoPath, AsPath(owner, name))
	_, err := Git(cwd, "pull")
	return err
}

func DiffInterval(baseRepoPath, owner, name string, since time.Time) (string, error) {
	cwd := filepath.Join(baseRepoPath, AsPath(owner, name))
	sinceStr := since.Format(time.RFC3339)

	commitHashBytes, err := Git(cwd, "log", "--before="+sinceStr, "-1", "--format=%H")
	if err != nil {
		return "", fmt.Errorf("failed to find commit before %s with %w", sinceStr, err)
	}
	commitHash := string(bytes.TrimSpace(commitHashBytes))
	if commitHash == "" {
		return "", fmt.Errorf("no commit found before or at %s in repository %s", sinceStr, cwd)
	}

	diffBytes, err := Git(cwd, "diff", commitHash, "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get diff from %s to HEAD: %w", commitHash, err)
	}

	return string(diffBytes), nil
}

func AsUrl(owner, name string) string {
	return fmt.Sprintf("http://github.com/%s/%s", owner, name)
}

func AsPath(owner, name string) string {
	return filepath.Join(owner, name)
}

func Git(cwd string, args ...string) ([]byte, error) {
	cmd := exec.Command("/usr/bin/git", args...)
	cmd.Dir = cwd
	stdout, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute '%s %s' with %w", gitPath, args, err)
	}
	return stdout, nil
}

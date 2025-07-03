package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

const gitPath = "/usr/bin/git"

func Clone(baseRepoPath, owner, name string) error {
	return Git(baseRepoPath, "clone", AsUrl(owner, name), AsPath(owner, name))
}

func Pull(baseRepoPath, owner, name string) error {
	cwd := filepath.Join(baseRepoPath, AsPath(owner, name))
	return Git(cwd, "pull")
}

func AsUrl(owner, name string) string {
	return fmt.Sprintf("http://github.com/%s/%s", owner, name)
}

func AsPath(owner, name string) string {
	return filepath.Join(owner, name)
}

func Git(cwd string, args ...string) error {
	cmd := exec.Command("/usr/bin/git", args...)
	cmd.Dir = cwd

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute '%s %s' with %w", gitPath, args, err)
	}
	return nil
}

package scraper

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/persist"
	"mimi/internal/provider/git"
)

const (
	githubWebhookSecretEnv = "GITHUB_WEBHOOK_SECRET"
	baseRepositoryPathEnv  = "GITHUB_REPOSITORY_BASE_PATH"
)

type PushEventHook struct {
	RepoName  string
	RepoOwner string
	Hook      func(ctx context.Context, repoPath string) error
}

// Run starts a web server on the provided `port` and listens until `ctx` will be cancelled
// `hooks` should contain single element for each unique repository
// otherwise only the first found will be executed
func Run(ctx context.Context, db *pgxpool.Pool, hooks ...PushEventHook) error {
	slog.Info("starting GitHub webhook listener", "hooks", hooks)
	// Load environment
	pk := os.Getenv(githubWebhookSecretEnv)
	if pk == "" {
		return fmt.Errorf("missing %s env variable", githubWebhookSecretEnv)
	}
	basePath := os.Getenv(baseRepositoryPathEnv)
	if basePath == "" {
		return fmt.Errorf("missing %s env variable", baseRepositoryPathEnv)
	}

	// Create base repository path does not exist
	err := os.MkdirAll(basePath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create base GitHub repository path with %w", err)
	}
	slog.Info("GitHub repositories path ensured", "path", basePath)

	q := persist.New(db)
	ticker := time.NewTicker(60 * time.Second)
	for {
		repos, err := q.FindGitHubRepositories(ctx)
		if err != nil {
			return fmt.Errorf("failed to read GitHub repositories with %w", err)
		}
		for _, repo := range repos {
			p := filepath.Join(basePath, git.AsPath(repo.Owner, repo.Name))
			if _, err := os.Stat(p); err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					// Got unexpected error
					return fmt.Errorf("failed to stat repository path '%s' for %#v with %w", p, repo, err)
				}

				// Repository does not exist locally
				slog.Info("cloning GitHub repository", "info", repo)
				err := git.Clone(basePath, repo.Owner, repo.Name)
				if err != nil {
					return fmt.Errorf("failed to clone repository %#v to %s with %w", repo, p, err)
				}
			} else {
				// Repo already cloned
				slog.Info("pulling updates of GitHub repository", "info", repo)
				err := git.Pull(basePath, repo.Owner, repo.Name)
				if err != nil {
					return fmt.Errorf("failed to pull repository %#v at %s with %w", repo, p, err)
				}
			}

			// Run hooks
			cwd := filepath.Join(basePath, git.AsPath(repo.Owner, repo.Name))

			// Find hook
			hookIdx := slices.IndexFunc(hooks, func(h PushEventHook) bool {
				return h.RepoOwner == repo.Owner && h.RepoName == repo.Name
			})
			if hookIdx == -1 {
				slog.Info("hook not found", "cwd", cwd, "repo", repo)
				continue
			}

			// Execute hook
			if err := hooks[hookIdx].Hook(ctx, cwd); err != nil {
				return fmt.Errorf("failed to run hook %d for %s with %w", hookIdx, cwd, err)
			}
			slog.Info("hook succeeded", "cwd", cwd)
		}
		select {
		case <-ctx.Done():
			ticker.Stop()
			return nil
		case <-ticker.C:
			continue
		}
	}
}

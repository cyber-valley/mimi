package scraper

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/google/go-github/v72/github"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/persist"
)

const (
	githubWebhookSecretEnv = "GITHUB_WEBHOOK_SECRET"
	baseRepositoryPathEnv  = "GITHUB_REPOSITORY_BASE_PATH"
	gitPath                = "/usr/bin/git"
)

type webhookHandler struct {
	webhookSecretKey   []byte
	db                 *pgxpool.Pool
	baseRepositoryPath string
	hooks              []PushEventHook
}

type PushEventHook struct {
	RepoName  string
	RepoOwner string
	Hook      func(repoPath string) error
}

// Run starts a web server on the provided `port` and listens until `ctx` will be cancelled
// `hooks` should contain single element for each unique repository
// otherwise only the first found will be executed
func Run(ctx context.Context, port int, db *pgxpool.Pool, hooks ...PushEventHook) error {
	slog.Info("starting GitHub webhook listener")
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

	// Init handler
	h := webhookHandler{
		webhookSecretKey:   []byte(pk),
		db:                 db,
		baseRepositoryPath: basePath,
	}

	// Clone missing repositories
	q := persist.New(db)
	repos, err := q.FindGitHubRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to read GitHub repositories with %w", err)
	}
	for _, repo := range repos {
		p := filepath.Join(basePath, repoPath(repo.Owner, repo.Name))
		if _, err := os.Stat(p); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				// Got unexpected error
				return fmt.Errorf("failed to stat repository path '%s' for %#v with %w", p, repo, err)
			}

			// Clone repository
			slog.Info("cloning GitHub repository", "info", repo)
			err := cloneRepo(basePath, repo.Owner, repo.Name)
			if err != nil {
				return fmt.Errorf("failed to clone repository %#v to %s with %w", repo, p, err)
			}

			// Run hook if found
			h.runPushEventHook(repo.Owner, repo.Name)
		}
	}

	// Setup multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc("POST /github/webhook", h.handleWebhook)
	slog.Info("Starting listening for GitHub webhooks", "port", port)

	// Serve
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	serverFailed := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverFailed <- err
		}
	}()

	select {
	case err := <-serverFailed:
		return err
	case <-ctx.Done():
		return server.Shutdown(ctx)
	}
}

func (h webhookHandler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, h.webhookSecretKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch event := event.(type) {
	case *github.PushEvent:
		slog.Info("Got push event")
		q := persist.New(h.db)
		repos, err := q.FindGitHubRepositories(r.Context())
		if err != nil {
			slog.Error("failed to get list of subscribed GitHub repositories", "with", err)
			http.Error(w, err.Error(), http.StatusNotImplemented)
			return
		}
		watched := slices.ContainsFunc(repos, func(r persist.GithubRepository) bool {
			return r.Name == *event.Repo.Name && r.Owner == *event.Repo.Owner.Login
		})
		if !watched {
			slog.Warn("got push to unwatched GitHub repository", "event", event)
			return
		}
		err = pullRepo(h.baseRepositoryPath, *event.Repo.Owner.Login, *event.Repo.Name)
		if err != nil {
			slog.Error("failed to pull GitHub repository", "with", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = h.runPushEventHook(*event.Repo.Owner.Login, *event.Repo.Name)
		if err != nil {
			slog.Error("failed to execute hook for GitHub repository", "with", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		slog.Warn("Got unexpected event", "value", fmt.Sprintf("%#v", event))
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h webhookHandler) runPushEventHook(owner, name string) error {
	cwd := filepath.Join(h.baseRepositoryPath, repoPath(owner, name))

	// Find hook
	hookIdx := slices.IndexFunc(h.hooks, func(h PushEventHook) bool {
		return h.RepoOwner == owner && h.RepoName == name
	})
	if hookIdx == -1 {
		slog.Info("hook not found", "cwd", cwd)
		return nil
	}

	// Execute hook
	if err := h.hooks[hookIdx].Hook(cwd); err != nil {
		return fmt.Errorf("failed to run hook %d for %s with %w", hookIdx, cwd, err)
	}
	slog.Info("hook succeeded", "cwd", cwd)

	return nil
}

func cloneRepo(baseRepoPath, owner, name string) error {
	return git(baseRepoPath, "clone", repoUrl(owner, name), repoPath(owner, name))
}

func pullRepo(baseRepoPath, owner, name string) error {
	cwd := filepath.Join(baseRepoPath, repoPath(owner, name))
	return git(cwd, "pull")
}

func repoUrl(owner, name string) string {
	return fmt.Sprintf("http://github.com/%s/%s", owner, name)
}

func repoPath(owner, name string) string {
	return filepath.Join(owner, name)
}

func git(cwd string, args ...string) error {
	cmd := exec.Command("/usr/bin/git", args...)
	cmd.Dir = cwd

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute '%s %s' with %w", gitPath, args, err)
	}
	return nil
}

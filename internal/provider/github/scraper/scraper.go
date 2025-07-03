package scraper

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/golang/glog"
	"github.com/google/go-github/v72/github"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/persist"
)

const (
	githubWebhookSecretEnv = "GITHUB_WEBHOOK_SECRET"
	baseRepositoryPathEnv  = "GITHUB_REPOSITORY_BASE_PATH"
)

type webhookHandler struct {
	webhookSecretKey   []byte
	db                 *pgxpool.Pool
	baseRepositoryPath string
}

func Run(port int, db *pgxpool.Pool) error {
	glog.Info("Setting up")
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

	// Setup multiplexer
	h := webhookHandler{
		webhookSecretKey:   []byte(pk),
		db:                 db,
		baseRepositoryPath: basePath,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /github/webhook", h.handleWebhook)
	glog.Info("Starting")

	// Serve
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}

func (h webhookHandler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, h.webhookSecretKey)
	if err != nil {
		glog.Errorf("signature validation %s", err)
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
		glog.Infof("Got push event %#v", event)
		q := persist.New(h.db)
		repos, err := q.FindGitHubRepositories(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotImplemented)
			return
		}
		watched := slices.ContainsFunc(repos, func(r persist.GithubRepository) bool {
			return r.Name == *event.Repo.Name && r.Owner == *event.Repo.Owner.Login
		})
		if !watched {
			slog.Warn("got push to unwatched GitHub repository", "event", event)
		}
	default:
		glog.Warning("Got unexpected event %#v", event)
	}
	w.WriteHeader(http.StatusCreated)
}

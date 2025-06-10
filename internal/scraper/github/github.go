package github

import (
	"errors"
	"github.com/golang/glog"
	"github.com/google/go-github/v72/github"
	"net/http"
	"os"
)

type eventMonitor struct {
	webhookSecretKey []byte
}

func Run() error {
	glog.Info("Setting up")
	pk := os.Getenv("Github_webhook_secret")
	if pk == "" {
		return errors.New("missing GITHUB_WEBHOOK_SECRET env variable")
	}
	m := eventMonitor{
		webhookSecretKey: []byte(pk),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /github/webhook", m.handleWebhook)
	glog.Info("Starting")
	return http.ListenAndServe(":8000", mux)
}

func (m eventMonitor) handleWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, m.webhookSecretKey)
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
	case *github.IssueEvent:
		glog.Infof("Got issue event %#v", event)
	case *github.IssueCommentEvent:
		glog.Infof("Got issue comment event %#v", event)
	case *github.ProjectV2Event:
		glog.Infof("Got project event %#v", event)
	case *github.ProjectV2ItemEvent:
		glog.Infof("Got project item event %#v", event)
	default:
		glog.Warning("Got unexpected event %#v", event)
		http.Error(w, "", http.StatusNotImplemented)
	}
	w.WriteHeader(http.StatusCreated)
}

package github

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v72/github"
)

const (
	secret = "test-secret"
)

func TestHandleWebhook_PushEvent(t *testing.T) {
	m := eventMonitor{webhookSecretKey: []byte(secret)}

	payload, err := json.Marshal(github.PushEvent{
		PushID: github.Ptr(int64(12345)),
		Head:   github.Ptr("test-head"),
		Ref:    github.Ptr("refs/heads/main"),
		Size:   github.Ptr(1),
		Commits: []*github.HeadCommit{
			{
				ID:      github.Ptr("test-commit-id"),
				Message: github.Ptr("Test commit"),
			},
		},
		Before:       github.Ptr("test-before"),
		DistinctSize: github.Ptr(1),
		Action:       github.Ptr("test-action"),
		After:        github.Ptr("test-after"),
		Created:      github.Ptr(false),
		Deleted:      github.Ptr(false),
		Forced:       github.Ptr(false),
		BaseRef:      github.Ptr("test-base-ref"),
		Compare:      github.Ptr("test-compare"),
		Repo: &github.PushEventRepository{
			Name:     github.Ptr("test-repo"),
			FullName: github.Ptr("test-owner/test-repo"),
			Owner: &github.User{
				Login: github.Ptr("test-owner"),
			},
		},
		HeadCommit: &github.HeadCommit{
			ID:        github.Ptr("test-commit-id"),
			Message:   github.Ptr("Test commit"),
			Timestamp: &github.Timestamp{Time: time.Now()},
			Author: &github.CommitAuthor{
				Name:  github.Ptr("Test Author"),
				Email: github.Ptr("test@example.com"),
			},
			Committer: &github.CommitAuthor{
				Name:  github.Ptr("Test Committer"),
				Email: github.Ptr("test-committer@example.com"),
			},
		},
		Pusher: &github.CommitAuthor{
			Name:  github.Ptr("Test Pusher"),
			Email: github.Ptr("test-pusher@example.com"),
		},
		Sender: &github.User{
			Login: github.Ptr("test-sender"),
		},
		Installation: &github.Installation{
			ID: github.Ptr(int64(123)),
		},
		Organization: &github.Organization{
			Login: github.Ptr("test-org"),
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal push event: %v", err)
	}

	req, err := http.NewRequest("POST", "/github/webhook", bytes.NewBuffer(payload))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("Content-Type", "application/json")
	signature := generateHmacSignature(payload, []byte(secret))
	req.Header.Set("X-Hub-Signature-256", "sha256="+signature)

	rr := httptest.NewRecorder()

	m.handleWebhook(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}
}

func generateHmacSignature(payload []byte, secretKey []byte) string {
	h := hmac.New(sha256.New, secretKey)
	h.Write(payload)
	signature := h.Sum(nil)
	return hex.EncodeToString(signature)
}

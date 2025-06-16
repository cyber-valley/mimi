package db

import (
	"log/slog"
	"testing"
)

func TestQueryProjectV2(t *testing.T) {
	slog.Info("test")
	client := New("https://api.github.com/graphql")
	proj, err := client.GetOrgProject(t.Context(), "cyber-valley", 3)
	if err != nil {
		t.Error(err)
	}
	slog.Info("Got project", "project", proj)
}

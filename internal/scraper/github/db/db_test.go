package db

import (
	"log/slog"
	"testing"
)

func TestQueryProjectV2(t *testing.T) {
	client := New("https://api.github.com/graphql")
	columnNames := []string{"monthly plan", "ordered", "shipped"}
	issues, err := client.GetOrgProject(t.Context(), "cyber-valley", 3, columnNames)
	if err != nil {
		t.Error(err)
	}
	slog.Info("Got issues", "length", len(issues), "val", issues)
}

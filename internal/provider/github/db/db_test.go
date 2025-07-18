package db

import (
	"testing"
	"time"
)

func TestQueryProjectV2(t *testing.T) {
	client := New("https://api.github.com/graphql")
	since := time.Now().Add(-24 * time.Hour)
	issues, err := client.GetOrgProject(t.Context(), "cyber-valley", 3, since)
	if err != nil {
		t.Error(err)
	}
	t.Logf("issues length %d, value %s", len(issues), issues)
}

func TestListProjects(t *testing.T) {
	client := New("https://api.github.com/graphql")
	projects, err := client.ListProjects(t.Context(), "cyber-valley")
	if err != nil {
		t.Error(err)
	}
	t.Logf("projects length %d, values %#v", len(projects), projects)
}

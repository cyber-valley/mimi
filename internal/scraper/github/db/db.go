package db

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

//go:embed queries/project.graphql
var projectQuery string

// GraphQLQuery is the structure for a GraphQL request.
type GraphQLQuery struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// GraphQLResponse is a generic wrapper for GraphQL responses.
type GraphQLResponse struct {
	Data   json.RawMessage          `json:"data"`
	Errors []map[string]interface{} `json:"errors,omitempty"`
}

// ProjectV2Response holds the structure for unmarshaling your query.
type ProjectV2Response struct {
	Organization struct {
		ProjectV2 struct {
			ID               string `json:"id"`
			Title            string `json:"title"`
			ShortDescription string `json:"shortDescription"`
			Closed           bool   `json:"closed"`
			URL              string `json:"url"`
			Fields           struct {
				Nodes []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					DataType string `json:"dataType"`
				} `json:"nodes"`
			} `json:"fields"`
			Items struct {
				Nodes []struct {
					ID      string `json:"id"`
					Content struct {
						Title    string `json:"title"`
						URL      string `json:"url"`
						State    string `json:"state"`
						Body     string `json:"body"`
					} `json:"content"`
				} `json:"nodes"`
				PageInfo struct {
					EndCursor   string `json:"endCursor"`
					HasNextPage bool   `json:"hasNextPage"`
				} `json:"pageInfo"`
			} `json:"items"`
		} `json:"projectV2"`
	} `json:"organization"`
}

// Client encapsulates the GitHub GraphQL endpoint and OAuth token.
type Client struct {
	Endpoint string
	Token    string
	HTTP     *http.Client
}

// NewClient initializes a new GraphQL client.
// If token is empty, it tries to read from the GITHUB_TOKEN environment variable.
func New(endpoint string) *Client {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		panic("set GITHUB_TOKEN env variable")
	}
	return &Client{
		Endpoint: endpoint,
		Token:    token,
		HTTP:     &http.Client{Timeout: 15 * time.Second},
	}
}

// DoQuery executes a GraphQL query with variables and decodes the response into result.
func (c *Client) DoQuery(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	reqBody := GraphQLQuery{
		Query:     query,
		Variables: variables,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var gqlResp GraphQLResponse
	if err := json.Unmarshal(b, &gqlResp); err != nil {
		return err
	}
	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("failed to execute project GraphQL with %#v", gqlResp.Errors)
	}
	return json.Unmarshal(gqlResp.Data, result)
}

// GetOrgProject queries organization project board information.
func (c *Client) GetOrgProject(ctx context.Context, org string, projectNumber int, columnNames []string) ([]Issue, error) {
	var issues []Issue
	after := ""
	var times int

	for {
		variables := map[string]interface{}{
			"org":           org,
			"projectNumber": projectNumber,
			"columnNames":   columnNames,
			"after":         nil,
		}
		if after != "" {
			variables["after"] = after
		}

		var resp ProjectV2Response
		err := c.DoQuery(ctx, projectQuery, variables, &resp)
		if err != nil {
			return issues, err
		}

		nodes := resp.Organization.ProjectV2.Items.Nodes
		for _, node := range nodes {
			content := node.Content
				issues = append(issues, Issue{
					Title: content.Title,
					URL:   content.URL,
					State: content.State,
					Body:  content.Body,
				})
		}

		pageInfo := resp.Organization.ProjectV2.Items.PageInfo
		if !pageInfo.HasNextPage || pageInfo.EndCursor == "" {
			break
		}
		after = pageInfo.EndCursor
		times += 1
	}
	slog.Info("GraphQL was queried", "times", times)
	return issues, nil
}

type Issue struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	State string `json:"state"`
	Body  string `json:"body"`
}

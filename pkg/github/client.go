package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/v57/github"
)

// IssueFetcher handles fetching issues from GitHub.
type IssueFetcher struct {
	client *github.Client
}

// NewIssueFetcher creates a new IssueFetcher.
func NewIssueFetcher(client *github.Client) *IssueFetcher {
	return &IssueFetcher{client: client}
}

// FetchIssues retrieves all issues for a repository updated since a specific time.
func (f *IssueFetcher) FetchIssues(ctx context.Context, owner, repo string, since time.Time, perPage int) ([]*github.Issue, error) {
	opt := &github.IssueListByRepoOptions{
		Since: since,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	var allIssues []*github.Issue
	for {
		issues, resp, err := f.client.Issues.ListByRepository(ctx, owner, repo, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list issues: %w", err)
		}
		allIssues = append(allIssues, issues...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allIssues, nil
}

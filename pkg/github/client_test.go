package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

func TestFetchIssues_Pagination(t *testing.T) {
	sinceTime := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	sinceStr := sinceTime.Format(time.RFC3339)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("since") != sinceStr {
			t.Errorf("expected since parameter %q, got %q", sinceStr, q.Get("since"))
		}

		pageStr := q.Get("page")
		page := 1
		if pageStr != "" {
			var err error
			page, err = strconv.Atoi(pageStr)
			if err != nil {
				t.Fatalf("invalid page parameter: %v", err)
			}
		}

		var issues []*github.Issue
		var nextPage int

		switch page {
		case 1:
			issues = []*github.Issue{
				{ID: github.Int64(1), Title: github.String("Issue 1")},
				{ID: github.Int64(2), Title: github.String("Issue 2")},
			}
			nextPage = 2
		case 2:
			issues = []*github.Issue{
				{ID: github.Int64(3), Title: github.String("Issue 3")},
				{ID: github.Int64(4), Title: github.String("Issue 4")},
			}
			nextPage = 3
		case 3:
			issues = []*github.Issue{
				{ID: github.Int64(5), Title: github.String("Issue 5")},
			}
			nextPage = 0
		default:
			t.Fatalf("unexpected page request: %d", page)
		}

		if nextPage != 0 {
			linkHeader := fmt.Sprintf("<%s?page=%d&since=%s>; rel=\"next\"", server.URL+r.URL.Path, nextPage, url.QueryEscape(sinceStr))
			w.Header().Set("Link", linkHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issues)
	}))
	defer server.Close()

	httpClient := server.Client()
	client := github.NewClient(httpClient)
	baseURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = baseURL

	fetcher := NewIssueFetcher(client)
	issues, err := fetcher.FetchIssues(context.Background(), "owner", "repo", sinceTime, 2)
	if err != nil {
		t.Fatalf("FetchIssues failed: %v", err)
	}

	if len(issues) != 5 {
		t.Errorf("expected 5 issues, got %d", len(issues))
	}

	expectedIDs := []int64{1, 2, 3, 4, 5}
	for i, issue := range issues {
		if issue.GetID() != expectedIDs[i] {
			t.Errorf("at index %d: expected ID %d, got %d", i, expectedIDs[i], issue.GetID())
		}
	}
}

func TestFetchIssues_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	httpClient := server.Client()
	client := github.NewClient(httpClient)
	baseURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = baseURL

	fetcher := NewIssueFetcher(client)
	_, err := fetcher.FetchIssues(context.Background(), "owner", "repo", time.Now(), 2)
	if err == nil {
		t.Fatal("expected error due to rate limit, got nil")
	}

	if !strings.Contains(err.Error(), "rate limit") && !strings.Contains(err.Error(), "403") {
		t.Errorf("expected rate limit or 403 error, got: %v", err)
	}
}

func TestFetchIssues_EmptyPageWithNext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		pageStr := q.Get("page")
		page := 1
		if pageStr != "" {
			var err error
			page, err = strconv.Atoi(pageStr)
			if err != nil {
				t.Fatalf("invalid page parameter: %v", err)
			}
		}

		var issues []*github.Issue
		var nextPage int

		switch page {
		case 1:
			issues = []*github.Issue{}
			nextPage = 2
		case 2:
			issues = []*github.Issue{
				{ID: github.Int64(1), Title: github.String("Issue 1")},
			}
			nextPage = 0
		default:
			t.Fatalf("unexpected page request: %d", page)
		}

		if nextPage != 0 {
			linkHeader := fmt.Sprintf("<%s?page=%d>; rel=\"next\"", server.URL+r.URL.Path, nextPage)
			w.Header().Set("Link", linkHeader)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issues)
	}))
	defer server.Close()

	httpClient := server.Client()
	client := github.NewClient(httpClient)
	baseURL, _ := url.Parse(server.URL + "/")
	client.BaseURL = baseURL

	fetcher := NewIssueFetcher(client)
	issues, err := fetcher.FetchIssues(context.Background(), "owner", "repo", time.Now(), 2)
	if err != nil {
		t.Fatalf("FetchIssues failed: %v", err)
	}

	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

func TestFetchIssues_Integration(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	repoFullName := os.Getenv("GITHUB_REPOSITORY")
	if token == "" || repoFullName == "" {
		t.Skip("Skipping integration test; GITHUB_TOKEN and GITHUB_REPOSITORY must be set")
	}

	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		t.Fatalf("invalid GITHUB_REPOSITORY: %s", repoFullName)
	}
	owner, repo := parts[0], parts[1]

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	fetcher := NewIssueFetcher(client)
	since := time.Now().Add(-30 * 24 * time.Hour)

	issues, err := fetcher.FetchIssues(ctx, owner, repo, since, 2)
	if err != nil {
		t.Fatalf("FetchIssues failed: %v", err)
	}

	t.Logf("Successfully fetched %d issues", len(issues))
}

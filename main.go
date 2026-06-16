package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/alisteuber4ee1/API-Integration-Pagination/pkg/github"
	google_github "github.com/google/go-github/v57/github"
)

func main() {
	fmt.Println("Hello, Bounty Hunter!")

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("GITHUB_TOKEN not set, skipping demo run.")
		return
	}

	client := google_github.NewClient(nil)
	fetcher := github.NewIssueFetcher(client)
	since := time.Now().Add(-7 * 24 * time.Hour)
	issues, err := fetcher.FetchIssues(context.Background(), "google", "go-github", since, 10)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("Fetched %d issues from google/go-github\n", len(issues))
}

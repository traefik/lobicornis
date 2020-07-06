package search

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/google/go-github/v32/github"
)

type byUpdated []*github.Issue

func (a byUpdated) Len() int      { return len(a) }
func (a byUpdated) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byUpdated) Less(i, j int) bool {
	return a[i].GetUpdatedAt().Before(a[j].GetUpdatedAt())
}

// FindOpenPR find open PR.
func FindOpenPR(ctx context.Context, client *github.Client, owner, repositoryName string, debug bool, parameters ...Parameter) ([]*github.Issue, error) {
	var filter string
	for _, param := range parameters {
		if param != nil {
			filter += param()
		}
	}

	query := fmt.Sprintf("repo:%s/%s type:pr state:open %s", owner, repositoryName, filter)
	if debug {
		log.Println(query)
	}

	options := &github.SearchOptions{
		Sort:        "updated",
		Order:       "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	issues, err := findIssues(ctx, client, query, options)
	if err != nil {
		return nil, err
	}

	sort.Sort(byUpdated(issues))

	return issues, nil
}

func findIssues(ctx context.Context, client *github.Client, query string, searchOptions *github.SearchOptions) ([]*github.Issue, error) {
	var allIssues []*github.Issue
	for {
		issuesSearchResult, resp, err := client.Search.Issues(ctx, query, searchOptions)
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, issuesSearchResult.Issues...)
		if resp.NextPage == 0 {
			break
		}

		searchOptions.Page = resp.NextPage
	}
	return allIssues, nil
}

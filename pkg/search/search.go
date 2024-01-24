package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v58/github"
	"github.com/rs/zerolog/log"
	"github.com/traefik/lobicornis/v3/pkg/conf"
)

// Finder a pull request search manager.
type Finder struct {
	client  *github.Client
	markers conf.Markers
	retry   conf.Retry
}

// New creates a new finder.
func New(client *github.Client, markers conf.Markers, retry conf.Retry) Finder {
	return Finder{
		client:  client,
		markers: markers,
		retry:   retry,
	}
}

// Search searches all PR in all repositories of the user.
func (f Finder) Search(ctx context.Context, user string, parameters ...Parameter) (map[string][]*github.Issue, error) {
	query := fmt.Sprintf("user:%s type:pr state:open ", user)

	for _, param := range parameters {
		if param != nil {
			query += param()
		}
	}

	searchOpts := &github.SearchOptions{
		Sort:        "updated",
		Order:       "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	overview := make(map[string][]*github.Issue)

	var count int
	for {
		count++

		searchResult, resp, err := f.client.Search.Issues(ctx, query, searchOpts)
		if err != nil {
			return nil, err
		}

		for _, issue := range searchResult.Issues {
			fullName := getFullName(issue.GetRepositoryURL())

			overview[fullName] = append(overview[fullName], issue)
		}

		if resp.NextPage == 0 {
			break
		}

		searchOpts.Page = resp.NextPage
	}

	log.Debug().Str("query", query).Int("count", count).Msg("search queries count")

	return overview, nil
}

// GetCurrentPull gets the current pull request.
// priorities: ff > retry > in progress > need merge
func (f Finder) GetCurrentPull(ctx context.Context, issues []*github.Issue) (*github.Issue, error) {
	if len(issues) == 0 {
		return nil, nil
	}

	ff := findIssuesWithLabel(issues, f.markers.MergeMethodPrefix+conf.MergeMethodFastForward)
	if len(ff) > 1 {
		f.displayIssues(ff)

		return nil, fmt.Errorf("multiple pull requests with the label %s", f.markers.MergeMethodPrefix+conf.MergeMethodFastForward)
	}

	if len(ff) > 0 {
		return ff[0], nil
	}

	inProgress := findIssuesWithLabel(issues, f.markers.MergeInProgress)

	if len(inProgress) == 0 {
		f.displayIssues(issues)

		return issues[0], nil
	}

	if len(inProgress) > 2 {
		return nil, fmt.Errorf("illegal state: multiple PR with the label: %s", f.markers.MergeInProgress)
	}

	if f.retry.Number > 0 {
		// find retries

		var issuesRetry []*github.Issue
		for _, issue := range inProgress {
			if findLabelPrefix(issue.Labels, f.markers.MergeRetryPrefix) != "" {
				issuesRetry = append(issuesRetry, issue)
			}
		}

		if len(issuesRetry) > 0 {
			logger := log.Ctx(ctx)

			for _, issue := range issuesRetry {
				if time.Since(issue.GetUpdatedAt().Time) > f.retry.Interval {
					logger.Debug().Msgf("Find PR updated at %v", issue.GetUpdatedAt())

					return issue, nil
				}
			}

			return nil, nil
		}
	}

	f.displayIssues(inProgress)

	return inProgress[0], nil
}

func (f Finder) displayIssues(issues []*github.Issue) {
	for _, issue := range issues {
		log.Debug().Int("pr", issue.GetNumber()).Msgf("Find PR updated at %v", issue.GetUpdatedAt())
	}
}

func findIssuesWithLabel(issues []*github.Issue, lbl string) []*github.Issue {
	var result []*github.Issue

	for _, issue := range issues {
		for _, label := range issue.Labels {
			if strings.EqualFold(label.GetName(), lbl) {
				result = append(result, issue)
			}
		}
	}

	return result
}

func getFullName(repoURL string) string {
	n := strings.Split(repoURL, "/")

	return n[len(n)-2] + "/" + n[len(n)-1]
}

// findLabelPrefix Find an issue with a specific label prefix.
func findLabelPrefix(labels []*github.Label, prefix string) string {
	for _, lbl := range labels {
		if strings.HasPrefix(lbl.GetName(), prefix) {
			return lbl.GetName()
		}
	}

	return ""
}

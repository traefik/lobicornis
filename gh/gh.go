package gh

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Merge Methods
const (
	MergeMethodSquash      = "squash"
	MergeMethodRebase      = "rebase"
	MergeMethodMerge       = "merge"
	MergeMethodFastForward = "ff"
)

// GHub GitHub helper
type GHub struct {
	client *github.Client
	dryRun bool
	debug  bool
}

// NewGHub create a new GHub
func NewGHub(client *github.Client, dryRun bool, debug bool) *GHub {
	return &GHub{client: client, dryRun: dryRun, debug: debug}
}

// FindFirstCommit find the first commit of a PR
func (g *GHub) FindFirstCommit(ctx context.Context, pr *github.PullRequest) (*github.RepositoryCommit, error) {
	options := &github.ListOptions{
		PerPage: 1,
	}

	commits, _, err := g.client.PullRequests.ListCommits(
		ctx,
		pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(),
		pr.GetNumber(),
		options)
	if err != nil {
		return nil, err
	}

	return commits[0], nil
}

// RemoveLabels remove some labels on an issue (PR)
func (g *GHub) RemoveLabels(ctx context.Context, issue *github.Issue, repoID types.RepoID, labelsToRemove []string) error {
	freshIssue, _, err := g.client.Issues.Get(ctx, repoID.Owner, repoID.RepositoryName, issue.GetNumber())
	if err != nil {
		return err
	}

	var newLabels []string
	for _, lbl := range freshIssue.Labels {
		if !contains(labelsToRemove, lbl.GetName()) {
			newLabels = append(newLabels, lbl.GetName())
		}
	}

	if len(freshIssue.Labels) != len(newLabels) {
		if newLabels == nil {
			// Due to go-github/GitHub API constraint
			newLabels = []string{}
		}
		_, _, errLabels := g.client.Issues.ReplaceLabelsForIssue(ctx, repoID.Owner, repoID.RepositoryName, issue.GetNumber(), newLabels)
		return errLabels
	}
	return nil
}

// RemoveLabel remove a label on an issue (PR)
func (g *GHub) RemoveLabel(ctx context.Context, issue *github.Issue, repoID types.RepoID, label string) error {
	if HasLabel(issue, label) {
		log.Printf("Remove label: %s. Dry run: %v", label, g.dryRun)

		if g.dryRun {
			return nil
		}

		resp, err := g.client.Issues.RemoveLabelForIssue(ctx, repoID.Owner, repoID.RepositoryName, issue.GetNumber(), label)

		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to remove label %s. Status code: %d", label, resp.StatusCode)
		}
	}
	return nil
}

// AddLabels add some labels on an issue (PR)
func (g *GHub) AddLabels(ctx context.Context, issue *github.Issue, repoID types.RepoID, labels ...string) error {
	log.Printf("Add labels: %s. Dry run: %v", labels, g.dryRun)

	if g.dryRun {
		return nil
	}

	_, resp, err := g.client.Issues.AddLabelsToIssue(ctx, repoID.Owner, repoID.RepositoryName, issue.GetNumber(), labels)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to add labels %v. Status code: %d", labels, resp.StatusCode)
	}

	return nil
}

// AddComment add a comment on a PR
func (g *GHub) AddComment(ctx context.Context, pr *github.PullRequest, msg string) error {
	comment := &github.IssueComment{Body: github.String(msg)}

	_, resp, err := g.client.Issues.CreateComment(ctx, pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), comment)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to add comment %s. Status code: %d", msg, resp.StatusCode)
	}

	return nil
}

// HasLabel checks if an issue has a specific label
func HasLabel(issue *github.Issue, label string) bool {
	for _, lbl := range issue.Labels {
		if lbl.GetName() == label {
			return true
		}
	}
	return false
}

// FindLabelPrefix Find an issue with a specific label prefix
func FindLabelPrefix(issue *github.Issue, prefix string) string {
	for _, lbl := range issue.Labels {
		if strings.HasPrefix(lbl.GetName(), prefix) {
			return lbl.GetName()
		}
	}
	return ""
}

// IsOnMainRepository checks if the branch of the Pull Request in on the main repository.
func IsOnMainRepository(pr *github.PullRequest) bool {
	return pr.Base.Repo.GetGitURL() == pr.Head.Repo.GetGitURL()
}

// NewGitHubClient create a new GitHub client
func NewGitHubClient(ctx context.Context, token string, gitHubURL *url.URL) *github.Client {
	var client *github.Client
	if len(token) == 0 {
		client = github.NewClient(nil)
	} else {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	}

	if gitHubURL != nil {
		client.BaseURL = gitHubURL
	}

	return client
}

func contains(values []string, value string) bool {
	for _, val := range values {
		if value == val {
			return true
		}
	}
	return false
}

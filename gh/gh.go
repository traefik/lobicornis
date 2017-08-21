package gh

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	// MergeMethodSquash 'squash' merge method.
	MergeMethodSquash = "squash"
	// MergeMethodRebase 'rebase' merge method.
	MergeMethodRebase = "rebase"
	// MergeMethodMerge 'merge' merge method.
	MergeMethodMerge = "merge"
)

// GHub GitHub helper
type GHub struct {
	ctx    context.Context
	client *github.Client
	dryRun bool
}

// NewGHub create a new GHub
func NewGHub(ctx context.Context, client *github.Client, dryRun bool) *GHub {
	return &GHub{ctx: ctx, client: client, dryRun: dryRun}
}

// FindFirstCommitSHA find the first commit SHA of a PR
func (g *GHub) FindFirstCommitSHA(pr *github.PullRequest) (string, error) {
	options := &github.ListOptions{
		PerPage: 1,
	}

	commits, _, err := g.client.PullRequests.ListCommits(
		g.ctx,
		pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(),
		pr.GetNumber(),
		options)
	if err != nil {
		return "", err
	}

	return commits[0].GetSHA(), nil
}

// RemoveLabel remove a label on an issue (PR)
func (g *GHub) RemoveLabel(issue *github.Issue, owner string, repositoryName string, label string) error {
	if hasLabel(issue, label) {
		log.Printf("Remove label: %s. Dry run: %v", label, g.dryRun)

		if g.dryRun {
			return nil
		}

		resp, err := g.client.Issues.RemoveLabelForIssue(g.ctx, owner, repositoryName, issue.GetNumber(), label)

		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Failed to remove label %s. Status code: %d", label, resp.StatusCode)
		}
	}
	return nil
}

// AddLabels add some labels on an issue (PR)
func (g *GHub) AddLabels(issue *github.Issue, owner string, repositoryName string, labels ...string) error {
	log.Printf("Add labels: %s. Dry run: %v", labels, g.dryRun)

	if g.dryRun {
		return nil
	}

	_, resp, err := g.client.Issues.AddLabelsToIssue(g.ctx, owner, repositoryName, issue.GetNumber(), labels)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to add labels %v. Status code: %d", labels, resp.StatusCode)
	}

	return nil
}

// AddComment add a comment on a PR
func (g *GHub) AddComment(pr *github.PullRequest, msg string) error {
	comment := &github.IssueComment{Body: github.String(msg)}

	_, resp, err := g.client.Issues.CreateComment(g.ctx, pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), comment)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Failed to add comment %s. Status code: %d", msg, resp.StatusCode)
	}

	return nil
}

func hasLabel(issue *github.Issue, label string) bool {
	for _, lbl := range issue.Labels {
		if lbl.GetName() == label {
			return true
		}
	}
	return false
}

// NewGitHubClient create a new GitHub client
func NewGitHubClient(ctx context.Context, token string) *github.Client {
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
	return client
}

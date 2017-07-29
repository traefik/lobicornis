package gh

import (
	"errors"
	"fmt"

	"github.com/google/go-github/github"
)

const (
	// Pending Check state
	Pending = "pending"
	// Success Check state
	Success = "success"

	// Approved Review state
	Approved = "APPROVED"
	// Commented Review state
	Commented = "COMMENTED"
)

// HasReviewsApprove check if a PR have the required number of review
func (g *GHub) HasReviewsApprove(pr *github.PullRequest, minReview int) error {

	owner := pr.Base.Repo.Owner.GetLogin()
	repositoryName := pr.Base.Repo.GetName()
	prNumber := pr.GetNumber()

	reviews, _, err := g.client.PullRequests.ListReviews(g.ctx, owner, repositoryName, prNumber, nil)
	if err != nil {
		return err
	}

	reviewsState := make(map[string]string)
	for _, review := range reviews {
		if review.GetState() != Commented {
			reviewsState[review.User.GetLogin()] = review.GetState()
			// TODO debug level: log.Printf("PR%d - %s: %s\n", prNumber, review.User.GetLogin(), review.GetState())
		}
	}

	if len(reviewsState) < minReview {
		return fmt.Errorf("Need more review [%v/2]", len(reviewsState))
	}

	for login, state := range reviewsState {
		if state != Approved {
			return fmt.Errorf("%s by %s", state, login)
		}
	}

	return nil
}

// IsUpdatedBranch check if a PR is up to date
func (g *GHub) IsUpdatedBranch(pr *github.PullRequest) (bool, error) {

	branch := pr.Base.GetRef()

	ref, _, err := g.client.Git.GetRef(
		g.ctx,
		pr.Base.Repo.Owner.GetLogin(),
		pr.Base.Repo.GetName(),
		fmt.Sprintf("heads/%s", branch))
	if err != nil {
		return false, err
	}

	prSHA := pr.Base.GetSHA()
	branchHeadSHA := ref.Object.GetSHA()

	return prSHA == branchHeadSHA, nil
}

// GetStatus provide checks status (CI)
func (g *GHub) GetStatus(pr *github.PullRequest) (string, error) {

	owner := pr.Base.Repo.Owner.GetLogin()
	repositoryName := pr.Base.Repo.GetName()
	prRef := pr.Head.GetSHA()

	sts, _, err := g.client.Repositories.GetCombinedStatus(g.ctx, owner, repositoryName, prRef, nil)
	if err != nil {
		return "", err
	}

	if sts.GetState() == Pending || sts.GetState() == Success {
		return sts.GetState(), nil
	}

	statuses, _, err := g.client.Repositories.ListStatuses(g.ctx, owner, repositoryName, prRef, nil)
	if err != nil {
		return "", err
	}
	var summary string
	for _, stat := range statuses {
		if stat.GetState() != Success {
			summary += stat.GetDescription() + "\n"
		}
	}
	return "", errors.New(summary)
}

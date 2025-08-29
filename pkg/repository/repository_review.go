package repository

import (
	"context"
	"fmt"

	"github.com/google/go-github/v74/github"
)

const (
	// Pending Check state.
	Pending = "pending"
	// Success Check state.
	Success = "success"
	// Neutral Check state.
	Neutral = "neutral"
	// InProgress Check state.
	InProgress = "in_progress"
	// Queued Check state.
	Queued = "queued"
	// Skipped Check state.
	Skipped = "skipped"

	// Approved Review state.
	Approved = "APPROVED"
	// Commented Review state.
	Commented = "COMMENTED"
	// Dismissed Review state.
	Dismissed = "DISMISSED"
)

// hasReviewsApprove check if a PR have the required number of review.
func (r *Repository) hasReviewsApprove(ctx context.Context, pr *github.PullRequest) error {
	minReview := r.getMinReview(pr)

	if minReview == 0 {
		return nil
	}

	opt := &github.ListOptions{
		PerPage: 100,
	}

	reviewsState := make(map[string]string)
	for {
		reviews, resp, err := r.client.PullRequests.ListReviews(ctx, r.owner, r.name, pr.GetNumber(), opt)
		if err != nil {
			return err
		}

		for _, review := range reviews {
			if review.GetState() == Dismissed {
				delete(reviewsState, review.User.GetLogin())
			} else if review.GetState() != Commented {
				reviewsState[review.User.GetLogin()] = review.GetState()
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	if len(reviewsState) < minReview {
		return fmt.Errorf("need more review [%d/%d]", len(reviewsState), minReview)
	}

	for login, state := range reviewsState {
		if state != Approved {
			return fmt.Errorf("%s by %s", state, login)
		}
	}

	return nil
}

// getMinReview Get minimal number of review for an issue.
func (r *Repository) getMinReview(pr *github.PullRequest) int {
	if r.config.GetMinLightReview() != 0 && hasLabel(pr, r.markers.LightReview) {
		return r.config.GetMinLightReview()
	}

	return r.config.GetMinReview()
}

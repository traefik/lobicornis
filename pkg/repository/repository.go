package repository

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/containous/lobicornis/v2/pkg/conf"
	"github.com/google/go-github/v32/github"
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
	// Dismissed Review state
	Dismissed = "DISMISSED"
)

const mainBranch = "master"

type numbered interface {
	GetNumber() int
}

// Repository a new repository manager.
type Repository struct {
	client *github.Client

	clone   Clone
	mjolnir Mjolnir

	debug  bool
	dryRun bool

	markers conf.Markers
	retry   conf.Retry

	owner string
	name  string

	token string

	config conf.RepoConfig
}

// New creates a new repository manager.
func New(client *github.Client, fullName, token string, markers conf.Markers, retry conf.Retry, gitConfig conf.Git, config conf.RepoConfig, extra conf.Extra) *Repository {
	repoFragments := strings.Split(fullName, "/")

	owner := repoFragments[0]
	repoName := repoFragments[1]

	return &Repository{
		client:  client,
		clone:   newClone(gitConfig, token, extra.Debug),
		mjolnir: newMjolnir(client, owner, repoName, extra.Debug, extra.DryRun),
		debug:   extra.Debug,
		dryRun:  extra.DryRun,
		markers: markers,
		retry:   retry,
		owner:   owner,
		name:    repoName,
		token:   token,
		config:  config,
	}
}

// Process try to merge a pull request.
func (r Repository) Process(ctx context.Context, prNumber int) error {
	pr, _, err := r.client.PullRequests.Get(ctx, r.owner, r.name, prNumber)
	if err != nil {
		return err
	}

	log.Println(pr.GetHTMLURL())

	if r.config.GetNeedMilestone() && pr.Milestone == nil {
		log.Printf("PR #%d: Must have a milestone.", prNumber)

		r.callHuman(ctx, pr, "error: The milestone is missing.")

		return nil
	}

	err = r.hasReviewsApprove(ctx, pr)
	if err != nil {
		log.Printf("PR #%d: Needs more reviews: %v", prNumber, err)

		r.callHuman(ctx, pr, fmt.Sprintf("error: %v", err))

		return nil
	}

	status, err := r.getAggregatedState(ctx, pr)
	if err != nil {
		log.Printf("PR #%d: Checks status: %v", prNumber, err)

		r.manageRetryLabel(ctx, pr, r.retry.OnStatuses)

		return nil
	}

	if status == Pending {
		// skip
		log.Printf("PR #%d: State: pending. Waiting for the CI.", prNumber)
		return nil
	}

	if pr.GetMerged() {
		labelsToRemove := []string{
			r.markers.MergeInProgress,
			r.markers.NeedMerge,
			r.markers.LightReview,
			r.markers.MergeMethodPrefix + MergeMethodSquash,
			r.markers.MergeMethodPrefix + MergeMethodMerge,
			r.markers.MergeMethodPrefix + MergeMethodRebase,
			r.markers.MergeMethodPrefix + MergeMethodFastForward,
		}
		errLabel := r.removeLabels(ctx, pr, labelsToRemove)
		if errLabel != nil {
			log.Println(errLabel)
		}

		log.Printf("the PR #%d is already merged", prNumber)
		return nil
	}

	if !pr.GetMergeable() {
		log.Printf("PR #%d: Conflicts must be resolve in the PR.", prNumber)

		r.manageRetryLabel(ctx, pr, r.retry.OnMergeable)

		return nil
	}

	retry := r.retry.OnMergeable || r.retry.OnStatuses
	r.cleanRetryLabel(ctx, pr, retry)

	// Get status checks
	var needUpdate bool
	if r.config.GetCheckNeedUpToDate() {
		rcs, _, errCheck := r.client.Repositories.GetRequiredStatusChecks(ctx, r.owner, r.name, pr.Base.GetRef())
		if errCheck != nil {
			return fmt.Errorf("PR #%d: unable to get status checks: %w", prNumber, errCheck)
		}

		needUpdate = rcs.Strict
	} else if r.config.GetForceNeedUpToDate() {
		needUpdate = true
	}

	mergeMethod, err := r.getMergeMethod(pr)
	if err != nil {
		return err
	}

	upToDateBranch, err := r.isUpToDateBranch(ctx, pr)
	if err != nil {
		return err
	}

	if !upToDateBranch && mergeMethod == MergeMethodFastForward {
		r.callHuman(ctx, pr, fmt.Sprintf("merge method [%s] is impossible when a branch is not up-to-date", mergeMethod))

		return fmt.Errorf("PR #%d: merge method [%s] is impossible when a branch is not up-to-date", prNumber, mergeMethod)
	}

	// Need to be up to date?
	if needUpdate {
		if !pr.GetMaintainerCanModify() && !isOnMainRepository(pr) {
			repo, _, err := r.client.Repositories.Get(ctx, r.owner, r.name)
			if err != nil {
				return err
			}

			if !repo.GetPrivate() && !repo.GetFork() {
				r.callHuman(ctx, pr, "the contributor doesn't allow maintainer modification (GitHub option)")

				return fmt.Errorf("PR #%d: the contributor doesn't allow maintainer modification (GitHub option)", prNumber)
			}
		}

		if upToDateBranch {
			err := r.merge(ctx, pr, mergeMethod)
			if err != nil {
				return err
			}
		} else {
			err := r.update(ctx, pr)
			if err != nil {
				return fmt.Errorf("PR #%d: failed to update", pr.GetNumber())
			}
		}
	} else {
		err := r.merge(ctx, pr, mergeMethod)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r Repository) callHuman(ctx context.Context, pr *github.PullRequest, message string) {
	err := r.addComment(ctx, pr, message)
	if err != nil {
		log.Println(err)
	}

	err = r.addLabels(ctx, pr, r.markers.NeedHumanMerge)
	if err != nil {
		log.Println(err)
	}

	err = r.removeLabel(ctx, pr, r.markers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}
}

func (r Repository) addComment(ctx context.Context, pr *github.PullRequest, message string) error {
	if !r.config.GetAddErrorInComment() {
		return nil
	}

	msg := strings.ReplaceAll(message, r.token, "xxx")

	if r.dryRun {
		log.Println("Add comment:", msg)
		return nil
	}

	comment := &github.IssueComment{
		Body: github.String(msg),
	}

	_, _, err := r.client.Issues.CreateComment(ctx, r.owner, r.name, pr.GetNumber(), comment)
	if err != nil {
		return err
	}

	return nil
}

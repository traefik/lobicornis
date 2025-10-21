package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v74/github"
	"github.com/rs/zerolog/log"
	"github.com/traefik/lobicornis/v3/pkg/conf"
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
		clone:   newClone(gitConfig, token),
		mjolnir: newMjolnir(client, owner, repoName, extra.DryRun),
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
func (r *Repository) Process(ctx context.Context, prNumber int) error {
	pr, _, err := r.client.PullRequests.Get(ctx, r.owner, r.name, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	err = r.process(ctx, pr)
	if err != nil {
		r.callHuman(ctx, pr, err.Error())

		return err
	}

	return nil
}

// process try to merge a pull request.
func (r *Repository) process(ctx context.Context, pr *github.PullRequest) error {
	logger := log.Ctx(ctx)

	if r.config.GetNeedMilestone() && pr.Milestone == nil {
		return errors.New("the milestone is missing")
	}

	err := r.hasReviewsApprove(ctx, pr)
	if err != nil {
		return fmt.Errorf("error related to review: %w", err)
	}

	status, err := r.getAggregatedState(ctx, pr)
	if err != nil {
		logger.Error().Err(err).Msg("Checks status")

		return r.manageRetryLabel(ctx, pr, r.retry.OnStatuses, fmt.Errorf("checks status: %w", err))
	}

	if status == Pending || status == Queued || status == InProgress {
		// skip
		logger.Info().Msg("State: pending. Waiting for the CI.")
		return nil
	}

	if pr.GetMerged() {
		logger.Info().Msg("the PR is already merged")

		labelsToRemove := []string{
			r.markers.MergeInProgress,
			r.markers.NeedMerge,
			r.markers.LightReview,
			r.markers.MergeMethodPrefix + conf.MergeMethodSquash,
			r.markers.MergeMethodPrefix + conf.MergeMethodMerge,
			r.markers.MergeMethodPrefix + conf.MergeMethodRebase,
			r.markers.MergeMethodPrefix + conf.MergeMethodFastForward,
			r.markers.MergeNoRebase,
		}
		err = r.removeLabels(ctx, pr, labelsToRemove)
		ignoreError(ctx, err)

		return nil
	}

	if !pr.GetMergeable() {
		logger.Info().Msg("Conflicts must be resolved in the PR.")

		return r.manageRetryLabel(ctx, pr, r.retry.OnMergeable, errors.New("conflicts must be resolved in the PR"))
	}

	switch pr.GetMergeableState() {
	case MergeableStateDraft, MergeableStateBlocked, MergeableStateUnknown:
		return fmt.Errorf("the mergeable state is %q", pr.GetMergeableState())
	}

	r.cleanRetryLabel(ctx, pr)

	// Get status checks
	var needUpdate bool
	if r.config.GetCheckNeedUpToDate() {
		rcs, _, errCheck := r.client.Repositories.GetRequiredStatusChecks(ctx, r.owner, r.name, pr.Base.GetRef())
		if errCheck != nil {
			return fmt.Errorf("unable to get status checks: %w", errCheck)
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

	if !upToDateBranch && mergeMethod == conf.MergeMethodFastForward {
		return fmt.Errorf("the use of the merge method [%s] is impossible when a branch is not up-to-date", mergeMethod)
	}

	// Need to be up to date?
	if needUpdate && !upToDateBranch && !hasLabel(pr, r.markers.MergeNoRebase) {
		err := r.update(ctx, pr)
		if err != nil {
			err = fmt.Errorf("failed to update: %w", err)
		}
		return err
	}

	return r.merge(ctx, pr, mergeMethod)
}

func (r *Repository) callHuman(ctx context.Context, pr *github.PullRequest, message string) {
	log.Ctx(ctx).Warn().Msg(message)

	err := r.addComment(ctx, pr, ":no_entry_sign: "+message)
	ignoreError(ctx, err)

	err = r.addLabels(ctx, pr, r.markers.NeedHumanMerge)
	ignoreError(ctx, err)

	err = r.removeLabel(ctx, pr, r.markers.MergeInProgress)
	ignoreError(ctx, err)
}

func (r *Repository) addComment(ctx context.Context, pr *github.PullRequest, message string) error {
	if !r.config.GetAddErrorInComment() && !pr.Base.Repo.GetPrivate() {
		return nil
	}

	msg := strings.ReplaceAll(message, r.token, "xxx")

	if r.dryRun {
		log.Ctx(ctx).Debug().Msgf("Add comment: %s", msg)
		return nil
	}

	comment := &github.IssueComment{Body: github.Ptr(msg)}

	_, _, err := r.client.Issues.CreateComment(ctx, r.owner, r.name, pr.GetNumber(), comment)

	return err
}

func ignoreError(ctx context.Context, err error) {
	if err != nil {
		log.Ctx(ctx).Debug().Err(err).Msg("ignored error")
	}
}

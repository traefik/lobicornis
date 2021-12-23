package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v41/github"
	"github.com/rs/zerolog/log"
)

// MergeableState Provides extra information regarding the mergeability of a pull request
// https://github.com/octokit/webhooks.net/blob/b8018e212f0d1c4af9f7faaaf620e4f65faa258c/src/Octokit.Webhooks/Models/PullRequestEvent/PullRequest.cs
const (
	// MergeableStateDirty Merge conflict. Merging is blocked.
	MergeableStateDirty = "dirty"

	// MergeableStateUnknown Mergeability was not checked yet. Merging is blocked.
	MergeableStateUnknown = "unknown"

	// MergeableStateBlocked Failing/missing required status check.  Merging is blocked.
	MergeableStateBlocked = "blocked"

	// MergeableStateBehind Head branch is behind the base branch. Only if required status checks is enabled but loose policy is not. Merging is blocked.
	MergeableStateBehind = "behind"

	// MergeableStateUnstable Failing/pending commit status that is not part of the required status checks. Merging is still allowed.
	MergeableStateUnstable = "unstable"

	// MergeableStateHasHooks GitHub Enterprise only, if a repo has custom pre-receive hooks. Merging is allowed.
	MergeableStateHasHooks = "has_hooks"

	// MergeableStateClean No conflicts, everything good. Merging is allowed.
	MergeableStateClean = "clean"

	// MergeableStateDraft Not ready for review. Merging is blocked.
	MergeableStateDraft = "draft"
)

// isUpToDateBranch check if a PR is up to date.
func (r *Repository) isUpToDateBranch(ctx context.Context, pr *github.PullRequest) (bool, error) {
	head := fmt.Sprintf("%s:%s", pr.Head.User.GetLogin(), pr.Head.GetRef())

	cc, _, err := r.client.Repositories.CompareCommits(ctx, r.owner, r.name, pr.Base.GetRef(), head, nil)
	if err != nil {
		return false, fmt.Errorf("failed to compare commits: %w", err)
	}

	log.Ctx(ctx).Debug().Str("sha", cc.MergeBaseCommit.GetSHA()).Msgf("Merge base commit, behind By %d", cc.GetBehindBy())

	return cc.GetBehindBy() == 0, nil
}

// getAggregatedState provide checks status (status + checksSuite).
func (r *Repository) getAggregatedState(ctx context.Context, pr *github.PullRequest) (string, error) {
	status, err := r.getStatus(ctx, pr)
	if err != nil {
		return "", err
	}

	if status == Pending {
		return status, nil
	}

	return r.getCheckRunsState(ctx, pr)
}

// getStatus provide checks status (status).
func (r *Repository) getStatus(ctx context.Context, pr *github.PullRequest) (string, error) {
	prRef := pr.Head.GetSHA()

	sts, _, err := r.client.Repositories.GetCombinedStatus(ctx, r.owner, r.name, prRef, nil)
	if err != nil {
		return "", err
	}

	if sts.GetState() == Success {
		return sts.GetState(), nil
	}

	// pending: if there are no statuses or a context is pending
	// https://developer.github.com/v3/repos/statuses/#get-the-combined-status-for-a-specific-ref
	if sts.GetState() == Pending {
		if sts.GetTotalCount() == 0 {
			return Success, nil
		}
		return sts.GetState(), nil
	}

	statuses, _, err := r.client.Repositories.ListStatuses(ctx, r.owner, r.name, prRef, nil)
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

// getCheckRunsState provide checks status (checksRun).
func (r *Repository) getCheckRunsState(ctx context.Context, pr *github.PullRequest) (string, error) {
	prRef := pr.Head.GetSHA()

	checkSuites, _, err := r.client.Checks.ListCheckSuitesForRef(ctx, r.owner, r.name, prRef, nil)
	if err != nil {
		return "", err
	}

	if checkSuites.GetTotal() == 0 {
		return Success, nil
	}

	var msg []string
	for _, v := range checkSuites.CheckSuites {
		if v.App != nil && strings.EqualFold(v.GetApp().GetName(), "Dependabot") {
			continue
		}

		if v.GetStatus() != "completed" {
			return Pending, nil
		}

		if v.GetConclusion() == "success" || v.GetConclusion() == "neutral" {
			msg = append(msg, fmt.Sprintf("%s %s %s", v.GetApp().GetName(), v.GetStatus(), v.GetConclusion()))
		}
	}

	if len(msg) > 0 {
		return Success, nil
	}

	return "", errors.New(strings.Join(msg, ", "))
}

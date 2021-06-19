package repository

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/merge"
	"github.com/ldez/go-git-cmd-wrapper/push"
	"github.com/ldez/go-git-cmd-wrapper/rebase"
	"github.com/ldez/go-git-cmd-wrapper/types"
	"github.com/rs/zerolog/log"
)

// Merge action.
const (
	ActionMerge  = "merge"
	ActionRebase = "rebase"
)

func (r *Repository) update(ctx context.Context, pr *github.PullRequest) error {
	logger := log.Ctx(ctx)
	logger.Info().Msg("UPDATE")

	err := r.addLabels(ctx, pr, r.markers.MergeInProgress)
	if err != nil {
		logger.Error().Err(err).Msg("unable to add labels")
	}

	// use GitHub API (update button)
	if !pr.GetMaintainerCanModify() && !isOnMainRepository(pr) {
		if r.dryRun {
			logger.Debug().Msg("Updated via a merge with the GitHub API.")
			return nil
		}

		_, _, err := r.client.PullRequests.UpdateBranch(ctx, pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), nil)
		if err != nil {
			return fmt.Errorf("update branch: %w", err)
		}

		return nil
	}

	return r.cloneAndUpdate(ctx, pr)
}

// Process clone a PR and update if needed.
func (r *Repository) cloneAndUpdate(ctx context.Context, pr *github.PullRequest) error {
	logger := log.Ctx(ctx)

	logger.Info().Msgf("Base branch: %s - Fork branch: %s", pr.Base.GetRef(), pr.Head.GetRef())

	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	if err != nil {
		return err
	}

	defer func() { ignoreError(ctx, os.RemoveAll(dir)) }()

	err = os.Chdir(dir)
	if err != nil {
		return err
	}

	tempDir, _ := os.Getwd()
	logger.Info().Msg(tempDir)

	if isOnMainRepository(pr) && pr.Head.GetRef() == mainBranch {
		return errors.New("the branch master on a main repository cannot be rebased")
	}

	mainRemote, err := r.clone.PullRequestForUpdate(ctx, pr)
	if err != nil {
		return fmt.Errorf("failed to clone: %w", err)
	}

	output, err := r.updatePullRequest(ctx, pr, mainRemote)
	logger.Info().Msg(output)

	if err != nil {
		return fmt.Errorf("failed to update the pull request: %w", err)
	}

	return nil
}

// updatePullRequest Update a pull request.
func (r *Repository) updatePullRequest(ctx context.Context, pr *github.PullRequest, mainRemote string) (string, error) {
	action, err := r.getUpdateAction(ctx, pr)
	if err != nil {
		return "", err
	}

	logger := log.Ctx(ctx)

	if action == ActionRebase {
		logger.Info().Msg("Rebase")

		// rebase
		output, errRebase := rebasePR(pr, mainRemote, r.debug)
		if errRebase != nil {
			logger.Error().Err(errRebase).Msg("unable to rebase PR")
			return output, fmt.Errorf("failed to rebase:\n %s", output)
		}
	} else {
		logger.Info().Msg("Merge")

		// merge
		output, errMerge := mergeBaseHeadIntoPR(pr, mainRemote, r.debug)
		if errMerge != nil {
			logger.Error().Err(errMerge).Msg("unable to merge base head into PR")
			return output, fmt.Errorf("failed to merge base HEAD:\n %s", output)
		}
	}

	// push
	output, err := git.Push(
		git.Cond(r.dryRun, push.DryRun),
		git.Cond(action == ActionRebase, push.ForceWithLease),
		push.Remote(RemoteOrigin),
		push.RefSpec(pr.Head.GetRef()),
		git.Debugger(r.debug))
	if err != nil {
		return output, fmt.Errorf("failed to push branch %s: %w\n %s", pr.Head.GetRef(), err, output)
	}

	return output, nil
}

func (r *Repository) getUpdateAction(ctx context.Context, pr *github.PullRequest) (string, error) {
	// find the first commit of the PR
	firstCommit, err := r.findFirstCommit(ctx, pr)
	if err != nil {
		return "", fmt.Errorf("unable to find the first commit: %w", err)
	}

	// check if PR contains merges
	output, err := git.Raw("log", func(g *types.Cmd) {
		g.AddOptions("--oneline")
		g.AddOptions("--merges")
		g.AddOptions(fmt.Sprintf("%s^..HEAD", firstCommit.GetSHA()))
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg(output)
		return "", fmt.Errorf("failed to display git log: %w", err)
	}

	if len(strings.TrimSpace(output)) > 0 {
		// action merge
		return ActionMerge, nil
	}

	// action rebase
	return ActionRebase, nil
}

// findFirstCommit find the first commit of a PR.
func (r *Repository) findFirstCommit(ctx context.Context, pr *github.PullRequest) (*github.RepositoryCommit, error) {
	options := &github.ListOptions{
		PerPage: 1,
	}

	commits, _, err := r.client.PullRequests.ListCommits(
		ctx,
		pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(),
		pr.GetNumber(),
		options)
	if err != nil {
		return nil, err
	}

	return commits[0], nil
}

func rebasePR(pr *github.PullRequest, remoteName string, debug bool) (string, error) {
	return git.Rebase(
		rebase.PreserveMerges,
		rebase.Branch(fmt.Sprintf("%s/%s", remoteName, pr.Base.GetRef())),
		git.Debugger(debug))
}

func mergeBaseHeadIntoPR(pr *github.PullRequest, remoteName string, debug bool) (string, error) {
	return git.Merge(
		merge.Commits(fmt.Sprintf("%s/%s", remoteName, pr.Base.GetRef())),
		git.Debugger(debug))
}

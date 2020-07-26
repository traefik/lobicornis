package repository

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/merge"
	"github.com/ldez/go-git-cmd-wrapper/push"
	"github.com/ldez/go-git-cmd-wrapper/rebase"
	"github.com/ldez/go-git-cmd-wrapper/types"
)

// Merge action.
const (
	ActionMerge  = "merge"
	ActionRebase = "rebase"
)

func (r *Repository) update(ctx context.Context, pr *github.PullRequest) error {
	log.Println("UPDATE")

	err := r.addLabels(ctx, pr, r.markers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}

	err = r.cloneAndUpdate(ctx, pr)
	if err != nil {
		r.callHuman(ctx, pr, fmt.Sprintf("error: %v", err))

		return err
	}

	return nil
}

// Process clone a PR and update if needed.
func (r *Repository) cloneAndUpdate(ctx context.Context, pr *github.PullRequest) error {
	log.Println("Base branch: ", pr.Base.GetRef(), "- Fork branch: ", pr.Head.GetRef())

	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	if err != nil {
		return err
	}

	defer func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			log.Println(errRemove)
		}
	}()

	err = os.Chdir(dir)
	if err != nil {
		return err
	}

	tempDir, _ := os.Getwd()
	log.Println(tempDir)

	if isOnMainRepository(pr) && pr.Head.GetRef() == "master" {
		return errors.New("the branch master on a main repository cannot be rebased")
	}

	mainRemote, err := r.clone.PullRequestForUpdate(pr)
	if err != nil {
		return fmt.Errorf("failed to clone: %w", err)
	}

	output, err := r.updatePullRequest(ctx, pr, mainRemote)
	log.Println(output)

	return fmt.Errorf("failed to update the pull request: %w", err)
}

// updatePullRequest Update a pull request.
func (r *Repository) updatePullRequest(ctx context.Context, pr *github.PullRequest, mainRemote string) (string, error) {
	action, err := r.getUpdateAction(ctx, pr)
	if err != nil {
		return "", err
	}

	if action == ActionRebase {
		log.Printf("Rebase PR #%d", pr.GetNumber())

		// rebase
		output, errRebase := rebasePR(pr, mainRemote, r.debug)
		if errRebase != nil {
			log.Print(errRebase)
			return output, fmt.Errorf("failed to rebase:\n %s", output)
		}
	} else {
		log.Printf("Merge PR #%d", pr.GetNumber())

		// merge
		output, errMerge := mergeBaseHeadIntoPR(pr, mainRemote, r.debug)
		if errMerge != nil {
			log.Print(errMerge)
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
		log.Print(err)
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
		log.Println(output)
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

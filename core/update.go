package core

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"os"

	"github.com/containous/lobicornis/clone"
	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/types"
	"github.com/containous/lobicornis/update"
	"github.com/google/go-github/v32/github"
)

func updatePR(ctx context.Context, ghub *gh.GHub, issuePR *github.Issue, pr *github.PullRequest, repoID types.RepoID, markers *types.LabelMarkers, gitConfig types.GitConfig, extra types.Extra) error {
	log.Printf("PR #%d: UPDATE", issuePR.GetNumber())

	err := ghub.AddLabels(ctx, issuePR, repoID, markers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}

	err = cloneAndUpdate(ctx, ghub, pr, gitConfig, extra.DryRun, extra.Debug)
	if err != nil {
		errL := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
		if errL != nil {
			log.Println(errL)
		}

		errL = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
		if errL != nil {
			log.Println(errL)
		}

		return err
	}

	return nil
}

// Process clone a PR and update if needed.
func cloneAndUpdate(ctx context.Context, ghub *gh.GHub, pr *github.PullRequest, gitConfig types.GitConfig, dryRun bool, debug bool) error {
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

	if gh.IsOnMainRepository(pr) && pr.Head.GetRef() == "master" {
		return errors.New("master branch cannot be rebase")
	}

	mainRemote, err := clone.PullRequestForUpdate(pr, gitConfig, debug)
	if err != nil {
		return err
	}

	output, err := update.PullRequest(ctx, ghub, pr, mainRemote, dryRun, debug)
	log.Println(output)

	return err
}

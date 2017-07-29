package updater

import (
	"fmt"
	"log"
	"strings"

	"github.com/containous/brahma/gh"
	"github.com/google/go-github/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/merge"
	"github.com/ldez/go-git-cmd-wrapper/push"
	"github.com/ldez/go-git-cmd-wrapper/rebase"
	"github.com/ldez/go-git-cmd-wrapper/types"
)

func updatePR(ghub *gh.GHub, pr *github.PullRequest, mainRemote string, dryRun bool, debug bool) (string, error) {

	action, err := getUpdateAction(ghub, pr)
	if err != nil {
		return "", err
	}

	if action == ActionRebase {
		log.Printf("Rebase PR #%d", pr.GetNumber())

		//rebase
		output, err := rebasePR(pr, mainRemote, debug)
		if err != nil {
			log.Print(err)
			return output, fmt.Errorf("PR #%d: failed to rebase:\n %s", pr.GetNumber(), output)
		}
	} else {
		log.Printf("Merge PR #%d", pr.GetNumber())

		// merge
		output, err := mergeBaseHeadIntoPR(pr, mainRemote, debug)
		if err != nil {
			log.Print(err)
			return output, fmt.Errorf("PR #%d: failed to merge base HEAD:\n %s", pr.GetNumber(), output)
		}
	}

	// push
	output, err := git.Push(
		git.Cond(dryRun, push.DryRun),
		git.Cond(action == ActionRebase, push.ForceWithLease),
		push.Remote(remoteOrigin),
		push.RefSpec(pr.Head.GetRef()),
		git.Debugger(debug))
	if err != nil {
		log.Print(err)
		return output, fmt.Errorf("PR #%d: failed to push branch %s:\n %s", pr.GetNumber(), pr.Head.GetRef(), output)
	}

	return output, nil
}

func getUpdateAction(ghub *gh.GHub, pr *github.PullRequest) (string, error) {
	// find the first commit of the PR
	firstCommitSHA, err := ghub.FindFirstCommitSHA(pr)
	if err != nil {
		return "", fmt.Errorf("PR #%d: unable to find the first commit: %v", pr.GetNumber(), err)
	}

	// check if PR contains merges
	output, err := git.Raw("log", func(g *types.Cmd) {
		g.AddOptions("--oneline")
		g.AddOptions("--merges")
		g.AddOptions(fmt.Sprintf("%s^..HEAD", firstCommitSHA))
	})
	if err != nil {
		log.Println(output)
		return "", fmt.Errorf("PR #%d: failed to display git log: %v", pr.GetNumber(), err)
	}

	if len(strings.TrimSpace(output)) > 0 {
		// action merge
		return ActionMerge, nil
	}
	//action rebase
	return ActionRebase, nil
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

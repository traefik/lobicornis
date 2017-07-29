package updater

import (
	"errors"
	"fmt"
	"log"

	"github.com/google/go-github/github"
	"github.com/ldez/go-git-cmd-wrapper/checkout"
	"github.com/ldez/go-git-cmd-wrapper/clone"
	"github.com/ldez/go-git-cmd-wrapper/config"
	"github.com/ldez/go-git-cmd-wrapper/fetch"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/remote"
)

func clonePR(pr *github.PullRequest, forkURL, baseURL string, debug bool) (string, error) {

	remoteName := remoteUpstream

	if forkURL == baseURL {
		log.Print("It's not a fork, it's a branch on the main repository.")

		if pr.Head.GetRef() == "master" {
			return "", errors.New("master branch cannot be rebase")
		}

		remoteName = remoteOrigin

		output, err := cloneFromMainRepository(pr, baseURL, debug)
		if err != nil {
			log.Print(err)
			return "", errors.New(output)
		}
	} else {
		output, err := cloneFromFork(pr, remoteName, forkURL, baseURL, debug)
		if err != nil {
			log.Print(err)
			return "", errors.New(output)
		}
	}

	return remoteName, nil
}

func cloneFromMainRepository(pr *github.PullRequest, baseURL string, debug bool) (string, error) {

	output, err := git.Clone(clone.Repository(baseURL), clone.Directory("."), git.Debugger(debug))
	if err != nil {
		return output, err
	}

	git.Config(config.Entry("rebase.autoSquash", "true"))
	git.Config(config.Entry("push.default", "current"))

	output, err = git.Checkout(checkout.Branch(pr.Head.GetRef()), git.Debugger(debug))
	if err != nil {
		return output, fmt.Errorf("PR #%d: Failed to checkout branch %s: %v", pr.GetNumber(), pr.Head.GetRef(), err)
	}

	return "", nil
}

func cloneFromFork(pr *github.PullRequest, remoteName, forkURL, baseURL string, debug bool) (string, error) {

	output, err := git.Clone(
		clone.Repository(forkURL),
		clone.Branch(pr.Head.GetRef()),
		clone.Directory("."),
		git.Debugger(debug))
	if err != nil {
		return output, err
	}

	git.Config(config.Entry("rebase.autoSquash", "true"), git.Debugger(debug))
	git.Config(config.Entry("push.default", "current"))

	output, err = git.Remote(remote.Add(remoteName, baseURL), git.Debugger(debug))
	if err != nil {
		return output, fmt.Errorf("PR #%d: failed to add remote: %v", pr.GetNumber(), err)
	}

	output, err = git.Fetch(fetch.NoTags, fetch.Remote(remoteName), fetch.RefSpec(pr.Base.GetRef()), git.Debugger(debug))
	if err != nil {
		return output, fmt.Errorf("PR #%d: failed to fetch %s/%s : %v", pr.GetNumber(), remoteName, pr.Base.GetRef(), err)
	}

	return "", nil
}

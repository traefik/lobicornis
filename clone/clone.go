package clone

import (
	"fmt"
	"log"
	"strings"

	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/v28/github"
	"github.com/ldez/go-git-cmd-wrapper/checkout"
	"github.com/ldez/go-git-cmd-wrapper/clone"
	"github.com/ldez/go-git-cmd-wrapper/config"
	"github.com/ldez/go-git-cmd-wrapper/fetch"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/remote"
)

type remoteModel struct {
	url string
	ref string
}

type prModel struct {
	number    int
	unchanged remoteModel
	changed   remoteModel
}

// PullRequestForMerge Clone a pull request for a merge
func PullRequestForMerge(pr *github.PullRequest, gitConfig types.GitConfig, debug bool) (string, error) {
	var forkURL string
	if pr.Base.Repo.GetPrivate() {
		forkURL = makeRepositoryURL(pr.Head.Repo.GetGitURL(), gitConfig.SSH, gitConfig.GitHubToken)
	} else {
		forkURL = makeRepositoryURL(pr.Head.Repo.GetGitURL(), gitConfig.SSH, "")
	}

	model := prModel{
		number: pr.GetNumber(),
		// fork
		unchanged: remoteModel{
			url: forkURL,
			ref: pr.Head.GetRef(),
		},
		// base
		changed: remoteModel{
			url: makeRepositoryURL(pr.Base.Repo.GetGitURL(), gitConfig.SSH, gitConfig.GitHubToken),
			ref: pr.Base.GetRef(),
		},
	}

	return pullRequest(pr, model, gitConfig, debug)
}

// PullRequestForUpdate Clone a pull request for an update (rebase)
func PullRequestForUpdate(pr *github.PullRequest, gitConfig types.GitConfig, debug bool) (string, error) {
	var unchangedURL string
	if pr.Base.Repo.GetPrivate() {
		unchangedURL = makeRepositoryURL(pr.Base.Repo.GetGitURL(), gitConfig.SSH, gitConfig.GitHubToken)
	} else {
		unchangedURL = makeRepositoryURL(pr.Base.Repo.GetGitURL(), gitConfig.SSH, "")
	}

	model := prModel{
		number: pr.GetNumber(),
		// base
		unchanged: remoteModel{
			url: unchangedURL,
			ref: pr.Base.GetRef(),
		},
		// fork
		changed: remoteModel{
			url: makeRepositoryURL(pr.Head.Repo.GetGitURL(), gitConfig.SSH, gitConfig.GitHubToken),
			ref: pr.Head.GetRef(),
		},
	}

	return pullRequest(pr, model, gitConfig, debug)
}

func pullRequest(pr *github.PullRequest, prModel prModel, gitConfig types.GitConfig, debug bool) (string, error) {
	if gh.IsOnMainRepository(pr) {
		log.Print("It's not a fork, it's a branch on the main repository.")

		remoteName := types.RemoteOrigin

		output, err := fromMainRepository(prModel.changed, prModel.number, gitConfig, debug)
		if err != nil {
			log.Print(output)
			return "", err
		}
		return remoteName, nil
	}

	remoteName := types.RemoteUpstream
	output, err := fromFork(prModel.changed, prModel.unchanged, prModel.number, gitConfig, remoteName, debug)

	if err != nil {
		log.Print(output)
		return "", err
	}
	return remoteName, nil
}

func fromMainRepository(remoteModel remoteModel, prNumber int, gitConfig types.GitConfig, debug bool) (string, error) {
	output, err := git.Clone(clone.Repository(remoteModel.url), clone.Directory("."), git.Debugger(debug))
	if err != nil {
		return output, err
	}

	output, err = configureGit(gitConfig)
	if err != nil {
		return output, err
	}

	output, err = git.Checkout(checkout.Branch(remoteModel.ref), git.Debugger(debug))
	if err != nil {
		return output, fmt.Errorf("PR #%d: Failed to checkout branch %s: %v", prNumber, remoteModel.ref, err)
	}

	return "", nil
}

func fromFork(origin, upstream remoteModel, prNumber int, gitConfig types.GitConfig, remoteName string, debug bool) (string, error) {
	output, err := git.Clone(
		clone.Repository(origin.url),
		clone.Branch(origin.ref),
		clone.Directory("."),
		git.Debugger(debug))
	if err != nil {
		return output, err
	}

	output, err = configureGit(gitConfig)
	if err != nil {
		return output, err
	}

	output, err = git.Remote(remote.Add(remoteName, upstream.url), git.Debugger(debug))
	if err != nil {
		return output, fmt.Errorf("PR #%d: failed to add remote: %v", prNumber, err)
	}

	output, err = git.Fetch(fetch.NoTags, fetch.Remote(remoteName), fetch.RefSpec(upstream.ref), git.Debugger(debug))
	if err != nil {
		return output, fmt.Errorf("PR #%d: failed to fetch %s/%s : %v", prNumber, remoteName, upstream.ref, err)
	}

	return "", nil
}

func makeRepositoryURL(url string, ssh bool, token string) string {
	if ssh {
		return strings.Replace(url, "git://github.com/", "git@github.com:", -1)
	}

	prefix := "https://"
	if len(token) > 0 {
		prefix += token + "@"
	}
	return strings.Replace(url, "git://", prefix, -1)
}

func configureGit(gitConfig types.GitConfig) (string, error) {
	output, err := git.Config(config.Entry("rebase.autoSquash", "true"))
	if err != nil {
		return output, err
	}
	output, err = git.Config(config.Entry("push.default", "current"))
	if err != nil {
		return output, err
	}

	return configureGitUserInfo(gitConfig.UserName, gitConfig.Email)
}

func configureGitUserInfo(gitUserName string, gitUserEmail string) (string, error) {
	if len(gitUserEmail) != 0 {
		output, err := git.Config(config.Entry("user.email", gitUserEmail))
		if err != nil {
			return output, err
		}
	}

	if len(gitUserName) != 0 {
		output, err := git.Config(config.Entry("user.name", gitUserName))
		if err != nil {
			return output, err
		}
	}

	return "", nil
}

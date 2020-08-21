package repository

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/ldez/go-git-cmd-wrapper/checkout"
	"github.com/ldez/go-git-cmd-wrapper/clone"
	"github.com/ldez/go-git-cmd-wrapper/config"
	"github.com/ldez/go-git-cmd-wrapper/fetch"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/remote"
	"github.com/traefik/lobicornis/v2/pkg/conf"
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

// Clone a clone manager.
type Clone struct {
	git   conf.Git
	token string
	debug bool
}

func newClone(gitConfig conf.Git, token string, debug bool) Clone {
	return Clone{
		git:   gitConfig,
		token: token,
		debug: debug,
	}
}

// PullRequestForMerge Clone a pull request for a merge.
func (c Clone) PullRequestForMerge(pr *github.PullRequest) (string, error) {
	var forkURL string
	if pr.Base.Repo.GetPrivate() {
		forkURL = makeRepositoryURL(pr.Head.Repo.GetGitURL(), c.git.SSH, c.token)
	} else {
		forkURL = makeRepositoryURL(pr.Head.Repo.GetGitURL(), c.git.SSH, "")
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
			url: makeRepositoryURL(pr.Base.Repo.GetGitURL(), c.git.SSH, c.token),
			ref: pr.Base.GetRef(),
		},
	}

	return c.pullRequest(pr, model)
}

// PullRequestForUpdate Clone a pull request for an update (rebase).
func (c Clone) PullRequestForUpdate(pr *github.PullRequest) (string, error) {
	var unchangedURL string
	if pr.Base.Repo.GetPrivate() {
		unchangedURL = makeRepositoryURL(pr.Base.Repo.GetGitURL(), c.git.SSH, c.token)
	} else {
		unchangedURL = makeRepositoryURL(pr.Base.Repo.GetGitURL(), c.git.SSH, "")
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
			url: makeRepositoryURL(pr.Head.Repo.GetGitURL(), c.git.SSH, c.token),
			ref: pr.Head.GetRef(),
		},
	}

	return c.pullRequest(pr, model)
}

func (c Clone) pullRequest(pr *github.PullRequest, prModel prModel) (string, error) {
	if isOnMainRepository(pr) {
		log.Print("It's not a fork, it's a branch on the main repository.")

		remoteName := RemoteOrigin

		output, err := c.fromMainRepository(prModel.changed)
		if err != nil {
			log.Print(output)
			return "", err
		}

		return remoteName, nil
	}

	remoteName := RemoteUpstream
	output, err := c.fromFork(prModel.changed, prModel.unchanged, remoteName)
	if err != nil {
		log.Print(output)
		return "", err
	}

	return remoteName, nil
}

func (c Clone) fromMainRepository(remoteModel remoteModel) (string, error) {
	output, err := git.Clone(clone.Repository(remoteModel.url), clone.Directory("."), git.Debugger(c.debug))
	if err != nil {
		return output, err
	}

	output, err = configureGit(c.git)
	if err != nil {
		return output, err
	}

	output, err = git.Checkout(checkout.Branch(remoteModel.ref), git.Debugger(c.debug))
	if err != nil {
		return output, fmt.Errorf("failed to checkout branch %s: %w", remoteModel.ref, err)
	}

	return "", nil
}

func (c Clone) fromFork(origin, upstream remoteModel, remoteName string) (string, error) {
	output, err := git.Clone(
		clone.Repository(origin.url),
		clone.Branch(origin.ref),
		clone.Directory("."),
		git.Debugger(c.debug))
	if err != nil {
		return output, err
	}

	output, err = configureGit(c.git)
	if err != nil {
		return output, err
	}

	output, err = git.Remote(remote.Add(remoteName, upstream.url), git.Debugger(c.debug))
	if err != nil {
		return output, fmt.Errorf("failed to add remote: %w", err)
	}

	output, err = git.Fetch(fetch.NoTags, fetch.Remote(remoteName), fetch.RefSpec(upstream.ref), git.Debugger(c.debug))
	if err != nil {
		return output, fmt.Errorf("failed to fetch %s/%s : %w", remoteName, upstream.ref, err)
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

func configureGit(gitConfig conf.Git) (string, error) {
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

func configureGitUserInfo(gitUserName, gitUserEmail string) (string, error) {
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

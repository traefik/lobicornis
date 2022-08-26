package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v47/github"
	"github.com/ldez/go-git-cmd-wrapper/v2/checkout"
	"github.com/ldez/go-git-cmd-wrapper/v2/clone"
	"github.com/ldez/go-git-cmd-wrapper/v2/config"
	"github.com/ldez/go-git-cmd-wrapper/v2/fetch"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/remote"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/traefik/lobicornis/v3/pkg/conf"
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

func newClone(gitConfig conf.Git, token string) Clone {
	return Clone{
		git:   gitConfig,
		token: token,
		debug: log.Logger.GetLevel() == zerolog.DebugLevel,
	}
}

// PullRequestForMerge Clone a pull request for a merge.
func (c Clone) PullRequestForMerge(ctx context.Context, pr *github.PullRequest) (string, error) {
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

	return c.pullRequest(ctx, pr, model)
}

// PullRequestForUpdate Clone a pull request for an update (rebase).
func (c Clone) PullRequestForUpdate(ctx context.Context, pr *github.PullRequest) (string, error) {
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

	return c.pullRequest(ctx, pr, model)
}

func (c Clone) pullRequest(ctx context.Context, pr *github.PullRequest, prModel prModel) (string, error) {
	logger := log.Ctx(ctx)

	if isOnMainRepository(pr) {
		logger.Info().Msg("It's not a fork, it's a branch on the main repository.")

		remoteName := RemoteOrigin

		output, err := c.fromMainRepository(ctx, prModel.changed)
		if err != nil {
			logger.Error().Err(err).Msg(output)
			return "", err
		}

		return remoteName, nil
	}

	remoteName := RemoteUpstream
	output, err := c.fromFork(ctx, prModel.changed, prModel.unchanged, remoteName)
	if err != nil {
		logger.Error().Err(err).Msg(output)
		return "", err
	}

	return remoteName, nil
}

func (c Clone) fromMainRepository(ctx context.Context, remoteModel remoteModel) (string, error) {
	output, err := git.CloneWithContext(ctx, clone.Repository(remoteModel.url), clone.Directory("."), git.Debugger(c.debug))
	if err != nil {
		return output, err
	}

	output, err = configureGit(ctx, c.git)
	if err != nil {
		return output, err
	}

	output, err = git.CheckoutWithContext(ctx, checkout.Branch(remoteModel.ref), git.Debugger(c.debug))
	if err != nil {
		return output, fmt.Errorf("failed to checkout branch %s: %w", remoteModel.ref, err)
	}

	return "", nil
}

func (c Clone) fromFork(ctx context.Context, origin, upstream remoteModel, remoteName string) (string, error) {
	output, err := git.CloneWithContext(ctx,
		clone.Repository(origin.url),
		clone.Branch(origin.ref),
		clone.Directory("."),
		git.Debugger(c.debug))
	if err != nil {
		return output, err
	}

	output, err = configureGit(ctx, c.git)
	if err != nil {
		return output, err
	}

	output, err = git.RemoteWithContext(ctx, remote.Add(remoteName, upstream.url), git.Debugger(c.debug))
	if err != nil {
		return output, fmt.Errorf("failed to add remote: %w", err)
	}

	output, err = git.FetchWithContext(ctx, fetch.NoTags, fetch.Remote(remoteName), fetch.RefSpec(upstream.ref), git.Debugger(c.debug))
	if err != nil {
		return output, fmt.Errorf("failed to fetch %s/%s : %w", remoteName, upstream.ref, err)
	}

	return "", nil
}

func makeRepositoryURL(url string, ssh bool, token string) string {
	if ssh {
		return strings.ReplaceAll(url, "git://github.com/", "git@github.com:")
	}

	prefix := "https://"
	if len(token) > 0 {
		prefix += token + "@"
	}

	return strings.ReplaceAll(url, "git://", prefix)
}

func configureGit(ctx context.Context, gitConfig conf.Git) (string, error) {
	output, err := git.ConfigWithContext(ctx, config.Entry("rebase.autoSquash", "true"))
	if err != nil {
		return output, err
	}

	output, err = git.ConfigWithContext(ctx, config.Entry("push.default", "current"))
	if err != nil {
		return output, err
	}

	return configureGitUserInfo(ctx, gitConfig.UserName, gitConfig.Email)
}

func configureGitUserInfo(ctx context.Context, gitUserName, gitUserEmail string) (string, error) {
	if len(gitUserEmail) != 0 {
		output, err := git.ConfigWithContext(ctx, config.Entry("user.email", gitUserEmail))
		if err != nil {
			return output, err
		}
	}

	if len(gitUserName) != 0 {
		output, err := git.ConfigWithContext(ctx, config.Entry("user.name", gitUserName))
		if err != nil {
			return output, err
		}
	}

	return "", nil
}

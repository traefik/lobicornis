package updater

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/containous/lobicornis/gh"
	"github.com/google/go-github/github"
	"github.com/ldez/go-git-cmd-wrapper/config"
	"github.com/ldez/go-git-cmd-wrapper/git"
)

const (
	remoteUpstream = "upstream"
	remoteOrigin   = "origin"
	// ActionMerge name of the "merge" action
	ActionMerge = "merge"
	// ActionRebase name of the "rebase" action
	ActionRebase = "rebase"
)

// Process clone a PR and update if needed.
func Process(ghub *gh.GHub, pr *github.PullRequest, ssh bool, gitHubToken string, gitUserName string, gitUserEmail string, dryRun bool, debug bool) error {
	log.Println("Base branch: ", pr.Base.GetRef(), "- Fork branch: ", pr.Head.GetRef())

	forkURL := makeRepositoryURL(pr.Head.Repo.GetGitURL(), ssh, gitHubToken)
	baseURL := makeRepositoryURL(pr.Base.Repo.GetGitURL(), ssh, "")

	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	defer func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			log.Println(errRemove)
		}
	}()
	if err != nil {
		return err
	}

	err = os.Chdir(dir)
	if err != nil {
		return err
	}

	tempDir, _ := os.Getwd()
	log.Println(tempDir)

	mainRemote, err := clonePR(pr, forkURL, baseURL, debug)
	if err != nil {
		return err
	}

	err = configureGitUserInfo(gitUserName, gitUserEmail)
	if err != nil {
		return err
	}

	output, err := updatePR(ghub, pr, mainRemote, dryRun, debug)
	log.Println(output)

	return err
}

func configureGitUserInfo(gitUserName string, gitUserEmail string) error {
	if len(gitUserEmail) != 0 {
		output, err := git.Config(config.Entry("user.email", gitUserEmail))
		if err != nil {
			log.Println(output)
			return err
		}
	}

	if len(gitUserName) != 0 {
		output, err := git.Config(config.Entry("user.name", gitUserName))
		if err != nil {
			log.Println(output)
			return err
		}
	}

	return nil
}

func makeRepositoryURL(cloneURL string, ssh bool, token string) string {
	if ssh {
		return strings.Replace(cloneURL, "git://github.com/", "git@github.com:", -1)
	}

	prefix := "https://"
	if len(token) > 0 {
		prefix += token + "@"
	}
	return strings.Replace(cloneURL, "git://", prefix, -1)
}

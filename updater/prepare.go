package updater

import (
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/containous/lobicornis/gh"
	"github.com/google/go-github/github"
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
func Process(ghub *gh.GHub, pr *github.PullRequest, ssh bool, gitHubToken string, dryRun bool, debug bool) error {
	log.Println("Base branch: ", pr.Base.GetRef(), "- Fork branch: ", pr.Head.GetRef())

	forkURL := makeRepositoryURL(pr.Head.Repo.GetGitURL(), ssh, gitHubToken)
	baseURL := makeRepositoryURL(pr.Base.Repo.GetGitURL(), ssh, "")

	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	defer os.RemoveAll(dir)
	if err != nil {
		return err
	}

	os.Chdir(dir)

	tempDir, _ := os.Getwd()
	log.Println(tempDir)

	mainRemote, err := clonePR(pr, forkURL, baseURL, debug)
	if err != nil {
		return err
	}

	output, err := updatePR(ghub, pr, mainRemote, dryRun, debug)
	log.Println(output)
	if err != nil {
		return err
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

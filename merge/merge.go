package merge

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/containous/lobicornis/clone"
	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/v27/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/merge"
	"github.com/ldez/go-git-cmd-wrapper/push"
)

// PullRequest Merge a Pull Request.
func PullRequest(ctx context.Context, client *github.Client, pr *github.PullRequest, mergeMethod string, gitConfig types.GitConfig, debug, dryRun bool) (types.Result, error) {

	if mergeMethod == gh.MergeMethodFastForward {
		return fastForward(pr, gitConfig, debug, dryRun)
	}

	return githubMerge(ctx, client, pr, mergeMethod, dryRun)
}

func githubMerge(ctx context.Context, client *github.Client, pr *github.PullRequest, mergeMethod string, dryRun bool) (types.Result, error) {
	if dryRun {
		return types.Result{Message: "Fake merge: dry run", Merged: true}, nil
	}

	options := &github.PullRequestOptions{
		MergeMethod: mergeMethod,
		CommitTitle: pr.GetTitle(),
	}

	var message string
	if mergeMethod == gh.MergeMethodSquash {
		message = strings.Join(getCoAuthors(pr), "\n")
	}

	result, _, err := client.PullRequests.Merge(ctx, pr.Base.Repo.Owner.GetLogin(), pr.Base.Repo.GetName(), pr.GetNumber(), message, options)
	if err != nil {
		log.Println(err)
		return types.Result{Message: err.Error(), Merged: false}, err
	}
	return types.Result{
		Message: result.GetMessage(),
		Merged:  result.GetMerged(),
	}, nil
}

func fastForward(pr *github.PullRequest, gitConfig types.GitConfig, debug, dryRun bool) (types.Result, error) {
	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	if err != nil {
		return types.Result{Message: err.Error(), Merged: false}, err
	}
	defer func() {
		errRemove := os.RemoveAll(dir)
		if errRemove != nil {
			log.Println(errRemove)
		}
	}()

	err = os.Chdir(dir)
	if err != nil {
		return types.Result{Message: err.Error(), Merged: false}, err
	}

	tempDir, _ := os.Getwd()
	log.Println(tempDir)

	output, err := clone.PullRequestForMerge(pr, gitConfig, debug)
	if err != nil {
		log.Println(output)
		return types.Result{Message: err.Error(), Merged: false}, err
	}

	remoteName := types.RemoteUpstream
	if gh.IsOnMainRepository(pr) {
		remoteName = types.RemoteOrigin
	}

	ref := fmt.Sprintf("%s/%s", remoteName, pr.Head.GetRef())

	output, err = git.Merge(merge.FfOnly, merge.Commits(ref), git.Debugger(debug))
	if err != nil {
		log.Println(output)
		return types.Result{Message: err.Error(), Merged: false}, err
	}

	output, err = git.Push(
		git.Cond(dryRun, push.DryRun),
		push.Remote(types.RemoteOrigin),
		push.RefSpec(pr.Base.GetRef()),
		git.Debugger(debug))
	if err != nil {
		log.Println(output)
		return types.Result{Message: err.Error(), Merged: false}, err
	}

	return types.Result{Merged: true, Message: "Merged"}, nil
}

// getCoAuthors Extracts co-author from PR description.
// Co-authored-by: login <email@email.com>
func getCoAuthors(pr *github.PullRequest) []string {
	exp := regexp.MustCompile(`^(?i)Co-authored-by:\s+(.+)\s+<(.+)>$`)

	var coAuthors []string
	scanner := bufio.NewScanner(bytes.NewBufferString(pr.GetBody()))
	for scanner.Scan() {
		line := scanner.Text()
		if exp.MatchString(line) {
			s := exp.FindStringSubmatch(line)
			coAuthors = append(coAuthors, fmt.Sprintf("Co-authored-by: %s <%s>", s[1], s[2]))
		}
	}

	return coAuthors
}

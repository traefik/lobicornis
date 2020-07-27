package repository

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/merge"
	"github.com/ldez/go-git-cmd-wrapper/push"
)

// Merge Methods.
const (
	MergeMethodSquash      = "squash"
	MergeMethodRebase      = "rebase"
	MergeMethodMerge       = "merge"
	MergeMethodFastForward = "ff"
)

// Remote name.
const (
	RemoteOrigin   = "origin"
	RemoteUpstream = "upstream"
)

// Result Merge result.
type Result struct {
	Message string
	Merged  bool
}

func (r Repository) getMergeMethod(pr *github.PullRequest) (string, error) {
	if r.markers.MergeMethodPrefix == "" {
		return r.config.GetMergeMethod(), nil
	}

	var labels []string
	for _, lbl := range pr.Labels {
		if strings.HasPrefix(lbl.GetName(), r.markers.MergeMethodPrefix) {
			labels = append(labels, lbl.GetName())
		}
	}

	if len(labels) == 0 {
		return r.config.GetMergeMethod(), nil
	}

	if len(labels) > 1 {
		return "", fmt.Errorf("too many custom merge method labels: %v", labels)
	}

	switch labels[0] {
	case r.markers.MergeMethodPrefix + MergeMethodSquash:
		return MergeMethodSquash, nil
	case r.markers.MergeMethodPrefix + MergeMethodMerge:
		return MergeMethodMerge, nil
	case r.markers.MergeMethodPrefix + MergeMethodRebase:
		return MergeMethodRebase, nil
	case r.markers.MergeMethodPrefix + MergeMethodFastForward:
		return MergeMethodFastForward, nil
	default:
		return r.config.GetMergeMethod(), nil
	}
}

func (r Repository) merge(ctx context.Context, pr *github.PullRequest, mergeMethod string) error {
	log.Printf("MERGE(%s)\n", mergeMethod)

	err := r.removeLabel(ctx, pr, r.markers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}

	if !r.dryRun {
		result, errMerge := r.mergePullRequest(ctx, pr, mergeMethod)
		if errMerge != nil {
			log.Println(errMerge)
		}

		log.Println(result.Message)

		if !result.Merged {
			r.callHuman(ctx, pr, fmt.Sprintf("Failed to merge PR: %s", result.Message))

			return errors.New("failed to merge PR")
		}

		labelsToRemove := []string{
			r.markers.NeedMerge,
			r.markers.LightReview,
			r.markers.MergeMethodPrefix + MergeMethodSquash,
			r.markers.MergeMethodPrefix + MergeMethodMerge,
			r.markers.MergeMethodPrefix + MergeMethodRebase,
			r.markers.MergeMethodPrefix + MergeMethodFastForward,
		}
		err = r.removeLabels(ctx, pr, labelsToRemove)
		if err != nil {
			log.Println(err)
		}
	}

	err = r.mjolnir.CloseRelatedIssues(ctx, pr)
	if err != nil {
		log.Println(err)
	}

	return nil
}

// mergePullRequest Merge a Pull Request.
func (r Repository) mergePullRequest(ctx context.Context, pr *github.PullRequest, mergeMethod string) (Result, error) {
	if mergeMethod == MergeMethodFastForward {
		return r.fastForward(pr)
	}

	return r.githubMerge(ctx, pr, mergeMethod)
}

func (r Repository) githubMerge(ctx context.Context, pr *github.PullRequest, mergeMethod string) (Result, error) {
	if r.dryRun {
		return Result{Message: "Fake merge: dry run", Merged: true}, nil
	}

	options := &github.PullRequestOptions{
		MergeMethod: mergeMethod,
		CommitTitle: pr.GetTitle(),
	}

	var message string
	if mergeMethod == MergeMethodSquash {
		message = strings.Join(getCoAuthors(pr), "\n")
		if message == "" {
			// force the description in the commit message to be empty.
			message = "\n"
		}
	}

	result, _, err := r.client.PullRequests.Merge(ctx, r.owner, r.name, pr.GetNumber(), message, options)
	if err != nil {
		log.Println(err)
		return Result{Message: err.Error(), Merged: false}, err
	}

	return Result{
		Message: result.GetMessage(),
		Merged:  result.GetMerged(),
	}, nil
}

func (r Repository) fastForward(pr *github.PullRequest) (Result, error) {
	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	defer func() {
		if errR := os.RemoveAll(dir); errR != nil {
			log.Println(errR)
		}
	}()

	err = os.Chdir(dir)
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	tempDir, _ := os.Getwd()
	log.Println(tempDir)

	output, err := r.clone.PullRequestForMerge(pr)
	if err != nil {
		log.Println(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	remoteName := RemoteUpstream
	if isOnMainRepository(pr) {
		remoteName = RemoteOrigin
	}

	ref := fmt.Sprintf("%s/%s", remoteName, pr.Head.GetRef())

	output, err = git.Merge(merge.FfOnly, merge.Commits(ref), git.Debugger(r.debug))
	if err != nil {
		log.Println(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	output, err = git.Push(
		git.Cond(r.dryRun, push.DryRun),
		push.Remote(RemoteOrigin),
		push.RefSpec(pr.Base.GetRef()),
		git.Debugger(r.debug))
	if err != nil {
		log.Println(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	return Result{Merged: true, Message: "Merged"}, nil
}

// getCoAuthors Extracts co-author from PR description.
//     Co-authored-by: login <email@email.com>
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

// isOnMainRepository checks if the branch of the Pull Request in on the main repository.
func isOnMainRepository(pr *github.PullRequest) bool {
	return pr.Base.Repo.GetGitURL() == pr.Head.Repo.GetGitURL()
}

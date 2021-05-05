package repository

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v32/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/merge"
	"github.com/ldez/go-git-cmd-wrapper/push"
	"github.com/rs/zerolog/log"
	"github.com/traefik/lobicornis/v2/pkg/conf"
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
	case r.markers.MergeMethodPrefix + conf.MergeMethodSquash:
		return conf.MergeMethodSquash, nil
	case r.markers.MergeMethodPrefix + conf.MergeMethodMerge:
		return conf.MergeMethodMerge, nil
	case r.markers.MergeMethodPrefix + conf.MergeMethodRebase:
		return conf.MergeMethodRebase, nil
	case r.markers.MergeMethodPrefix + conf.MergeMethodFastForward:
		return conf.MergeMethodFastForward, nil
	default:
		return r.config.GetMergeMethod(), nil
	}
}

func (r Repository) merge(ctx context.Context, pr *github.PullRequest, mergeMethod string) error {
	if !pr.GetMaintainerCanModify() && !isOnMainRepository(pr) && mergeMethod == conf.MergeMethodFastForward {
		// note: it's not possible to edit a PR from an organization.
		return fmt.Errorf("the use of the merge method [%s] is impossible when a branch from an organization "+
			"or if the contributor doesn't allow maintainer modification (GitHub option)", mergeMethod)
	}

	log.Ctx(ctx).Info().Msgf("MERGE(%s)\n", mergeMethod)

	err := r.removeLabel(ctx, pr, r.markers.MergeInProgress)
	ignoreError(err)

	if !r.dryRun {
		var result Result
		result, err = r.mergePullRequest(ctx, pr, mergeMethod)
		ignoreError(err)

		log.Ctx(ctx).Info().Msg(result.Message)

		if !result.Merged {
			return fmt.Errorf("failed to merge PR: %s", result.Message)
		}

		labelsToRemove := []string{
			r.markers.NeedMerge,
			r.markers.LightReview,
			r.markers.MergeMethodPrefix + conf.MergeMethodSquash,
			r.markers.MergeMethodPrefix + conf.MergeMethodMerge,
			r.markers.MergeMethodPrefix + conf.MergeMethodRebase,
			r.markers.MergeMethodPrefix + conf.MergeMethodFastForward,
		}
		err = r.removeLabels(ctx, pr, labelsToRemove)
		ignoreError(err)
	}

	err = r.mjolnir.CloseRelatedIssues(ctx, pr)
	ignoreError(err)

	return nil
}

// mergePullRequest Merge a Pull Request.
func (r Repository) mergePullRequest(ctx context.Context, pr *github.PullRequest, mergeMethod string) (Result, error) {
	if mergeMethod == conf.MergeMethodFastForward {
		return r.fastForward(ctx, pr)
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

	message := r.getCommitMessage(mergeMethod, pr)

	result, _, err := r.client.PullRequests.Merge(ctx, r.owner, r.name, pr.GetNumber(), message, options)
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	return Result{
		Message: result.GetMessage(),
		Merged:  result.GetMerged(),
	}, nil
}

func (r Repository) getCommitMessage(mergeMethod string, pr *github.PullRequest) string {
	if mergeMethod != conf.MergeMethodSquash {
		return ""
	}

	switch r.config.GetCommitMessage() {
	case "github":
		return ""
	case "description":
		return pr.GetBody()
	default:
		message := strings.Join(getCoAuthors(pr), "\n")
		if message == "" {
			// force the description in the commit message to be empty.
			message = "\n"
		}
		return message
	}
}

func (r Repository) fastForward(ctx context.Context, pr *github.PullRequest) (Result, error) {
	dir, err := ioutil.TempDir("", "myrmica-lobicornis")
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	defer func() { ignoreError(os.RemoveAll(dir)) }()

	err = os.Chdir(dir)
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	tempDir, _ := os.Getwd()

	logger := log.Ctx(ctx)
	logger.Info().Msg(tempDir)

	output, err := r.clone.PullRequestForMerge(pr)
	if err != nil {
		logger.Error().Err(err).Msg(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	remoteName := RemoteUpstream
	if isOnMainRepository(pr) {
		remoteName = RemoteOrigin
	}

	ref := fmt.Sprintf("%s/%s", remoteName, pr.Head.GetRef())

	output, err = git.Merge(merge.FfOnly, merge.Commits(ref), git.Debugger(r.debug))
	if err != nil {
		logger.Error().Err(err).Msg(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	output, err = git.Push(
		git.Cond(r.dryRun, push.DryRun),
		push.Remote(RemoteOrigin),
		push.RefSpec(pr.Base.GetRef()),
		git.Debugger(r.debug))
	if err != nil {
		logger.Error().Err(err).Msg(output)
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

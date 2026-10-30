package repository

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/google/go-github/v74/github"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/merge"
	"github.com/ldez/go-git-cmd-wrapper/v2/push"
	"github.com/rs/zerolog/log"
	"github.com/traefik/lobicornis/v3/pkg/conf"
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

func (r *Repository) getMergeMethod(pr *github.PullRequest) (string, error) {
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

func (r *Repository) merge(ctx context.Context, pr *github.PullRequest, mergeMethod string) error {
	if !pr.GetMaintainerCanModify() && !isOnMainRepository(pr) && mergeMethod == conf.MergeMethodFastForward {
		// note: it's not possible to edit a PR from an organization.
		return fmt.Errorf("the use of the merge method [%s] is impossible when a branch from an organization "+
			"or if the contributor doesn't allow maintainer modification (GitHub option)", mergeMethod)
	}

	// the squash commit author is the PR opener.
	if login := pr.GetUser().GetLogin(); isAgentLogin(login) {
		return fmt.Errorf("the use of the merge method [%s] is impossible when an AI agent opened the pull request: %s", mergeMethod, login)
	}

	if mergeMethod != conf.MergeMethodSquash {
		agent, err := r.findAgentInCommits(ctx, pr)
		if err != nil {
			return fmt.Errorf("unable to get commits: %w", err)
		}

		if agent != "" {
			return fmt.Errorf("the use of the merge method [%s] is impossible when an AI agent is the author, "+
				"the committer or a co-author of a commit: %s", mergeMethod, agent)
		}
	}

	log.Ctx(ctx).Info().Msgf("MERGE(%s)\n", mergeMethod)

	err := r.removeLabel(ctx, pr, r.markers.MergeInProgress)
	ignoreError(ctx, err)

	if !r.dryRun {
		var result Result
		result, err = r.mergePullRequest(ctx, pr, mergeMethod)
		ignoreError(ctx, err)

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
			r.markers.MergeNoRebase,
		}
		err = r.removeLabels(ctx, pr, labelsToRemove)
		ignoreError(ctx, err)
	}

	err = r.mjolnir.CloseRelatedIssues(ctx, pr)
	ignoreError(ctx, err)

	return nil
}

// mergePullRequest Merge a Pull Request.
func (r *Repository) mergePullRequest(ctx context.Context, pr *github.PullRequest, mergeMethod string) (Result, error) {
	if mergeMethod == conf.MergeMethodFastForward {
		return r.fastForward(ctx, pr)
	}

	return r.githubMerge(ctx, pr, mergeMethod)
}

func (r *Repository) githubMerge(ctx context.Context, pr *github.PullRequest, mergeMethod string) (Result, error) {
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

func (r *Repository) getCommitMessage(mergeMethod string, pr *github.PullRequest) string {
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

func (r *Repository) fastForward(ctx context.Context, pr *github.PullRequest) (Result, error) {
	dir, err := os.MkdirTemp("", "myrmica-lobicornis")
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	defer func() { ignoreError(ctx, os.RemoveAll(dir)) }()

	err = os.Chdir(dir)
	if err != nil {
		return Result{Message: err.Error(), Merged: false}, err
	}

	tempDir, _ := os.Getwd()

	logger := log.Ctx(ctx)
	logger.Info().Msg(tempDir)

	output, err := r.clone.PullRequestForMerge(ctx, pr)
	if err != nil {
		logger.Error().Err(err).Msg(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	remoteName := RemoteUpstream
	if isOnMainRepository(pr) {
		remoteName = RemoteOrigin
	}

	ref := fmt.Sprintf("%s/%s", remoteName, pr.Head.GetRef())

	output, err = git.MergeWithContext(ctx, merge.FfOnly, merge.Commits(ref), git.Debugger(r.debug))
	if err != nil {
		logger.Error().Err(err).Msg(output)
		return Result{Message: err.Error(), Merged: false}, err
	}

	output, err = git.PushWithContext(ctx,
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

// findAgentInCommits returns the first commit of a PR that has an AI agent as author, committer or co-author.
func (r *Repository) findAgentInCommits(ctx context.Context, pr *github.PullRequest) (string, error) {
	opt := &github.ListOptions{
		PerPage: 100,
	}

	for {
		commits, resp, err := r.client.PullRequests.ListCommits(ctx, r.owner, r.name, pr.GetNumber(), opt)
		if err != nil {
			return "", err
		}

		for _, commit := range commits {
			if agent := getCommitAgent(commit); agent != "" {
				return fmt.Sprintf("%.7s has the %s", commit.GetSHA(), agent), nil
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return "", nil
}

// getCommitAgent returns the AI agent that is the author, the committer or a co-author of a commit.
func getCommitAgent(commit *github.RepositoryCommit) string {
	author := commit.GetCommit().GetAuthor()
	if isAgentLogin(commit.GetAuthor().GetLogin()) || isAgentEmail(author.GetEmail()) {
		return fmt.Sprintf("author %s <%s>", author.GetName(), author.GetEmail())
	}

	committer := commit.GetCommit().GetCommitter()
	if isAgentLogin(commit.GetCommitter().GetLogin()) || isAgentEmail(committer.GetEmail()) {
		return fmt.Sprintf("committer %s <%s>", committer.GetName(), committer.GetEmail())
	}

	for _, ca := range parseCoAuthors(commit.GetCommit().GetMessage()) {
		if isAgentEmail(ca.email) {
			return fmt.Sprintf("co-author %s <%s>", ca.name, ca.email)
		}
	}

	return ""
}

// getCoAuthors Extracts co-author from PR description, without the AI agents.
//
//	Co-authored-by: login <email@email.com>
func getCoAuthors(pr *github.PullRequest) []string {
	var coAuthors []string
	for _, ca := range parseCoAuthors(pr.GetBody()) {
		if !isAgentEmail(ca.email) {
			coAuthors = append(coAuthors, fmt.Sprintf("Co-authored-by: %s <%s>", ca.name, ca.email))
		}
	}

	return coAuthors
}

type coAuthor struct {
	name  string
	email string
}

// parseCoAuthors Extracts the Co-authored-by lines from a text.
func parseCoAuthors(text string) []coAuthor {
	exp := regexp.MustCompile(`^(?i)Co-authored-by:\s+(.+)\s+<(.+)>$`)

	var coAuthors []coAuthor
	scanner := bufio.NewScanner(bytes.NewBufferString(text))
	for scanner.Scan() {
		line := scanner.Text()
		if exp.MatchString(line) {
			s := exp.FindStringSubmatch(line)
			coAuthors = append(coAuthors, coAuthor{name: s[1], email: s[2]})
		}
	}

	return coAuthors
}

// isAgentLogin checks if a GitHub account is an AI agent.
func isAgentLogin(login string) bool {
	logins := []string{
		"Copilot",
		"copilot-swe-agent[bot]",
		"claude",
		"claude[bot]",
		"anthropic-claude[bot]",
		"codex",
		"chatgpt-codex-connector[bot]",
		"openai-codex[bot]",
		"cursoragent",
		"cursor-agent",
		"cursor[bot]",
		"google-labs-jules[bot]",
		"gemini-code-assist[bot]",
		"devin-ai-integration[bot]",
		"amazon-q-developer[bot]",
		"openhands-agent",
		"windsurf-bot[bot]",
		"factory-droid[bot]",
		"kiro-agent",
		"junie-agent",
		"opencode-agent[bot]",
		"augmentcode[bot]",
		"mistral-vibe",
	}

	return slices.ContainsFunc(logins, func(l string) bool { return strings.EqualFold(l, login) })
}

// isAgentEmail checks if an email is the one of an AI agent, or the GitHub noreply email of an AI agent account.
func isAgentEmail(email string) bool {
	emails := []string{
		"noreply@anthropic.com",
		"noreply@openai.com",
		"codex@openai.com",
		"cursoragent@cursor.com",
	}

	if slices.ContainsFunc(emails, func(e string) bool { return strings.EqualFold(e, email) }) {
		return true
	}

	exp := regexp.MustCompile(`^(?i)(?:\d+\+)?(.+)@users\.noreply\.github\.com$`)

	s := exp.FindStringSubmatch(email)

	return s != nil && isAgentLogin(s[1])
}

// isOnMainRepository checks if the branch of the Pull Request in on the main repository.
func isOnMainRepository(pr *github.PullRequest) bool {
	return pr.Base.Repo.GetGitURL() == pr.Head.Repo.GetGitURL()
}

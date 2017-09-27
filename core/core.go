package core

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/search"
	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/github"
)

// Execute core process
func Execute(config types.Configuration) error {
	ctx := context.Background()
	client := gh.NewGitHubClient(ctx, config.GitHubToken)

	repoID := types.RepoID{
		Owner:          config.Owner,
		RepositoryName: config.RepositoryName,
	}

	gitConfig := types.GitConfig{
		GitHubToken: config.GitHubToken,
		SSH:         config.SSH,
		Email:       config.GitUserEmail,
		UserName:    config.GitUserName,
	}

	checks := types.Checks{
		NeedMilestone:     config.NeedMilestone,
		ForceNeedUpToDate: config.ForceNeedUpToDate,
		CheckNeedUpToDate: config.CheckNeedUpToDate,
		Review: types.Review{
			Min:      config.MinReview,
			MinLight: config.MinLightReview,
		},
	}

	extra := types.Extra{
		Debug:  config.Debug,
		DryRun: config.DryRun,
	}

	issue, err := searchIssuePR(ctx, client, repoID, config.LabelMarkers, checks.Review, extra.Debug)
	if err != nil {
		return err
	}

	if issue == nil {
		log.Println("Nothing to merge.")
	} else {
		err = process(ctx, client, issue, repoID, config.LabelMarkers, gitConfig, checks, config.DefaultMergeMethod, extra)
		if err != nil {
			return err
		}
	}
	return nil
}

func searchIssuePR(ctx context.Context, client *github.Client, repoID types.RepoID, markers *types.LabelMarkers, review types.Review, debug bool) (*github.Issue, error) {

	issuesMIP, err := search.FindOpenPR(ctx, client, repoID.Owner, repoID.RepositoryName, debug,
		search.WithLabels(markers.NeedMerge, markers.MergeInProgress),
		search.WithExcludedLabels(markers.NeedHumanMerge, markers.NoMerge),
		search.Cond(review.Min > 0, search.WithReviewApproved))
	if err != nil {
		return nil, err
	}
	if len(issuesMIP) > 1 {
		return nil, fmt.Errorf("illegal state: multiple PR with the label: %s", markers.MergeInProgress)
	}

	var issue *github.Issue

	if len(issuesMIP) == 1 {
		issue = &issuesMIP[0]
		log.Printf("Find PR #%d, updated at %v", issue.GetNumber(), issue.GetUpdatedAt())
	} else {
		issues, err := search.FindOpenPR(ctx, client, repoID.Owner, repoID.RepositoryName, debug,
			search.WithLabels(markers.NeedMerge),
			search.WithExcludedLabels(markers.NeedHumanMerge, markers.NoMerge),
			search.Cond(review.Min > 0, search.WithReviewApproved))
		if err != nil {
			return nil, err
		}

		if len(issues) != 0 {
			for _, issue := range issues {
				log.Printf("Find PR #%d, updated at %v", issue.GetNumber(), issue.GetUpdatedAt())
			}
			issue = &issues[0]
		}
	}

	return issue, nil
}

func process(ctx context.Context, client *github.Client, issuePR *github.Issue,
	repoID types.RepoID, markers *types.LabelMarkers, gitConfig types.GitConfig,
	checks types.Checks, defaultMergeMethod string, extra types.Extra) error {

	pr, _, err := client.PullRequests.Get(ctx, repoID.Owner, repoID.RepositoryName, issuePR.GetNumber())
	if err != nil {
		return err
	}

	ghub := gh.NewGHub(ctx, client, extra.DryRun, extra.Debug)

	prNumber := pr.GetNumber()

	if checks.NeedMilestone && pr.Milestone == nil {
		log.Printf("PR #%d: Must have a milestone.", prNumber)

		errLabel := ghub.AddLabels(issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		return nil
	}

	err = ghub.HasReviewsApprove(pr, getMinReview(issuePR, checks.Review, markers))
	if err != nil {
		log.Printf("PR #%d: Needs more reviews: %v", prNumber, err)

		errLabel := ghub.AddLabels(issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		return nil
	}

	status, err := ghub.GetStatus(pr)
	if err != nil {
		log.Printf("PR #%d: Checks status: %v", prNumber, err)

		errLabel := ghub.AddLabels(issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		return nil
	}

	if status == gh.Pending {
		// skip
		log.Println("State: pending. Waiting for the CI.")
		return nil
	}

	if pr.GetMerged() {
		labelsToRemove := []string{
			markers.MergeInProgress,
			markers.NeedMerge,
			markers.LightReview,
			markers.MergeMethodPrefix + gh.MergeMethodSquash,
			markers.MergeMethodPrefix + gh.MergeMethodMerge,
			markers.MergeMethodPrefix + gh.MergeMethodRebase,
			markers.MergeMethodPrefix + gh.MergeMethodFastForward,
		}
		errLabel := ghub.RemoveLabels(issuePR, repoID, labelsToRemove)
		if errLabel != nil {
			log.Println(errLabel)
		}

		log.Printf("the PR #%d is already merged", prNumber)
		return nil
	}

	if !pr.GetMergeable() {
		errLabel := ghub.AddLabels(issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		log.Printf("PR #%d: Conflicts must be resolve in the PR.", prNumber)
		return nil
	}

	// Get status checks
	var needUpdate bool
	if checks.CheckNeedUpToDate {
		rcs, _, errCheck := client.Repositories.GetRequiredStatusChecks(ctx, repoID.Owner, repoID.RepositoryName, pr.Base.GetRef())
		if errCheck != nil {
			return fmt.Errorf("unable to get status checks: %v", errCheck)
		}
		needUpdate = rcs.Strict
	} else if checks.ForceNeedUpToDate {
		needUpdate = true
	}

	mergeMethod, err := getMergeMethod(issuePR, markers, defaultMergeMethod)
	if err != nil {
		return err
	}

	upToDateBranch, err := ghub.IsUpToDateBranch(pr)
	if err != nil {
		return err
	}

	if !upToDateBranch && mergeMethod == gh.MergeMethodFastForward {
		return fmt.Errorf("merge method [%s] is impossible when a branch is not up-to-date", mergeMethod)
	}

	// Need to be up to date?
	if needUpdate {

		if !pr.GetMaintainerCanModify() && !gh.IsOnMainRepository(pr) {
			repo, _, err := client.Repositories.Get(ctx, repoID.Owner, repoID.RepositoryName)
			if err != nil {
				return err
			}

			if !repo.GetFork() {
				errLabel := ghub.AddLabels(issuePR, repoID, markers.NeedHumanMerge)
				if errLabel != nil {
					log.Println(errLabel)
				}
				errLabel = ghub.RemoveLabel(issuePR, repoID, markers.MergeInProgress)
				if errLabel != nil {
					log.Println(errLabel)
				}
				return fmt.Errorf("PR #%d: the contributor doesn't allow maintainer modification (GitHub option)", prNumber)
			}
		}

		if upToDateBranch {
			err := mergePR(ctx, client, ghub, issuePR, pr, repoID, markers, gitConfig, mergeMethod, extra)
			if err != nil {
				return err
			}
		} else {
			err := updatePR(ghub, issuePR, pr, repoID, markers, gitConfig, extra)
			if err != nil {
				return err
			}
		}
	} else {
		err := mergePR(ctx, client, ghub, issuePR, pr, repoID, markers, gitConfig, mergeMethod, extra)
		if err != nil {
			return err
		}
	}

	return nil
}

func getMergeMethod(issuePR *github.Issue, markers *types.LabelMarkers, defaultMergeMethod string) (string, error) {

	if len(markers.MergeMethodPrefix) != 0 {
		var labels []string
		for _, lbl := range issuePR.Labels {
			if strings.HasPrefix(lbl.GetName(), markers.MergeMethodPrefix) {
				labels = append(labels, lbl.GetName())
			}
		}

		if len(labels) == 0 {
			return defaultMergeMethod, nil
		}

		if len(labels) > 1 {
			return "", fmt.Errorf("PR #%d: too many custom merge method labels: %v", issuePR.GetNumber(), labels)
		}

		switch labels[0] {
		case markers.MergeMethodPrefix + gh.MergeMethodSquash:
			return gh.MergeMethodSquash, nil
		case markers.MergeMethodPrefix + gh.MergeMethodMerge:
			return gh.MergeMethodMerge, nil
		case markers.MergeMethodPrefix + gh.MergeMethodRebase:
			return gh.MergeMethodRebase, nil
		case markers.MergeMethodPrefix + gh.MergeMethodFastForward:
			return gh.MergeMethodFastForward, nil
		}
	}

	return defaultMergeMethod, nil
}

// getMinReview Get minimal number of review for an issue.
func getMinReview(issue *github.Issue, review types.Review, markers *types.LabelMarkers) int {
	if review.MinLight != 0 && gh.HasLabel(issue, markers.LightReview) {
		return review.MinLight
	}
	return review.Min
}

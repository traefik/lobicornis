package core

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/search"
	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/github"
)

// Execute core process
func Execute(config types.Configuration) error {
	ctx := context.Background()
	client := gh.NewGitHubClient(ctx, config.GitHubToken, config.GitHubURL)

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

	issue, err := searchIssuePR(ctx, client, repoID, config.LabelMarkers, checks.Review, config.Retry, extra.Debug)
	if err != nil {
		return err
	}

	if issue == nil {
		if extra.Debug {
			log.Println("Nothing to merge.")
		}
	} else {
		err = process(ctx, client, issue, repoID, config.LabelMarkers, gitConfig, checks, config.Retry, config.DefaultMergeMethod, extra)
		if err != nil {
			return err
		}
	}
	return nil
}

func searchIssuePR(ctx context.Context, client *github.Client, repoID types.RepoID,
	markers *types.LabelMarkers, review types.Review, retry *types.Retry, debug bool) (*github.Issue, error) {

	// Find Merge In Progress
	issues, err := search.FindOpenPR(ctx, client, repoID.Owner, repoID.RepositoryName, debug,
		search.WithLabels(markers.NeedMerge, markers.MergeInProgress),
		search.WithExcludedLabels(markers.NeedHumanMerge, markers.NoMerge),
		search.Cond(review.Min > 0, search.WithReviewApproved))
	if err != nil {
		return nil, err
	}

	switch len(issues) {
	case 1, 2:
		if retry != nil && retry.Number > 0 {
			// find retry
			var issuesRetry []github.Issue
			for _, issue := range issues {
				if len(gh.FindLabelPrefix(&issue, markers.MergeRetryPrefix)) > 0 {
					issuesRetry = append(issuesRetry, issue)
				}
			}

			if len(issuesRetry) > 0 {
				for _, issue := range issuesRetry {
					if time.Since(issue.GetUpdatedAt()) > time.Duration(retry.Interval) {
						log.Printf("Find PR #%d, updated at %v", issue.GetNumber(), issue.GetUpdatedAt())
						return &issue, nil
					}
				}
				return nil, nil
			}
		}
	case 0:
		// Find Need Merge
		issues, err = search.FindOpenPR(ctx, client, repoID.Owner, repoID.RepositoryName, debug,
			search.WithLabels(markers.NeedMerge),
			search.WithExcludedLabels(markers.NeedHumanMerge, markers.NoMerge),
			search.Cond(review.Min > 0, search.WithReviewApproved))
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("illegal state: multiple PR with the label: %s", markers.MergeInProgress)
	}

	if len(issues) != 0 {
		for _, issue := range issues {
			log.Printf("Find PR #%d, updated at %v", issue.GetNumber(), issue.GetUpdatedAt())
		}
		return &issues[0], nil
	}

	return nil, nil
}

// TODO simplify this function
// nolint: gocyclo
func process(ctx context.Context, client *github.Client, issuePR *github.Issue,
	repoID types.RepoID, markers *types.LabelMarkers, gitConfig types.GitConfig,
	checks types.Checks, retry *types.Retry, defaultMergeMethod string, extra types.Extra) error {

	pr, _, err := client.PullRequests.Get(ctx, repoID.Owner, repoID.RepositoryName, issuePR.GetNumber())
	if err != nil {
		return err
	}

	ghub := gh.NewGHub(client, extra.DryRun, extra.Debug)

	prNumber := pr.GetNumber()

	if checks.NeedMilestone && pr.Milestone == nil {
		log.Printf("PR #%d: Must have a milestone.", prNumber)

		errLabel := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		return nil
	}

	err = ghub.HasReviewsApprove(ctx, pr, getMinReview(issuePR, checks.Review, markers))
	if err != nil {
		log.Printf("PR #%d: Needs more reviews: %v", prNumber, err)

		errLabel := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		return nil
	}

	status, err := ghub.GetStatus(ctx, pr)
	if err != nil {
		log.Printf("PR #%d: Checks status: %v", prNumber, err)

		rt := retry != nil && retry.OnStatuses
		var rtNumber int
		if retry != nil {
			rtNumber = retry.Number
		}
		manageRetryLabel(ctx, ghub, repoID, issuePR, rt, rtNumber, markers)

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
		errLabel := ghub.RemoveLabels(ctx, issuePR, repoID, labelsToRemove)
		if errLabel != nil {
			log.Println(errLabel)
		}

		log.Printf("the PR #%d is already merged", prNumber)
		return nil
	}

	if !pr.GetMergeable() {
		log.Printf("PR #%d: Conflicts must be resolve in the PR.", prNumber)

		rt := retry != nil && retry.OnMergeable
		var rtNumber int
		if retry != nil {
			rtNumber = retry.Number
		}
		manageRetryLabel(ctx, ghub, repoID, issuePR, rt, rtNumber, markers)

		return nil
	}

	rtry := retry != nil && (retry.OnMergeable || retry.OnStatuses)
	cleanRetryLabel(ctx, ghub, repoID, issuePR, rtry, markers)

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

	upToDateBranch, err := ghub.IsUpToDateBranch(ctx, pr)
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
				errLabel := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
				if errLabel != nil {
					log.Println(errLabel)
				}
				errLabel = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
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
			err := updatePR(ctx, ghub, issuePR, pr, repoID, markers, gitConfig, extra)
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

func extractRetryNumber(label, prefix string) int {
	raw := strings.TrimPrefix(label, prefix)

	number, err := strconv.Atoi(raw)
	if err != nil {
		// TODO manage error
		log.Println(err)
		return 0
	}
	return number
}

func cleanRetryLabel(ctx context.Context, ghub *gh.GHub, repoID types.RepoID, issuePR *github.Issue, retry bool, markers *types.LabelMarkers) {
	if retry {
		currentRetryLabel := gh.FindLabelPrefix(issuePR, markers.MergeRetryPrefix)
		if len(currentRetryLabel) > 0 {
			err := ghub.RemoveLabel(ctx, issuePR, repoID, currentRetryLabel)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func manageRetryLabel(ctx context.Context, ghub *gh.GHub, repoID types.RepoID, issuePR *github.Issue, retry bool, retryNumber int, markers *types.LabelMarkers) {
	if retry && retryNumber > 0 {
		currentRetryLabel := gh.FindLabelPrefix(issuePR, markers.MergeRetryPrefix)
		if len(currentRetryLabel) > 0 {
			err := ghub.RemoveLabel(ctx, issuePR, repoID, currentRetryLabel)
			if err != nil {
				log.Println(err)
			}

			number := extractRetryNumber(currentRetryLabel, markers.MergeRetryPrefix)

			if number >= retryNumber {
				// Need Human
				errLabel := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
				if errLabel != nil {
					log.Println(errLabel)
				}
				errLabel = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
				if errLabel != nil {
					log.Println(errLabel)
				}
			} else {
				// retry
				newRetryLabel := markers.MergeRetryPrefix + strconv.Itoa(number+1)
				errLabel := ghub.AddLabels(ctx, issuePR, repoID, newRetryLabel)
				if errLabel != nil {
					log.Println(errLabel)
				}
			}
		} else {
			// first retry
			newRetryLabel := markers.MergeRetryPrefix + strconv.Itoa(1)
			errLabel := ghub.AddLabels(ctx, issuePR, repoID, newRetryLabel)
			if errLabel != nil {
				log.Println(errLabel)
			}
			errLabel = ghub.AddLabels(ctx, issuePR, repoID, markers.MergeInProgress)
			if errLabel != nil {
				log.Println(errLabel)
			}
		}
	} else {
		// Need Human
		errLabel := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}
	}
}

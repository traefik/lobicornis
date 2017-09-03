package core

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/mjolnir"
	"github.com/containous/lobicornis/search"
	"github.com/containous/lobicornis/updater"
	"github.com/google/go-github/github"
)

// Execute core process
func Execute(config Configuration) error {
	ctx := context.Background()
	client := gh.NewGitHubClient(ctx, config.GitHubToken)

	issue, err := searchIssuePR(ctx, client, config)
	if err != nil {
		return err
	}

	if issue == nil {
		log.Println("Nothing to merge.")
	} else {
		err = process(ctx, client, config, issue)
		if err != nil {
			return err
		}
	}
	return nil
}

func searchIssuePR(ctx context.Context, client *github.Client, config Configuration) (*github.Issue, error) {

	issuesMIP, err := search.FindOpenPR(ctx, client, config.Owner, config.RepositoryName, config.Debug,
		search.WithLabels(config.LabelMarkers.NeedMerge, config.LabelMarkers.MergeInProgress),
		search.WithExcludedLabels(config.LabelMarkers.NeedHumanMerge),
		search.Cond(config.MinReview > 0, search.WithReviewApproved))
	if err != nil {
		return nil, err
	}
	if len(issuesMIP) > 1 {
		return nil, fmt.Errorf("illegal state: multiple PR with the label: %s", config.LabelMarkers.MergeInProgress)
	}

	var issue *github.Issue

	if len(issuesMIP) == 1 {
		issue = &issuesMIP[0]
		log.Printf("Find PR #%d, updated at %v", issue.GetNumber(), issue.GetUpdatedAt())
	} else {
		issues, err := search.FindOpenPR(ctx, client, config.Owner, config.RepositoryName, config.Debug,
			search.WithLabels(config.LabelMarkers.NeedMerge),
			search.WithExcludedLabels(config.LabelMarkers.NeedHumanMerge),
			search.Cond(config.MinReview > 0, search.WithReviewApproved))
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

func process(ctx context.Context, client *github.Client, config Configuration, issuePR *github.Issue) error {

	pr, _, err := client.PullRequests.Get(ctx, config.Owner, config.RepositoryName, issuePR.GetNumber())
	if err != nil {
		return err
	}

	ghub := gh.NewGHub(ctx, client, config.DryRun, config.Debug)

	prNumber := pr.GetNumber()

	err = ghub.HasReviewsApprove(pr, getMinReview(config, issuePR))
	if err != nil {
		log.Printf("PR #%d: Needs more reviews: %v", prNumber, err)

		errLabel := ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		return nil
	}

	status, err := ghub.GetStatus(pr)
	if err != nil {
		log.Printf("PR #%d: Checks status: %v", prNumber, err)

		errLabel := ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
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
		errLabel := ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}

		log.Printf("the PR #%d is already merged", prNumber)
		return nil
	}

	if !pr.GetMergeable() {
		errLabel := ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
		if errLabel != nil {
			log.Println(errLabel)
		}
		errLabel = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if errLabel != nil {
			log.Println(errLabel)
		}

		log.Printf("PR #%d: Conflicts must be resolve in the PR.", prNumber)
		return nil
	}

	// Get status checks
	var needUpdate bool
	if config.CheckNeedUpToDate {
		rcs, _, err := client.Repositories.GetRequiredStatusChecks(ctx, config.Owner, config.RepositoryName, pr.Base.GetRef())
		if err != nil {
			return fmt.Errorf("unable to get status checks: %v", err)
		}
		needUpdate = rcs.Strict
	} else if config.ForceNeedUpToDate {
		needUpdate = true
	}

	// Need to be up to date?
	if needUpdate {

		if !pr.GetMaintainerCanModify() && !isOnMainRepository(pr) {

			repo, _, err := client.Repositories.Get(ctx, config.Owner, config.RepositoryName)
			if err != nil {
				return err
			}

			if !repo.GetFork() {
				errLabel := ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
				if errLabel != nil {
					log.Println(errLabel)
				}
				errLabel = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
				if errLabel != nil {
					log.Println(errLabel)
				}

				return fmt.Errorf("PR #%d: the contributor doesn't allow maintainer modification (GitHub option)", prNumber)
			}
		}

		ok, err := ghub.IsUpToDateBranch(pr)
		if err != nil {
			return err
		}
		if ok {
			err := mergePR(ctx, client, ghub, config, issuePR, pr)
			if err != nil {
				return err
			}
		} else {
			err := updatePR(ghub, config, issuePR, pr)
			if err != nil {
				return err
			}
		}
	} else {
		err := mergePR(ctx, client, ghub, config, issuePR, pr)
		if err != nil {
			return err
		}
	}

	return nil
}

func updatePR(ghub *gh.GHub, config Configuration, issuePR *github.Issue, pr *github.PullRequest) error {
	log.Printf("PR #%d: UPDATE", issuePR.GetNumber())

	err := ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}

	err = updater.Process(ghub, pr, config.SSH, config.GitHubToken, config.GitUserName, config.GitUserEmail, config.DryRun, config.Debug)
	if err != nil {
		err = ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}
		return err
	}
	return nil
}

func mergePR(ctx context.Context, client *github.Client, ghub *gh.GHub, config Configuration, issuePR *github.Issue, pr *github.PullRequest) error {

	mergeMethod, err := getMergeMethod(issuePR, config.LabelMarkers, config.DefaultMergeMethod)
	if err != nil {
		return err
	}

	prNumber := issuePR.GetNumber()

	log.Printf("PR #%d: MERGE(%s)\n", prNumber, mergeMethod)

	err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}

	if !config.DryRun {
		mergeOptions := &github.PullRequestOptions{
			MergeMethod: mergeMethod,
			CommitTitle: pr.GetTitle(),
		}
		result, _, err := client.PullRequests.Merge(ctx, config.Owner, config.RepositoryName, prNumber, "", mergeOptions)
		if err != nil {
			log.Println(err)
		}

		log.Println(result.GetMessage())

		if !result.GetMerged() {
			err = ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
			if err != nil {
				log.Println(err)
			}
			err := ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
			if err != nil {
				log.Println(err)
			}
			return fmt.Errorf("failed to merge PR #%d", prNumber)
		}
	}

	err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedMerge)
	if err != nil {
		log.Println(err)
	}
	err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeMethodPrefix+gh.MergeMethodSquash)
	if err != nil {
		log.Println(err)
	}
	err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeMethodPrefix+gh.MergeMethodMerge)
	if err != nil {
		log.Println(err)
	}
	err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeMethodPrefix+gh.MergeMethodRebase)
	if err != nil {
		log.Println(err)
	}

	err = mjolnir.CloseRelatedIssues(ctx, client, config.Owner, config.RepositoryName, pr, config.DryRun)
	if err != nil {
		log.Println(err)
	}

	return nil
}

func getMergeMethod(issuePR *github.Issue, markers *LabelMarkers, defaultMergeMethod string) (string, error) {

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
		}
	}

	return defaultMergeMethod, nil
}

func isOnMainRepository(pr *github.PullRequest) bool {
	return pr.Base.Repo.GetGitURL() == pr.Head.Repo.GetGitURL()
}

// getMinReview Get minimal number of review for an issue.
func getMinReview(config Configuration, issue *github.Issue) int {
	if config.MinLightReview != 0 && gh.HasLabel(issue, config.LabelMarkers.LightReview) {
		return config.MinLightReview
	}
	return config.MinReview
}

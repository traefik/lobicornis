package core

import (
	"context"
	"fmt"
	"log"

	"github.com/containous/brahma/gh"
	"github.com/containous/brahma/mjolnir"
	"github.com/containous/brahma/search"
	"github.com/containous/brahma/updater"
	"github.com/google/go-github/github"
)

// Execute core process
func Execute(config Configuration) {
	ctx := context.Background()
	client := gh.NewGitHubClient(ctx, config.GitHubToken)

	issue, err := searchIssuePR(ctx, client, config)
	if err != nil {
		log.Fatal(err)
	}

	if issue == nil {
		log.Println("Nothing to merge.")
	} else {
		err = process(ctx, client, config, issue)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func searchIssuePR(ctx context.Context, client *github.Client, config Configuration) (*github.Issue, error) {

	issuesMIP, err := search.FindOpenPR(ctx, client, config.Owner, config.RepositoryName,
		search.WithLabels(config.LabelMarkers.NeedMerge, config.LabelMarkers.MergeInProgress),
		search.WithExcludedLabels(config.LabelMarkers.NeedHumanMerge),
		search.Cond(config.MinReview > 0, search.WithReviewApproved))
	if err != nil {
		return nil, err
	}
	if len(issuesMIP) > 1 {
		return nil, fmt.Errorf("Illegal state: multiple PR with the label: %s", config.LabelMarkers.MergeInProgress)
	}

	var issue *github.Issue

	if len(issuesMIP) == 1 {
		issue = &issuesMIP[0]
		log.Printf("Find PR #%d, updated at %v", issue.GetNumber(), issue.GetUpdatedAt())
	} else {
		issues, err := search.FindOpenPR(ctx, client, config.Owner, config.RepositoryName,
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

	ghub := gh.NewGHub(ctx, client, config.DryRun)

	prNumber := pr.GetNumber()

	err = ghub.HasReviewsApprove(pr, config.MinReview)
	if err != nil {
		log.Printf("PR #%d: needs more reviews: %v", prNumber, err)

		err = ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		return nil
	}

	status, err := ghub.GetStatus(pr)
	if err != nil {
		log.Printf("checks status: %v", err)

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
		err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedMerge)
		if err != nil {
			log.Println(err)
		}

		log.Printf("the PR #%d is already merged", prNumber)
		return nil
	}

	if !pr.GetMergeable() {
		err = ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		log.Printf("conflicts must be resolve in the PR #%d", prNumber)
		return nil
	}

	// rebase
	ok, err := ghub.IsUpdatedBranch(pr)
	if err != nil {
		return err
	}
	if ok {
		mergeMethod := getMergeMethod(issuePR, config)

		fmt.Printf("MERGE(%s): PR #%d\n", mergeMethod, prNumber)

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

		err = mjolnir.CloseRelatedIssues(ctx, client, config.Owner, config.RepositoryName, pr, config.DryRun)
		if err != nil {
			log.Println(err)
		}

	} else {
		fmt.Printf("UPDATE: PR #%d\n", prNumber)

		err := ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		err = updater.Process(ghub, pr, config.SSH, config.GitHubToken, config.DryRun, config.Debug)
		if err != nil {
			err = ghub.AddLabels(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.NeedHumanMerge)
			if err != nil {
				log.Println(err)
			}
			// if
			err = ghub.RemoveLabel(issuePR, config.Owner, config.RepositoryName, config.LabelMarkers.MergeInProgress)
			if err != nil {
				log.Println(err)
			}
			return err
		}
	}

	return nil
}

func getMergeMethod(issue *github.Issue, config Configuration) string {
	for _, lbl := range issue.Labels {
		if lbl.GetName() == config.MergeMethodPrefix+gh.MergeMethodSquash {
			return gh.MergeMethodSquash
		}
		if lbl.GetName() == config.MergeMethodPrefix+gh.MergeMethodMerge {
			return gh.MergeMethodMerge
		}

		if lbl.GetName() == config.MergeMethodPrefix+gh.MergeMethodRebase {
			return gh.MergeMethodRebase
		}
	}
	return config.DefaultMergeMethod
}

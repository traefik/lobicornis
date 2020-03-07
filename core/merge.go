package core

import (
	"context"
	"fmt"
	"log"

	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/merge"
	"github.com/containous/lobicornis/mjolnir"
	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/v29/github"
)

func mergePR(ctx context.Context, client *github.Client, ghub *gh.GHub, issuePR *github.Issue, pr *github.PullRequest,
	repoID types.RepoID, markers *types.LabelMarkers, gitConfig types.GitConfig, mergeMethod string, extra types.Extra) error {
	prNumber := issuePR.GetNumber()

	log.Printf("PR #%d: MERGE(%s)\n", prNumber, mergeMethod)

	err := ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
	if err != nil {
		log.Println(err)
	}

	if !extra.DryRun {
		result, errMerge := merge.PullRequest(ctx, client, pr, mergeMethod, gitConfig, extra.Debug, extra.DryRun)
		if errMerge != nil {
			log.Println(errMerge)
		}

		log.Println(result.Message)

		if !result.Merged {
			errLabel := ghub.AddLabels(ctx, issuePR, repoID, markers.NeedHumanMerge)
			if errLabel != nil {
				log.Println(errLabel)
			}

			errLabel = ghub.RemoveLabel(ctx, issuePR, repoID, markers.MergeInProgress)
			if errLabel != nil {
				log.Println(errLabel)
			}

			return fmt.Errorf("failed to merge PR #%d", prNumber)
		}

		labelsToRemove := []string{
			markers.NeedMerge,
			markers.LightReview,
			markers.MergeMethodPrefix + gh.MergeMethodSquash,
			markers.MergeMethodPrefix + gh.MergeMethodMerge,
			markers.MergeMethodPrefix + gh.MergeMethodRebase,
			markers.MergeMethodPrefix + gh.MergeMethodFastForward,
		}
		err = ghub.RemoveLabels(ctx, issuePR, repoID, labelsToRemove)
		if err != nil {
			log.Println(err)
		}
	}

	err = mjolnir.CloseRelatedIssues(ctx, client, repoID.Owner, repoID.RepositoryName, pr, extra.DryRun)
	if err != nil {
		log.Println(err)
	}

	return nil
}

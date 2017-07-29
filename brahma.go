package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/containous/brahma/gh"
	"github.com/containous/brahma/search"
	"github.com/containous/brahma/updater"
	"github.com/containous/flaeg"
	"github.com/google/go-github/github"
)

const mergeMethod = "squash"

// Configuration task configuration.
type Configuration struct {
	Owner          string        `short:"o" description:"Repository owner. [required]"`
	RepositoryName string        `long:"repo-name" short:"r" description:"Repository name. [required]"`
	GitHubToken    string        `long:"token" short:"t" description:"GitHub Token. [required]"`
	MinReview      int           `long:"min-review" description:"Minimal number of review."`
	DryRun         bool          `long:"dry-run" description:"Dry run mode."`
	Debug          bool          `long:"debug" description:"Debug mode."`
	SSH            bool          `description:"Use SSH instead HTTPS."`
	LabelMarkers   *LabelMarkers `long:"marker" description:"GitHub Labels."`
}

// LabelMarkers Labels use to control actions.
type LabelMarkers struct {
	NeedHumanMerge  string `long:"need-human-merge" description:"Label use when the bot cannot perform a merge."`
	NeedMerge       string `long:"need-merge" description:"Label use when you want the bot perform a merge."`
	MergeInProgress string `long:"merge-in-progress" description:"Label use when the bot update the PR (merge/rebase)."`
}

func main() {
	config := &Configuration{
		MinReview: 1,
		DryRun:    true,
		LabelMarkers: &LabelMarkers{
			NeedHumanMerge:  "bot/need-human-merge",
			NeedMerge:       "status/3-needs-merge",
			MergeInProgress: "status/4-merge-in-progress",
		},
	}

	defaultPointersConfig := &Configuration{LabelMarkers: &LabelMarkers{}}
	rootCmd := &flaeg.Command{
		Name: "brahma",
		Description: `Brahma, God of Creation.
Update and Merge Pull Request from GitHub.
		`,
		Config:                config,
		DefaultPointersConfig: defaultPointersConfig,
		Run: func() error {
			if config.Debug {
				log.Printf("Run Brahma command with config : %+v\n", config)
			}

			if config.DryRun {
				log.Print("IMPORTANT: you are using the dry-run mode. Use `--dry-run=false` to disable this mode.")
			}

			required(config.GitHubToken, "token")
			required(config.Owner, "owner")
			required(config.RepositoryName, "repo-name")

			execute(*config)
			return nil
		},
	}

	flag := flaeg.New(rootCmd, os.Args[1:])
	flag.Run()
}

func required(field string, fieldName string) error {
	if len(field) == 0 {
		log.Fatalf("%s is mandatory.", fieldName)
	}
	return nil
}

func execute(config Configuration) {
	ctx := context.Background()
	client := gh.NewGitHubClient(ctx, config.GitHubToken)

	issuesMIP, err := search.FindOpenPR(ctx, client, config.Owner, config.RepositoryName,
		search.WithLabels(config.LabelMarkers.NeedMerge, config.LabelMarkers.MergeInProgress),
		search.WithExcludedLabels(config.LabelMarkers.NeedHumanMerge))
	if err != nil {
		log.Fatal(err)
	}
	if len(issuesMIP) > 1 {
		log.Fatalf("Illegal state: multiple label: %s", config.LabelMarkers.MergeInProgress)
	}

	if len(issuesMIP) == 1 {
		issue := issuesMIP[0]
		fmt.Println(issue.GetNumber(), issue.GetUpdatedAt())

		err = process(ctx, client, config, issue)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		issues, err := search.FindOpenPR(ctx, client, config.Owner, config.RepositoryName,
			search.WithLabels(config.LabelMarkers.NeedMerge))
		if err != nil {
			log.Fatal(err)
		}

		if len(issues) == 0 {
			log.Println("Nothing to merge.")
		} else {
			for _, issue := range issues {
				log.Println(issue.GetNumber(), issue.GetUpdatedAt())
			}

			err = process(ctx, client, config, issues[0])
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func process(ctx context.Context, client *github.Client, config Configuration, issue github.Issue) error {

	pr, _, err := client.PullRequests.Get(ctx, config.Owner, config.RepositoryName, issue.GetNumber())
	if err != nil {
		return err
	}

	ghub := gh.NewGHub(ctx, client, config.DryRun)

	prNumber := pr.GetNumber()

	err = ghub.HasReviewsApprove(pr, config.MinReview)
	if err != nil {
		err = ghub.AddLabelsToPR(pr, config.LabelMarkers.NeedHumanMerge)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		// STOP
		log.Printf("PR #%d: needs more reviews", prNumber)
		return err
	}

	status, err := ghub.GetStatus(pr)
	if err != nil {
		err = ghub.AddLabelsToPR(pr, config.LabelMarkers.NeedHumanMerge)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		// - STOP
		return err
	}
	if status == gh.Pending {
		// - skip
		log.Println("State: pending. Waiting for the CI.")
		return nil
	}

	if pr.GetMerged() {
		err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		// STOP
		return fmt.Errorf("the PR #%d is already merged", prNumber)
	}

	if !pr.GetMergeable() {
		err = ghub.AddLabelsToPR(pr, config.LabelMarkers.NeedHumanMerge)
		if err != nil {
			log.Println(err)
		}
		err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		// STOP
		return fmt.Errorf("conflicts must be resolve in the PR #%d", prNumber)
	}

	// rebase
	ok, err := ghub.IsUpdatedBranch(pr)
	if err != nil {
		return err
	}
	if ok {
		fmt.Printf("MERGE(%s): PR #%d\n", mergeMethod, prNumber)

		if !config.DryRun {
			err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
			if err != nil {
				log.Println(err)
			}

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
				err = ghub.AddLabelsToPR(pr, config.LabelMarkers.NeedHumanMerge)
				if err != nil {
					log.Println(err)
				}
				err := ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
				if err != nil {
					log.Println(err)
				}
				return fmt.Errorf("failed to merge PR #%d", prNumber)
			}

			err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.NeedMerge)
			if err != nil {
				log.Println(err)
			}
		}

	} else {
		fmt.Printf("UPDATE: PR #%d\n", prNumber)

		err := ghub.AddLabelsToPR(pr, config.LabelMarkers.MergeInProgress)
		if err != nil {
			log.Println(err)
		}

		err = updater.Process(ghub, pr, config.SSH, config.GitHubToken, config.DryRun, config.Debug)
		if err != nil {
			err = ghub.AddLabelsToPR(pr, config.LabelMarkers.NeedHumanMerge)
			if err != nil {
				log.Println(err)
			}
			err = ghub.RemoveLabelForPR(pr, config.LabelMarkers.MergeInProgress)
			if err != nil {
				log.Println(err)
			}
			return err
		}
	}

	return nil
}

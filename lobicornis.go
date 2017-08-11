package main

import (
	"log"
	"os"

	"github.com/containous/flaeg"
	"github.com/containous/lobicornis/core"
	"github.com/containous/lobicornis/gh"
)

func main() {
	config := &core.Configuration{
		MinReview:          1,
		DryRun:             true,
		DefaultMergeMethod: gh.MergeMethodSquash,
		MergeMethodPrefix:  "bot/merge-method-",
		LabelMarkers: &core.LabelMarkers{
			NeedHumanMerge:  "bot/need-human-merge",
			NeedMerge:       "status/3-needs-merge",
			MergeInProgress: "status/4-merge-in-progress",
		},
	}

	defaultPointersConfig := &core.Configuration{LabelMarkers: &core.LabelMarkers{}}
	rootCmd := &flaeg.Command{
		Name:                  "lobicornis",
		Description:           `Myrmica Lobicornis:  Update and Merge Pull Request from GitHub.`,
		Config:                config,
		DefaultPointersConfig: defaultPointersConfig,
		Run: func() error {
			if config.Debug {
				log.Printf("Run Lobicornis command with config : %+v\n", config)
			}

			if config.DryRun {
				log.Print("IMPORTANT: you are using the dry-run mode. Use `--dry-run=false` to disable this mode.")
			}

			required(config.GitHubToken, "token")
			required(config.Owner, "owner")
			required(config.RepositoryName, "repo-name")
			required(config.DefaultMergeMethod, "merge-method")

			required(config.LabelMarkers.NeedMerge, "need-merge")
			required(config.LabelMarkers.MergeInProgress, "merge-in-progress")
			required(config.LabelMarkers.NeedHumanMerge, "need-human-merge")

			core.Execute(*config)
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

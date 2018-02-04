package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/containous/flaeg"
	"github.com/containous/lobicornis/core"
	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/meta"
	"github.com/containous/lobicornis/types"
)

func main() {
	config := &types.Configuration{
		MinReview:          1,
		DryRun:             true,
		DefaultMergeMethod: gh.MergeMethodSquash,
		LabelMarkers: &types.LabelMarkers{
			NeedHumanMerge:    "bot/need-human-merge",
			NeedMerge:         "status/3-needs-merge",
			MergeInProgress:   "status/4-merge-in-progress",
			MergeMethodPrefix: "bot/merge-method-",
			MergeRetryPrefix:  "bot/merge-retry-",
			LightReview:       "bot/light-review",
			NoMerge:           "bot/no-merge",
		},
		ForceNeedUpToDate: true,
		ServerPort:        80,
		NeedMilestone:     true,
	}

	defaultPointersConfig := &types.Configuration{
		LabelMarkers: &types.LabelMarkers{},
		Retry: &types.Retry{
			Interval: flaeg.Duration(1 * time.Minute),
		},
	}

	rootCmd := &flaeg.Command{
		Name:                  "lobicornis",
		Description:           `Myrmica Lobicornis: Update and Merge Pull Request from GitHub.`,
		DefaultPointersConfig: defaultPointersConfig,
		Config:                config,
		Run:                   runCommand(config),
	}

	flag := flaeg.New(rootCmd, os.Args[1:])

	// version
	versionCmd := &flaeg.Command{
		Name:                  "version",
		Description:           "Display the version.",
		Config:                &types.NoOption{},
		DefaultPointersConfig: &types.NoOption{},
		Run: func() error {
			meta.DisplayVersion()
			return nil
		},
	}

	flag.AddCommand(versionCmd)

	err := flag.Run()
	if err != nil {
		log.Printf("Error: %v\n", err)
	}
}

func runCommand(config *types.Configuration) func() error {
	return func() error {
		if config.Debug {
			log.Printf("Run Lobicornis command with config : %+v\n", config)
		}

		if config.DryRun {
			log.Print("IMPORTANT: you are using the dry-run mode. Use `--dry-run=false` to disable this mode.")
		}

		if len(config.GitHubToken) == 0 {
			config.GitHubToken = os.Getenv("GITHUB_TOKEN")
		}

		err := validateConfig(config)
		if err != nil {
			log.Fatal(err)
		}

		err = launch(config)
		if err != nil {
			log.Fatal(err)
		}
		return nil
	}
}

func launch(config *types.Configuration) error {
	if config.ServerMode {
		server := &server{config: config}
		return server.ListenAndServe()
	}
	return core.Execute(*config)
}

func validateConfig(config *types.Configuration) error {
	err := required(config.GitHubToken, "token")
	if err != nil {
		return err
	}
	err = required(config.Owner, "owner")
	if err != nil {
		return err
	}
	err = required(config.RepositoryName, "repo-name")
	if err != nil {
		return err
	}
	err = required(config.DefaultMergeMethod, "merge-method")
	if err != nil {
		return err
	}

	err = required(config.LabelMarkers.NeedMerge, "need-merge")
	if err != nil {
		return err
	}
	err = required(config.LabelMarkers.MergeInProgress, "merge-in-progress")
	if err != nil {
		return err
	}
	err = required(config.LabelMarkers.NoMerge, "no-merge")
	if err != nil {
		return err
	}
	return required(config.LabelMarkers.NeedHumanMerge, "need-human-merge")
}

func required(field string, fieldName string) error {
	if len(field) == 0 {
		log.Fatalf("%s is mandatory.", fieldName)
	}
	return nil
}

type server struct {
	config *types.Configuration
}

func (s *server) ListenAndServe() error {
	return http.ListenAndServe(":"+strconv.Itoa(s.config.ServerPort), s)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Printf("Invalid http method: %s", r.Method)
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := core.Execute(*s.config)
	if err != nil {
		log.Printf("Report error: %v", err)
		http.Error(w, "Report error.", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Myrmica Lobicornis: Scheluded.\n")
}

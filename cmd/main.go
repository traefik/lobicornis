package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v58/github"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/traefik/lobicornis/v3/pkg/conf"
	"github.com/traefik/lobicornis/v3/pkg/repository"
	"github.com/traefik/lobicornis/v3/pkg/search"
	"golang.org/x/oauth2"
)

func main() {
	filename := flag.String("config", "./lobicornis.yml", "Path to the configuration file.")
	serverMode := flag.Bool("server", false, "Run as a web server.")
	version := flag.Bool("version", false, "Display version information.")
	help := flag.Bool("h", false, "Show this help.")

	flag.Usage = usage
	flag.Parse()

	if *help {
		usage()
		return
	}

	nArgs := flag.NArg()
	if nArgs > 0 {
		usage()
		return
	}

	if version != nil && *version {
		displayVersion()
		return
	}

	if filename == nil || *filename == "" {
		usage()
		return
	}

	cfg, err := conf.Load(*filename)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to load config")
	}

	setupLogger(cfg.Extra.DryRun, cfg.Extra.LogLevel)

	if *serverMode {
		err = launch(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to launch the server")
		}
	} else {
		err = run(cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("unable to run the command")
		}
	}
}

func launch(cfg conf.Configuration) error {
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			log.Error().Str("method", req.Method).Msg("Invalid http method")
			http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

			return
		}

		err := run(cfg)
		if err != nil {
			log.Error().Err(err).Msg("Report error")
			http.Error(rw, "Report error.", http.StatusInternalServerError)

			return
		}

		_, err = fmt.Fprint(rw, "Myrmica Lobicornis: Scheduled.\n")
		if err != nil {
			log.Error().Err(err).Msg("Report error")
			http.Error(rw, err.Error(), http.StatusInternalServerError)

			return
		}
	})

	return http.ListenAndServe(":"+strconv.Itoa(cfg.Server.Port), handler)
}

func run(cfg conf.Configuration) error {
	ctx := context.Background()

	client := newGitHubClient(ctx, cfg.Github.Token, cfg.Github.URL)

	finder := search.New(client, cfg.Markers, cfg.Retry)

	// search PRs with the FF merge method.
	ffResults, err := finder.Search(ctx, cfg.Github.User,
		search.WithLabels(cfg.Markers.MergeMethodPrefix+conf.MergeMethodFastForward),
		search.WithExcludedLabels(cfg.Markers.NoMerge, cfg.Markers.NeedMerge))
	if err != nil {
		return err
	}

	// search NeedMerge
	results, err := finder.Search(ctx, cfg.Github.User,
		search.WithLabels(cfg.Markers.NeedMerge),
		search.WithExcludedLabels(cfg.Markers.NeedHumanMerge, cfg.Markers.NoMerge))
	if err != nil {
		return err
	}

	for fullName, issues := range results {
		logger := log.With().Str("repo", fullName).Logger()

		if _, ok := ffResults[fullName]; ok {
			logger.Info().Msgf("Waiting for the merge of pull request with the label: %s", cfg.Markers.MergeMethodPrefix+conf.MergeMethodFastForward)
			continue
		}

		repoConfig := getRepoConfig(cfg, fullName)

		issue, err := finder.GetCurrentPull(logger.WithContext(ctx), issues)
		if err != nil {
			logger.Error().Err(err).Msg("unable to get the current pull request")
			continue
		}

		if issue == nil {
			logger.Debug().Msg("Nothing to merge.")
			continue
		}

		repo := repository.New(client, fullName, cfg.Github.Token, cfg.Markers, cfg.Retry, cfg.Git, repoConfig, cfg.Extra)

		loggerIssue := logger.With().Int("pr", issue.GetNumber()).Logger()

		err = repo.Process(loggerIssue.WithContext(ctx), issue.GetNumber())
		if err != nil {
			loggerIssue.Error().Err(err).Msg("Failed to process")
		}
	}

	return nil
}

// newGitHubClient create a new GitHub client.
func newGitHubClient(ctx context.Context, token string, gitHubURL string) *github.Client {
	var tc *http.Client

	if len(token) != 0 {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc = oauth2.NewClient(ctx, ts)
	}

	client := github.NewClient(tc)

	if gitHubURL != "" {
		baseURL, err := url.Parse(gitHubURL)
		if err == nil {
			client.BaseURL = baseURL
		}
	}

	return client
}

func getRepoConfig(cfg conf.Configuration, repoName string) conf.RepoConfig {
	if repoCfg, ok := cfg.Repositories[repoName]; ok && repoCfg != nil {
		return *repoCfg
	}

	return cfg.Default
}

func usage() {
	_, _ = os.Stderr.WriteString("Myrmica Lobicornis:\n")

	flag.PrintDefaults()
}

// setupLogger is configuring the logger.
func setupLogger(dryRun bool, level string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	log.Logger = zerolog.New(os.Stderr).With().Caller().Logger()

	logLevel := zerolog.DebugLevel

	if !dryRun {
		var err error

		logLevel, err = zerolog.ParseLevel(strings.ToLower(level))
		if err != nil {
			logLevel = zerolog.InfoLevel
		}
	}

	zerolog.SetGlobalLevel(logLevel)

	log.Trace().Msgf("Log level set to %s.", logLevel)
}

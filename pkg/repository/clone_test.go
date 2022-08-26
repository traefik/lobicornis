package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/go-github/v47/github"
	"github.com/ldez/go-git-cmd-wrapper/v2/git"
	"github.com/ldez/go-git-cmd-wrapper/v2/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/lobicornis/v3/pkg/conf"
)

func TestClone_PullRequestForUpdate(t *testing.T) {
	testCases := []struct {
		name                string
		sameRepo            bool
		expectedRemoteName  string
		expectedOriginURL   string
		expectedUpstreamURL string
	}{
		{
			name:                "PR with 2 different repositories",
			expectedRemoteName:  "upstream",
			expectedOriginURL:   "https://github.com/ldez/traefik.git",
			expectedUpstreamURL: "https://github.com/traefik/traefik.git",
		},
		{
			name:                "PR from main repository",
			sameRepo:            true,
			expectedRemoteName:  "origin",
			expectedOriginURL:   "https://github.com/traefik/traefik.git",
			expectedUpstreamURL: "https://github.com/traefik/traefik.git",
		},
	}

	gitConfig := conf.Git{
		UserName: "hubert",
		Email:    "hubert@foo.com",
		SSH:      false,
	}

	clone := newClone(gitConfig, "")

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "myrmica-lobicornis")
			require.NoError(t, err)

			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			err = os.Chdir(dir)
			require.NoError(t, err)

			tempDir, err := os.Getwd()
			require.NoError(t, err)

			t.Log(tempDir)

			pr := createFakePR(test.sameRepo)

			remoteName, err := clone.PullRequestForUpdate(context.Background(), pr)
			require.NoError(t, err)

			assert.Equal(t, test.expectedRemoteName, remoteName)

			localOriginURL, err := git.Remote(remote.GetURL("origin"))
			require.NoError(t, err)

			assert.Equal(t, test.expectedOriginURL, strings.TrimSpace(localOriginURL))

			localUpstreamURL, err := git.Remote(remote.GetURL(test.expectedRemoteName))
			require.NoError(t, err)

			assert.Equal(t, test.expectedUpstreamURL, strings.TrimSpace(localUpstreamURL))
		})
	}
}

func TestClone_PullRequestForMerge(t *testing.T) {
	testCases := []struct {
		name                string
		sameRepo            bool
		expectedRemoteName  string
		expectedOriginURL   string
		expectedUpstreamURL string
	}{
		{
			name:                "PR with 2 different repositories",
			expectedRemoteName:  "upstream",
			expectedOriginURL:   "https://github.com/traefik/traefik.git",
			expectedUpstreamURL: "https://github.com/ldez/traefik.git",
		},
		{
			name:                "PR from main repository",
			sameRepo:            true,
			expectedRemoteName:  "origin",
			expectedOriginURL:   "https://github.com/traefik/traefik.git",
			expectedUpstreamURL: "https://github.com/traefik/traefik.git",
		},
	}

	gitConfig := conf.Git{
		UserName: "hubert",
		Email:    "hubert@foo.com",
		SSH:      false,
	}

	clone := newClone(gitConfig, "")

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "myrmica-lobicornis")
			require.NoError(t, err)

			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			err = os.Chdir(dir)
			require.NoError(t, err)

			tempDir, err := os.Getwd()
			require.NoError(t, err)

			t.Log(tempDir)

			pr := createFakePR(test.sameRepo)

			remoteName, err := clone.PullRequestForMerge(context.Background(), pr)
			require.NoError(t, err)

			assert.Equal(t, test.expectedRemoteName, remoteName)

			localOriginURL, err := git.Remote(remote.GetURL("origin"))
			require.NoError(t, err)

			assert.Equal(t, test.expectedOriginURL, strings.TrimSpace(localOriginURL))

			localUpstreamURL, err := git.Remote(remote.GetURL(test.expectedRemoteName))
			require.NoError(t, err)

			assert.Equal(t, test.expectedUpstreamURL, strings.TrimSpace(localUpstreamURL))
		})
	}
}

func Test_makeRepositoryURL(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		ssh         bool
		token       string
		expectedURL string
	}{
		{
			name:        "HTTPS",
			url:         "git://github.com/traefik/traefik.git",
			expectedURL: "https://github.com/traefik/traefik.git",
		},
		{
			name:        "HTTPS with token",
			url:         "git://github.com/traefik/traefik.git",
			token:       "token",
			expectedURL: "https://token@github.com/traefik/traefik.git",
		},
		{
			name:        "SSH",
			url:         "git://github.com/traefik/traefik.git",
			ssh:         true,
			expectedURL: "git@github.com:traefik/traefik.git",
		},
		{
			name:        "SSH with token",
			url:         "git://github.com/traefik/traefik.git",
			ssh:         true,
			token:       "token",
			expectedURL: "git@github.com:traefik/traefik.git",
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			url := makeRepositoryURL(test.url, test.ssh, test.token)

			if url != test.expectedURL {
				t.Errorf("Got %s, want %s.", url, test.expectedURL)
			}
		})
	}
}

func createFakePR(sameRepo bool) *github.PullRequest {
	pr := &github.PullRequest{
		Number: github.Int(666),
	}
	pr.Base = &github.PullRequestBranch{
		Repo: &github.Repository{
			GitURL: github.String("git://github.com/traefik/traefik.git"),
		},
		Ref: github.String("master"),
	}

	if sameRepo {
		pr.Head = &github.PullRequestBranch{
			Repo: &github.Repository{
				GitURL: github.String("git://github.com/traefik/traefik.git"),
			},
			Ref: github.String("v1.3"),
		}
	} else {
		pr.Head = &github.PullRequestBranch{
			Repo: &github.Repository{
				GitURL: github.String("git://github.com/ldez/traefik.git"),
			},
			Ref: github.String("v1.3"),
		}
	}

	return pr
}

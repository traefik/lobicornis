package clone

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/github"
	"github.com/ldez/go-git-cmd-wrapper/git"
	"github.com/ldez/go-git-cmd-wrapper/remote"
)

func TestPullRequestForUpdate(t *testing.T) {

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
			expectedUpstreamURL: "https://github.com/containous/traefik.git",
		},
		{
			name:                "PR from main repository",
			sameRepo:            true,
			expectedRemoteName:  "origin",
			expectedOriginURL:   "https://github.com/containous/traefik.git",
			expectedUpstreamURL: "https://github.com/containous/traefik.git",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			dir, err := ioutil.TempDir("", "myrmica-lobicornis")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			err = os.Chdir(dir)
			if err != nil {
				t.Fatal(err)
			}

			tempDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println(tempDir)

			pr := createFakePR(test.sameRepo)

			gitConfig := types.GitConfig{
				GitHubToken: "",
				SSH:         false,
				UserName:    "hubert",
				Email:       "hubert@foo.com",
			}

			remoteName, err := PullRequestForUpdate(pr, gitConfig, true)
			if err != nil {
				t.Fatal(err)
			}

			if remoteName != test.expectedRemoteName {
				t.Errorf("Got %s, want %s.", remoteName, test.expectedRemoteName)
			}

			localOriginURL, err := git.Remote(remote.GetURL("origin"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(localOriginURL) != test.expectedOriginURL {
				t.Errorf("Got %s, want %s.", localOriginURL, test.expectedOriginURL)
			}

			localUpstreamURL, err := git.Remote(remote.GetURL(test.expectedRemoteName))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(localUpstreamURL) != test.expectedUpstreamURL {
				t.Errorf("Got %s, want %s.", localUpstreamURL, test.expectedUpstreamURL)
			}
		})
	}
}

func TestPullRequestForMerge(t *testing.T) {

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
			expectedOriginURL:   "https://github.com/containous/traefik.git",
			expectedUpstreamURL: "https://github.com/ldez/traefik.git",
		},
		{
			name:                "PR from main repository",
			sameRepo:            true,
			expectedRemoteName:  "origin",
			expectedOriginURL:   "https://github.com/containous/traefik.git",
			expectedUpstreamURL: "https://github.com/containous/traefik.git",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			dir, err := ioutil.TempDir("", "myrmica-lobicornis")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			err = os.Chdir(dir)
			if err != nil {
				t.Fatal(err)
			}

			tempDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println(tempDir)

			pr := createFakePR(test.sameRepo)

			gitConfig := types.GitConfig{
				GitHubToken: "",
				SSH:         false,
				UserName:    "hubert",
				Email:       "hubert@foo.com",
			}

			remoteName, err := PullRequestForMerge(pr, gitConfig, true)
			if err != nil {
				t.Fatal(err)
			}

			if remoteName != test.expectedRemoteName {
				t.Errorf("Got %s, want %s.", remoteName, test.expectedRemoteName)
			}

			localOriginURL, err := git.Remote(remote.GetURL("origin"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(localOriginURL) != test.expectedOriginURL {
				t.Errorf("Got %s, want %s.", localOriginURL, test.expectedOriginURL)
			}

			localUpstreamURL, err := git.Remote(remote.GetURL(test.expectedRemoteName))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(localUpstreamURL) != test.expectedUpstreamURL {
				t.Errorf("Got %s, want %s.", localUpstreamURL, test.expectedUpstreamURL)
			}
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
			url:         "git://github.com/containous/traefik.git",
			expectedURL: "https://github.com/containous/traefik.git",
		},
		{
			name:        "HTTPS with token",
			url:         "git://github.com/containous/traefik.git",
			token:       "token",
			expectedURL: "https://token@github.com/containous/traefik.git",
		},
		{
			name:        "SSH",
			url:         "git://github.com/containous/traefik.git",
			ssh:         true,
			expectedURL: "git@github.com:containous/traefik.git",
		},
		{
			name:        "SSH with token",
			url:         "git://github.com/containous/traefik.git",
			ssh:         true,
			token:       "token",
			expectedURL: "git@github.com:containous/traefik.git",
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
			GitURL: github.String("git://github.com/containous/traefik.git"),
		},
		Ref: github.String("master"),
	}

	if sameRepo {
		pr.Head = &github.PullRequestBranch{
			Repo: &github.Repository{
				GitURL: github.String("git://github.com/containous/traefik.git"),
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

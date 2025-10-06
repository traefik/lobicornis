package conf

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	testCases := []struct {
		desc     string
		filename string
		expected Configuration
	}{
		{
			desc:     "simple",
			filename: filepath.FromSlash("./fixtures/config.yml"),
			expected: Configuration{
				Github: Github{
					User:  "ldez",
					Token: "XXXX",
					URL:   "http://my-private-github.com",
				},
				Git: Git{
					Email:    "bot@example.com",
					UserName: "botname",
					SSH:      true,
				},
				Server: Server{
					Port: 80,
				},
				Markers: Markers{
					LightReview:       "bot/light-review",
					NeedMerge:         "status/3-needs-merge",
					MergeInProgress:   "status/4-merge-in-progress",
					MergeMethodPrefix: "bot/merge-method-",
					MergeRetryPrefix:  "bot/merge-retry-",
					NeedHumanMerge:    "bot/need-human-merge",
					NoMerge:           "bot/no-merge",
					NoRebase:          "bot/no-rebase",
				},
				Retry: Retry{
					Interval:    1 * time.Minute,
					Number:      0,
					OnMergeable: false,
					OnStatuses:  false,
				},
				Default: RepoConfig{
					MergeMethod:       String("squash"),
					MinLightReview:    Int(0),
					MinReview:         Int(1),
					NeedMilestone:     Bool(true),
					CheckNeedUpToDate: Bool(false),
					ForceNeedUpToDate: Bool(true),
					AddErrorInComment: Bool(false),
					CommitMessage:     String("empty"),
				},
				Extra: Extra{
					DryRun:   true,
					LogLevel: "info",
				},
				Repositories: map[string]*RepoConfig{
					"ldez/myrepo1": {
						MergeMethod:       String("squash"),
						MinLightReview:    Int(1),
						MinReview:         Int(0),
						NeedMilestone:     Bool(true),
						CheckNeedUpToDate: Bool(false),
						ForceNeedUpToDate: Bool(true),
						AddErrorInComment: Bool(false),
						CommitMessage:     String("empty"),
					},
					"ldez/myrepo2": {
						MergeMethod:       String("squash"),
						MinLightReview:    Int(1),
						MinReview:         Int(1),
						NeedMilestone:     Bool(false),
						CheckNeedUpToDate: Bool(false),
						ForceNeedUpToDate: Bool(true),
						AddErrorInComment: Bool(false),
						CommitMessage:     String("description"),
					},
				},
			},
		},
		{
			desc:     "defaulting",
			filename: filepath.FromSlash("./fixtures/config_01.yml"),
			expected: Configuration{
				Github: Github{
					User:  "ldez",
					Token: "XXXX",
					URL:   "http://my-private-github.com",
				},
				Git: Git{
					Email:    "bot@example.com",
					UserName: "botname",
					SSH:      true,
				},
				Server: Server{
					Port: 80,
				},
				Markers: Markers{
					LightReview:       "bot/ooo",
					NeedMerge:         "status/3-needs-merge",
					MergeInProgress:   "status/4-merge-in-progress",
					MergeMethodPrefix: "bot/merge-method-",
					MergeRetryPrefix:  "bot/merge-retry-",
					NeedHumanMerge:    "bot/need-human-merge",
					NoMerge:           "bot/no-merge",
					NoRebase:          "bot/no-rebase",
				},
				Retry: Retry{
					Interval:    1 * time.Minute,
					Number:      0,
					OnMergeable: false,
					OnStatuses:  false,
				},
				Default: RepoConfig{
					MergeMethod:       String("squash"),
					MinLightReview:    Int(25),
					MinReview:         Int(1),
					NeedMilestone:     Bool(true),
					CheckNeedUpToDate: Bool(false),
					ForceNeedUpToDate: Bool(true),
					AddErrorInComment: Bool(false),
					CommitMessage:     String("empty"),
				},
				Extra: Extra{
					DryRun:   true,
					LogLevel: "info",
				},
				Repositories: map[string]*RepoConfig{
					"ldez/myrepo1": {
						MergeMethod:       String("squash"),
						MinLightReview:    Int(25),
						MinReview:         Int(0),
						NeedMilestone:     Bool(true),
						CheckNeedUpToDate: Bool(false),
						ForceNeedUpToDate: Bool(true),
						AddErrorInComment: Bool(false),
						CommitMessage:     String("empty"),
					},
					"ldez/myrepo2": {
						MergeMethod:       String("squash"),
						MinLightReview:    Int(1),
						MinReview:         Int(1),
						NeedMilestone:     Bool(false),
						CheckNeedUpToDate: Bool(false),
						ForceNeedUpToDate: Bool(true),
						AddErrorInComment: Bool(false),
						CommitMessage:     String("empty"),
					},
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(test.filename)
			require.NoError(t, err)

			assert.Equal(t, test.expected, cfg)
		})
	}
}

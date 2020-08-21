package search

import (
	"testing"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/lobicornis/v2/pkg/conf"
)

func TestFinder_GetCurrentPull(t *testing.T) {
	markers := conf.Markers{
		LightReview:       "bot/light-review",
		NeedMerge:         "status/3-needs-merge",
		MergeInProgress:   "status/4-merge-in-progress",
		MergeMethodPrefix: "bot/merge-method-",
		MergeRetryPrefix:  "bot/merge-retry-",
		NeedHumanMerge:    "bot/need-human-merge",
		NoMerge:           "bot/no-merge",
	}

	retry := conf.Retry{
		Interval:    1 * time.Minute,
		Number:      1,
		OnMergeable: true,
		OnStatuses:  false,
	}

	finder := New(nil, true, markers, retry)

	testCases := []struct {
		desc     string
		issues   []*github.Issue
		expected int
	}{
		{
			desc:     "no pull request",
			issues:   nil,
			expected: 0,
		},
		{
			desc: "one pull request",
			issues: []*github.Issue{
				{
					Number: github.Int(1),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
					},
				},
			},
			expected: 1,
		},
		{
			desc: "take the most pull request",
			issues: []*github.Issue{
				{
					Number: github.Int(2),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
					},
				},
				{
					Number: github.Int(1),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
					},
				},
			},
			expected: 2,
		},
		{
			desc: "take the pull request with merge in progress",
			issues: []*github.Issue{
				{
					Number: github.Int(2),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
					},
				},
				{
					Number: github.Int(1),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
						{Name: github.String("status/4-merge-in-progress")},
					},
				},
			},
			expected: 1,
		},
		{
			desc: "take the pull request with ff merge method",
			issues: []*github.Issue{
				{
					Number: github.Int(2),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
					},
				},
				{
					Number: github.Int(1),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
						{Name: github.String("bot/merge-method-ff")},
					},
				},
				{
					Number: github.Int(3),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
						{Name: github.String("status/4-merge-in-progress")},
					},
				},
			},
			expected: 1,
		},
		{
			desc: "take the pull request with retry",
			issues: []*github.Issue{
				{
					Number: github.Int(2),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
						{Name: github.String("status/4-merge-in-progress")},
					},
				},
				{
					Number: github.Int(1),
					Labels: []*github.Label{
						{Name: github.String("status/3-needs-merge")},
						{Name: github.String("status/4-merge-in-progress")},
						{Name: github.String("bot/merge-retry-1")},
					},
				},
			},
			expected: 1,
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			pr, err := finder.GetCurrentPull(test.issues)
			require.NoError(t, err)

			assert.Equal(t, test.expected, pr.GetNumber())
		})
	}
}

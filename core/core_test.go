package core

import (
	"testing"

	"github.com/containous/lobicornis/gh"
	"github.com/containous/lobicornis/types"
	"github.com/google/go-github/v27/github"
)

func Test_getMergeMethod(t *testing.T) {

	testCases := []struct {
		name                string
		labels              []string
		defaultMergeMethod  string
		mergeMethodPrefix   string
		expectedMergeMethod string
		expectedError       bool
	}{
		{
			name:                "without merge method prefix",
			labels:              []string{"foo", "bar", "merge"},
			defaultMergeMethod:  gh.MergeMethodSquash,
			mergeMethodPrefix:   "",
			expectedMergeMethod: gh.MergeMethodSquash,
		},
		{
			name:                "use custom label for merge",
			labels:              []string{"foo", "bar", "go-merge"},
			defaultMergeMethod:  gh.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: gh.MergeMethodMerge,
		},
		{
			name:                "use custom label for squash",
			labels:              []string{"foo", "bar", "go-squash"},
			defaultMergeMethod:  gh.MergeMethodMerge,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: gh.MergeMethodSquash,
		},
		{
			name:                "use custom label for rebase",
			labels:              []string{"foo", "bar", "go-rebase"},
			defaultMergeMethod:  gh.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: gh.MergeMethodRebase,
		},
		{
			name:                "use custom label for ff",
			labels:              []string{"foo", "bar", "go-ff"},
			defaultMergeMethod:  gh.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: gh.MergeMethodFastForward,
		},
		{
			name:                "unknown custom label with prefix",
			labels:              []string{"foo", "bar", "go-run"},
			defaultMergeMethod:  gh.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: gh.MergeMethodSquash,
		},
		{
			name:               "multiple custom merge method",
			labels:             []string{"go-rebase", "go-squash", "go-merge"},
			defaultMergeMethod: gh.MergeMethodSquash,
			mergeMethodPrefix:  "go-",
			expectedError:      true,
		},
	}

	for i, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			issue := makeIssueWithLabels(test.labels, i)

			labelMarkers := &types.LabelMarkers{MergeMethodPrefix: test.mergeMethodPrefix}

			method, err := getMergeMethod(issue, labelMarkers, test.defaultMergeMethod)

			if test.expectedError && err == nil {
				t.Fatalf("Got no error, expected an error.")
			}
			if !test.expectedError && err != nil {
				t.Fatalf("Got error %v, expected no error.", err)
			}

			if method != test.expectedMergeMethod {
				t.Errorf("Got %s, expected %s.", method, test.expectedMergeMethod)
			}
		})
	}
}

func Test_getMinReview(t *testing.T) {

	testCases := []struct {
		name              string
		review            types.Review
		markers           *types.LabelMarkers
		labels            []string
		expectedMinReview int
	}{
		{
			name: "with light review label",
			review: types.Review{
				Min:      3,
				MinLight: 1,
			},
			markers: &types.LabelMarkers{
				LightReview: "bot/light-review",
			},
			labels:            []string{"bot/light-review"},
			expectedMinReview: 1,
		},
		{
			name: "without light review label",
			review: types.Review{
				Min:      3,
				MinLight: 1,
			},
			markers: &types.LabelMarkers{
				LightReview: "bot/light-review",
			},
			expectedMinReview: 3,
		},
	}

	for i, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			issue := makeIssueWithLabels(test.labels, i)

			minReview := getMinReview(issue, test.review, test.markers)

			if minReview != test.expectedMinReview {
				t.Errorf("Got %d, want %d.", minReview, test.expectedMinReview)
			}
		})
	}
}

func makeIssueWithLabels(labelNames []string, issueNumber int) *github.Issue {
	var labels []github.Label
	for _, labelName := range labelNames {
		labels = append(labels, github.Label{
			Name: github.String(labelName),
		})
	}
	return &github.Issue{Labels: labels, Number: github.Int(issueNumber)}
}

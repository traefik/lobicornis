package core

import (
	"testing"

	"github.com/containous/brahma/gh"
	"github.com/google/go-github/github"
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

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var labels []github.Label

			for _, labelName := range test.labels {
				labels = append(labels, github.Label{
					Name: github.String(labelName),
				})
			}

			issue := &github.Issue{Labels: labels}

			config := Configuration{
				DefaultMergeMethod: test.defaultMergeMethod,
				MergeMethodPrefix:  test.mergeMethodPrefix,
			}

			method, err := getMergeMethod(issue, config)

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

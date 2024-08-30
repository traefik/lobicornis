package repository

import (
	"testing"

	"github.com/google/go-github/v58/github"
	"github.com/stretchr/testify/assert"
	"github.com/traefik/lobicornis/v3/pkg/conf"
)

func Test_getCoAuthors(t *testing.T) {
	testCases := []struct {
		desc     string
		body     string
		expected []string
	}{
		{
			desc: "no co-author",
			body: `
Jarlsberg cheese strings say cheese.
Cheesy grin taleggio cheese and wine red leicester babybel edam everyone loves squirty cheese.

Fromage frais hard cheese mozzarella chalk and cheese chalk and cheese port-salut mascarpone cauliflower cheese.

Goat port-salut st. agur blue cheese camembert de normandie manchego.
`,
			expected: nil,
		},
		{
			desc: "one co-author",
			body: "Co-authored-by: another-name <another-name@example.com>",
			expected: []string{
				"Co-authored-by: another-name <another-name@example.com>",
			},
		},
		{
			desc: "one co-author (case insensitive)",
			body: "Co-Authored-By: test <test@test.com>",
			expected: []string{
				"Co-authored-by: test <test@test.com>",
			},
		},
		{
			desc: "multiple co-author",
			body: `
Co-authored-by: test1 <test1@test.com>
Jarlsberg cheese strings say cheese.
Cheesy grin taleggio cheese and wine red leicester babybel edam everyone loves squirty cheese.
Co-authored-by: test2 <test2@test.com>
Fromage frais hard cheese mozzarella chalk and cheese chalk and cheese port-salut mascarpone cauliflower cheese.
Co-authored-by: test3 <test3@test.com>
Goat port-salut st. agur blue cheese camembert de normandie manchego.
`,
			expected: []string{
				"Co-authored-by: test1 <test1@test.com>",
				"Co-authored-by: test2 <test2@test.com>",
				"Co-authored-by: test3 <test3@test.com>",
			},
		},
		{
			desc:     "spaces before co-author",
			body:     "           Co-authored-by: test <test@test.com>",
			expected: nil,
		},
		{
			desc:     "spaces after co-author",
			body:     "Co-authored-by: test <test@test.com>    ",
			expected: nil,
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			pr := &github.PullRequest{
				Body: github.String(test.body),
			}

			coAuthors := getCoAuthors(pr)

			assert.Equal(t, test.expected, coAuthors)
		})
	}
}

func TestRepository_getMergeMethod(t *testing.T) {
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
			defaultMergeMethod:  conf.MergeMethodSquash,
			mergeMethodPrefix:   "",
			expectedMergeMethod: conf.MergeMethodSquash,
		},
		{
			name:                "use custom label for merge",
			labels:              []string{"foo", "bar", "go-merge"},
			defaultMergeMethod:  conf.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: conf.MergeMethodMerge,
		},
		{
			name:                "use custom label for squash",
			labels:              []string{"foo", "bar", "go-squash"},
			defaultMergeMethod:  conf.MergeMethodMerge,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: conf.MergeMethodSquash,
		},
		{
			name:                "use custom label for rebase",
			labels:              []string{"foo", "bar", "go-rebase"},
			defaultMergeMethod:  conf.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: conf.MergeMethodRebase,
		},
		{
			name:                "use custom label for ff",
			labels:              []string{"foo", "bar", "go-ff"},
			defaultMergeMethod:  conf.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: conf.MergeMethodFastForward,
		},
		{
			name:                "unknown custom label with prefix",
			labels:              []string{"foo", "bar", "go-run"},
			defaultMergeMethod:  conf.MergeMethodSquash,
			mergeMethodPrefix:   "go-",
			expectedMergeMethod: conf.MergeMethodSquash,
		},
		{
			name:               "multiple custom merge method",
			labels:             []string{"go-rebase", "go-squash", "go-merge"},
			defaultMergeMethod: conf.MergeMethodSquash,
			mergeMethodPrefix:  "go-",
			expectedError:      true,
		},
	}

	for i, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := Repository{
				markers: conf.Markers{
					MergeMethodPrefix: test.mergeMethodPrefix,
				},
				config: conf.RepoConfig{
					MergeMethod: conf.String(test.defaultMergeMethod),
				},
			}

			pr := makePullRequestWithLabels(test.labels, i)

			method, err := repository.getMergeMethod(pr)

			if test.expectedError && err == nil {
				t.Fatalf("Got no error, expected an error.")
			}
			if !test.expectedError && err != nil {
				t.Fatalf("Got error %v, expected no error.", err)
			}

			assert.Equal(t, test.expectedMergeMethod, method)
		})
	}
}

func makePullRequestWithLabels(labelNames []string, issueNumber int) *github.PullRequest {
	var labels []*github.Label
	for _, labelName := range labelNames {
		labels = append(labels, &github.Label{
			Name: github.String(labelName),
		})
	}

	return &github.PullRequest{Labels: labels, Number: github.Int(issueNumber)}
}

package merge

import (
	"reflect"
	"testing"

	"github.com/google/go-github/v30/github"
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
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			pr := &github.PullRequest{
				Body: github.String(test.body),
			}
			coAuthors := getCoAuthors(pr)

			if !reflect.DeepEqual(coAuthors, test.expected) {
				t.Fatalf("Got %q, want %q", coAuthors, test.expected)
			}
		})
	}
}

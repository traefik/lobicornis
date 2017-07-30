package mjolnir

import (
	"reflect"
	"testing"
)

func Test_parseIssueFixes(t *testing.T) {
	testCases := []struct {
		name            string
		text            string
		expectedNumbers []int
	}{
		{
			name: "only letters",
			text: `
	Fixes dlsqj
`,
			expectedNumbers: []int{},
		},
		{
			name: "valid issue numbers",
			text: `
	Fixes #13 #14, #15,#16,
`,
			expectedNumbers: []int{13, 14, 15, 16},
		},
		{
			name: "invalid pattern",
			text: `
	Fixes #13#14,#15,#16,
`,
			expectedNumbers: []int{},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			issueNumbers := parseIssueFixes(test.text)

			if (len(issueNumbers) != 0 || len(test.expectedNumbers) != 0) && !reflect.DeepEqual(issueNumbers, test.expectedNumbers) {
				t.Errorf("Got %v, expected %v", issueNumbers, test.expectedNumbers)
			}
		})
	}
}

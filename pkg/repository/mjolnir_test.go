package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
			expectedNumbers: nil,
		},
		{
			name: "valid issue numbers coma",
			text: `
	Fixes #13 #14, #15,#16,
`,
			expectedNumbers: []int{13, 14, 15, 16},
		},
		{
			name: "valid issue numbers space",
			text: `
	Fixes #13 #14 #15 #16
`,
			expectedNumbers: []int{13, 14, 15, 16},
		},
		{
			name: "invalid pattern",
			text: `
	Fixes #13#14,#15,#16,
`,
			expectedNumbers: nil,
		},
		{
			name: "french style",
			text: `
	Fixes : #13,#14,#15,#16,
`,
			expectedNumbers: []int{13, 14, 15, 16},
		},
		{
			name: "valid issue numbers coma and :",
			text: `
	Fixes: #13,#14,#15,#16,
`,
			expectedNumbers: []int{13, 14, 15, 16},
		},
	}

	mjolnir := newMjolnir(nil, "", "", true)

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			issueNumbers := mjolnir.parseIssueFixes(context.Background(), test.text)

			assert.Equal(t, test.expectedNumbers, issueNumbers)
		})
	}
}

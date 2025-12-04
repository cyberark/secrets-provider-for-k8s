package pushtofile

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// errorReader is a mock reader that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}

type pullFromReaderTestCase struct {
	description string
	content     string
	assert      func(*testing.T, string, error)
}

func (tc pullFromReaderTestCase) Run(t *testing.T) {
	t.Run(tc.description, func(t *testing.T) {
		buf := bytes.NewBufferString(tc.content)
		readContent, err := pullFromReader(buf)
		tc.assert(t, readContent, err)
	})
}

var pullFromReaderTestCases = []pullFromReaderTestCase{
	{
		description: "happy case",
		content:     "template file content",
		assert:      assertGoodOutput("template file content"),
	},
}

func TestPullFromReader(t *testing.T) {
	for _, tc := range pullFromReaderTestCases {
		tc.Run(t)
	}
}

func TestPullFromReaderWithError(t *testing.T) {
	t.Run("error case - reader returns error", func(t *testing.T) {
		// Use a mock reader that returns an error
		errReader := &errorReader{}
		content, err := pullFromReader(errReader)

		// Should return empty string and the error
		assert.Equal(t, "", content)
		assert.EqualError(t, err, "simulated read error")
	})
}

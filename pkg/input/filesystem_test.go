package input

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSystemInput(t *testing.T) {
	input, err := NewFSInput("testfiles/test1")
	require.NoError(t, err)
	entries, err := input.Root().Entries()
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assertContains(t, entries, "/test1.yaml", true)
	assertContains(t, entries, "/test2.yaml", true)
	assertContains(t, entries, "/testdir", false)
}

func assertContains(t *testing.T, entries []DirEntry, path string, file bool) {
	t.Helper()
	var found bool
	for _, e := range entries {
		if file {
			if f, ok := e.(*fsFile); ok {
				if f.Path() == path {
					found = true
					break
				}
			}
		} else {
			if d, ok := e.(*fsDir); ok {
				if d.Path() == path {
					found = true
					break
				}
			}
		}
	}
	assert.Truef(t, found, "%#v does not contain %s", entries, path)
}

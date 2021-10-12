package omitter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlinkOmitterHappyPath(t *testing.T) {
	tmpInputDir, err := os.MkdirTemp(os.TempDir(), "symlink-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpInputDir)
	}()

	omitter := NewSymlinkOmitter(tmpInputDir)

	textFile := filepath.Join(tmpInputDir, "link.txt")
	err = os.WriteFile(textFile, []byte("text"), 0644)
	require.NoError(t, err)

	linkPath := filepath.Join(tmpInputDir, "link")
	err = os.Symlink(textFile, linkPath)
	require.NoError(t, err)

	result, err := omitter.OmitPath("link")
	require.NoError(t, err)
	assert.Truef(t, result, "%s should've been omitted, but wasn't", linkPath)

	result, err = omitter.OmitPath("link.txt")
	require.NoError(t, err)
	assert.Falsef(t, result, "%s should've NOT been omitted, but was", textFile)
}

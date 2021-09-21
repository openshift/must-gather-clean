package watermarking

import (
	"os"
	"path/filepath"
	"testing"

	version "github.com/openshift/must-gather-clean/pkg/version"
	"github.com/stretchr/testify/require"
)

func TestSimpleWaterMarkingHappyPath(t *testing.T) {
	w := NewSimpleWaterMarker()

	tmpInputDir, err := os.MkdirTemp(os.TempDir(), "watermarker-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpInputDir)
	}()

	err = w.WriteWaterMarkFile(tmpInputDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(tmpInputDir, "watermark.txt"))
	data, err := os.ReadFile(filepath.Join(tmpInputDir, "watermark.txt"))
	require.NoError(t, err)
	require.Contains(t, string(data), version.GetVersion().Version)
}

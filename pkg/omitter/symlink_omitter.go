package omitter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/must-gather-clean/pkg/fsutil"
)

type symlinkOmitter struct {
	inputFolder string
}

func (s symlinkOmitter) OmitPath(path string) (bool, error) {
	stat, err := os.Lstat(filepath.Join(s.inputFolder, path))
	if err != nil {
		return false, fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	return fsutil.IsSymbolicLink(stat), nil
}

func NewSymlinkOmitter(inputFolder string) FileOmitter {
	return symlinkOmitter{inputFolder: inputFolder}
}

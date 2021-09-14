package omitter

import (
	"errors"
	"path/filepath"
)

type filePatternOmitter struct {
	filePattern string
}

func (f *filePatternOmitter) OmitPath(path string) (bool, error) {
	return filepath.Match(f.filePattern, path)
}

// NewFilenamePatternOmitter return an omitter which omits files based on a globbing pattern.
func NewFilenamePatternOmitter(pattern string) (FileOmitter, error) {
	if pattern == "" {
		return nil, errors.New("pattern for file omitter cannot be empty")
	}
	return &filePatternOmitter{filePattern: pattern}, nil
}

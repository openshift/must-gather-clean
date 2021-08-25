package omitter

import (
	"errors"
	"path/filepath"
)

type filePatternOmitter struct {
	filePattern string
}

func (f *filePatternOmitter) Omit(name, path string) (bool, error) {
	match, err := filepath.Match(f.filePattern, path)
	// if there is a match or an error return now
	if match || err != nil {
		return match, err
	}
	// else check for match with file name
	return filepath.Match(f.filePattern, name)
}

// NewFilenamePatternOmitter return an omitter which omits files based on a globbing pattern.
func NewFilenamePatternOmitter(pattern string) (FileOmitter, error) {
	if pattern == "" {
		return nil, errors.New("pattern for file omitter cannot be empty")
	}
	return &filePatternOmitter{filePattern: pattern}, nil
}

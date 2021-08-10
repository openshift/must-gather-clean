package omitter

import "path/filepath"

type fileOmitter struct {
	filePattern string
}

func (f *fileOmitter) File(filename, _ string) (bool, error) {
	return filepath.Match(f.filePattern, filename)
}

func (f *fileOmitter) Contents(_ string) (bool, error) {
	return false, nil
}

// NewFilenamePatternOmitter return an omitter which omits files based on a globbing pattern.
func NewFilenamePatternOmitter(pattern string) Omitter {
	return &fileOmitter{filePattern: pattern}
}

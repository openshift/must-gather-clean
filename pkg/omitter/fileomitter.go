package omitter

import "path/filepath"

type fileOmitter struct {
	filePattern string
}

func (f *fileOmitter) File(name, path string) (bool, error) {
	match, err := filepath.Match(f.filePattern, path)
	// if there is a match or an error return now
	if match || err != nil {
		return match, err
	}
	// else check for match with file name
	return filepath.Match(f.filePattern, name)
}

func (f *fileOmitter) Contents(_ string) (bool, error) {
	return false, nil
}

// NewFilenamePatternOmitter return an omitter which omits files based on a globbing pattern.
func NewFilenamePatternOmitter(pattern string) Omitter {
	return &fileOmitter{filePattern: pattern}
}

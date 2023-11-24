//go:build windows
// +build windows

package fsutil

import (
	"io/fs"
)

// no-op
func chown(path string, stat fs.FileInfo) error {
	return nil
}

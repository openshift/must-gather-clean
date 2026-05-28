//go:build !windows
// +build !windows

package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func chown(path string, stat fs.FileInfo) error {
	uid := stat.Sys().(*syscall.Stat_t).Uid
	gid := stat.Sys().(*syscall.Stat_t).Gid
	err := os.Chown(path, int(uid), int(gid))
	if err != nil {
		// Permission denied is expected for non-root users
		// The file is still created with correct permissions for the current user
		if errors.Is(err, syscall.EPERM) || errors.Is(err, os.ErrPermission) {
			return nil
		}
		return fmt.Errorf("failed to chown '%s' back to owner (%d, %d): %w", path, uid, gid, err)
	}
	return nil
}

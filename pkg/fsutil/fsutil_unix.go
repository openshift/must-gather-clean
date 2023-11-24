//go:build !windows
// +build !windows

package fsutil

import (
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
		return fmt.Errorf("failed to chown '%s' back to owner (%d, %d): %w", path, uid, gid, err)
	}
	return nil
}

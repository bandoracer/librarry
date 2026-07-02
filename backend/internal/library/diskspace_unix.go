//go:build unix

package library

import "syscall"

// freeSpaceBytes reports the bytes available to unprivileged users on the
// filesystem containing path.
func freeSpaceBytes(path string) (int64, bool) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, false
	}
	return int64(stat.Bavail) * int64(stat.Bsize), true
}

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

// probeDisk reports free/total bytes and the device id of the filesystem
// containing path. The device id (from stat, portable across unix flavors)
// lets callers dedupe paths that share a disk.
func probeDisk(path string) diskProbe {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return diskProbe{}
	}
	var fileStat syscall.Stat_t
	if err := syscall.Stat(path, &fileStat); err != nil {
		return diskProbe{}
	}
	return diskProbe{
		free:   int64(stat.Bavail) * int64(stat.Bsize),
		total:  int64(stat.Blocks) * int64(stat.Bsize),
		device: uint64(fileStat.Dev),
		ok:     true,
	}
}

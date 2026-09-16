//go:build !unix

package library

import (
	"errors"
	"io/fs"
)

// freeSpaceBytes is unsupported on this platform.
func freeSpaceBytes(string) (int64, bool) {
	return 0, false
}

// probeDisk is unsupported on this platform.
func probeDisk(string) diskProbe {
	return diskProbe{}
}

func scanRootIdentity(path string) (string, error) {
	return "", errors.New("durable scan root identity is unavailable on this platform")
}

func scanFileDevice(path string) (string, error) {
	return "", errors.New("scan device identity unavailable on this platform")
}

func scanFileStamp(fs.FileInfo) (string, error) {
	return "", errors.New("scan file identity unavailable on this platform")
}

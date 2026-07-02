//go:build !unix

package library

// freeSpaceBytes is unsupported on this platform.
func freeSpaceBytes(string) (int64, bool) {
	return 0, false
}

// probeDisk is unsupported on this platform.
func probeDisk(string) diskProbe {
	return diskProbe{}
}

package library

// DiskPath is a labelled filesystem location whose disk usage should be
// reported (library roots and the book torrent root).
type DiskPath struct {
	Path  string
	Label string
}

// DiskSpace reports usage for the filesystem containing a path.
type DiskSpace struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	FreeBytes  int64  `json:"freeBytes"`
	TotalBytes int64  `json:"totalBytes"`
}

// diskProbe is the per-OS statfs result. device identifies the backing
// filesystem so paths on the same disk collapse into one entry.
type diskProbe struct {
	free   int64
	total  int64
	device uint64
	ok     bool
}

// DiskSpaces resolves usage for each path, skipping paths that cannot be
// statted and deduplicating paths that live on the same filesystem (first
// label wins).
func DiskSpaces(paths []DiskPath) []DiskSpace {
	return diskSpacesWithProbe(paths, probeDisk)
}

func diskSpacesWithProbe(paths []DiskPath, probe func(string) diskProbe) []DiskSpace {
	seen := map[uint64]bool{}
	disks := make([]DiskSpace, 0, len(paths))
	for _, path := range paths {
		if path.Path == "" {
			continue
		}
		result := probe(path.Path)
		if !result.ok {
			continue
		}
		if seen[result.device] {
			continue
		}
		seen[result.device] = true
		disks = append(disks, DiskSpace{
			Path:       path.Path,
			Label:      path.Label,
			FreeBytes:  result.free,
			TotalBytes: result.total,
		})
	}
	return disks
}

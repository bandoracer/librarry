package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const recycleBinDayLayout = "2006-01-02"

// discardFile removes a library file, moving it into the configured recycle
// bin (<bin>/<yyyy-mm-dd>/<original-name>) when one is set. It degrades to a
// plain remove when the bin cannot be used (cross-filesystem copy failure,
// unwritable bin, ...), so deletes never wedge on recycle-bin problems.
func (s *Service) discardFile(path string) error {
	config := s.Config()
	return discardLibraryFile(config.RecycleBin, path, time.Now().UTC())
}

func discardLibraryFile(bin string, path string, now time.Time) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return errors.New("file path is required")
	}
	bin = filepath.Clean(strings.TrimSpace(bin))
	if bin == "" || bin == "." {
		return removeLibraryFile(path)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	day := filepath.Join(bin, now.Format(recycleBinDayLayout))
	if err := os.MkdirAll(day, 0o755); err != nil {
		return removeLibraryFile(path)
	}
	destination := availableDestination(filepath.Join(day, filepath.Base(path)))
	if err := os.Rename(path, destination); err == nil {
		return nil
	}
	// Rename fails across filesystems; fall back to copy + remove, and to a
	// plain remove when even the copy fails.
	if err := copyFile(path, destination); err == nil {
		return removeLibraryFile(path)
	}
	_ = os.Remove(destination)
	return removeLibraryFile(path)
}

// CleanupRecycleBin deletes recycle-bin day folders older than the configured
// retention. It returns the number of day folders removed.
func (s *Service) CleanupRecycleBin(now time.Time) (int, error) {
	config := s.Config()
	return cleanupRecycleBin(config.RecycleBin, config.RecycleBinRetention, now)
}

func cleanupRecycleBin(bin string, retention time.Duration, now time.Time) (int, error) {
	bin = filepath.Clean(strings.TrimSpace(bin))
	if bin == "" || bin == "." {
		return 0, nil
	}
	if retention <= 0 {
		retention = defaultRecycleBinRetention
	}
	entries, err := os.ReadDir(bin)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	var firstErr error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		day, err := time.Parse(recycleBinDayLayout, entry.Name())
		if err != nil {
			// Not one of our day folders; leave it alone.
			continue
		}
		// Entries land in the folder throughout its day; expire from day end.
		expiresAt := day.Add(24 * time.Hour).Add(retention)
		if expiresAt.After(now) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(bin, entry.Name())); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

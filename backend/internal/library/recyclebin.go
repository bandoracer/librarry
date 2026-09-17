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
// bin (<bin>/<yyyy-mm-dd>/<original-name>) when one is set. An unavailable
// configured bin retains the original and reports an error.
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
		return err
	}
	destination := availableDestination(filepath.Join(day, filepath.Base(path)))
	// Exclusive publication preserves a concurrent recycled file with the same
	// name. The copy helper checks bytes before the original is removed.
	if err := os.Link(path, destination); err != nil {
		if err := copyFile(path, destination); err != nil {
			return err
		}
	}
	if err := syncImportDirectory(day); err != nil {
		return err
	}
	if err := removeLibraryFile(path); err != nil {
		return err
	}
	return syncImportDirectory(filepath.Dir(path))
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

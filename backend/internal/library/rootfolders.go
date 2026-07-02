package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RootFolder is the native multi-root model (Readarr-style root folders with
// per-root defaults). It supersedes the two fixed config roots; those legacy
// fields keep working by mapping to the per-format default root.
type RootFolder struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	Path                     string    `json:"path"`
	MediaFormat              string    `json:"mediaFormat"`
	DefaultQualityProfile    string    `json:"defaultQualityProfile"`
	DefaultMissingBookPolicy string    `json:"defaultMissingBookPolicy"`
	DefaultTags              string    `json:"defaultTags"`
	IsDefault                bool      `json:"isDefault"`
	Accessible               bool      `json:"accessible"`
	FreeSpaceBytes           *int64    `json:"freeSpaceBytes,omitempty"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

// ConflictError marks operations refused because they would leave the library
// in a broken state (HTTP handlers map it to 409).
type ConflictError struct {
	Reason string
}

func (e *ConflictError) Error() string {
	return e.Reason
}

const rootFolderColumns = `
	id::text, name, path, media_format, default_quality_profile,
	default_missing_book_policy, default_tags, is_default, created_at, updated_at
`

func (s *Store) ListRootFolders(ctx context.Context) ([]RootFolder, error) {
	if !s.Configured() {
		return nil, errors.New("library store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select `+rootFolderColumns+`
		from root_folders
		order by media_format, created_at, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []RootFolder
	for rows.Next() {
		folder, err := scanRootFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, rows.Err()
}

func (s *Store) GetRootFolder(ctx context.Context, id string) (RootFolder, error) {
	if !s.Configured() {
		return RootFolder{}, errors.New("library store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return RootFolder{}, errors.New("root folder id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		select `+rootFolderColumns+`
		from root_folders
		where id::text = $1
	`, id)
	return scanRootFolder(row)
}

func (s *Store) CreateRootFolder(ctx context.Context, folder RootFolder) (RootFolder, error) {
	if !s.Configured() {
		return RootFolder{}, errors.New("library store is unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RootFolder{}, err
	}
	defer tx.Rollback()

	if folder.IsDefault {
		if _, err := tx.ExecContext(ctx, `
			update root_folders set is_default = false, updated_at = now()
			where media_format = $1 and is_default = true
		`, folder.MediaFormat); err != nil {
			return RootFolder{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `
		insert into root_folders (
			name, path, media_format, default_quality_profile,
			default_missing_book_policy, default_tags, is_default
		) values ($1, $2, $3, $4, $5, $6, $7)
		returning `+rootFolderColumns+`
	`, folder.Name, folder.Path, folder.MediaFormat, folder.DefaultQualityProfile,
		folder.DefaultMissingBookPolicy, folder.DefaultTags, folder.IsDefault)
	created, err := scanRootFolder(row)
	if err != nil {
		return RootFolder{}, err
	}
	if err := tx.Commit(); err != nil {
		return RootFolder{}, err
	}
	return created, nil
}

func (s *Store) UpdateRootFolder(ctx context.Context, id string, folder RootFolder) (RootFolder, error) {
	if !s.Configured() {
		return RootFolder{}, errors.New("library store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return RootFolder{}, errors.New("root folder id is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RootFolder{}, err
	}
	defer tx.Rollback()

	if folder.IsDefault {
		if _, err := tx.ExecContext(ctx, `
			update root_folders set is_default = false, updated_at = now()
			where media_format = $1 and is_default = true and id::text <> $2
		`, folder.MediaFormat, id); err != nil {
			return RootFolder{}, err
		}
	}
	row := tx.QueryRowContext(ctx, `
		update root_folders set
			name = $2,
			path = $3,
			media_format = $4,
			default_quality_profile = $5,
			default_missing_book_policy = $6,
			default_tags = $7,
			is_default = $8,
			updated_at = now()
		where id::text = $1
		returning `+rootFolderColumns+`
	`, id, folder.Name, folder.Path, folder.MediaFormat, folder.DefaultQualityProfile,
		folder.DefaultMissingBookPolicy, folder.DefaultTags, folder.IsDefault)
	updated, err := scanRootFolder(row)
	if err != nil {
		return RootFolder{}, err
	}
	if err := tx.Commit(); err != nil {
		return RootFolder{}, err
	}
	return updated, nil
}

func (s *Store) DeleteRootFolder(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("library store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("root folder id is required")
	}
	result, err := s.db.ExecContext(ctx, `delete from root_folders where id::text = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountFilesUnderPath counts tracked library files stored at or below root.
func (s *Store) CountFilesUnderPath(ctx context.Context, root string) (int, error) {
	if !s.Configured() {
		return 0, errors.New("library store is unavailable")
	}
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" || root == "." {
		return 0, nil
	}
	pattern := escapeLikePattern(root) + "/%"
	var count int
	err := s.db.QueryRowContext(ctx, `
		select count(*) from files
		where path = $1 or path like $2 escape '\'
	`, root, pattern).Scan(&count)
	return count, err
}

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func scanRootFolder(row fileScanner) (RootFolder, error) {
	var folder RootFolder
	if err := row.Scan(
		&folder.ID, &folder.Name, &folder.Path, &folder.MediaFormat, &folder.DefaultQualityProfile,
		&folder.DefaultMissingBookPolicy, &folder.DefaultTags, &folder.IsDefault, &folder.CreatedAt, &folder.UpdatedAt,
	); err != nil {
		return RootFolder{}, err
	}
	return folder, nil
}

// ListRootFolders returns the native root folders, seeding the table from the
// legacy two-root config on first read so existing deployments migrate
// transparently.
func (s *Service) ListRootFolders(ctx context.Context) ([]RootFolder, error) {
	if !s.Available() {
		return nil, errors.New("library service requires database persistence")
	}
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return nil, err
	}
	if len(folders) == 0 {
		folders, err = s.seedRootFoldersFromConfig(ctx)
		if err != nil {
			return nil, err
		}
	}
	s.applyDefaultRootsToConfig(folders)
	return decorateRootFolders(folders), nil
}

func (s *Service) CreateRootFolder(ctx context.Context, folder RootFolder) (RootFolder, error) {
	if !s.Available() {
		return RootFolder{}, errors.New("library service requires database persistence")
	}
	folder, err := normalizeRootFolderInput(folder)
	if err != nil {
		return RootFolder{}, err
	}
	existing, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return RootFolder{}, err
	}
	if !folder.IsDefault && !rootFoldersHaveFormat(existing, folder.MediaFormat) {
		// First root of a format becomes the destination default automatically.
		folder.IsDefault = true
	}
	created, err := s.store.CreateRootFolder(ctx, folder)
	if err != nil {
		return RootFolder{}, err
	}
	s.refreshConfigRoots(ctx)
	return decorateRootFolder(created), nil
}

func (s *Service) UpdateRootFolder(ctx context.Context, id string, folder RootFolder) (RootFolder, error) {
	if !s.Available() {
		return RootFolder{}, errors.New("library service requires database persistence")
	}
	folder, err := normalizeRootFolderInput(folder)
	if err != nil {
		return RootFolder{}, err
	}
	updated, err := s.store.UpdateRootFolder(ctx, id, folder)
	if err != nil {
		return RootFolder{}, err
	}
	s.refreshConfigRoots(ctx)
	return decorateRootFolder(updated), nil
}

// DeleteRootFolder removes a root folder. It refuses (ConflictError) to delete
// the last root of a media format while tracked files still live under it.
func (s *Service) DeleteRootFolder(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("library service requires database persistence")
	}
	folder, err := s.store.GetRootFolder(ctx, id)
	if err != nil {
		return err
	}
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return err
	}
	sameFormat := 0
	for _, candidate := range folders {
		if normalizeFormat(candidate.MediaFormat) == normalizeFormat(folder.MediaFormat) {
			sameFormat++
		}
	}
	if sameFormat <= 1 {
		count, err := s.store.CountFilesUnderPath(ctx, folder.Path)
		if err != nil {
			return err
		}
		if count > 0 {
			return &ConflictError{Reason: fmt.Sprintf(
				"root folder %q is the last %s root and %d tracked files still point at it; add another %s root or remove the files first",
				folder.Path, folder.MediaFormat, count, folder.MediaFormat,
			)}
		}
	}
	if err := s.store.DeleteRootFolder(ctx, id); err != nil {
		return err
	}
	s.refreshConfigRoots(ctx)
	return nil
}

// SyncDefaultRootFolders maps the legacy two-root config onto the per-format
// default native roots so PUT /api/v1/library/config keeps working.
func (s *Service) SyncDefaultRootFolders(ctx context.Context, config Config) error {
	if !s.Available() {
		return nil
	}
	config = NormalizeConfig(config)
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return err
	}
	if len(folders) == 0 {
		_, err := s.seedRootFolders(ctx, config)
		return err
	}
	for _, target := range []struct {
		format string
		path   string
	}{
		{format: "ebook", path: config.EbookRoot},
		{format: "audiobook", path: config.AudiobookRoot},
	} {
		path := filepath.Clean(strings.TrimSpace(target.path))
		if path == "" || path == "." {
			continue
		}
		current, ok := defaultRootFolder(folders, target.format)
		if !ok {
			seeded := seedRootFolder(target.format, path)
			seeded.IsDefault = true
			if _, err := s.store.CreateRootFolder(ctx, seeded); err != nil {
				return err
			}
			continue
		}
		if filepath.Clean(current.Path) == path {
			continue
		}
		current.Path = path
		if _, err := s.store.UpdateRootFolder(ctx, current.ID, current); err != nil {
			return err
		}
	}
	s.refreshConfigRoots(ctx)
	return nil
}

// SyncConfigFromRootFolders refreshes the in-memory legacy roots from the
// native default root folders (startup convergence).
func (s *Service) SyncConfigFromRootFolders(ctx context.Context) error {
	if !s.Available() {
		return nil
	}
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return err
	}
	s.applyDefaultRootsToConfig(folders)
	return nil
}

func (s *Service) seedRootFoldersFromConfig(ctx context.Context) ([]RootFolder, error) {
	return s.seedRootFolders(ctx, s.Config())
}

func (s *Service) seedRootFolders(ctx context.Context, config Config) ([]RootFolder, error) {
	config = NormalizeConfig(config)
	for _, target := range []struct {
		format string
		path   string
	}{
		{format: "ebook", path: config.EbookRoot},
		{format: "audiobook", path: config.AudiobookRoot},
	} {
		path := filepath.Clean(strings.TrimSpace(target.path))
		if path == "" || path == "." {
			continue
		}
		folder := seedRootFolder(target.format, path)
		folder.IsDefault = true
		if _, err := s.store.CreateRootFolder(ctx, folder); err != nil {
			// Concurrent seeding can trip the unique path constraint; the
			// authoritative list below wins either way.
			continue
		}
	}
	return s.store.ListRootFolders(ctx)
}

func seedRootFolder(format string, path string) RootFolder {
	name := "Ebooks"
	if normalizeFormat(format) == "audiobook" {
		name = "Audiobooks"
	}
	return RootFolder{
		Name:        name,
		Path:        path,
		MediaFormat: normalizeFormat(format),
	}
}

func (s *Service) refreshConfigRoots(ctx context.Context) {
	if !s.Available() {
		return
	}
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return
	}
	s.applyDefaultRootsToConfig(folders)
}

func (s *Service) applyDefaultRootsToConfig(folders []RootFolder) {
	if s == nil || len(folders) == 0 {
		return
	}
	ebook := defaultRootFolderPath(folders, "ebook")
	audiobook := defaultRootFolderPath(folders, "audiobook")
	if ebook == "" && audiobook == "" {
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if ebook != "" {
		s.config.EbookRoot = ebook
	}
	if audiobook != "" {
		s.config.AudiobookRoot = audiobook
	}
}

func normalizeRootFolderInput(folder RootFolder) (RootFolder, error) {
	folder.Path = filepath.Clean(strings.TrimSpace(folder.Path))
	if folder.Path == "" || folder.Path == "." {
		return RootFolder{}, errors.New("root folder path is required")
	}
	format := normalizeFormat(folder.MediaFormat)
	if format == "" {
		format = "ebook"
	}
	if format != "ebook" && format != "audiobook" {
		return RootFolder{}, errors.New("root folder media format must be ebook or audiobook")
	}
	folder.MediaFormat = format
	folder.Name = strings.TrimSpace(folder.Name)
	if folder.Name == "" {
		folder.Name = filepath.Base(folder.Path)
	}
	folder.DefaultQualityProfile = strings.TrimSpace(folder.DefaultQualityProfile)
	folder.DefaultMissingBookPolicy = strings.TrimSpace(folder.DefaultMissingBookPolicy)
	folder.DefaultTags = strings.TrimSpace(folder.DefaultTags)
	return folder, nil
}

func rootFoldersHaveFormat(folders []RootFolder, format string) bool {
	format = normalizeFormat(format)
	for _, folder := range folders {
		if normalizeFormat(folder.MediaFormat) == format {
			return true
		}
	}
	return false
}

func defaultRootFolder(folders []RootFolder, format string) (RootFolder, bool) {
	format = normalizeFormat(format)
	for _, folder := range folders {
		if folder.IsDefault && normalizeFormat(folder.MediaFormat) == format && strings.TrimSpace(folder.Path) != "" {
			return folder, true
		}
	}
	for _, folder := range folders {
		if normalizeFormat(folder.MediaFormat) == format && strings.TrimSpace(folder.Path) != "" {
			return folder, true
		}
	}
	return RootFolder{}, false
}

func defaultRootFolderPath(folders []RootFolder, format string) string {
	folder, ok := defaultRootFolder(folders, format)
	if !ok {
		return ""
	}
	return filepath.Clean(strings.TrimSpace(folder.Path))
}

func rootFolderPathByID(folders []RootFolder, id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	for _, folder := range folders {
		if folder.ID == id && strings.TrimSpace(folder.Path) != "" {
			return filepath.Clean(strings.TrimSpace(folder.Path))
		}
	}
	return ""
}

// importRootPath picks the destination root for an import: the wanted item's
// explicit root when set, then the default native root for the format, then
// the legacy config roots.
func (s *Service) importRootPath(ctx context.Context, format string, rootFolderID string) string {
	if s.store.Configured() {
		folders, err := s.store.ListRootFolders(ctx)
		if err == nil && len(folders) > 0 {
			if path := rootFolderPathByID(folders, rootFolderID); path != "" {
				return path
			}
			if path := defaultRootFolderPath(folders, format); path != "" {
				return path
			}
		}
	}
	return s.legacyRootForFormat(format)
}

// renameRootForFile keeps a rename inside the native root the file already
// lives under, falling back to the format default root, then legacy config.
func (s *Service) renameRootForFile(ctx context.Context, file FileRecord) string {
	format := normalizeFormat(file.MediaFormat)
	if s.store.Configured() {
		folders, err := s.store.ListRootFolders(ctx)
		if err == nil && len(folders) > 0 {
			best := ""
			bestLength := -1
			for _, folder := range folders {
				path := filepath.Clean(strings.TrimSpace(folder.Path))
				if path == "" || path == "." {
					continue
				}
				if pathWithinRoot(file.Path, path) && len(path) > bestLength {
					best = path
					bestLength = len(path)
				}
			}
			if best != "" {
				return best
			}
			if path := defaultRootFolderPath(folders, format); path != "" {
				return path
			}
		}
	}
	return s.legacyRootForFormat(format)
}

// rootsForFormat lists every scan root for a format: all native roots of the
// format, or the legacy config root when none exist.
func (s *Service) rootsForFormat(ctx context.Context, format string) []string {
	format = normalizeFormat(format)
	var roots []string
	if s.store.Configured() {
		folders, err := s.store.ListRootFolders(ctx)
		if err == nil {
			for _, folder := range folders {
				if normalizeFormat(folder.MediaFormat) != format {
					continue
				}
				path := filepath.Clean(strings.TrimSpace(folder.Path))
				if path != "" && path != "." {
					roots = append(roots, path)
				}
			}
		}
	}
	if len(roots) == 0 {
		if legacy := s.legacyRootForFormat(format); legacy != "" {
			roots = append(roots, legacy)
		}
	}
	return roots
}

func (s *Service) legacyRootForFormat(format string) string {
	if normalizeFormat(format) == "audiobook" {
		return s.audiobookRoot()
	}
	return s.ebookRoot()
}

func decorateRootFolders(folders []RootFolder) []RootFolder {
	decorated := make([]RootFolder, 0, len(folders))
	for _, folder := range folders {
		decorated = append(decorated, decorateRootFolder(folder))
	}
	return decorated
}

func decorateRootFolder(folder RootFolder) RootFolder {
	folder.Accessible = false
	folder.FreeSpaceBytes = nil
	info, err := os.Stat(folder.Path)
	if err == nil && info.IsDir() {
		folder.Accessible = true
		if free, ok := freeSpaceBytes(folder.Path); ok {
			folder.FreeSpaceBytes = &free
		}
	}
	return folder
}

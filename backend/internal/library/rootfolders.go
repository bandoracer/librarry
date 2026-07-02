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
	ID                       string            `json:"id"`
	Name                     string            `json:"name"`
	Path                     string            `json:"path"`
	MediaFormat              string            `json:"mediaFormat"`
	DefaultQualityProfile    string            `json:"defaultQualityProfile"`
	DefaultMissingBookPolicy string            `json:"defaultMissingBookPolicy"`
	DefaultTags              string            `json:"defaultTags"`
	IsDefault                bool              `json:"isDefault"`
	Calibre                  RootFolderCalibre `json:"calibre"`
	Accessible               bool              `json:"accessible"`
	FreeSpaceBytes           *int64            `json:"freeSpaceBytes,omitempty"`
	CreatedAt                time.Time         `json:"createdAt"`
	UpdatedAt                time.Time         `json:"updatedAt"`
}

// RootFolderCalibre is the per-root Calibre Content Server connection. When
// Enabled, imports into the root hand the file to Calibre (add-book plus
// optional conversion) instead of the move/hardlink+naming path, and rename
// operations skip files under the root. The Calibre content server requires
// authentication, so host, port, username, and password are mandatory while
// enabled. Password is redacted to "" in API responses; a blank password on
// update keeps the stored credential (notification-target pattern).
type RootFolderCalibre struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	URLBase        string `json:"urlBase"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Library        string `json:"library"`
	ConvertFormats string `json:"convertFormats"`
	OutputProfile  string `json:"outputProfile"`
	UseSSL         bool   `json:"useSsl"`
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
	default_missing_book_policy, default_tags, is_default,
	use_calibre, calibre_host, calibre_port, calibre_url_base, calibre_username,
	calibre_password, calibre_library, calibre_convert_formats, calibre_output_profile,
	calibre_use_ssl, created_at, updated_at
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
			default_missing_book_policy, default_tags, is_default,
			use_calibre, calibre_host, calibre_port, calibre_url_base, calibre_username,
			calibre_password, calibre_library, calibre_convert_formats, calibre_output_profile,
			calibre_use_ssl
		) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		returning `+rootFolderColumns+`
	`, folder.Name, folder.Path, folder.MediaFormat, folder.DefaultQualityProfile,
		folder.DefaultMissingBookPolicy, folder.DefaultTags, folder.IsDefault,
		folder.Calibre.Enabled, folder.Calibre.Host, folder.Calibre.Port, folder.Calibre.URLBase,
		folder.Calibre.Username, folder.Calibre.Password, folder.Calibre.Library,
		folder.Calibre.ConvertFormats, folder.Calibre.OutputProfile, folder.Calibre.UseSSL)
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
			use_calibre = $9,
			calibre_host = $10,
			calibre_port = $11,
			calibre_url_base = $12,
			calibre_username = $13,
			calibre_password = $14,
			calibre_library = $15,
			calibre_convert_formats = $16,
			calibre_output_profile = $17,
			calibre_use_ssl = $18,
			updated_at = now()
		where id::text = $1
		returning `+rootFolderColumns+`
	`, id, folder.Name, folder.Path, folder.MediaFormat, folder.DefaultQualityProfile,
		folder.DefaultMissingBookPolicy, folder.DefaultTags, folder.IsDefault,
		folder.Calibre.Enabled, folder.Calibre.Host, folder.Calibre.Port, folder.Calibre.URLBase,
		folder.Calibre.Username, folder.Calibre.Password, folder.Calibre.Library,
		folder.Calibre.ConvertFormats, folder.Calibre.OutputProfile, folder.Calibre.UseSSL)
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
		&folder.DefaultMissingBookPolicy, &folder.DefaultTags, &folder.IsDefault,
		&folder.Calibre.Enabled, &folder.Calibre.Host, &folder.Calibre.Port, &folder.Calibre.URLBase,
		&folder.Calibre.Username, &folder.Calibre.Password, &folder.Calibre.Library,
		&folder.Calibre.ConvertFormats, &folder.Calibre.OutputProfile, &folder.Calibre.UseSSL,
		&folder.CreatedAt, &folder.UpdatedAt,
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
	if err := validateRootFolderCalibre(folder.Calibre); err != nil {
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
	// Blank (or redacted-echo) Calibre passwords mean "keep the stored
	// credential" (notification-target pattern).
	if folder.Calibre.Password == "" {
		if current, err := s.store.GetRootFolder(ctx, id); err == nil {
			folder.Calibre.Password = current.Calibre.Password
		}
	}
	if err := validateRootFolderCalibre(folder.Calibre); err != nil {
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
	folder.Calibre = normalizeRootFolderCalibre(folder.Calibre)
	return folder, nil
}

func normalizeRootFolderCalibre(calibre RootFolderCalibre) RootFolderCalibre {
	calibre.Host = strings.TrimSpace(calibre.Host)
	calibre.URLBase = strings.TrimSpace(calibre.URLBase)
	calibre.Username = strings.TrimSpace(calibre.Username)
	calibre.Library = strings.TrimSpace(calibre.Library)
	calibre.ConvertFormats = strings.TrimSpace(calibre.ConvertFormats)
	calibre.OutputProfile = strings.TrimSpace(calibre.OutputProfile)
	if calibre.Port < 0 {
		calibre.Port = 0
	}
	return calibre
}

// validateRootFolderCalibre enforces the Calibre content server connection
// requirements: the server always authenticates, so an enabled root needs
// host, port, username, and password.
func validateRootFolderCalibre(calibre RootFolderCalibre) error {
	if !calibre.Enabled {
		return nil
	}
	switch {
	case calibre.Host == "":
		return errors.New("calibre host is required when calibre is enabled")
	case calibre.Port <= 0:
		return errors.New("calibre port is required when calibre is enabled")
	case calibre.Username == "":
		return errors.New("calibre username is required when calibre is enabled")
	case calibre.Password == "":
		return errors.New("calibre password is required when calibre is enabled")
	}
	return nil
}

// RedactRootFolderSecrets blanks the stored Calibre password for API
// responses; a blank password on update means "keep the stored credential".
func RedactRootFolderSecrets(folder RootFolder) RootFolder {
	folder.Calibre.Password = ""
	return folder
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
	folder, ok := rootFolderByID(folders, id)
	if !ok {
		return ""
	}
	return filepath.Clean(strings.TrimSpace(folder.Path))
}

func rootFolderByID(folders []RootFolder, id string) (RootFolder, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return RootFolder{}, false
	}
	for _, folder := range folders {
		if folder.ID == id && strings.TrimSpace(folder.Path) != "" {
			return folder, true
		}
	}
	return RootFolder{}, false
}

// resolveImportRootFolder picks the destination native root folder for an
// import: the wanted item's explicit root when set, then the default root for
// the format. False means no native root applies (legacy config roots win).
func resolveImportRootFolder(folders []RootFolder, format string, rootFolderID string) (RootFolder, bool) {
	if folder, ok := rootFolderByID(folders, rootFolderID); ok {
		return folder, true
	}
	return defaultRootFolder(folders, format)
}

// calibreRootForPath finds the deepest Calibre-managed native root containing
// path; files under such roots are handed to Calibre on import and skipped by
// rename operations.
func calibreRootForPath(folders []RootFolder, path string) (RootFolder, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	var best RootFolder
	bestLength := -1
	for _, folder := range folders {
		if !folder.Calibre.Enabled {
			continue
		}
		root := filepath.Clean(strings.TrimSpace(folder.Path))
		if root == "" || root == "." || !pathWithinRoot(path, root) {
			continue
		}
		if len(root) > bestLength {
			best = folder
			bestLength = len(root)
		}
	}
	return best, bestLength >= 0
}

// nativeRootFolders lists the native root folders when the store is available
// (empty otherwise).
func (s *Service) nativeRootFolders(ctx context.Context) []RootFolder {
	if s == nil || !s.store.Configured() {
		return nil
	}
	folders, err := s.store.ListRootFolders(ctx)
	if err != nil {
		return nil
	}
	return folders
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

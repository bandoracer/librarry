package library

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// RemotePathMapping rewrites a download-client-reported path prefix to the
// path Librarry can reach it at (split-host and Docker setups). Host matches
// the download client name; an empty host matches every client.
type RemotePathMapping struct {
	ID           string    `json:"id"`
	Host         string    `json:"host"`
	RemotePrefix string    `json:"remotePrefix"`
	LocalPrefix  string    `json:"localPrefix"`
	CreatedAt    time.Time `json:"createdAt"`
}

const remotePathMappingColumns = `id::text, host, remote_prefix, local_prefix, created_at`

func (s *Store) ListRemotePathMappings(ctx context.Context) ([]RemotePathMapping, error) {
	if !s.Configured() {
		return nil, errors.New("library store is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, `
		select `+remotePathMappingColumns+`
		from remote_path_mappings
		order by created_at, remote_prefix
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mappings []RemotePathMapping
	for rows.Next() {
		mapping, err := scanRemotePathMapping(rows)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mapping)
	}
	return mappings, rows.Err()
}

func (s *Store) CreateRemotePathMapping(ctx context.Context, mapping RemotePathMapping) (RemotePathMapping, error) {
	if !s.Configured() {
		return RemotePathMapping{}, errors.New("library store is unavailable")
	}
	row := s.db.QueryRowContext(ctx, `
		insert into remote_path_mappings (host, remote_prefix, local_prefix)
		values ($1, $2, $3)
		returning `+remotePathMappingColumns+`
	`, mapping.Host, mapping.RemotePrefix, mapping.LocalPrefix)
	return scanRemotePathMapping(row)
}

func (s *Store) UpdateRemotePathMapping(ctx context.Context, id string, mapping RemotePathMapping) (RemotePathMapping, error) {
	if !s.Configured() {
		return RemotePathMapping{}, errors.New("library store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return RemotePathMapping{}, errors.New("remote path mapping id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		update remote_path_mappings set
			host = $2,
			remote_prefix = $3,
			local_prefix = $4
		where id::text = $1
		returning `+remotePathMappingColumns+`
	`, id, mapping.Host, mapping.RemotePrefix, mapping.LocalPrefix)
	return scanRemotePathMapping(row)
}

func (s *Store) DeleteRemotePathMapping(ctx context.Context, id string) error {
	if !s.Configured() {
		return errors.New("library store is unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("remote path mapping id is required")
	}
	result, err := s.db.ExecContext(ctx, `delete from remote_path_mappings where id::text = $1`, id)
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

func scanRemotePathMapping(row fileScanner) (RemotePathMapping, error) {
	var mapping RemotePathMapping
	if err := row.Scan(&mapping.ID, &mapping.Host, &mapping.RemotePrefix, &mapping.LocalPrefix, &mapping.CreatedAt); err != nil {
		return RemotePathMapping{}, err
	}
	return mapping, nil
}

func (s *Service) ListRemotePathMappings(ctx context.Context) ([]RemotePathMapping, error) {
	if !s.Available() {
		return nil, errors.New("library service requires database persistence")
	}
	return s.store.ListRemotePathMappings(ctx)
}

func (s *Service) CreateRemotePathMapping(ctx context.Context, mapping RemotePathMapping) (RemotePathMapping, error) {
	if !s.Available() {
		return RemotePathMapping{}, errors.New("library service requires database persistence")
	}
	mapping, err := normalizeRemotePathMappingInput(mapping)
	if err != nil {
		return RemotePathMapping{}, err
	}
	return s.store.CreateRemotePathMapping(ctx, mapping)
}

func (s *Service) UpdateRemotePathMapping(ctx context.Context, id string, mapping RemotePathMapping) (RemotePathMapping, error) {
	if !s.Available() {
		return RemotePathMapping{}, errors.New("library service requires database persistence")
	}
	mapping, err := normalizeRemotePathMappingInput(mapping)
	if err != nil {
		return RemotePathMapping{}, err
	}
	return s.store.UpdateRemotePathMapping(ctx, id, mapping)
}

func (s *Service) DeleteRemotePathMapping(ctx context.Context, id string) error {
	if !s.Available() {
		return errors.New("library service requires database persistence")
	}
	return s.store.DeleteRemotePathMapping(ctx, id)
}

func normalizeRemotePathMappingInput(mapping RemotePathMapping) (RemotePathMapping, error) {
	mapping.Host = strings.TrimSpace(mapping.Host)
	mapping.RemotePrefix = strings.TrimSpace(mapping.RemotePrefix)
	mapping.LocalPrefix = strings.TrimSpace(mapping.LocalPrefix)
	if mapping.RemotePrefix == "" {
		return RemotePathMapping{}, errors.New("remote path prefix is required")
	}
	if mapping.LocalPrefix == "" {
		return RemotePathMapping{}, errors.New("local path prefix is required")
	}
	return mapping, nil
}

// remotePathMappings loads the mapping table, tolerating an unavailable store
// (path rewriting is best-effort and never blocks an import tick).
func (s *Service) remotePathMappings(ctx context.Context) []RemotePathMapping {
	if s == nil || !s.store.Configured() {
		return nil
	}
	mappings, err := s.store.ListRemotePathMappings(ctx)
	if err != nil {
		return nil
	}
	return mappings
}

// rewriteRemotePath applies the longest matching remote prefix (empty mapping
// host matches every client) as a dumb prefix rewrite.
func rewriteRemotePath(path string, host string, mappings []RemotePathMapping) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || len(mappings) == 0 {
		return path
	}
	host = strings.ToLower(strings.TrimSpace(host))
	best := RemotePathMapping{}
	bestLength := -1
	for _, mapping := range mappings {
		remote := normalizeRemotePrefix(mapping.RemotePrefix)
		if remote == "" {
			continue
		}
		mappingHost := strings.ToLower(strings.TrimSpace(mapping.Host))
		if mappingHost != "" && mappingHost != host {
			continue
		}
		if !pathHasPrefix(trimmed, remote) {
			continue
		}
		if len(remote) > bestLength {
			best = mapping
			bestLength = len(remote)
		}
	}
	if bestLength < 0 {
		return path
	}
	suffix := trimmed[bestLength:]
	suffix = strings.ReplaceAll(suffix, `\`, "/")
	local := strings.TrimRight(strings.TrimSpace(best.LocalPrefix), `/\`)
	if suffix == "" {
		if local == "" {
			return "/"
		}
		return local
	}
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	return local + suffix
}

// normalizeRemotePrefix trims trailing separators so prefix matching works the
// same whether the operator typed "/remote/downloads" or "/remote/downloads/".
func normalizeRemotePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	trimmed := strings.TrimRight(prefix, `/\`)
	if trimmed == "" {
		// The prefix was only separators (e.g. "/"); keep a single one.
		return prefix[:1]
	}
	return trimmed
}

func pathHasPrefix(path string, prefix string) bool {
	if path == prefix {
		return true
	}
	if prefix == "/" || prefix == `\` {
		return strings.HasPrefix(path, prefix)
	}
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	next := path[len(prefix)]
	return next == '/' || next == '\\'
}

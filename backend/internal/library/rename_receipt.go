package library

import (
	"context"
	"errors"
)

// Import manifests remain immutable. Follow only later committed renames for
// the same file identity and bytes, then require the current record to agree.
// An arbitrary database path edit is not proof that an import is still intact.
func (s *Service) currentManifestDestination(ctx context.Context, op ImportOperation, f ImportOperationFile) (string, error) {
	if f.Format == "sidecar" {
		return f.DestinationPath, nil
	}
	if f.FileID == "" {
		return "", errors.New("committed import lost its library record")
	}
	var path, checksum string
	var size int64
	if err := s.store.db.QueryRowContext(ctx, `select path,coalesce(checksum,''),coalesce(size_bytes,0) from files where id=$1`, f.FileID).Scan(&path, &checksum, &size); err != nil {
		return "", err
	}
	if checksum != f.SHA256 || size != f.SizeBytes {
		return "", errors.New("import file evidence changed")
	}
	resolved := f.DestinationPath
	rows, err := s.store.db.QueryContext(ctx, `select m.source_path,m.destination_path,m.sha256,m.size_bytes from import_operations o join import_operation_files m on m.operation_id=o.id where o.source_kind='manual' and o.metadata ? 'renameFileId' and o.metadata->>'renameFileId'=$1 and o.state='committed' and o.created_at>$2 order by o.created_at,o.id`, f.FileID, op.CreatedAt)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var source, destination, hash string
		var bytes int64
		if err := rows.Scan(&source, &destination, &hash, &bytes); err != nil {
			return "", err
		}
		if source != resolved || hash != f.SHA256 || bytes != f.SizeBytes {
			return "", errors.New("rename history no longer proves this import")
		}
		resolved = destination
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if path != resolved {
		return "", errors.New("import file location changed without a verified rename")
	}
	return resolved, nil
}

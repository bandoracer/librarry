package library

import (
	"context"
	"errors"
)

// Import manifests remain immutable. Follow only later committed renames for
// the same file identity and bytes, then require the current record to agree.
// An arbitrary database path edit is not proof that an import is still intact.
func (s *Service) currentManifestDestination(ctx context.Context, op ImportOperation, f ImportOperationFile) (string, error) {
	if f.Format != "sidecar" && f.FileID == "" {
		return "", errors.New("committed import lost its library record")
	}
	path := ""
	if f.Format != "sidecar" {
		var checksum string
		var size int64
		if err := s.store.db.QueryRowContext(ctx, `select path,coalesce(checksum,''),coalesce(size_bytes,0) from files where id=$1`, f.FileID).Scan(&path, &checksum, &size); err != nil {
			return "", err
		}
		if checksum != f.SHA256 || size != f.SizeBytes {
			return "", errors.New("import file evidence changed")
		}
	}
	origin := firstNonEmpty(f.RenameOriginFileID, f.ID)
	resolved := f.DestinationPath
	rows, err := s.store.db.QueryContext(ctx, `select source_path,destination_path,sha256,size_bytes from (
      select m.source_path,m.destination_path,m.sha256,m.size_bytes,coalesce(o.committed_at,o.created_at) as moved_at,o.id::text as event_id
      from import_operations o join import_operation_files m on m.operation_id=o.id
      where o.source_kind='manual' and o.metadata ? 'renameFileId'
       and ((m.file_id=nullif($1,'')::uuid and m.media_format<>'sidecar') or (m.media_format='sidecar' and m.rename_origin_file_id=nullif($2,'')::uuid))
       and o.state='committed' and o.created_at>$3
      union all
      select previous_path,current_path,sha256,size_bytes,created_at,job_id::text from library_scan_moves where file_id=nullif($1,'')::uuid and created_at>$3
     ) moves order by moved_at,event_id`, f.FileID, origin, op.CreatedAt)
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
			return "", errors.New("recorded relocation history no longer proves this import")
		}
		resolved = destination
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if f.Format != "sidecar" && path != resolved {
		return "", errors.New("import file location changed without a verified relocation")
	}
	return resolved, nil
}

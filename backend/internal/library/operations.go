package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

var ErrImportBusy = errors.New("this import is already being processed")

type ImportOperation struct {
	SourceKind              string                `json:"sourceKind"`
	RequestKey              string                `json:"-"`
	ID                      string                `json:"id"`
	DownloadRecordID        string                `json:"downloadRecordId"`
	Client                  string                `json:"client"`
	DownloadID              string                `json:"downloadId"`
	WantedID                string                `json:"wantedId"`
	SourceRoot              string                `json:"sourceRoot"`
	DestinationRoot         string                `json:"destinationRoot"`
	Format                  string                `json:"format"`
	Mode                    string                `json:"mode"`
	State                   string                `json:"state"`
	CleanupState            string                `json:"cleanupState"`
	LastError               string                `json:"lastError,omitempty"`
	ReplacementCleanupState string                `json:"replacementCleanupState"`
	ReplacementCleanupError string                `json:"replacementCleanupError,omitempty"`
	CleanupError            string                `json:"cleanupError,omitempty"`
	Attempts                int                   `json:"attempts"`
	Metadata                map[string]any        `json:"metadata"`
	Files                   []ImportOperationFile `json:"files"`
	CreatedAt               time.Time             `json:"createdAt"`
	UpdatedAt               time.Time             `json:"updatedAt"`
	LeaseToken              string                `json:"-"`
}

type ImportOperationFile struct {
	PreviousPath      string `json:"previousPath,omitempty"`
	PreviousSHA256    string `json:"previousSha256,omitempty"`
	PreviousSizeBytes int64  `json:"previousSizeBytes,omitempty"`
	SourceRemoved     bool   `json:"sourceRemoved,omitempty"`
	StagePath         string `json:"stagePath,omitempty"`
	StageLeaseToken   string `json:"-"`
	WantedID          string `json:"wantedId,omitempty"`
	ID                string `json:"id"`
	Order             int    `json:"order"`
	RelativePath      string `json:"relativePath"`
	SourcePath        string `json:"sourcePath"`
	DestinationPath   string `json:"destinationPath"`
	SizeBytes         int64  `json:"sizeBytes"`
	SHA256            string `json:"sha256"`
	Format            string `json:"format"`
	Required          bool   `json:"required"`
	State             string `json:"state"`
	FileID            string `json:"fileId,omitempty"`
}

func (s *Store) operationForDownload(ctx context.Context, client, externalID string) (ImportOperation, error) {
	var id string
	var count int
	err := s.db.QueryRowContext(ctx, `select io.id::text,(select count(*) from downloads where lower(client)=lower($1) and external_id=$2) from import_operations io join downloads d on d.id=io.download_record_id where lower(d.client)=lower($1) and d.external_id=$2`, client, externalID).Scan(&id, &count)
	if err == nil && count != 1 {
		return ImportOperation{}, errors.New("ambiguous download identity")
	}
	if err != nil {
		return ImportOperation{}, err
	}
	return s.getOperation(ctx, id)
}

func (s *Store) getOperation(ctx context.Context, id string) (ImportOperation, error) {
	var op ImportOperation
	var raw []byte
	err := s.db.QueryRowContext(ctx, `select io.id::text,coalesce(io.download_record_id::text,''),coalesce(d.client,''),coalesce(d.external_id,''),coalesce(io.wanted_item_id::text,''),
      io.source_root,io.destination_root,io.media_format,io.import_mode,io.state,io.cleanup_state,io.last_error,io.cleanup_error,
      io.attempts,io.metadata,io.created_at,io.updated_at,io.source_kind,io.request_key,io.replacement_cleanup_state,io.replacement_cleanup_error
      from import_operations io left join downloads d on d.id=io.download_record_id where io.id=$1`, id).Scan(
		&op.ID, &op.DownloadRecordID, &op.Client, &op.DownloadID, &op.WantedID, &op.SourceRoot, &op.DestinationRoot, &op.Format, &op.Mode, &op.State, &op.CleanupState, &op.LastError, &op.CleanupError, &op.Attempts, &raw, &op.CreatedAt, &op.UpdatedAt, &op.SourceKind, &op.RequestKey, &op.ReplacementCleanupState, &op.ReplacementCleanupError)
	if err != nil {
		return op, err
	}
	if err = json.Unmarshal(raw, &op.Metadata); err != nil {
		return op, err
	}
	rows, err := s.db.QueryContext(ctx, `select id::text,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,required,state,coalesce(file_id::text,''),coalesce(wanted_item_id::text,''),stage_path,coalesce(stage_lease_token::text,''),previous_path,previous_sha256,previous_size_bytes,source_removed from import_operation_files where operation_id=$1 order by file_order,id`, id)
	if err != nil {
		return op, err
	}
	defer rows.Close()
	op.Files = []ImportOperationFile{}
	for rows.Next() {
		var f ImportOperationFile
		if err := rows.Scan(&f.ID, &f.Order, &f.RelativePath, &f.SourcePath, &f.DestinationPath, &f.SizeBytes, &f.SHA256, &f.Format, &f.Required, &f.State, &f.FileID, &f.WantedID, &f.StagePath, &f.StageLeaseToken, &f.PreviousPath, &f.PreviousSHA256, &f.PreviousSizeBytes, &f.SourceRemoved); err != nil {
			return op, err
		}
		op.Files = append(op.Files, f)
	}
	return op, rows.Err()
}

func (s *Store) planOperation(ctx context.Context, op ImportOperation) (ImportOperation, error) {
	if len(op.Files) == 0 {
		return op, errors.New("import manifest is empty")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return op, err
	}
	defer tx.Rollback()
	if op.SourceKind == "" {
		op.SourceKind = "completed"
	}
	if op.isRename() {
		existing, e := reserveRenameFile(ctx, tx, op)
		if e != nil {
			return op, e
		}
		if existing != "" {
			if e = tx.Commit(); e != nil {
				return op, e
			}
			return s.getOperation(ctx, existing)
		}
	}
	var downloadID sql.NullString
	var existing string
	if op.SourceKind == "manual" {
		if op.RequestKey == "" {
			return op, errors.New("manual import requires request identity")
		}
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,2))`, op.RequestKey); err != nil {
			return op, err
		}
		err = tx.QueryRowContext(ctx, `select id::text from import_operations where source_kind='manual' and request_key=$1`, op.RequestKey).Scan(&existing)
	} else {
		var count int
		if err := tx.QueryRowContext(ctx, `select count(*),(array_agg(id::text order by id))[1] from downloads where lower(client)=lower($1) and external_id=$2`, op.Client, op.DownloadID).Scan(&count, &downloadID); err != nil {
			return op, err
		}
		if count != 1 {
			return op, errors.New("download identity is missing or ambiguous; retain source for review")
		}
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, downloadID.String); err != nil {
			return op, err
		}
		err = tx.QueryRowContext(ctx, `select id::text from import_operations where download_record_id=$1`, downloadID.String).Scan(&existing)
	}
	if err == nil {
		if err := tx.Commit(); err != nil {
			return op, err
		}
		return s.getOperation(ctx, existing)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return op, err
	}

	if op.Metadata == nil {
		op.Metadata = map[string]any{}
	}
	// Reserve paths across operations before any filesystem publication. All
	// contenders acquire locks in sorted order to avoid multi-file deadlocks.
	paths := make([]string, 0, len(op.Files))
	manifestByPath := map[string]ImportOperationFile{}
	for _, f := range op.Files {
		paths = append(paths, f.DestinationPath)
		if op.isRename() {
			paths = append(paths, f.SourcePath)
		}
		manifestByPath[f.DestinationPath] = f
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,1))`, path); err != nil {
			return op, err
		}
		if op.isRename() && path == op.Files[0].SourcePath {
			continue
		}
		var occupied bool
		if err := tx.QueryRowContext(ctx, `select exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id where f.destination_path=$1 and (o.state<>'committed' or not $2) and not($3<>'' and f.file_id=nullif($3,'')::uuid and o.state='committed' and (o.source_kind<>'manual' or o.cleanup_state='cleaned') and o.replacement_cleanup_state<>'pending')) or (not $2 and exists(select 1 from files where path=$1)) or exists(select 1 from import_operation_files f join import_operations o on o.id=f.operation_id where f.source_path=$1 and o.source_kind='manual' and o.metadata ? 'renameFileId' and (o.state<>'committed' or o.cleanup_state<>'cleaned'))`, path, (op.SourceKind != "manual" && op.Metadata["conflictAction"] == "replace") || manifestByPath[path].PreviousPath != "" || (op.SourceKind == "manual" && manifestByPath[path].SourcePath == path), metadataString(op.Metadata, "renameFileId")).Scan(&occupied); err != nil {
			return op, err
		}
		if occupied {
			return op, errors.New("destination belongs to an existing import; choose another destination")
		}
	}
	for _, file := range op.Files {
		if op.SourceKind != "manual" && file.PreviousPath != "" {
			op.ReplacementCleanupState = "pending"
		}
	}
	if op.ReplacementCleanupState == "" {
		op.ReplacementCleanupState = "none"
	}
	raw, err := json.Marshal(op.Metadata)
	if err != nil {
		return op, err
	}
	err = tx.QueryRowContext(ctx, `insert into import_operations(download_record_id,wanted_item_id,source_root,destination_root,media_format,import_mode,metadata,source_kind,request_key,replacement_cleanup_state)
      values($1,nullif($2,'')::uuid,$3,$4,$5,$6,$7::jsonb,$8,$9,$10) returning id::text`, downloadID, op.WantedID, op.SourceRoot, op.DestinationRoot, op.Format, op.Mode, string(raw), op.SourceKind, op.RequestKey, op.ReplacementCleanupState).Scan(&op.ID)
	if err != nil {
		return op, err
	}
	for _, f := range op.Files {
		if _, err := tx.ExecContext(ctx, `insert into import_operation_files(operation_id,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,required,wanted_item_id,previous_path,previous_sha256,previous_size_bytes,file_id)
          values($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,'')::uuid,$11,$12,$13,nullif($14,'')::uuid)`, op.ID, f.Order, f.RelativePath, f.SourcePath, f.DestinationPath, f.SizeBytes, f.SHA256, f.Format, f.Required, f.WantedID, f.PreviousPath, f.PreviousSHA256, f.PreviousSizeBytes, f.FileID); err != nil {
			return op, err
		}
	}
	if err := tx.Commit(); err != nil {
		return op, err
	}
	return s.getOperation(ctx, op.ID)
}

func (s *Store) claimOperation(ctx context.Context, id string) (string, error) {
	var token string
	err := s.db.QueryRowContext(ctx, `update import_operations set lease_token=gen_random_uuid(),lease_expires_at=now()+interval '2 minutes',attempts=attempts+1,state='transferring',last_error='',updated_at=now()
      where id=$1 and state<>'committed' and (lease_expires_at is null or lease_expires_at<now()) returning lease_token::text`, id).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrImportBusy
	}
	return token, err
}

func (s *Store) renewOperation(ctx context.Context, id, token string) error {
	result, err := s.db.ExecContext(ctx, `update import_operations set lease_expires_at=now()+interval '2 minutes' where id=$1 and lease_token=$2 and lease_expires_at>now()`, id, token)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrImportBusy
	}
	return err
}

func (s *Store) failOperation(ctx context.Context, id, token, message string) {
	_, _ = s.db.ExecContext(ctx, `update import_operations set state='failed',last_error=$3,cleanup_state='blocked',lease_token=null,lease_expires_at=null,updated_at=now() where id=$1 and lease_token=$2 and state<>'committed'`, id, token, message)
}

func (s *Store) markOperationFile(ctx context.Context, op ImportOperation, fileID string) error {
	result, err := s.db.ExecContext(ctx, `update import_operation_files set state='verified',updated_at=now() where id=$1 and operation_id=$2 and exists(select 1 from import_operations where id=$2 and lease_token=$3 and lease_expires_at>now())`, fileID, op.ID, op.LeaseToken)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrImportBusy
	}
	return err
}

// commitOperation is the visibility boundary: files, associations, wanted status,
// download projection and operation state commit together after every hash passes.
func (s *Store) commitOperation(ctx context.Context, op ImportOperation, records []FileRecord) ([]FileRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var valid bool
	err = tx.QueryRowContext(ctx, `select coalesce(lease_token=$2::uuid and lease_expires_at>now(),false) from import_operations where id=$1 for update`, op.ID, op.LeaseToken).Scan(&valid)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, ErrImportBusy
	}
	var outstanding int
	if err := tx.QueryRowContext(ctx, `select count(*) from import_operation_files where operation_id=$1 and (state<>'verified' or stage_path<>'')`, op.ID).Scan(&outstanding); err != nil {
		return nil, err
	}
	if outstanding != 0 || len(records) == 0 {
		return nil, errors.New("manifest is not completely verified")
	}
	if op.SourceKind != "manual" {
		for _, file := range op.Files {
			if op.Metadata["conflictAction"] == "replace" {
				if err := fenceCompletedReplacementOwner(ctx, tx, file); err != nil {
					return nil, err
				}
			}
		}
	}
	// The visibility trigger sees this transaction's state, while scanners
	// continue seeing the unfinished operation until the whole transaction commits.
	if _, err := tx.ExecContext(ctx, `update import_operations set state='committed' where id=$1`, op.ID); err != nil {
		return nil, err
	}
	if op.isRename() {
		stored, e := commitRenameFile(ctx, tx, op)
		if e != nil {
			return nil, e
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
		return stored, nil
	}
	stored := make([]FileRecord, 0, len(records))
	for _, record := range records {
		wantedID, _ := record.Metadata["wantedId"].(string)
		if wantedID == "" && op.SourceKind != "manual" {
			return nil, errors.New("manifest book association is missing")
		}
		if op.SourceKind != "manual" && op.Metadata["conflictAction"] == "replace" {
			var err error
			record, err = preserveCompletedReplacementMetadata(ctx, tx, record)
			if err != nil {
				return nil, err
			}
		}
		file, err := persistFile(ctx, tx, record, false)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `update files set presence_state='present' where id=$1`, file.ID); err != nil {
			return nil, err
		}
		file.PresenceState = "present"
		result, err := tx.ExecContext(ctx, `update import_operation_files set file_id=$3,state='committed',updated_at=now() where operation_id=$1 and destination_path=$2 and media_format<>'sidecar' and coalesce(wanted_item_id::text,'')=$4`, op.ID, file.Path, file.ID, wantedID)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return nil, errors.New("file does not match import manifest")
		}
		if wantedID != "" {
			if _, err := tx.ExecContext(ctx, `insert into file_wanted_links(file_id,wanted_item_id) values($1,$2) on conflict do nothing`, file.ID, wantedID); err != nil {
				return nil, err
			}
		}
		if op.DownloadRecordID != "" {
			if _, err := tx.ExecContext(ctx, `insert into file_download_links(file_id,download_record_id) values($1,$2) on conflict do nothing`, file.ID, op.DownloadRecordID); err != nil {
				return nil, err
			}
		}
		stored = append(stored, file)
	}
	if err := tx.QueryRowContext(ctx, `select count(*) from import_operation_files where operation_id=$1 and media_format<>'sidecar' and file_id is null`, op.ID).Scan(&outstanding); err != nil {
		return nil, err
	}
	if outstanding != 0 {
		return nil, errors.New("some required book files are not registered")
	}
	if _, err := tx.ExecContext(ctx, `update import_operation_files set state='committed',updated_at=now() where operation_id=$1`, op.ID); err != nil {
		return nil, err
	}
	importedBooks := map[string]string{}
	for _, record := range records {
		id, _ := record.Metadata["wantedId"].(string)
		if id != "" {
			importedBooks[id] = record.MediaFormat
		}
	}
	for id, format := range importedBooks {
		result, err := tx.ExecContext(ctx, `update wanted_items set status='imported',updated_at=now() where id=$1 and status not in ('removed','ignored') and wanted_format in ('any',$2)`, id, format)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return nil, errors.New("book was removed or its format changed while importing")
		}
	}
	if err := commitImportBookkeeping(ctx, tx, op, stored, importedBooks); err != nil {
		return nil, err
	}
	if op.DownloadRecordID != "" {
		if _, err := tx.ExecContext(ctx, `update downloads set import_status='imported',imported_file_id=$2,imported_at=now(),import_error='',updated_at=now() where id=$1`, op.DownloadRecordID, stored[0].ID); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `update import_operations set state='committed',committed_at=now(),lease_token=null,lease_expires_at=null,last_error='',updated_at=now() where id=$1`, op.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import records: %w", err)
	}
	return stored, nil
}

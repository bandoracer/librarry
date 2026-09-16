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
	ID               string                `json:"id"`
	DownloadRecordID string                `json:"downloadRecordId"`
	Client           string                `json:"client"`
	DownloadID       string                `json:"downloadId"`
	WantedID         string                `json:"wantedId"`
	SourceRoot       string                `json:"sourceRoot"`
	DestinationRoot  string                `json:"destinationRoot"`
	Format           string                `json:"format"`
	Mode             string                `json:"mode"`
	State            string                `json:"state"`
	CleanupState     string                `json:"cleanupState"`
	LastError        string                `json:"lastError,omitempty"`
	CleanupError     string                `json:"cleanupError,omitempty"`
	Attempts         int                   `json:"attempts"`
	Metadata         map[string]any        `json:"metadata"`
	Files            []ImportOperationFile `json:"files"`
	CreatedAt        time.Time             `json:"createdAt"`
	UpdatedAt        time.Time             `json:"updatedAt"`
	LeaseToken       string                `json:"-"`
}

type ImportOperationFile struct {
	StagePath       string `json:"stagePath,omitempty"`
	StageLeaseToken string `json:"-"`
	WantedID        string `json:"wantedId,omitempty"`
	ID              string `json:"id"`
	Order           int    `json:"order"`
	RelativePath    string `json:"relativePath"`
	SourcePath      string `json:"sourcePath"`
	DestinationPath string `json:"destinationPath"`
	SizeBytes       int64  `json:"sizeBytes"`
	SHA256          string `json:"sha256"`
	Format          string `json:"format"`
	Required        bool   `json:"required"`
	State           string `json:"state"`
	FileID          string `json:"fileId,omitempty"`
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
	err := s.db.QueryRowContext(ctx, `select io.id::text,io.download_record_id::text,d.client,d.external_id,io.wanted_item_id::text,
      io.source_root,io.destination_root,io.media_format,io.import_mode,io.state,io.cleanup_state,io.last_error,io.cleanup_error,
      io.attempts,io.metadata,io.created_at,io.updated_at
      from import_operations io join downloads d on d.id=io.download_record_id where io.id=$1`, id).Scan(
		&op.ID, &op.DownloadRecordID, &op.Client, &op.DownloadID, &op.WantedID, &op.SourceRoot, &op.DestinationRoot, &op.Format, &op.Mode, &op.State, &op.CleanupState, &op.LastError, &op.CleanupError, &op.Attempts, &raw, &op.CreatedAt, &op.UpdatedAt)
	if err != nil {
		return op, err
	}
	if err = json.Unmarshal(raw, &op.Metadata); err != nil {
		return op, err
	}
	rows, err := s.db.QueryContext(ctx, `select id::text,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,required,state,coalesce(file_id::text,''),coalesce(wanted_item_id::text,''),stage_path,coalesce(stage_lease_token::text,'') from import_operation_files where operation_id=$1 order by file_order,id`, id)
	if err != nil {
		return op, err
	}
	defer rows.Close()
	op.Files = []ImportOperationFile{}
	for rows.Next() {
		var f ImportOperationFile
		if err := rows.Scan(&f.ID, &f.Order, &f.RelativePath, &f.SourcePath, &f.DestinationPath, &f.SizeBytes, &f.SHA256, &f.Format, &f.Required, &f.State, &f.FileID, &f.WantedID, &f.StagePath, &f.StageLeaseToken); err != nil {
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
	var count int
	var downloadID sql.NullString
	if err := tx.QueryRowContext(ctx, `select count(*),(array_agg(id::text order by id))[1] from downloads where lower(client)=lower($1) and external_id=$2`, op.Client, op.DownloadID).Scan(&count, &downloadID); err != nil {
		return op, err
	}
	if count != 1 {
		return op, errors.New("download identity is missing or ambiguous; retain source for review")
	}
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, downloadID.String); err != nil {
		return op, err
	}
	var existing string
	err = tx.QueryRowContext(ctx, `select id::text from import_operations where download_record_id=$1`, downloadID.String).Scan(&existing)
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
	for _, f := range op.Files {
		paths = append(paths, f.DestinationPath)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,1))`, path); err != nil {
			return op, err
		}
		var occupied bool
		if err := tx.QueryRowContext(ctx, `select exists(select 1 from import_operation_files where destination_path=$1) or exists(select 1 from files where path=$1)`, path).Scan(&occupied); err != nil {
			return op, err
		}
		if occupied {
			return op, errors.New("destination belongs to an existing import; choose another destination")
		}
	}
	raw, err := json.Marshal(op.Metadata)
	if err != nil {
		return op, err
	}
	err = tx.QueryRowContext(ctx, `insert into import_operations(download_record_id,wanted_item_id,source_root,destination_root,media_format,import_mode,metadata)
      values($1,$2,$3,$4,$5,$6,$7::jsonb) returning id::text`, downloadID.String, op.WantedID, op.SourceRoot, op.DestinationRoot, op.Format, op.Mode, string(raw)).Scan(&op.ID)
	if err != nil {
		return op, err
	}
	for _, f := range op.Files {
		if _, err := tx.ExecContext(ctx, `insert into import_operation_files(operation_id,file_order,relative_path,source_path,destination_path,size_bytes,sha256,media_format,required,wanted_item_id)
          values($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,'')::uuid)`, op.ID, f.Order, f.RelativePath, f.SourcePath, f.DestinationPath, f.SizeBytes, f.SHA256, f.Format, f.Required, f.WantedID); err != nil {
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
	// The visibility trigger sees this transaction's state, while scanners
	// continue seeing the unfinished operation until the whole transaction commits.
	if _, err := tx.ExecContext(ctx, `update import_operations set state='committed' where id=$1`, op.ID); err != nil {
		return nil, err
	}
	stored := make([]FileRecord, 0, len(records))
	for _, record := range records {
		wantedID, _ := record.Metadata["wantedId"].(string)
		if wantedID == "" {
			return nil, errors.New("manifest book association is missing")
		}
		file, err := persistFile(ctx, tx, record, false)
		if err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, `update import_operation_files set file_id=$3,state='committed',updated_at=now() where operation_id=$1 and destination_path=$2 and media_format<>'sidecar' and wanted_item_id=$4`, op.ID, file.Path, file.ID, wantedID)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return nil, errors.New("file does not match import manifest")
		}
		if _, err := tx.ExecContext(ctx, `insert into file_wanted_links(file_id,wanted_item_id) values($1,$2) on conflict do nothing`, file.ID, wantedID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `insert into file_download_links(file_id,download_record_id) values($1,$2) on conflict do nothing`, file.ID, op.DownloadRecordID); err != nil {
			return nil, err
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
		importedBooks[id] = record.MediaFormat
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
	if _, err := tx.ExecContext(ctx, `update downloads set import_status='imported',imported_file_id=$2,imported_at=now(),import_error='',updated_at=now() where id=$1`, op.DownloadRecordID, stored[0].ID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `update import_operations set state='committed',committed_at=now(),lease_token=null,lease_expires_at=null,last_error='',updated_at=now() where id=$1`, op.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import records: %w", err)
	}
	return stored, nil
}

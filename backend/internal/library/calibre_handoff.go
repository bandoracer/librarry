package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/calibre"
)

// CalibreHandoff is the operator-facing projection. Credentials never enter the journal.
type CalibreHandoff struct {
	ID           string                     `json:"id"`
	SourcePath   string                     `json:"sourcePath"`
	RootFolderID string                     `json:"rootFolderId"`
	WantedID     string                     `json:"wantedId,omitempty"`
	Phase        string                     `json:"phase"`
	BookID       int                        `json:"bookId,omitempty"`
	LastError    string                     `json:"lastError,omitempty"`
	Attempts     int                        `json:"attempts"`
	Conversions  []calibreHandoffConversion `json:"conversions"`
}
type calibreHandoffConversion struct {
	Format string                    `json:"format"`
	State  string                    `json:"state"`
	JobID  *int64                    `json:"jobId,omitempty"`
	Status *calibre.ConversionStatus `json:"status,omitempty"`
}
type calibreHandoffPlan struct {
	Record        FileRecord
	Target        string
	WantedRoot    string
	DownloadID    string
	Client        string
	FileID        string
	OutputProfile string
}
type calibreHandoffJournal struct {
	CalibreHandoff
	Plan             calibreHandoffPlan
	DownloadRecordID string
	FileID           string
	Token            string
}

const handoffColumns = `id::text,source_path,root_folder_id::text,coalesce(wanted_item_id::text,''),phase,coalesce(book_id,0),last_error,attempts,plan,progress,coalesce(download_record_id::text,''),coalesce(file_id::text,''),coalesce(run_token::text,'')`

func scanCalibreHandoff(row fileScanner) (calibreHandoffJournal, error) {
	var h calibreHandoffJournal
	var plan, progress []byte
	err := row.Scan(&h.ID, &h.SourcePath, &h.RootFolderID, &h.WantedID, &h.Phase, &h.BookID, &h.LastError, &h.Attempts, &plan, &progress, &h.DownloadRecordID, &h.FileID, &h.Token)
	if err != nil {
		return h, err
	}
	if err = json.Unmarshal(plan, &h.Plan); err != nil {
		return h, err
	}
	err = json.Unmarshal(progress, &h.Conversions)
	if h.Conversions == nil {
		h.Conversions = []calibreHandoffConversion{}
	}
	return h, err
}
func calibreTarget(settings calibre.Settings) string {
	// Password rotation is allowed; changing identity or destination is not.
	settings.Password = ""
	settings.OutputFormat = ""
	settings.OutputProfile = ""
	raw, _ := json.Marshal(settings)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
func (s *Service) resumeCalibreRequest(ctx context.Context, r ImportRequest) (ImportOutcome, bool, error) {
	h, err := scanCalibreHandoff(s.store.db.QueryRowContext(ctx, `select `+handoffColumns+` from calibre_handoffs where source_path=$1`, r.SourcePath))
	if errors.Is(err, sql.ErrNoRows) {
		return ImportOutcome{}, false, nil
	}
	if err != nil {
		return ImportOutcome{}, true, err
	}
	if (r.WantedID != "" && r.WantedID != h.WantedID) || r.DownloadID != h.Plan.DownloadID || (r.DownloadID != "" && acquisition.DownloadClientFromContext(ctx) != "" && !strings.EqualFold(acquisition.DownloadClientFromContext(ctx), h.Plan.Client)) {
		return ImportOutcome{}, true, errors.New("source already has a saved Calibre handoff with another book or download identity")
	}
	if r.DownloadID != "" && acquisition.DownloadClientFromContext(ctx) == "" {
		var unique bool
		if err = s.store.db.QueryRowContext(ctx, `select count(*)=1 and coalesce(min(id::text),'')=$2 from downloads where external_id=$1`, r.DownloadID, h.DownloadRecordID).Scan(&unique); err != nil {
			return ImportOutcome{}, true, err
		}
		if !unique {
			return ImportOutcome{}, true, errors.New("download identity is ambiguous; specify its client")
		}
	}
	out, err := s.RetryCalibreHandoff(ctx, h.ID)
	return out, true, err
}
func (s *Service) planCalibreHandoff(ctx context.Context, r ImportRequest, root RootFolder, source, format string, parsed parsedBook, info fs.FileInfo) (ImportOutcome, error) {
	if s.calibre == nil {
		return ImportOutcome{}, errors.New("Calibre integration is unavailable")
	}
	checksum, err := scanContentHash(ctx, source)
	if err != nil {
		return ImportOutcome{}, err
	}
	record := fileRecordFromPath(source, format, info, "calibre")
	record.SourcePath = source
	record.Checksum = checksum
	record.Title = firstNonEmpty(parsed.Title, record.Title)
	record.AuthorName = firstNonEmpty(parsed.AuthorName, record.AuthorName)
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	p := calibreHandoffPlan{Record: record, Target: calibreTarget(calibreSettingsFromRootFolder(root)), OutputProfile: root.Calibre.OutputProfile, DownloadID: r.DownloadID, Client: acquisition.DownloadClientFromContext(ctx)}
	if r.WantedID != "" {
		item, e := s.lookupWanted(ctx, r.WantedID)
		if e != nil {
			return ImportOutcome{}, e
		}
		p.WantedRoot = item.RootFolderID
	}
	existing, err := s.store.FindFiles(ctx, nil, []string{source})
	if err != nil {
		return ImportOutcome{}, err
	}
	if len(existing) > 0 {
		p.FileID = existing[0].ID
	}
	downloadRecordID := ""
	if r.DownloadID != "" {
		err = s.store.db.QueryRowContext(ctx, `select id::text,client from downloads where external_id=$1 and ($2='' or lower(client)=lower($2)) and (select count(*) from downloads where external_id=$1 and ($2='' or lower(client)=lower($2)))=1`, r.DownloadID, p.Client).Scan(&downloadRecordID, &p.Client)
		if err != nil {
			return ImportOutcome{}, errors.New("download identity is missing or ambiguous")
		}
	}
	conversions := []calibreHandoffConversion{}
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(root.Calibre.ConvertFormats, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
		f = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(f), "."))
		if f != "" && !seen[f] {
			seen[f] = true
			conversions = append(conversions, calibreHandoffConversion{Format: f, State: "planned"})
		}
	}
	plan, _ := json.Marshal(p)
	progress, _ := json.Marshal(conversions)
	var id string
	err = s.store.db.QueryRowContext(ctx, `insert into calibre_handoffs(source_path,root_folder_id,wanted_item_id,download_record_id,plan,progress) values($1,$2,nullif($3,'')::uuid,nullif($4,'')::uuid,$5,$6) on conflict(source_path) do update set source_path=excluded.source_path returning id::text`, source, root.ID, r.WantedID, downloadRecordID, string(plan), string(progress)).Scan(&id)
	if err != nil {
		return ImportOutcome{}, err
	}
	// Always re-read the winning plan: concurrent callers cannot substitute a target.
	out, _, err := s.resumeCalibreRequest(ctx, r)
	return out, err
}

// The dedicated session lock serializes remote work across API/worker processes.
// A process or connection loss releases it; a persisted sending phase then
// requires review. Every write additionally checks a fresh run token, fencing a
// previous owner whose remote call outlived its database connection.
func (s *Service) withCalibreHandoff(ctx context.Context, id string, run func(*sql.Conn, *calibreHandoffJournal) (ImportOutcome, error)) (ImportOutcome, error) {
	if !s.Available() {
		return ImportOutcome{}, errors.New("Calibre recovery requires database persistence")
	}
	id = strings.ToLower(strings.TrimSpace(id))
	conn, err := s.store.db.Conn(ctx)
	if err != nil {
		return ImportOutcome{}, err
	}
	defer conn.Close()
	var locked bool
	if err = conn.QueryRowContext(ctx, `select pg_try_advisory_lock(hashtextextended('calibre-handoff:'||$1,0))`, id).Scan(&locked); err != nil {
		return ImportOutcome{}, err
	}
	if !locked {
		return ImportOutcome{}, errors.New("Calibre handoff is already running")
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if e := conn.QueryRowContext(c, `select pg_advisory_unlock(hashtextextended('calibre-handoff:'||$1,0))`, id).Scan(&unlocked); e != nil || !unlocked {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	h, err := scanCalibreHandoff(conn.QueryRowContext(ctx, `update calibre_handoffs set run_token=gen_random_uuid(),attempts=attempts+1 where id::text=$1 returning `+handoffColumns, id))
	if err != nil {
		return ImportOutcome{}, err
	}
	out, err := run(conn, &h)
	if err != nil {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Persist only our own safe error messages, never remote response bodies.
		_, _ = conn.ExecContext(c, `update calibre_handoffs set last_error=$3,updated_at=now() where id=$1 and run_token=$2`, h.ID, h.Token, err.Error())
	}
	out.CalibreHandoffID = h.ID
	return out, err
}
func saveCalibreHandoff(conn *sql.Conn, h *calibreHandoffJournal) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	raw, err := json.Marshal(h.Conversions)
	if err != nil {
		return err
	}
	result, err := conn.ExecContext(ctx, `update calibre_handoffs set phase=$3,book_id=nullif($4,0),progress=$5,last_error='',updated_at=now() where id=$1 and run_token=$2`, h.ID, h.Token, h.Phase, h.BookID, string(raw))
	if err != nil {
		return errors.New("could not save Calibre acknowledgement; review the saved handoff before retrying")
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return errors.New("Calibre handoff ownership changed")
	}
	return nil
}
func (s *Service) handoffSettings(ctx context.Context, h *calibreHandoffJournal) (calibre.Settings, error) {
	root, err := s.store.GetRootFolder(ctx, h.RootFolderID)
	if err != nil {
		return calibre.Settings{}, errors.New("original Calibre root is unavailable")
	}
	settings := calibreSettingsFromRootFolder(root)
	if !root.Calibre.Enabled || calibreTarget(settings) != h.Plan.Target {
		return calibre.Settings{}, errors.New("original Calibre target changed; restore its configuration before recovery")
	}
	settings.OutputProfile = h.Plan.OutputProfile
	return settings, nil
}
func (s *Service) RetryCalibreHandoff(ctx context.Context, id string) (ImportOutcome, error) {
	return s.withCalibreHandoff(ctx, id, func(conn *sql.Conn, h *calibreHandoffJournal) (ImportOutcome, error) {
		return s.runCalibreHandoff(ctx, conn, h)
	})
}
func (s *Service) runCalibreHandoff(ctx context.Context, conn *sql.Conn, h *calibreHandoffJournal) (ImportOutcome, error) {
	if h.Phase == "committed" {
		files, err := s.store.FindFiles(ctx, []string{h.FileID}, nil)
		if err != nil {
			return ImportOutcome{}, err
		}
		if len(files) != 1 {
			return ImportOutcome{}, errors.New("committed Calibre file is unavailable")
		}
		return ImportOutcome{File: files[0], DestinationPath: files[0].Path, Imported: true, Skipped: true, ImportMode: "calibre", Message: "Saved Calibre handoff already committed"}, nil
	}
	if s.calibre == nil {
		return ImportOutcome{}, errors.New("Calibre integration is unavailable")
	}
	settings, err := s.handoffSettings(ctx, h)
	if err != nil {
		return ImportOutcome{}, err
	}
	if h.Phase == "uploading" {
		return ImportOutcome{}, errors.New("upload acknowledgement is uncertain; inspect Calibre and attach its book ID or confirm the upload is absent")
	}
	record, wantedVersion, err := s.currentHandoffRecord(ctx, h)
	if err != nil {
		return ImportOutcome{}, err
	}
	if h.Phase == "planned" {
		info, e := os.Lstat(h.SourcePath)
		if e != nil || !info.Mode().IsRegular() {
			return ImportOutcome{}, errors.New("saved Calibre source is unavailable or no longer a regular file")
		}
		hash, e := scanContentHash(ctx, h.SourcePath)
		if e != nil || hash != h.Plan.Record.Checksum || info.Size() != h.Plan.Record.SizeBytes {
			return ImportOutcome{}, errors.New("saved Calibre source bytes changed")
		}
		h.Phase = "uploading"
		if err = saveCalibreHandoff(conn, h); err != nil {
			return ImportOutcome{}, err
		}
		result, e := s.calibre.AddBook(ctx, calibre.AddBookRequest{Settings: settings, Path: h.SourcePath})
		if e != nil || result.ID <= 0 {
			return ImportOutcome{}, errors.New("upload acknowledgement is uncertain; inspect Calibre before another upload")
		}
		h.BookID = result.ID
		h.Phase = "accepted"
		if err = saveCalibreHandoff(conn, h); err != nil {
			return ImportOutcome{}, err
		}
	}
	// Metadata is idempotent and is refreshed from current owner data on recovery.
	if err = s.calibre.SetFields(ctx, calibre.SetFieldsRequest{Settings: settings, ID: h.BookID, Metadata: calibreMetadataFromRecord(record)}); err != nil {
		return ImportOutcome{}, errors.New("Calibre accepted the upload; metadata sync failed and can be retried without uploading")
	}
	h.Phase = "converting"
	if err = saveCalibreHandoff(conn, h); err != nil {
		return ImportOutcome{}, err
	}
	pending := false
	for i := range h.Conversions {
		c := &h.Conversions[i]
		switch c.State {
		case "starting", "polling", "unknown", "failed":
			return ImportOutcome{}, fmt.Errorf("Calibre %s conversion needs review (%s); the book will not be uploaded again", c.Format, c.State)
		case "done", "skipped":
			continue
		}
		if c.State == "planned" {
			c.State = "starting"
			if err = saveCalibreHandoff(conn, h); err != nil {
				return ImportOutcome{}, err
			}
			targetSettings := settings
			targetSettings.OutputFormat = c.Format
			result, e := s.calibre.Convert(ctx, calibre.ConvertRequest{Settings: targetSettings, ID: h.BookID, InputFormat: record.Extension})
			if len(result.Jobs) == 1 && result.Jobs[0].JobID >= 0 && strings.EqualFold(result.Jobs[0].OutputFormat, c.Format) {
				c.JobID = &result.Jobs[0].JobID
				c.State = "submitted"
			} else if e == nil && len(result.Jobs) == 0 && len(result.Skipped) == 1 && strings.EqualFold(result.Skipped[0], c.Format) {
				c.State = "skipped"
			} else {
				c.State = "unknown"
			}
			if err = saveCalibreHandoff(conn, h); err != nil {
				return ImportOutcome{}, err
			}
			if c.State == "unknown" {
				return ImportOutcome{}, errors.New("conversion start acknowledgement is uncertain; inspect the saved Calibre book")
			}
			if c.State == "skipped" {
				continue
			}
		}
		if c.JobID == nil {
			return ImportOutcome{}, errors.New("saved conversion job identity is missing")
		}
		c.State = "polling"
		if err = saveCalibreHandoff(conn, h); err != nil {
			return ImportOutcome{}, err
		}
		statuses, e := s.calibre.PollConversions(ctx, calibre.PollConversionsRequest{Settings: settings, Jobs: []calibre.ConvertJob{{OutputFormat: c.Format, JobID: *c.JobID}}, MaxAttempts: 1})
		if e != nil || len(statuses) != 1 || statuses[0].JobID != *c.JobID {
			return ImportOutcome{}, errors.New("conversion status acknowledgement is uncertain; inspect the saved Calibre book")
		}
		status := statuses[0]
		status.Log = ""
		status.Traceback = "" // Remote diagnostics can contain credentials and private paths.
		c.Status = &status
		if status.Running {
			c.State = "submitted"
			pending = true
		} else if status.OK {
			c.State = "done"
		} else {
			c.State = "failed"
		}
		if err = saveCalibreHandoff(conn, h); err != nil {
			return ImportOutcome{}, err
		}
		if c.State == "failed" {
			return ImportOutcome{}, errors.New("Calibre conversion failed; review it before retrying")
		}
	}
	if pending {
		return ImportOutcome{Skipped: true, ImportMode: "calibre", Message: "Calibre conversion is still running; check this saved handoff again"}, nil
	}
	h.Phase = "ready"
	if err = saveCalibreHandoff(conn, h); err != nil {
		return ImportOutcome{}, err
	}
	return s.commitCalibreHandoff(ctx, conn, h, record, wantedVersion)
}

func (s *Service) currentHandoffRecord(ctx context.Context, h *calibreHandoffJournal) (FileRecord, time.Time, error) {
	record := h.Plan.Record
	record.Metadata = map[string]any{}
	for k, v := range h.Plan.Record.Metadata {
		record.Metadata[k] = v
	}
	files, err := s.store.FindFiles(ctx, nil, []string{h.SourcePath})
	if err != nil {
		return record, time.Time{}, err
	}
	if (len(files) == 0 && h.Plan.FileID != "") || (len(files) > 0 && files[0].ID != h.Plan.FileID) {
		return record, time.Time{}, errors.New("source file ownership changed after Calibre planning")
	}
	if len(files) > 0 {
		record = files[0]
		var foreign bool
		if err = s.store.db.QueryRowContext(ctx, `select exists(select 1 from file_wanted_links where file_id=$1 and wanted_item_id::text<>$2) or exists(select 1 from file_download_links where file_id=$1 and download_record_id::text<>$3)`, record.ID, h.WantedID, h.DownloadRecordID).Scan(&foreign); err != nil {
			return record, time.Time{}, err
		}
		if foreign {
			return record, time.Time{}, errors.New("source file belongs to another book or download")
		}
	}
	if record.MediaFormat != h.Plan.Record.MediaFormat {
		return record, time.Time{}, errors.New("source file format changed after planning")
	}
	record.Extension = h.Plan.Record.Extension
	record.SizeBytes = h.Plan.Record.SizeBytes
	record.ModifiedAt = h.Plan.Record.ModifiedAt
	var version time.Time
	if h.WantedID != "" {
		item, e := s.lookupWanted(ctx, h.WantedID)
		if e != nil {
			return record, version, e
		}
		if item.Status == "removed" || item.Status == "ignored" || item.RootFolderID != h.Plan.WantedRoot || (item.Format != "any" && item.Format != record.MediaFormat) {
			return record, version, errors.New("wanted book was removed or its destination or format changed")
		}
		version = item.UpdatedAt
		// Existing file titles are owner data. Wanted metadata supplies new files.
		if h.Plan.FileID == "" {
			record.Title = item.Title
			record.AuthorName = item.AuthorName
		}
	}
	return record, version, nil
}
func (s *Service) commitCalibreHandoff(ctx context.Context, conn *sql.Conn, h *calibreHandoffJournal, record FileRecord, wantedVersion time.Time) (ImportOutcome, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return ImportOutcome{}, err
	}
	defer tx.Rollback()
	var token, phase string
	if err = tx.QueryRowContext(ctx, `select run_token::text,phase from calibre_handoffs where id=$1 for update`, h.ID).Scan(&token, &phase); err != nil {
		return ImportOutcome{}, err
	}
	if token != h.Token || phase != "ready" {
		return ImportOutcome{}, errors.New("Calibre handoff ownership changed")
	}
	// Lock root configuration until commit, then verify the captured target again.
	root, err := scanRootFolder(tx.QueryRowContext(ctx, `select `+rootFolderColumns+` from root_folders where id=$1 for share`, h.RootFolderID))
	if err != nil {
		return ImportOutcome{}, err
	}
	if !root.Calibre.Enabled || calibreTarget(calibreSettingsFromRootFolder(root)) != h.Plan.Target {
		return ImportOutcome{}, errors.New("original Calibre target changed before commit")
	}
	if h.WantedID != "" {
		var version time.Time
		if err = tx.QueryRowContext(ctx, `select updated_at from wanted_items where id=$1 for update`, h.WantedID).Scan(&version); err != nil {
			return ImportOutcome{}, err
		}
		if !version.Equal(wantedVersion) {
			return ImportOutcome{}, errors.New("book changed during metadata sync; retry to sync the current owner data")
		}
	}
	// Serialize registration with other inserts/updates. This short lock contains
	// only local bookkeeping, never network requests.
	if _, err = tx.ExecContext(ctx, `lock table files in share row exclusive mode`); err != nil {
		return ImportOutcome{}, err
	}
	var currentID string
	var currentVersion time.Time
	err = tx.QueryRowContext(ctx, `select id::text,updated_at from files where path=$1 for update`, h.SourcePath).Scan(&currentID, &currentVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ImportOutcome{}, err
	}
	if currentID != h.Plan.FileID || (currentID != "" && !currentVersion.Equal(record.UpdatedAt)) {
		return ImportOutcome{}, errors.New("source file changed during metadata sync; retry to preserve current owner data")
	}
	if currentID != "" {
		// Lock relationship tables for the short commit to prevent concurrent links
		// from being overwritten by the compatibility metadata trigger.
		if _, err = tx.ExecContext(ctx, `lock table file_wanted_links,file_download_links in share row exclusive mode`); err != nil {
			return ImportOutcome{}, err
		}
		var foreign bool
		if err = tx.QueryRowContext(ctx, `select exists(select 1 from file_wanted_links where file_id=$1 and wanted_item_id::text<>$2) or exists(select 1 from file_download_links where file_id=$1 and download_record_id::text<>$3)`, currentID, h.WantedID, h.DownloadRecordID).Scan(&foreign); err != nil {
			return ImportOutcome{}, err
		}
		if foreign {
			return ImportOutcome{}, errors.New("source book or download association changed")
		}
	}
	if record.Metadata == nil {
		record.Metadata = map[string]any{}
	}
	for _, k := range []string{"wantedId", "wantedIds", "downloadId", "downloadRecordId", "downloadClient", "verifiedDownload"} {
		delete(record.Metadata, k)
	}
	record.Metadata["calibreId"] = h.BookID
	record.Metadata["calibreHandoffId"] = h.ID
	record.Metadata["rootFolderId"] = h.RootFolderID
	record.Metadata["importMode"] = "calibre"
	record.Metadata["calibreImportedAt"] = time.Now().UTC().Format(time.RFC3339)
	if h.WantedID != "" {
		record.Metadata["wantedId"] = h.WantedID
	}
	if h.DownloadRecordID != "" {
		record.Metadata["downloadId"] = h.Plan.DownloadID
		record.Metadata["downloadRecordId"] = h.DownloadRecordID
		record.Metadata["downloadClient"] = h.Plan.Client
	}
	jobs := []calibre.ConvertJob{}
	statuses := []calibre.ConversionStatus{}
	for _, c := range h.Conversions {
		if c.JobID != nil {
			jobs = append(jobs, calibre.ConvertJob{OutputFormat: c.Format, JobID: *c.JobID})
		}
		if c.Status != nil {
			statuses = append(statuses, *c.Status)
		}
	}
	record.Metadata["calibreConversionJobs"] = calibreConversionJobMetadata(jobs)
	record.Metadata["calibreConversionStatuses"] = calibreConversionStatusMetadata(statuses)
	record.ImportStatus = "calibre"
	record.Checksum = h.Plan.Record.Checksum
	record.SourcePath = h.SourcePath
	stored, err := persistFile(ctx, tx, record, false)
	if err != nil {
		return ImportOutcome{}, err
	}
	books := map[string]string{}
	if h.WantedID != "" {
		books[h.WantedID] = record.MediaFormat
		if _, err = tx.ExecContext(ctx, `insert into file_wanted_links(file_id,wanted_item_id) values($1,$2) on conflict do nothing`, stored.ID, h.WantedID); err != nil {
			return ImportOutcome{}, err
		}
		if _, err = tx.ExecContext(ctx, `update wanted_items set status='imported',updated_at=now() where id=$1`, h.WantedID); err != nil {
			return ImportOutcome{}, err
		}
	}
	kind := "manual"
	if h.DownloadRecordID != "" {
		kind = "calibre"
	}
	if err = commitImportBookkeeping(ctx, tx, ImportOperation{ID: h.ID, SourceKind: kind, DownloadRecordID: h.DownloadRecordID, DownloadID: h.Plan.DownloadID, Client: h.Plan.Client, Metadata: map[string]any{}}, []FileRecord{stored}, books); err != nil {
		return ImportOutcome{}, err
	}
	if h.DownloadRecordID != "" {
		if _, err = tx.ExecContext(ctx, `insert into file_download_links(file_id,download_record_id) values($1,$2) on conflict do nothing`, stored.ID, h.DownloadRecordID); err != nil {
			return ImportOutcome{}, err
		}
		if _, err = tx.ExecContext(ctx, `update downloads set import_status='imported',imported_file_id=$2,imported_at=now(),import_error='',updated_at=now() where id=$1`, h.DownloadRecordID, stored.ID); err != nil {
			return ImportOutcome{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `update calibre_handoffs set phase='committed',file_id=$2,last_error='',updated_at=now() where id=$1 and run_token=$3`, h.ID, stored.ID, h.Token); err != nil {
		return ImportOutcome{}, err
	}
	if err = tx.Commit(); err != nil {
		return ImportOutcome{}, err
	}
	return ImportOutcome{File: stored, Files: []FileRecord{stored}, Imported: true, ImportMode: "calibre", DestinationPath: stored.Path, Message: "Calibre handoff committed; source retained"}, nil
}

// ResolveCalibreHandoff requires a fresh operator decision, never an automatic
// retry after an uncertain send. Only a positive read of the original server
// can attach an existing ID or accept an already present conversion format.
type CalibreHandoffResolution struct {
	Action  string `json:"action"`
	Confirm bool   `json:"confirm"`
	BookID  int    `json:"bookId,omitempty"`
	Format  string `json:"format,omitempty"`
}
type calibreBookInspector interface {
	BookFormats(context.Context, calibre.Settings, int) ([]string, error)
}

func (s *Service) ResolveCalibreHandoff(ctx context.Context, id string, r CalibreHandoffResolution) (ImportOutcome, error) {
	if !r.Confirm {
		return ImportOutcome{}, errors.New("confirm that you inspected the original Calibre library and any prior request has stopped")
	}
	return s.withCalibreHandoff(ctx, id, func(conn *sql.Conn, h *calibreHandoffJournal) (ImportOutcome, error) {
		settings, err := s.handoffSettings(ctx, h)
		if err != nil {
			return ImportOutcome{}, err
		}
		hasFormat := func(bookID int, format string) error {
			inspector, ok := s.calibre.(calibreBookInspector)
			if !ok {
				return errors.New("Calibre book inspection is unavailable")
			}
			formats, e := inspector.BookFormats(ctx, settings, bookID)
			if e != nil {
				return errors.New("could not verify the selected book on the original Calibre server")
			}
			for _, f := range formats {
				if strings.EqualFold(strings.TrimPrefix(f, "."), strings.TrimPrefix(format, ".")) {
					return nil
				}
			}
			return errors.New("selected Calibre book does not contain the required format")
		}
		switch r.Action {
		case "attach-book":
			if h.Phase != "uploading" || r.BookID <= 0 {
				return ImportOutcome{}, errors.New("attach requires an uncertain upload and a positive existing book ID")
			}
			if err = hasFormat(r.BookID, h.Plan.Record.Extension); err != nil {
				return ImportOutcome{}, err
			}
			h.BookID = r.BookID
			h.Phase = "accepted"
		case "retry-upload":
			if h.Phase != "uploading" {
				return ImportOutcome{}, errors.New("only an uncertain upload can be released for another send")
			}
			h.Phase = "planned"
		case "use-format", "retry-conversion":
			found := false
			for i := range h.Conversions {
				c := &h.Conversions[i]
				if !strings.EqualFold(c.Format, r.Format) {
					continue
				}
				if c.State != "unknown" && c.State != "starting" && c.State != "polling" && c.State != "failed" {
					return ImportOutcome{}, errors.New("conversion does not need an operator resolution")
				}
				if r.Action == "use-format" {
					if err = hasFormat(h.BookID, c.Format); err != nil {
						return ImportOutcome{}, err
					}
					c.State = "skipped"
				} else {
					c.State = "planned"
				}
				c.JobID = nil
				c.Status = nil
				found = true
			}
			if !found {
				return ImportOutcome{}, errors.New("saved conversion format was not found")
			}
		default:
			return ImportOutcome{}, errors.New("unknown Calibre recovery action")
		}
		// Resolution and its audit record commit together, before any further send.
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return ImportOutcome{}, err
		}
		defer tx.Rollback()
		progress, _ := json.Marshal(h.Conversions)
		data, _ := json.Marshal(map[string]any{"handoffId": h.ID, "action": r.Action, "bookId": r.BookID, "format": r.Format})
		result, err := tx.ExecContext(ctx, `update calibre_handoffs set phase=$3,book_id=nullif($4,0),progress=$5,last_error='',updated_at=now() where id=$1 and run_token=$2`, h.ID, h.Token, h.Phase, h.BookID, string(progress))
		if err != nil {
			return ImportOutcome{}, err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return ImportOutcome{}, errors.New("Calibre handoff ownership changed")
		}
		if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,message,data) values('calibre_handoff_resolved','calibre_handoff',$1,'Operator resolved Calibre handoff',$2)`, h.ID, string(data)); err != nil {
			return ImportOutcome{}, err
		}
		if err = tx.Commit(); err != nil {
			return ImportOutcome{}, err
		}
		return ImportOutcome{Skipped: true, ImportMode: "calibre", Message: "Recovery decision saved; retry the handoff to continue"}, nil
	})
}
func (s *Store) listCalibreHandoffs(ctx context.Context, limit int) ([]CalibreHandoff, int, error) {
	items := []CalibreHandoff{}
	var count int
	if err := s.db.QueryRowContext(ctx, `select count(*) from calibre_handoffs where phase<>'committed'`).Scan(&count); err != nil {
		return items, count, err
	}
	rows, err := s.db.QueryContext(ctx, `select `+handoffColumns+` from calibre_handoffs order by (phase<>'committed') desc,updated_at desc,id limit $1`, limit)
	if err != nil {
		return items, count, err
	}
	defer rows.Close()
	for rows.Next() {
		h, e := scanCalibreHandoff(rows)
		if e != nil {
			return items, count, e
		}
		items = append(items, h.CalibreHandoff)
	}
	return items, count, rows.Err()
}

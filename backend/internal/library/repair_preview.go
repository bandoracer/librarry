package library

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// RepairPreview is evidence for operator review, never authorization to change
// files, associations, cleanup receipts, or immutable import manifests.
type RepairPreview struct {
	Findings    []RepairFinding `json:"findings"`
	Checked     int             `json:"checked"`
	Section     string          `json:"section"`
	NextCursor  string          `json:"nextCursor,omitempty"`
	GeneratedAt time.Time       `json:"generatedAt"`
	ReadOnly    bool            `json:"readOnly"`
}
type RepairFinding struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	SubjectID      string         `json:"subjectId"`
	Path           string         `json:"path"`
	Reason         string         `json:"reason"`
	ProposedAction string         `json:"proposedAction"`
	Evidence       map[string]any `json:"evidence"`
}
type repairCursor struct {
	Section string `json:"section"`
	After   string `json:"after"`
}

var repairUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var ErrRepairCursor = errors.New("invalid repair preview cursor")

func decodeRepairCursor(raw string) (repairCursor, error) {
	c := repairCursor{Section: "files"}
	if raw != "" {
		if len(raw) > 256 {
			return c, ErrRepairCursor
		}
		b, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return c, ErrRepairCursor
		}
		if err = json.Unmarshal(b, &c); err != nil {
			return c, ErrRepairCursor
		}
	}
	if c.Section != "files" && c.Section != "downloads" && c.Section != "imports" {
		return c, ErrRepairCursor
	}
	if c.After != "" && !repairUUID.MatchString(c.After) {
		return c, ErrRepairCursor
	}
	return c, nil
}
func encodeRepairCursor(c repairCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Each page inspects at most 100 entities in one read-only database snapshot.
// The cursor crosses all three sections even when a page has no findings.
// Pages are live observations, not a frozen report across concurrent mutations.
func (s *Service) PreviewLibraryRepair(ctx context.Context, cursor string) (RepairPreview, error) {
	out := RepairPreview{Findings: []RepairFinding{}, ReadOnly: true}
	c, err := decodeRepairCursor(cursor)
	if err != nil {
		return out, err
	}
	out.Section = c.Section
	if !s.store.Configured() {
		return out, errors.New("library repair preview requires persistence")
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `select now()`).Scan(&out.GeneratedAt); err != nil {
		return out, err
	}
	tables := map[string]string{"files": "files", "downloads": "downloads", "imports": "import_operations"}
	filters := map[string]string{"files": "true", "downloads": "import_status='imported'", "imports": "state='committed'"}
	rows, err := tx.QueryContext(ctx, `select id::text from `+tables[c.Section]+` where `+filters[c.Section]+` and ($1='' or id>nullif($1,'')::uuid) order by id limit 101`, c.After)
	if err != nil {
		return out, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(ids) > 100 {
		ids = ids[:100]
		out.NextCursor = encodeRepairCursor(repairCursor{c.Section, ids[99]})
	} else if c.Section == "files" {
		out.NextCursor = encodeRepairCursor(repairCursor{Section: "downloads"})
	} else if c.Section == "downloads" {
		out.NextCursor = encodeRepairCursor(repairCursor{Section: "imports"})
	}
	out.Checked = len(ids)
	raw, _ := json.Marshal(ids)
	queries := map[string]string{"files": repairFileQuery, "downloads": repairDownloadQuery, "imports": repairImportQuery}
	rows, err = tx.QueryContext(ctx, queries[c.Section], string(raw))
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var f RepairFinding
		var evidence []byte
		if err = rows.Scan(&f.Kind, &f.SubjectID, &f.Path, &f.Reason, &f.ProposedAction, &evidence); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal(evidence, &f.Evidence); err != nil {
			rows.Close()
			return out, err
		}
		f.ID = fmt.Sprintf("%s:%s", f.Kind, f.SubjectID)
		out.Findings = append(out.Findings, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}

const repairFileQuery = `with selected as (select f.* from files f where id in (select value::uuid from jsonb_array_elements_text($1::jsonb)))
select 'wanted_link',f.id::text,f.path,
 case when exists(select 1 from wanted_items w where w.id::text=lower(trim(f.metadata->>'wantedId'))) then 'Saved book identity is not represented by the current relational link.' else 'Saved book identifier no longer resolves.' end,
 'Review the saved book identifier and current assignment. Restore only a confirmed exact association; retain manual assignments.',
 jsonb_build_object('savedWantedId',f.metadata->>'wantedId','currentWantedIds',(select coalesce(jsonb_agg(l.wanted_item_id order by l.wanted_item_id),'[]') from file_wanted_links l where l.file_id=f.id))
from selected f where coalesce(trim(f.metadata->>'wantedId'),'')<>'' and not exists(select 1 from file_wanted_links l where l.file_id=f.id and l.wanted_item_id::text=lower(trim(f.metadata->>'wantedId')))
union all
select 'download_link',f.id::text,f.path,
 case when candidates.n=0 then 'Saved download identifier no longer resolves.' when candidates.n>1 then 'Saved download identifier matches multiple clients.' else 'Saved download identity is not represented by the current relational link.' end,
 'Review the original client and exact download ID. Do not infer an association from the title or authorize source cleanup.',
 jsonb_build_object('savedDownloadId',key.id,'savedClient',key.client,'candidateCount',candidates.n)
from selected f
cross join lateral (select coalesce(nullif(trim(f.metadata->'verifiedDownload'->>'id'),''),nullif(trim(f.metadata->>'downloadId'),'')) id,coalesce(nullif(trim(f.metadata->'verifiedDownload'->>'client'),''),nullif(trim(f.metadata->>'downloadClient'),'')) client) key
cross join lateral (select count(*) n from downloads d where d.external_id=key.id and (key.client is null or lower(d.client)=lower(key.client))) candidates
where key.id is not null and (candidates.n<>1 or not exists(select 1 from file_download_links l join downloads d on d.id=l.download_record_id where l.file_id=f.id and d.external_id=key.id and (key.client is null or lower(d.client)=lower(key.client))))
union all
select 'duplicate_content',f.id::text,f.path,'Multiple file records have the same recorded SHA-256, size and format.',
 'Compare current bytes, filesystem identity and book assignments. Keep intentional copies or hardlinks; do not bulk-delete records.',
 jsonb_build_object('sha256',f.checksum,'sizeBytes',f.size_bytes,'recordCount',(select count(*) from files p where p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format),'sampleLimit',20,'records',(select jsonb_agg(to_jsonb(p)) from (select id,path,presence_state as presence from files p where p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format order by id limit 20) p))
from selected f where f.checksum ~ '^[0-9a-f]{64}$' and f.size_bytes>0
 and exists(select 1 from files p where p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format and p.id>f.id)
 and not exists(select 1 from files p where p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format and p.id<f.id)
union all
select 'possible_move',f.id::text,f.path,
 case when candidates.n=1 then 'A missing file has one present record with the same recorded content.' else 'A missing file has multiple present records with the same recorded content.' end,
 'Verify both paths and fresh content hashes before reattaching the original file ID. Ambiguous matches require operator review; preserve manual metadata and import history.',
 jsonb_build_object('sha256',f.checksum,'candidateCount',candidates.n,'sampleLimit',20,'candidates',(select jsonb_agg(to_jsonb(p)) from (select p.id,p.path from files p join library_scan_jobs j on j.id=p.last_seen_scan_id and j.state='completed' where p.id<>f.id and p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format and p.presence_state='present' order by p.id limit 20) p))
from selected f cross join lateral (select count(*) n from files p join library_scan_jobs j on j.id=p.last_seen_scan_id and j.state='completed' where p.id<>f.id and p.checksum=f.checksum and p.size_bytes=f.size_bytes and p.media_format=f.media_format and p.presence_state='present') candidates
where f.presence_state='missing' and f.checksum ~ '^[0-9a-f]{64}$' and f.size_bytes>0 and candidates.n>0
union all
select 'legacy_audio_file',f.id::text,f.path,'Audiobook file is marked imported without a committed file manifest; completeness is unverified.',
 'Review the original source inventory and every destination chapter. A single-file audiobook may be valid; do not infer completeness from the imported flag.',
 jsonb_build_object('fileId',f.id,'recordedPresence',f.presence_state)
from selected f where f.media_format='audiobook' and f.import_status='imported'
 and not exists(select 1 from import_operation_files m join import_operations o on o.id=m.operation_id where m.file_id=f.id and m.state='committed' and o.state='committed')
 and not exists(select 1 from file_download_links l join downloads d on d.id=l.download_record_id where l.file_id=f.id and d.import_status='imported')
order by 2,1`

const repairDownloadQuery = `with selected as (select d.* from downloads d where id in(select value::uuid from jsonb_array_elements_text($1::jsonb)))
select 'imported_download_unlinked',d.id::text,d.save_path,'Download is marked imported but has no relational file association.',
 'Review the original import and retained payload. Reconstruct exact file associations only after verifying the destinations; do not re-grab or delete sources based on this flag.',
 jsonb_build_object('client',d.client,'downloadId',d.external_id,'projectedFileId',d.imported_file_id)
from selected d where not exists(select 1 from file_download_links l where l.download_record_id=d.id)
union all
select 'audiobook_completeness',d.id::text,d.save_path,'Audiobook import has no committed complete-file manifest; chapter completeness is unverified.',
 'Compare the original complete client inventory with every destination chapter and sidecar. A single-file audiobook may be valid; retain sources until completeness is established.',
 jsonb_build_object('client',d.client,'downloadId',d.external_id,'linkedAudioFiles',(select count(*) from file_download_links l join files f on f.id=l.file_id where l.download_record_id=d.id and f.media_format='audiobook'))
from selected d where
 (exists(select 1 from file_download_links l join files f on f.id=l.file_id where l.download_record_id=d.id and f.media_format='audiobook') or exists(select 1 from releases r join wanted_items w on w.id=r.wanted_item_id where r.id=d.release_id and w.wanted_format='audiobook'))
 and not exists(select 1 from import_operations o where o.download_record_id=d.id and o.state='committed' and exists(select 1 from import_operation_files m where m.operation_id=o.id and m.required and m.media_format='audiobook') and not exists(select 1 from import_operation_files m where m.operation_id=o.id and m.required and (m.state<>'committed' or (m.media_format<>'sidecar' and m.file_id is null))))
order by 2,1`

const repairImportQuery = `with selected as (select o.* from import_operations o where id in(select value::uuid from jsonb_array_elements_text($1::jsonb))), gaps as (
select m.operation_id,m.id,m.destination_path,m.file_id,
 case when m.state<>'committed' then 'Required manifest entry is not committed.' when m.media_format<>'sidecar' and f.id is null then 'Required media has no tracked file.' when f.presence_state='missing' then 'Required media is recorded missing.' when f.path<>m.destination_path then 'Tracked file path differs from the historical import destination.' when f.id is not null and (f.checksum is distinct from m.sha256 or f.size_bytes is distinct from m.size_bytes) then 'Recorded content differs from the historical import manifest.' end reason
from import_operation_files m join selected o on o.id=m.operation_id left join files f on f.id=m.file_id where m.required)
select 'import_manifest_gap',o.id::text,o.destination_root,
 case when not exists(select 1 from import_operation_files m where m.operation_id=o.id and m.required) then 'Committed import has no required file manifest.' else 'Committed import evidence disagrees with current tracked files.' end,
 'Review each discrepancy against current files and the original manifest. A deliberate move or replacement can explain differences; preserve the historical manifest and keep cleanup blocked until independently verified.',
 jsonb_build_object('sourceKind',o.source_kind,'gapCount',(select count(*) from gaps g where g.operation_id=o.id and g.reason is not null),'sampleLimit',20,'gaps',(select coalesce(jsonb_agg(to_jsonb(g)),'[]') from (select id,destination_path as path,file_id,reason from gaps g where g.operation_id=o.id and g.reason is not null order by id limit 20) g))
from selected o where not exists(select 1 from import_operation_files m where m.operation_id=o.id and m.required) or exists(select 1 from gaps g where g.operation_id=o.id and g.reason is not null)
order by 2,1`

package library

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func allRepairFindings(t *testing.T, s *Service) []RepairFinding {
	t.Helper()
	out := []RepairFinding{}
	cursor := ""
	seen := map[string]bool{}
	for n := 0; n < 50; n++ {
		page, err := s.PreviewLibraryRepair(context.Background(), cursor)
		if err != nil {
			t.Fatal(err)
		}
		if !page.ReadOnly || page.GeneratedAt.IsZero() || page.Findings == nil {
			t.Fatal(page)
		}
		for _, f := range page.Findings {
			if seen[f.ID] {
				t.Fatal("duplicate finding", f.ID)
			}
			seen[f.ID] = true
			out = append(out, f)
		}
		if page.NextCursor == "" {
			return out
		}
		if page.NextCursor == cursor {
			t.Fatal("cursor stuck")
		}
		cursor = page.NextCursor
	}
	t.Fatal("report did not finish")
	return nil
}
func TestRepairPreviewExplainsLegacyDamageWithoutMutation(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), Config{}, nil, nil)
	// Deliberately malformed and ambiguous legacy identifiers must remain data.
	var book string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title) values('audiobook','Fixture') returning id::text`).Scan(&book); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"qBittorrent", "Transmission"} {
		if _, err := db.Exec(`insert into downloads(client,external_id,category,save_path,state,import_status) values($1,'shared-id','books','/downloads','complete','imported')`, client); err != nil {
			t.Fatal(err)
		}
	}
	hash := strings.Repeat("a", 64)
	for i := 0; i < 2; i++ {
		_, err := s.store.UpsertFile(context.Background(), FileRecord{Path: fmt.Sprintf("/library/fixture-%d.mp3", i), MediaFormat: "audiobook", ImportStatus: "imported", Checksum: hash, SizeBytes: 12, Metadata: map[string]any{"wantedId": "not-a-uuid", "downloadId": "shared-id", "privateToken": "must-not-appear"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	// One exact-client record gets a proven link but no complete chapter receipt.
	file, err := s.store.UpsertFile(context.Background(), FileRecord{Path: "/library/exact.mp3", MediaFormat: "audiobook", ImportStatus: "imported", Metadata: map[string]any{"wantedId": book, "downloadId": "shared-id", "downloadClient": "qBittorrent"}})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate old linkage damage without manufacturing a report at migration time.
	if _, err = db.Exec(`delete from file_wanted_links where file_id=$1`, file.ID); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var raw string
		err := db.QueryRow(`select jsonb_build_object('files',(select jsonb_agg(to_jsonb(f) order by id) from files f),'wanted',(select jsonb_agg(to_jsonb(w) order by id) from wanted_items w),'downloads',(select jsonb_agg(to_jsonb(d) order by id) from downloads d),'links',(select jsonb_agg(to_jsonb(l) order by file_id) from file_wanted_links l),'issues',(select jsonb_agg(to_jsonb(i) order by file_id,kind) from import_reconciliation_issues i))::text`).Scan(&raw)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := snapshot()
	findings := allRepairFindings(t, s)
	after := snapshot()
	if before != after {
		t.Fatal("preview mutated persistent data")
	}
	kinds := map[string]int{}
	for _, f := range findings {
		kinds[f.Kind]++
		if f.Reason == "" || f.ProposedAction == "" {
			t.Fatal(f)
		}
	}
	for kind, want := range map[string]int{"wanted_link": 3, "download_link": 2, "duplicate_content": 1, "imported_download_unlinked": 1, "audiobook_completeness": 1, "legacy_audio_file": 2} {
		if kinds[kind] != want {
			t.Fatalf("%s=%d want %d; %+v", kind, kinds[kind], want, findings)
		}
	}
	raw, _ := json.Marshal(findings)
	if strings.Contains(string(raw), "must-not-appear") || strings.Contains(string(raw), "privateToken") {
		t.Fatal("raw metadata leaked")
	}
}
func TestRepairPreviewPaginatesEvenWhenFilesHaveNoFindings(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), Config{}, nil, nil)
	if _, err := db.Exec(`insert into files(media_format,path) select 'ebook','/library/'||n::text||'.epub' from generate_series(1,205) n`); err != nil {
		t.Fatal(err)
	}
	cursor := ""
	checked := 0
	for n := 0; n < 5; n++ {
		page, err := s.PreviewLibraryRepair(context.Background(), cursor)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Findings) != 0 {
			t.Fatal(page)
		}
		checked += page.Checked
		if n < 4 && page.NextCursor == "" {
			t.Fatal("empty finding page truncated report", page)
		}
		if n == 4 && page.NextCursor != "" {
			t.Fatal("report did not end", page)
		}
		cursor = page.NextCursor
	}
	if checked != 205 {
		t.Fatal(checked)
	}
	for _, cursor := range []string{"broken", encodeRepairCursor(repairCursor{Section: "files; delete from files"}), encodeRepairCursor(repairCursor{Section: "files", After: "not-uuid"}), strings.Repeat("a", 257)} {
		if _, err := s.PreviewLibraryRepair(context.Background(), cursor); err != ErrRepairCursor {
			t.Fatal(cursor, err)
		}
	}
}
func TestRepairPreviewDistinguishesPossibleMovesFromAmbiguousCopies(t *testing.T) {
	db := testdb.Open(t)
	s := NewService(NewStore(db), Config{}, nil, nil)
	var job string
	if err := db.QueryRow(`insert into library_scan_jobs(scope_key,media_format,roots,state,phase) values('fixture','any','[]','completed','complete') returning id::text`).Scan(&job); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("b", 64)
	if _, err := db.Exec(`insert into files(media_format,path,size_bytes,checksum,presence_state,last_seen_scan_id) values('ebook','/old.epub',20,$1,'missing',$2),('ebook','/new.epub',20,$1,'present',$2)`, hash, job); err != nil {
		t.Fatal(err)
	}
	check := func(want float64) {
		t.Helper()
		n := 0
		for _, f := range allRepairFindings(t, s) {
			if f.Kind == "possible_move" {
				n++
				if f.Evidence["candidateCount"] != want {
					t.Fatal(f)
				}
			}
		}
		if n != 1 {
			t.Fatal(n)
		}
	}
	check(1)
	if _, err := db.Exec(`insert into files(media_format,path,size_bytes,checksum,presence_state,last_seen_scan_id) values('ebook','/copy.epub',20,$1,'present',$2)`, hash, job); err != nil {
		t.Fatal(err)
	}
	check(2)
	// An incomplete scan is insufficient evidence of a candidate move.
	if _, err := db.Exec(`update library_scan_jobs set state='failed' where id=$1`, job); err != nil {
		t.Fatal(err)
	}
	for _, f := range allRepairFindings(t, s) {
		if f.Kind == "possible_move" {
			t.Fatal(f)
		}
	}
}
func TestRepairPreviewHealthyImportAndLaterManifestDamage(t *testing.T) {
	s, db, download, _ := operationFixture(t)
	result, err := s.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || result.Imported != 1 {
		t.Fatal(result, err)
	}
	if findings := allRepairFindings(t, s); len(findings) != 0 {
		t.Fatal(findings)
	}
	if _, err := db.Exec(`update files set presence_state='missing'`); err != nil {
		t.Fatal(err)
	}
	findings := allRepairFindings(t, s)
	if len(findings) != 1 || findings[0].Kind != "import_manifest_gap" {
		t.Fatal(findings)
	}
	if _, err := db.Exec(`delete from files`); err != nil {
		t.Fatal(err)
	}
	findings = allRepairFindings(t, s)
	kinds := map[string]bool{}
	for _, f := range findings {
		kinds[f.Kind] = true
	}
	if !kinds["imported_download_unlinked"] || !kinds["import_manifest_gap"] {
		t.Fatal(findings)
	}
}
func TestRepairPreviewUnavailableStore(t *testing.T) {
	s := NewService(nil, Config{}, nil, nil)
	if _, err := s.PreviewLibraryRepair(context.Background(), ""); err == nil {
		t.Fatal("preview claimed success without persistence")
	}
}

func TestRepairPreviewRecognizesCompleteAudiobookAndReportsMissingChapter(t *testing.T) {
	s, download, _ := audiobookFixture(t)
	out, err := s.ImportCompletedDownloads(context.Background(), []acquisition.DownloadStatus{download}, CompletedImportRequest{ImportMode: "copy"})
	if err != nil || out.Imported != 1 {
		t.Fatal(out, err)
	}
	if findings := allRepairFindings(t, s); len(findings) != 0 {
		t.Fatal("complete chapter set was treated as legacy", findings)
	}
	if _, err := s.store.db.Exec(`delete from files where id=$1`, out.Results[0].Import.Files[1].ID); err != nil {
		t.Fatal(err)
	}
	findings := allRepairFindings(t, s)
	kinds := map[string]int{}
	for _, f := range findings {
		kinds[f.Kind]++
	}
	if kinds["audiobook_completeness"] != 1 || kinds["import_manifest_gap"] != 1 {
		t.Fatal(findings)
	}
}

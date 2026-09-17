package library

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/database"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestLegacyRelationshipUpgradeDoesNotGrantVerification(t *testing.T) {
	db := testdb.OpenThrough(t, "0029_zzzz.sql")
	ctx := context.Background()
	var wantedID string
	if err := db.QueryRow(`insert into wanted_items(wanted_format,title) values('ebook','Fixture') returning id::text`).Scan(&wantedID); err != nil {
		t.Fatal(err)
	}
	_, err := db.Exec(`insert into downloads(client,external_id,category,save_path,state) values('qBittorrent','shared','books','/downloads','pausedUP'),('Transmission','shared','books','/downloads','stopped')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`insert into files(media_format,path,metadata,import_status) values
      ('ebook','/fixture/ambiguous.epub',jsonb_build_object('wantedId',$1::text,'downloadId','shared'),'imported'),
      ('ebook','/fixture/scoped.epub',jsonb_build_object('wantedId',$1::text,'downloadId','shared','downloadClient','Transmission'),'imported'),
      ('ebook','/fixture/unresolved.epub','{"wantedId":"invalid-uuid","downloadId":"missing"}','imported')`, wantedID)
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	for i := 0; i < 2; i++ {
		if err := database.ApplyMigrations(ctx, db, filepath.Join(filepath.Dir(file), "../../migrations")); err != nil {
			t.Fatal(err)
		}
	}
	for query, expected := range map[string]int{
		`select count(*) from files`:                                                  3,
		`select count(*) from file_wanted_links`:                                      2,
		`select count(*) from file_download_links`:                                    1,
		`select count(*) from import_reconciliation_issues where resolved_at is null`: 3,
		`select count(*) from import_operations`:                                      0,
		`select count(*) from files where metadata ? 'verifiedDownload'`:              0,
		`select count(*) from import_migration_reports where report->>'filesBefore'='3' and report->>'filesAfter'='3' and report->>'verifiedLegacyImports'='0'`: 1,
	} {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil || count != expected {
			t.Fatalf("%s: got %d want %d (%v)", query, count, expected, err)
		}
	}
	var client string
	if err := db.QueryRow(`select d.client from file_download_links fl join downloads d on d.id=fl.download_record_id`).Scan(&client); err != nil || client != "Transmission" {
		t.Fatalf("identity %s %v", client, err)
	}
	// Removing a legacy JSON field during a scan cannot erase a proven link.
	if _, err := db.Exec(`update files set metadata='{}' where path='/fixture/scoped.epub'`); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`select count(*) from file_wanted_links`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("links lost: %d %v", count, err)
	}
}

func TestRelationalBookLinkFollowsExplicitCorrection(t *testing.T) {
	db := testdb.Open(t)
	var first, second string
	for _, id := range []*string{&first, &second} {
		if err := db.QueryRow(`insert into wanted_items(wanted_format,title) values('ebook','Fixture') returning id::text`).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	store := NewStore(db)
	record, err := store.UpsertFile(context.Background(), FileRecord{Path: "/fixture/book.epub", MediaFormat: "ebook", Metadata: map[string]any{"wantedId": first}})
	if err != nil {
		t.Fatal(err)
	}
	record.Metadata["wantedId"] = second
	if _, err := store.UpsertFile(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	old, err := store.ListFiles(context.Background(), FileListQuery{WantedID: first})
	if err != nil || len(old) != 0 {
		t.Fatalf("old link: %+v %v", old, err)
	}
	current, err := store.ListFiles(context.Background(), FileListQuery{WantedID: second})
	if err != nil || len(current) != 1 {
		t.Fatalf("new link: %+v %v", current, err)
	}
}

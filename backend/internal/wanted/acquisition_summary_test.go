package wanted

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/testdb"
)

func TestAcquisitionSummaryCountsAllBooksBeyondActionPreview(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	s := NewService(NewStore(db), nil)
	for _, query := range []string{
		`insert into wanted_items(id,wanted_format,title,status,created_at) select md5(i::text)::uuid,'ebook','Fixture '||i,case when i=10002 then 'ignored' when i=10003 then 'removed' when i<=100 then 'imported' else 'wanted' end,'2020-01-01'::timestamptz+i*interval '1 second' from generate_series(1,10003)i`,
		`insert into releases(wanted_item_id,indexer,title,protocol,download_url,source_id,approved) select md5(i::text)::uuid,'Fixture','Fixture release','torrent','https://fixture.invalid/never-requested',i::text,i<=300 from generate_series(101,700)i`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	started := time.Now()
	queue, err := s.AcquisitionQueue(ctx, AcquisitionQueueQuery{Limit: 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue.Items) != 8 || queue.Summary.Total != 10001 || queue.Summary.Imported != 100 || queue.Summary.ReadyToGrab != 200 || queue.Summary.Blocked != 400 || queue.Summary.NeedsSearch != 9301 || queue.PreviewLimit != 8 || queue.Downloads != "notConfigured" {
		t.Fatalf("incomplete summary: %+v", queue.Summary)
	}
	durations := []time.Duration{time.Since(started)}
	for i := 0; i < 19; i++ {
		start := time.Now()
		again, err := s.AcquisitionQueue(ctx, AcquisitionQueueQuery{Limit: 8})
		durations = append(durations, time.Since(start))
		if err != nil || again.Summary != queue.Summary {
			t.Fatal(again.Summary, err)
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	t.Logf("10,001 books with 8-item preview, 20 reads: p95 %s", durations[18])
	for _, item := range queue.Items {
		if item.WantedItem.Status == "removed" || item.WantedItem.Status == "ignored" {
			t.Fatal("inactive preview item")
		}
	}
	for _, limit := range []int{1, 100, 200} {
		page, err := s.AcquisitionQueue(ctx, AcquisitionQueueQuery{Limit: limit})
		if err != nil || page.Summary != queue.Summary || len(page.Items) != limit {
			t.Fatal(limit, page.Summary, err)
		}
	}
	imported, err := s.AcquisitionQueue(ctx, AcquisitionQueueQuery{Status: "imported", Limit: 1})
	if err != nil || imported.Summary.Total != 100 || imported.Summary.Imported != 100 {
		t.Fatal(imported.Summary, err)
	}
	if _, err = db.Exec(`update wanted_items set status='removed' where id=md5('101')::uuid`); err != nil {
		t.Fatal(err)
	}
	next, err := s.AcquisitionQueue(ctx, AcquisitionQueueQuery{Limit: 1})
	if err != nil || next.Summary.Total != 10000 || next.Summary.ReadyToGrab != 199 {
		t.Fatal(next.Summary, err)
	}
}

func TestAcquisitionSummaryKeepsMissingClientEvidenceUnknown(t *testing.T) {
	db := testdb.Open(t)
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		if _, err := db.Exec(`insert into wanted_items(id,wanted_format,title,status) values(md5($1)::uuid,'ebook','Fixture',case when $1='4' then 'imported' else 'wanted' end)`, fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	var id string
	if err := db.QueryRow(`select id::text from wanted_items where id=md5('1')::uuid`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	fixture := &workerFixture{evidence: acquisition.DownloadEvidence{Status: "partial", Downloads: []acquisition.DownloadStatus{{ID: "known-failure", State: "error", Tags: []string{"wanted:" + id}, FailureReason: "Fixture failed"}}}}
	s := NewService(NewStore(db), fixture)
	q, err := s.AcquisitionQueue(ctx, AcquisitionQueueQuery{})
	if err != nil || q.Summary.Unknown != 2 || q.Summary.Blocked != 1 || q.Summary.Imported != 1 || q.Downloads != "partial" {
		t.Fatal(q.Summary, err)
	}
	for _, row := range q.Items {
		if row.WantedItem.ID != id && row.WantedItem.Status != "imported" && row.State != "unknown" {
			t.Fatal("offered action without client evidence")
		}
	}
	fixture.evidence = acquisition.DownloadEvidence{Status: "unavailable"}
	q, err = s.AcquisitionQueue(ctx, AcquisitionQueueQuery{})
	if err != nil || q.Summary.Unknown != 3 || q.Summary.NeedsSearch != 0 {
		t.Fatal(q.Summary, err)
	}
	fixture.evidence = acquisition.DownloadEvidence{Status: "fresh"}
	q, err = s.AcquisitionQueue(ctx, AcquisitionQueueQuery{})
	if err != nil || q.Summary.Unknown != 0 || q.Summary.NeedsSearch != 3 {
		t.Fatal(q.Summary, err)
	}
}

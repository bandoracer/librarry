package library

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bandoracer/librarry/backend/internal/calibre"
)

type interruptedCalibre struct {
	*calibre.Client
	adds          int
	acceptedID    int
	failMetadata  bool
	loseUploadAck bool
}

func (c *interruptedCalibre) AddBook(ctx context.Context, r calibre.AddBookRequest) (calibre.AddBookResult, error) {
	c.adds++
	result, err := c.Client.AddBook(ctx, r)
	if err == nil {
		c.acceptedID = result.ID
		if c.loseUploadAck {
			return calibre.AddBookResult{}, errors.New("simulated lost acknowledgement")
		}
	}
	return result, err
}
func (c *interruptedCalibre) SetFields(ctx context.Context, r calibre.SetFieldsRequest) error {
	if c.failMetadata {
		c.failMetadata = false
		return errors.New("simulated metadata outage after real upload")
	}
	return c.Client.SetFields(ctx, r)
}
func TestDisposableCalibreHandoffRecovery(t *testing.T) {
	endpoint := os.Getenv("LIBRARRY_CALIBRE_FIXTURE_URL")
	if endpoint == "" {
		t.Skip("run scripts/test-calibre.py with an isolated Postgres database")
	}
	parsed, e := url.Parse(endpoint)
	if e != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.User != nil {
		t.Fatal("fixture must be a disposable loopback server")
	}
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "metadata interruption", true: "upload acknowledgement loss"}[lost], func(t *testing.T) {
			s, db, _, r, ctx := handoffFixture(t, "TXT")
			ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
			defer cancel()
			bytes, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "fixtures", "e2e", "librarry-public-domain-e2e-book.epub"))
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(r.SourcePath, bytes, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`update root_folders set calibre_host=$1,calibre_port=$2,calibre_username='fixture',calibre_password='fixture-password'`, endpoint, 0); err != nil {
				t.Fatal(err)
			}
			client := &interruptedCalibre{Client: calibre.NewClient(&http.Client{Timeout: 30 * time.Second}), failMetadata: !lost, loseUploadAck: lost}
			s.calibre = client
			settings := calibre.Settings{Host: endpoint, Username: "fixture", Password: "fixture-password", Library: "library"}
			defer func() {
				if client.acceptedID > 0 {
					if e := client.DeleteBooks(context.Background(), calibre.DeleteBooksRequest{Settings: settings, IDs: []int{client.acceptedID}}); e != nil {
						t.Error(e)
					}
				}
			}()
			if _, err = s.Import(ctx, r); err == nil {
				t.Fatal("expected interruption")
			}
			h := handoffRow(t, s)
			// A new Service models a process restart; all acknowledgement state is in PG.
			restarted := NewService(NewStore(db), s.Config(), s.wanted, nil).WithCalibre(client, nil)
			if lost {
				if _, err = restarted.RetryCalibreHandoff(ctx, h.ID); err == nil || client.adds != 1 {
					t.Fatal("uncertain upload repeated")
				}
				if _, err = restarted.ResolveCalibreHandoff(ctx, h.ID, CalibreHandoffResolution{Action: "attach-book", BookID: client.acceptedID, Confirm: true}); err != nil {
					t.Fatal(err)
				}
			}
			for {
				out, e := restarted.RetryCalibreHandoff(ctx, h.ID)
				if e != nil {
					t.Fatal(e)
				}
				if out.Imported {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(250 * time.Millisecond):
				}
			}
			if client.adds != 1 {
				t.Fatalf("uploaded %d times", client.adds)
			}
			formats, err := client.BookFormats(ctx, settings, client.acceptedID)
			if err != nil {
				t.Fatal(err)
			}
			hasTXT := false
			for _, f := range formats {
				if f == "TXT" {
					hasTXT = true
				}
			}
			if !hasTXT {
				t.Fatal(formats)
			}
			if _, err = os.Stat(r.SourcePath); err != nil {
				t.Fatal("source was removed", err)
			}
			var events int
			if err = db.QueryRow(`select count(*) from history_events where event_type='book_imported'`).Scan(&events); err != nil || events != 1 {
				t.Fatalf("history %d %v", events, err)
			}
		})
	}
}

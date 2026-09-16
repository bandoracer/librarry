package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/library"
)

type bookRenameFixture struct {
	fakeLibrary
	previews, applied int
	book, revision    string
}

func (f *bookRenameFixture) PreviewBookRename(_ context.Context, id string) (library.BookRenamePreview, error) {
	f.previews++
	f.book = id
	return library.BookRenamePreview{WantedID: id, Revision: "proof", MediaFiles: 1501}, nil
}
func (f *bookRenameFixture) RenameBook(_ context.Context, id, revision string) (library.ImportOutcome, error) {
	f.applied++
	f.book = id
	f.revision = revision
	return library.ImportOutcome{Imported: true, OperationID: "saved"}, nil
}
func TestBookRenameAPIRequiresExplicitPreviewRevision(t *testing.T) {
	f := &bookRenameFixture{}
	router := NewRouter(Dependencies{Library: f})
	for _, body := range []string{`null`, `{}`, `{"revision":"proof","extra":true}`, `{"revision":"proof"}{}`, `{"revision":"` + strings.Repeat("x", 1<<20) + `"}`} {
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest("POST", "/api/v1/library/books/book/rename", strings.NewReader(body)))
		if res.Code != 400 || f.applied != 0 {
			t.Fatal(res.Code, f.applied, res.Body.String())
		}
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("POST", "/api/v1/library/books/book/rename/preview", strings.NewReader(`{}`)))
	if res.Code != 200 || f.previews != 1 || f.book != "book" || !strings.Contains(res.Body.String(), `"files":[]`) {
		t.Fatal(res.Code, res.Body.String(), f)
	}
	res = httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest("POST", "/api/v1/library/books/book/rename", strings.NewReader(`{"revision":"proof"}`)))
	if res.Code != 200 || f.applied != 1 || f.revision != "proof" {
		t.Fatal(res.Code, res.Body.String(), f)
	}
}

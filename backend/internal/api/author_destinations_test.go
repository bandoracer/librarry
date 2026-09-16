package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/testdb"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

func TestAuthorDestinationAPIValidationAndClear(t *testing.T) {
	db := testdb.Open(t)
	var ebook, audio string
	for format, target := range map[string]*string{"ebook": &ebook, "audiobook": &audio} {
		if err := db.QueryRow(`insert into root_folders(name,path,media_format) values($1,'/'||$1,$1) returning id::text`, format).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	router := NewRouter(Dependencies{Wanted: wanted.NewService(wanted.NewStore(db), nil)})
	call := func(method, path, body string, status int) wanted.AuthorSubscription {
		t.Helper()
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(body)))
		if res.Code != status {
			t.Fatal(method, path, res.Code, res.Body.String())
		}
		var sub wanted.AuthorSubscription
		if status == 200 && json.Unmarshal(res.Body.Bytes(), &sub) != nil {
			t.Fatal(res.Body.String())
		}
		return sub
	}
	for _, root := range []string{audio, "invalid"} {
		call("POST", "/api/v1/authors", fmt.Sprintf(`{"authorName":"Fixture","format":"ebook","rootFolderId":%q}`, root), 400)
	}
	sub := call("POST", "/api/v1/authors", fmt.Sprintf(`{"authorName":"Fixture","format":"ebook","rootFolderId":%q}`, ebook), 200)
	if sub.RootFolderID != ebook {
		t.Fatal(sub)
	}
	path := "/api/v1/authors/" + sub.ID
	call("PATCH", path, fmt.Sprintf(`{"rootFolderId":%q}`, audio), 400)
	if got := call("PATCH", path, `{"missingBookPolicy":"future"}`, 200); got.RootFolderID != ebook {
		t.Fatal(got)
	}
	if got := call("PATCH", path, `{"rootFolderId":""}`, 200); got.RootFolderID != "" {
		t.Fatal(got)
	}
}

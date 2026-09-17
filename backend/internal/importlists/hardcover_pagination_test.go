package importlists

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func listPage(count, after, size int) map[string]any {
	rows := []any{}
	for id := after + 1; id <= min(count, after+size); id++ {
		rows = append(rows, map[string]any{"id": id, "list_id": 7, "book_id": id, "book": map[string]any{"id": id, "title": fmt.Sprintf("Book %d", id)}})
	}
	return map[string]any{"id": 7, "books_count": count, "updated_at": "2026-09-16T00:00:00Z", "list_books": rows}
}
func TestHardcoverListTraversesBeyondLegacyLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Query     string
			Variables struct {
				ListID int `json:"listId"`
				After  int
				Limit  int
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Variables.ListID != 7 || req.Variables.Limit != 200 || !strings.Contains(req.Query, "order_by: {id: asc}") || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("bad request", req)
		}
		// Simulate a provider that returns fewer rows than requested. A short page
		// must not be mistaken for the end of the list.
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"lists_by_pk": listPage(605, req.Variables.After, 73)}})
	}))
	defer server.Close()
	rows, err := NewHardcoverClient(server.Client(), "fixture").WithURL(server.URL).FetchList(context.Background(), map[string]string{"listId": "7"}, 200)
	if err != nil || len(rows) != 605 || calls != 10 || rows[604].SourceKey != "hardcover:605" {
		t.Fatal(len(rows), calls, err)
	}
}
func TestHardcoverListIncompleteTraversalNeverReturnsPartialEntries(t *testing.T) {
	for _, test := range []string{"http", "graphql", "missing-list", "missing-rows", "missing-timestamp", "invalid-timestamp", "count-change", "timestamp-change", "early-empty", "repeated-id", "wrong-list", "wrong-book", "missing-title", "overcount", "oversize", "too-many", "page-limit"} {
		t.Run(test, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var req struct {
					Variables struct {
						After int
						Limit int
					}
				}
				json.NewDecoder(r.Body).Decode(&req)
				count := 3
				if test == "page-limit" {
					count = 200
				}
				page := listPage(count, req.Variables.After, 1)
				if calls > 1 {
					switch test {
					case "http":
						w.WriteHeader(503)
						return
					case "graphql":
						fmt.Fprint(w, `{"errors":[{"message":"fixture-secret"}]}`)
						return
					case "missing-list":
						page = nil
					case "missing-rows":
						delete(page, "list_books")
					case "missing-timestamp":
						delete(page, "updated_at")
					case "invalid-timestamp":
						page["updated_at"] = 42
					case "count-change":
						page["books_count"] = 4
					case "timestamp-change":
						page["updated_at"] = "2026-09-17T00:00:00Z"
					case "early-empty":
						page["list_books"] = []any{}
					case "repeated-id":
						page = listPage(3, 0, 1)
					case "wrong-list":
						page["list_books"].([]any)[0].(map[string]any)["list_id"] = 9
					case "wrong-book":
						page["list_books"].([]any)[0].(map[string]any)["book_id"] = 99
					case "missing-title":
						delete(page["list_books"].([]any)[0].(map[string]any)["book"].(map[string]any), "title")
					case "overcount":
						page["list_books"] = listPage(100, req.Variables.After, 10)["list_books"]
					case "oversize":
						fmt.Fprint(w, strings.Repeat(" ", (4<<20)+1))
						return
					case "too-many":
						page["books_count"] = 10001
					}
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"lists_by_pk": page}})
			}))
			defer server.Close()
			rows, err := NewHardcoverClient(server.Client(), "fixture").WithURL(server.URL).FetchList(context.Background(), map[string]string{"listId": "7"}, 1)
			if err == nil || rows != nil || strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal(rows, err)
			}
			if test == "page-limit" && calls != 101 {
				t.Fatal(calls)
			}
		})
	}
}
func TestHardcoverListValidatesIDAndEmptyVisibility(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"lists_by_pk": listPage(0, 0, 200)}})
	}))
	defer server.Close()
	client := NewHardcoverClient(server.Client(), "fixture").WithURL(server.URL)
	for _, id := range []string{"", "0", "-1", "2147483648", "fixture-secret"} {
		if _, err := client.FetchList(context.Background(), map[string]string{"listId": id}, 200); err == nil || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatal(id, err)
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	rows, err := client.FetchList(context.Background(), map[string]string{"listID": "7"}, 200)
	if err != nil || rows == nil || len(rows) != 0 || calls != 1 {
		t.Fatal(rows, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rows, err := client.FetchList(ctx, map[string]string{"listId": "7"}, 200); err == nil || rows != nil || calls != 1 {
		t.Fatal(rows, err, calls)
	}
}

func TestHardcoverListNullableTimestampAndRepeatedBookMembership(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := listPage(2, 0, 200)
		page["updated_at"] = nil
		if calls == 1 {
			row := page["list_books"].([]any)[1].(map[string]any)
			row["book_id"] = 1
			row["book"].(map[string]any)["id"] = 1
		} else {
			page["list_books"] = []any{}
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"lists_by_pk": page}})
	}))
	defer server.Close()
	rows, err := NewHardcoverClient(server.Client(), "fixture").WithURL(server.URL).FetchList(context.Background(), map[string]string{"listId": "7"}, 200)
	if err != nil || len(rows) != 1 || calls != 2 {
		t.Fatal(rows, err, calls)
	}
}

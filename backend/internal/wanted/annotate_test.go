package wanted

import (
	"context"
	"testing"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

func TestAnnotateDownloadsWithItemsFillsLinkage(t *testing.T) {
	downloads := []acquisition.DownloadStatus{
		{ID: "a", Tags: []string{"librarry", "wanted:w-1"}},
		{ID: "b", Tags: []string{"librarry", " wanted:w-2 "}},
		{ID: "c", Tags: []string{"librarry"}},
		{ID: "d", Tags: []string{"wanted:missing"}},
		{ID: "e", Tags: []string{"wanted:"}},
	}
	items := []WantedItem{
		{ID: "w-1", Title: "Project Hail Mary", AuthorName: "Andy Weir"},
		{ID: "w-2", Title: "A Brief History of Time", AuthorName: "Stephen Hawking"},
	}

	annotated := annotateDownloadsWithItems(downloads, items)

	if annotated[0].WantedID != "w-1" || annotated[0].WantedTitle != "Project Hail Mary" || annotated[0].WantedAuthor != "Andy Weir" {
		t.Fatalf("expected first download linked to w-1, got %+v", annotated[0])
	}
	if annotated[1].WantedID != "w-2" || annotated[1].WantedTitle != "A Brief History of Time" {
		t.Fatalf("expected whitespace-tagged download linked to w-2, got %+v", annotated[1])
	}
	for _, index := range []int{2, 3, 4} {
		if annotated[index].WantedID != "" || annotated[index].WantedTitle != "" || annotated[index].WantedAuthor != "" {
			t.Fatalf("expected download %q to stay unannotated, got %+v", annotated[index].ID, annotated[index])
		}
	}
}

func TestAnnotateDownloadsWithoutPersistenceIsNoOp(t *testing.T) {
	downloads := []acquisition.DownloadStatus{{ID: "a", Tags: []string{"wanted:w-1"}}}
	annotated := NewService(nil, nil).AnnotateDownloads(context.Background(), downloads)
	if len(annotated) != 1 || annotated[0].WantedID != "" {
		t.Fatalf("expected pass-through without database, got %+v", annotated)
	}
}

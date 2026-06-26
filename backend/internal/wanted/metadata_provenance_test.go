package wanted

import (
	"testing"
	"time"
)

func TestMetadataFieldEvidenceMarksProtectedConflict(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	item := WantedItem{
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		ManualOverrides: []ManualOverride{{
			FieldName: "title",
			Value:     "Project Hail Mary",
		}},
	}
	fields := metadataFieldEvidence(item, []ProviderMetadataRecord{
		{
			Provider:    "Hardcover",
			ProviderKey: "hardcover:123",
			EntityType:  "work",
			Confidence:  0.98,
			FetchedAt:   now,
			Values: MetadataRecordValues{
				Title:      "Project Hail Mary",
				AuthorName: "Andy Weir",
				Format:     "ebook",
				MatchedOn:  []string{"title", "author"},
			},
		},
		{
			Provider:    "Open Library",
			ProviderKey: "openlibrary:OL1W",
			EntityType:  "work",
			Confidence:  0.84,
			FetchedAt:   now,
			Values: MetadataRecordValues{
				Title:      "Project Hail Mary: A Novel",
				AuthorName: "Andy Weir",
				ISBNs:      []string{"9780593135204"},
				MatchedOn:  []string{"title"},
			},
		},
	})

	title, ok := findMetadataField(fields, "title")
	if !ok {
		t.Fatalf("expected title evidence in %+v", fields)
	}
	if !title.Protected || !title.Conflict {
		t.Fatalf("expected protected title conflict, got %+v", title)
	}
	if len(title.Candidates) != 2 {
		t.Fatalf("expected two title candidates, got %+v", title.Candidates)
	}
	author, ok := findMetadataField(fields, "author_name")
	if !ok {
		t.Fatalf("expected author evidence in %+v", fields)
	}
	if author.Conflict {
		t.Fatalf("expected matching author candidates not to conflict: %+v", author)
	}
	isbn, ok := findMetadataField(fields, "isbn")
	if !ok || len(isbn.Candidates) != 1 {
		t.Fatalf("expected ISBN candidate evidence, got ok=%v %+v", ok, isbn)
	}
}

func findMetadataField(fields []MetadataFieldEvidence, fieldName string) (MetadataFieldEvidence, bool) {
	for _, field := range fields {
		if field.FieldName == fieldName {
			return field, true
		}
	}
	return MetadataFieldEvidence{}, false
}

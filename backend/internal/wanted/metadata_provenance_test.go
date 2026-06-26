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
		}, {
			FieldName: "publisher",
			Value:     "Ballantine Books",
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
				Publisher:  "Random House Worlds",
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
	publisher, ok := findMetadataField(fields, "publisher")
	if !ok {
		t.Fatalf("expected publisher evidence in %+v", fields)
	}
	if !publisher.Protected || publisher.CanonicalValue != "Ballantine Books" || !publisher.Conflict {
		t.Fatalf("expected protected publisher conflict, got %+v", publisher)
	}
}

func TestMetadataReviewItemSummarizesReviewFields(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	item := WantedItem{
		ID:             "wanted-1",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		ManualOverrides: []ManualOverride{{
			FieldName: "title",
			Value:     "Project Hail Mary",
		}},
	}
	fields := metadataFieldEvidence(item, []ProviderMetadataRecord{{
		Provider:    "Hardcover",
		ProviderKey: "hardcover:123",
		EntityType:  "work",
		Confidence:  0.98,
		FetchedAt:   now.Add(-time.Hour),
		Values: MetadataRecordValues{
			Title: "Project Hail Mary",
		},
	}, {
		Provider:    "Open Library",
		ProviderKey: "openlibrary:OL1W",
		EntityType:  "work",
		Confidence:  0.84,
		FetchedAt:   now,
		Values: MetadataRecordValues{
			Title: "Project Hail Mary: A Novel",
		},
	}})
	review := metadataReviewItem(MetadataProvenance{
		WantedItem: item,
		Fields:     fields,
		Records: []ProviderMetadataRecord{{
			Provider:  "Hardcover",
			FetchedAt: now.Add(-time.Hour),
		}, {
			Provider:  "Open Library",
			FetchedAt: now,
		}},
	})

	if review.ConflictCount != 1 || review.ProtectedCount != 1 || review.RecordCount != 2 || review.CandidateCount != 2 {
		t.Fatalf("unexpected metadata review summary: %+v", review)
	}
	if review.LastFetchedAt == nil || !review.LastFetchedAt.Equal(now) {
		t.Fatalf("expected latest fetched time %s, got %+v", now, review.LastFetchedAt)
	}
	if len(review.Fields) != 1 || review.Fields[0].FieldName != "title" {
		t.Fatalf("expected only reviewable title field, got %+v", review.Fields)
	}
}

func TestMetadataReviewItemDoesNotTreatProtectedOnlyFieldsAsReviewWork(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	item := WantedItem{
		ID:             "wanted-1",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		ManualOverrides: []ManualOverride{{
			FieldName: "publisher",
			Value:     "Random House Worlds",
		}},
	}
	fields := metadataFieldEvidence(item, []ProviderMetadataRecord{{
		Provider:    "Open Library",
		ProviderKey: "openlibrary:OL1W",
		EntityType:  "edition",
		Confidence:  0.84,
		FetchedAt:   now,
		Values: MetadataRecordValues{
			Publisher: "Random House Worlds",
		},
	}})
	review := metadataReviewItem(MetadataProvenance{
		WantedItem: item,
		Fields:     fields,
		Records: []ProviderMetadataRecord{{
			Provider:  "Open Library",
			FetchedAt: now,
		}},
	})

	if review.ConflictCount != 0 || review.ProtectedCount != 1 || review.RecordCount != 1 || review.CandidateCount != 1 {
		t.Fatalf("unexpected protected-only review summary: %+v", review)
	}
	if len(review.Fields) != 0 {
		t.Fatalf("expected protected-only fields to be hidden from review work, got %+v", review.Fields)
	}
	if metadataReviewRequiresOperator(review) {
		t.Fatalf("did not expect protected-only review to require operator action: %+v", review)
	}
	conflictReview := review
	conflictReview.ConflictCount = 1
	if !metadataReviewRequiresOperator(conflictReview) {
		t.Fatalf("expected conflict review to require operator action: %+v", conflictReview)
	}
}

func TestMetadataReviewItemTreatsAcceptedCanonicalAsResolved(t *testing.T) {
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	item := WantedItem{
		ID:             "wanted-1",
		Title:          "Project Hail Mary",
		AuthorName:     "Andy Weir",
		Format:         "ebook",
		QualityProfile: "standard",
		ManualOverrides: []ManualOverride{{
			FieldName: "title",
			Value:     "Project Hail Mary",
			Reason:    manualOverrideReasonCanonicalAccepted,
		}},
	}
	fields := metadataFieldEvidence(item, []ProviderMetadataRecord{{
		Provider:    "Hardcover",
		ProviderKey: "hardcover:123",
		EntityType:  "work",
		Confidence:  0.98,
		FetchedAt:   now.Add(-time.Hour),
		Values: MetadataRecordValues{
			Title: "Project Hail Mary",
		},
	}, {
		Provider:    "Open Library",
		ProviderKey: "openlibrary:OL1W",
		EntityType:  "work",
		Confidence:  0.84,
		FetchedAt:   now,
		Values: MetadataRecordValues{
			Title: "Project Hail Mary: A Novel",
		},
	}})
	title, ok := findMetadataField(fields, "title")
	if !ok {
		t.Fatalf("expected title evidence in %+v", fields)
	}
	if !title.Protected || !title.ReviewResolved || title.Conflict {
		t.Fatalf("expected accepted canonical title to be protected and resolved, got %+v", title)
	}
	review := metadataReviewItem(MetadataProvenance{
		WantedItem: item,
		Fields:     fields,
		Records: []ProviderMetadataRecord{{
			Provider:  "Hardcover",
			FetchedAt: now.Add(-time.Hour),
		}, {
			Provider:  "Open Library",
			FetchedAt: now,
		}},
	})
	if review.ConflictCount != 0 || len(review.Fields) != 0 || metadataReviewRequiresOperator(review) {
		t.Fatalf("expected accepted canonical field to leave review queue, got %+v", review)
	}
}

func TestMetadataCorrectionUpdateRequestMapsSupportedFields(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request MetadataCorrectionRequest
		assert  func(t *testing.T, update WantedUpdateRequest)
	}{
		{
			name:    "title",
			request: MetadataCorrectionRequest{FieldName: "title", Value: "Project Hail Mary"},
			assert: func(t *testing.T, update WantedUpdateRequest) {
				if update.Title != "Project Hail Mary" {
					t.Fatalf("expected title correction, got %+v", update)
				}
			},
		},
		{
			name:    "author alias",
			request: MetadataCorrectionRequest{FieldName: "authorName", Value: "Andy Weir"},
			assert: func(t *testing.T, update WantedUpdateRequest) {
				if update.AuthorName != "Andy Weir" {
					t.Fatalf("expected author correction, got %+v", update)
				}
			},
		},
		{
			name:    "cover alias",
			request: MetadataCorrectionRequest{FieldName: "coverUrl", Value: "https://example.test/cover.jpg"},
			assert: func(t *testing.T, update WantedUpdateRequest) {
				if update.CoverURL != "https://example.test/cover.jpg" {
					t.Fatalf("expected cover correction, got %+v", update)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update, err := metadataCorrectionUpdateRequest(tc.request)
			if err != nil {
				t.Fatalf("unexpected correction error: %v", err)
			}
			tc.assert(t, update)
		})
	}
}

func TestMetadataCorrectionUpdateRequestRejectsUnsupportedFields(t *testing.T) {
	if _, err := metadataCorrectionUpdateRequest(MetadataCorrectionRequest{FieldName: "publisher", Value: "Ballantine"}); err == nil {
		t.Fatal("expected unsupported field error")
	}
	if _, err := metadataCorrectionUpdateRequest(MetadataCorrectionRequest{FieldName: "title", Value: "  "}); err == nil {
		t.Fatal("expected empty value error")
	}
}

func TestMetadataCorrectionFieldValueSupportsBibliographicFields(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fieldName string
		wantField string
	}{
		{name: "language", fieldName: "language", wantField: "language"},
		{name: "publisher", fieldName: "publisher", wantField: "publisher"},
		{name: "published alias", fieldName: "publishedDate", wantField: "published_date"},
		{name: "series", fieldName: "series", wantField: "series"},
		{name: "series position alias", fieldName: "seriesIndex", wantField: "series_position"},
		{name: "isbn plural alias", fieldName: "isbns", wantField: "isbn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field, value, err := metadataCorrectionFieldValue(MetadataCorrectionRequest{
				FieldName: tc.fieldName,
				Value:     "  provider value  ",
			})
			if err != nil {
				t.Fatalf("unexpected correction field error: %v", err)
			}
			if field != tc.wantField || value != "provider value" {
				t.Fatalf("expected %s/provider value, got %s/%s", tc.wantField, field, value)
			}
		})
	}
}

func TestMetadataCorrectionValuesNormalizesAndDeduplicates(t *testing.T) {
	values, err := metadataCorrectionValues([]MetadataCorrectionRequest{
		{FieldName: "title", Value: "Project Hail Mary"},
		{FieldName: "publisher", Value: "Ballantine"},
		{FieldName: "publishedDate", Value: "2021-05-04"},
		{FieldName: "publisher", Value: "Random House Worlds"},
	})
	if err != nil {
		t.Fatalf("unexpected correction values error: %v", err)
	}
	if values["title"] != "Project Hail Mary" || values["publisher"] != "Random House Worlds" || values["published_date"] != "2021-05-04" {
		t.Fatalf("unexpected normalized correction values: %+v", values)
	}
	if !metadataCorrectionsIncludeWantedColumns(values) {
		t.Fatalf("expected display column update for title in %+v", values)
	}

	onlyBibliographic, err := metadataCorrectionValues([]MetadataCorrectionRequest{{FieldName: "isbn", Value: "9780593135204"}})
	if err != nil {
		t.Fatalf("unexpected bibliographic correction error: %v", err)
	}
	if metadataCorrectionsIncludeWantedColumns(onlyBibliographic) {
		t.Fatalf("did not expect wanted column update for bibliographic-only correction: %+v", onlyBibliographic)
	}
	if _, err := metadataCorrectionValues(nil); err == nil {
		t.Fatal("expected empty correction list error")
	}
	if _, err := metadataCorrectionValues([]MetadataCorrectionRequest{{FieldName: "format", Value: "ebook"}}); err == nil {
		t.Fatal("expected unsupported field error")
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

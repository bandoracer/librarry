package metadata

import "testing"

func TestScoreExactISBNIsHighConfidence(t *testing.T) {
	query := Query{Query: "9780593135204", Type: SearchTypeBook}
	score := scoreResult(query, "Project Hail Mary", "Andy Weir", []string{"9780593135204"})
	if score < 0.90 {
		t.Fatalf("expected exact ISBN high score, got %0.2f", score)
	}
	if confidence(score) != "high" {
		t.Fatalf("expected high confidence, got %s", confidence(score))
	}
}

func TestScoreTitleAuthorFuzzyMatch(t *testing.T) {
	query := Query{Query: "Project Hail Mary Andy Weir", Type: SearchTypeBook}
	score := scoreResult(query, "Project Hail Mary", "Andy Weir", nil)
	if score < 0.70 {
		t.Fatalf("expected title/author fuzzy match, got %0.2f", score)
	}
}

func TestDuplicateAuthorStableID(t *testing.T) {
	firstID := stableID("openlibrary-author", "Terry Pratchett")
	secondID := stableID("openlibrary-author", "terry   pratchett")
	if firstID != secondID {
		t.Fatalf("expected normalized duplicate author IDs to match: %s != %s", firstID, secondID)
	}
}

func TestRequestedAudiobookFormatSurvivesInference(t *testing.T) {
	format := inferFormat(FormatAudiobook, []string{"9798217287161"})
	if format != FormatAudiobook {
		t.Fatalf("expected audiobook, got %s", format)
	}
}

func TestLowConfidenceRoutesToReview(t *testing.T) {
	query := Query{Query: "Totally Different Book", Type: SearchTypeBook}
	score := scoreResult(query, "The Colour of Magic", "Terry Pratchett", nil)
	if confidence(score) != "review" {
		t.Fatalf("expected review confidence for weak match, got score=%0.2f confidence=%s", score, confidence(score))
	}
}

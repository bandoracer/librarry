package tags

import "testing"

func TestSplitLabelsNormalizesAndDedupes(t *testing.T) {
	got := SplitLabels(" Favorites ,sci-fi,FAVORITES,, sci-fi ")
	if len(got) != 2 || got[0] != "favorites" || got[1] != "sci-fi" {
		t.Fatalf("unexpected labels: %#v", got)
	}
	if got := SplitLabels("   "); got != nil {
		t.Fatalf("expected nil for blank value, got %#v", got)
	}
}

func TestRewriteLabelsRename(t *testing.T) {
	value, changed := RewriteLabels("favorites,sci-fi,signed", "sci-fi", "science-fiction")
	if !changed || value != "favorites,science-fiction,signed" {
		t.Fatalf("unexpected rewrite: %q changed=%v", value, changed)
	}

	// Renaming onto an existing label collapses duplicates.
	value, changed = RewriteLabels("favorites,sci-fi", "sci-fi", "favorites")
	if !changed || value != "favorites" {
		t.Fatalf("expected dedupe on collision, got %q changed=%v", value, changed)
	}

	// Untouched values report changed=false and keep the original text.
	value, changed = RewriteLabels("favorites,signed", "sci-fi", "science-fiction")
	if changed || value != "favorites,signed" {
		t.Fatalf("expected no change, got %q changed=%v", value, changed)
	}

	// Case-insensitive match on the old label.
	value, changed = RewriteLabels("Favorites,SCI-FI", "sci-fi", "space")
	if !changed || value != "favorites,space" {
		t.Fatalf("expected case-insensitive rename, got %q changed=%v", value, changed)
	}
}

func TestRewriteLabelsDelete(t *testing.T) {
	value, changed := RewriteLabels("favorites,sci-fi,signed", "sci-fi", "")
	if !changed || value != "favorites,signed" {
		t.Fatalf("unexpected delete rewrite: %q changed=%v", value, changed)
	}
	value, changed = RewriteLabels("sci-fi", "sci-fi", "")
	if !changed || value != "" {
		t.Fatalf("expected empty tags after deleting only label, got %q changed=%v", value, changed)
	}
}

func TestStableIDIsPositiveAndStable(t *testing.T) {
	first := StableID("favorites")
	second := StableID("favorites")
	if first != second || first <= 0 {
		t.Fatalf("expected stable positive id, got %d and %d", first, second)
	}
	if StableID("favorites") == StableID("sci-fi") {
		t.Fatal("expected distinct ids for distinct labels")
	}
}

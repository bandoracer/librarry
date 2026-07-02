package wanted

import (
	"strings"
	"testing"
)

func TestRootFolderFormatMismatchReason(t *testing.T) {
	if reason := rootFolderFormatMismatchReason("ebook", "ebook"); reason != "" {
		t.Fatalf("expected matching formats to be compatible, got %q", reason)
	}
	if reason := rootFolderFormatMismatchReason("Ebook", "ebook"); reason != "" {
		t.Fatalf("expected format comparison to normalize case, got %q", reason)
	}

	reason := rootFolderFormatMismatchReason("audiobook", "ebook")
	if reason == "" {
		t.Fatal("expected mismatched formats to be rejected")
	}
	// The handler maps root-folder validation to 400 by matching this phrase,
	// and the message doubles as the explainable rejection for operators.
	if !strings.Contains(reason, "root folder") || !strings.Contains(reason, "audiobook") || !strings.Contains(reason, "ebook") {
		t.Fatalf("expected explainable mismatch reason, got %q", reason)
	}
}

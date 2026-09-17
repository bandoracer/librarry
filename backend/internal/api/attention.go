package api

import (
	"context"
	"net/http"
	"time"
)

type attentionCounts struct {
	ObservedAt       time.Time `json:"observedAt"`
	ImportReviews    int       `json:"importReviews"`
	ImportOperations int       `json:"importOperations"`
	CalibreHandoffs  int       `json:"calibreHandoffs"`
	LegacyLinks      int       `json:"legacyLinks"`
}

// One database statement gives complete counts at one snapshot. No manifests,
// paths or client/provider requests are needed for dashboard triage.
func (h *handler) attentionCounts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Database == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Recovery counts require database persistence"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var result attentionCounts
	err := h.deps.Database.QueryRowContext(ctx, `select now(),
 (select count(*) from import_reviews where status='pending'),
 (select count(*) from import_operations where state<>'committed' or (source_kind='manual' and cleanup_state<>'cleaned') or replacement_cleanup_state='pending'),
 (select count(*) from calibre_handoffs where phase<>'committed'),
 (select count(*) from import_reconciliation_issues where resolved_at is null)`).Scan(&result.ObservedAt, &result.ImportReviews, &result.ImportOperations, &result.CalibreHandoffs, &result.LegacyLinks)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Recovery counts could not be loaded"})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

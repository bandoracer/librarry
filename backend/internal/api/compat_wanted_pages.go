package api

import (
	"net/http"
	"strconv"

	"github.com/bandoracer/librarry/backend/internal/wanted"
)

// Readarr wanted pages share the native presence definition. Legacy status and
// an arbitrary file-list prefix cannot establish presence or a missing book.
func (h *handler) compatWantedPage(w http.ResponseWriter, r *http.Request, state string) {
	for _, name := range []string{"page", "pageSize", "sortKey", "sortDirection"} {
		if values, ok := r.URL.Query()[name]; ok && (len(values) != 1 || values[0] == "") {
			writeJSON(w, 400, map[string]any{"error": "wanted paging parameters must have one nonempty value"})
			return
		}
	}
	page, size := 1, 100
	for name, target := range map[string]*int{"page": &page, "pageSize": &size} {
		if raw := r.URL.Query().Get(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 1000000 || (name == "pageSize" && n > 1000) {
				writeJSON(w, 400, map[string]any{"error": "page must be 1–1000000 and pageSize 1–1000"})
				return
			}
			*target = n
		}
	}
	sortKey := defaultString(r.URL.Query().Get("sortKey"), "title")
	switch sortKey {
	case "title", "authorTitle", "releaseDate", "id":
	default:
		writeJSON(w, 400, map[string]any{"error": "unsupported wanted sortKey"})
		return
	}
	direction := defaultString(r.URL.Query().Get("sortDirection"), "ascending")
	if direction != "ascending" && direction != "descending" {
		writeJSON(w, 400, map[string]any{"error": "sortDirection must be ascending or descending"})
		return
	}
	if h.deps.Wanted == nil {
		writeCompatBookError(w, errCompatServiceUnavailable)
		return
	}
	result, err := h.deps.Wanted.CompatibilityBookPage(r.Context(), wanted.CompatibilityBookPageQuery{Page: page, PageSize: size, State: state, SortKey: sortKey, SortDirection: direction})
	if err != nil {
		writeCompatBookError(w, err)
		return
	}
	profiles := []wanted.QualityProfile{}
	if state == wanted.DerivedStateCutoffUnmet && needsCompatProfiles(result.Books) {
		profiles, err = h.deps.Wanted.ListQualityProfiles(r.Context())
		if err != nil {
			writeCompatBookError(w, err)
			return
		}
	}
	records := []map[string]any{}
	for _, item := range result.Books {
		records = append(records, compatWantedStateRecord(item, state, profiles))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"page": page, "pageSize": size, "sortKey": sortKey, "sortDirection": direction, "totalRecords": result.Total, "records": records, "librarryStateCounts": result.StateCounts, "librarryUnknownBooks": result.Unknown, "librarryDownloads": result.Downloads})
}

func compatWantedStateRecord(item wanted.WantedItem, state string, profiles []wanted.QualityProfile) map[string]any {
	if state != wanted.DerivedStateCutoffUnmet {
		record := compatMissingRecord(item)
		record["librarryDerivedState"] = item.DerivedState
		record["librarryStateEvidence"] = item.StateEvidence
		return record
	}
	profile := wanted.DefaultQualityProfiles()[0]
	for _, p := range profiles {
		if p.Name == item.QualityProfile && (p.MediaFormat == item.Format || p.MediaFormat == "any") {
			profile = p
			if p.MediaFormat == item.Format {
				break
			}
		}
	}
	if item.CompatibilityProfile != nil {
		profile = *item.CompatibilityProfile
	}
	return compatCutoffRecord(item, profile)
}

func (h *handler) compatWantedStateItem(w http.ResponseWriter, r *http.Request, state string) {
	items, err := h.compatSelectBooks(r.Context(), []string{r.PathValue("id")})
	if err != nil {
		writeCompatBookError(w, err)
		return
	}
	item := items[0]
	if !item.Monitored || item.DerivedState != state {
		writeCompatBookError(w, errCompatBookMissing)
		return
	}
	profiles := []wanted.QualityProfile{}
	if state == wanted.DerivedStateCutoffUnmet && needsCompatProfiles(items) {
		profiles, err = h.deps.Wanted.ListQualityProfiles(r.Context())
		if err != nil {
			writeCompatBookError(w, err)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, compatWantedStateRecord(item, state, profiles))
}

func needsCompatProfiles(items []wanted.WantedItem) bool {
	for _, item := range items {
		if item.CompatibilityProfile == nil {
			return true
		}
	}
	return false
}

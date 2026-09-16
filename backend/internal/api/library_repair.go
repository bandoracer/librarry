package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/library"
)

type libraryRepairService interface {
	PreviewLibraryRepair(context.Context, string) (library.RepairPreview, error)
}

func (h *handler) libraryRepairPreview(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(libraryRepairService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library repair preview is unavailable"})
		return
	}
	report, err := service.PreviewLibraryRepair(r.Context(), r.URL.Query().Get("cursor"))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, library.ErrRepairCursor) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	if report.Findings == nil {
		report.Findings = []library.RepairFinding{}
	}
	writeJSON(w, http.StatusOK, report)
}

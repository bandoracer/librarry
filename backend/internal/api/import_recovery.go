package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/jackc/pgx/v5/pgtype"
)

type importRecoveryService interface {
	ImportRecovery(context.Context) (library.ImportRecoveryReport, error)
	RetryImportOperation(context.Context, string) (library.ImportOutcome, error)
}

func (h *handler) importRecovery(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(importRecoveryService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "import recovery is unavailable"})
		return
	}
	report, err := service.ImportRecovery(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *handler) retryImportOperation(w http.ResponseWriter, r *http.Request) {
	service, ok := h.deps.Library.(importRecoveryService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "import recovery is unavailable"})
		return
	}
	id := r.PathValue("id")
	var parsed pgtype.UUID
	if err := parsed.Scan(id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid import operation ID"})
		return
	}
	outcome, err := service.RetryImportOperation(r.Context(), id)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

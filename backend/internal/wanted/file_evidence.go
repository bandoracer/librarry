package wanted

import (
	"context"
	"errors"
)

// FileEvidence describes persisted observations, not a live filesystem probe.
// A complete audiobook requires one committed manifest for this book. Historical
// paths may differ after a proven rename; file identity/content must still match.
type FileEvidence struct {
	State         string `json:"state"` // present, missing, incomplete, unknown, unavailable
	Reason        string `json:"reason"`
	PresentFiles  int    `json:"presentFiles"`
	RequiredFiles int    `json:"requiredFiles,omitempty"`
}

// WantedFileEvidence exposes the shared SQL projection for collection filtering and
// counts. The statement observes one MVCC snapshot, including manifest/link data.
func (s *Store) WantedFileEvidence(ctx context.Context, ids []string) (map[string]FileEvidence, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	result := map[string]FileEvidence{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `select wanted_id,file_state,file_reason,present_files,required_files
	 from librarry_book_file_evidence($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var evidence FileEvidence
		if err := rows.Scan(&id, &evidence.State, &evidence.Reason, &evidence.PresentFiles, &evidence.RequiredFiles); err != nil {
			return nil, err
		}
		result[id] = evidence
	}
	return result, rows.Err()
}

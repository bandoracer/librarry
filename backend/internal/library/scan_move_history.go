package library

import (
	"context"
	"errors"
	"time"
)

type ScanMoveHistory struct {
	Moves      []ScanMoveRecord `json:"moves"`
	NextCursor string           `json:"nextCursor,omitempty"`
}
type ScanMoveRecord struct {
	FileID       string    `json:"fileId"`
	PreviousPath string    `json:"previousPath"`
	CurrentPath  string    `json:"currentPath"`
	SHA256       string    `json:"sha256"`
	SizeBytes    int64     `json:"sizeBytes"`
	CreatedAt    time.Time `json:"createdAt"`
}

var ErrScanMoveCursor = errors.New("invalid scan ID or move history cursor")

func (s *Service) ScanMoveHistory(ctx context.Context, id, cursor string) (ScanMoveHistory, error) {
	out := ScanMoveHistory{Moves: []ScanMoveRecord{}}
	if !repairUUID.MatchString(id) || (cursor != "" && !repairUUID.MatchString(cursor)) {
		return out, ErrScanMoveCursor
	}
	if !s.Available() {
		return out, errors.New("scan move history requires persistence")
	}
	if _, err := s.GetScan(ctx, id); err != nil {
		return out, err
	}
	rows, err := s.store.db.QueryContext(ctx, `select file_id::text,previous_path,current_path,sha256,size_bytes,created_at from library_scan_moves where job_id=$1 and ($2='' or file_id>nullif($2,'')::uuid) order by file_id limit 101`, id, cursor)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var m ScanMoveRecord
		if err := rows.Scan(&m.FileID, &m.PreviousPath, &m.CurrentPath, &m.SHA256, &m.SizeBytes, &m.CreatedAt); err != nil {
			return out, err
		}
		out.Moves = append(out.Moves, m)
	}
	if len(out.Moves) > 100 {
		out.Moves = out.Moves[:100]
		out.NextCursor = out.Moves[99].FileID
	}
	return out, rows.Err()
}

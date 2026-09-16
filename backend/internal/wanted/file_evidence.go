package wanted

import (
	"context"
	"database/sql"
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

type fileObservation struct{ present, missing, unknown int }
type manifestObservation struct {
	fileObservation
	required int
}

func (s *Store) WantedFileEvidence(ctx context.Context, ids []string) (map[string]FileEvidence, error) {
	if !s.Configured() {
		return nil, errors.New("wanted store is unavailable")
	}
	result := map[string]FileEvidence{}
	if len(ids) == 0 {
		return result, nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Restrict every read to this page. Unfinished publication at a tracked path
	// invalidates certainty until that operation commits or is resolved.
	rows, err := tx.QueryContext(ctx, `select wi.id,wi.wanted_format, f.id is not null,
	 coalesce(f.presence_state,''),coalesce(f.import_status,''),
	 exists(select 1 from import_operation_files m join import_operations o on o.id=m.operation_id where m.destination_path=f.path and o.state<>'committed')
	 from wanted_items wi left join file_wanted_links l on l.wanted_item_id=wi.id
	 left join files f on f.id=l.file_id and f.media_format=wi.wanted_format
	 where wi.id=any($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	observed := map[string]*fileObservation{}
	formats := map[string]string{}
	for rows.Next() {
		var id, format, presence, status string
		var exists, pending bool
		if err = rows.Scan(&id, &format, &exists, &presence, &status, &pending); err != nil {
			rows.Close()
			return nil, err
		}
		formats[id] = format
		if observed[id] == nil {
			observed[id] = &fileObservation{}
		}
		if !exists {
			continue
		}
		o := observed[id]
		switch {
		case pending:
			o.unknown++
		case presence == "missing":
			o.missing++
		case presence == "present" && (status == "available" || status == "imported"):
			o.present++
		default:
			o.unknown++
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, `select m.wanted_item_id,m.operation_id,
	 case when f.id is null or f.presence_state='missing' then 'missing'
	 when m.state='committed' and f.presence_state='present' and f.import_status in ('available','imported')
	 and f.media_format=m.media_format and f.checksum=m.sha256 and f.size_bytes=m.size_bytes
	 and exists(select 1 from file_wanted_links l where l.file_id=f.id and l.wanted_item_id=m.wanted_item_id)
	 and not exists(select 1 from import_operation_files pending join import_operations p on p.id=pending.operation_id where pending.destination_path=f.path and p.state<>'committed')
	 then 'present' else 'unknown' end
	 from import_operation_files m join import_operations o on o.id=m.operation_id
	 join wanted_items wi on wi.id=m.wanted_item_id
	 left join files f on f.id=m.file_id
	 where m.wanted_item_id=any($1::uuid[]) and o.state='committed' and m.required and m.media_format=wi.wanted_format
	 order by o.created_at desc,m.operation_id,m.file_order`, ids)
	if err != nil {
		return nil, err
	}
	manifests := map[string]map[string]*manifestObservation{}
	for rows.Next() {
		var id, op, state string
		if err = rows.Scan(&id, &op, &state); err != nil {
			rows.Close()
			return nil, err
		}
		if manifests[id] == nil {
			manifests[id] = map[string]*manifestObservation{}
		}
		if manifests[id][op] == nil {
			manifests[id][op] = &manifestObservation{}
		}
		m := manifests[id][op]
		m.required++
		switch state {
		case "present":
			m.present++
		case "missing":
			m.missing++
		default:
			m.unknown++
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	for id, o := range observed {
		e := FileEvidence{State: "missing", Reason: "No present library media is recorded.", PresentFiles: o.present}
		if formats[id] == "ebook" && o.present > 0 {
			e.State = "present"
			e.Reason = "Library media was observed present by an import or scan."
		} else if o.unknown > 0 || (formats[id] == "audiobook" && o.present > 0) {
			e.State = "unknown"
			e.Reason = "File presence or audiobook completeness has not been verified."
		}
		if formats[id] == "audiobook" {
			// Prefer a complete alternate import. Otherwise choose the strongest
			// known partial evidence deterministically, independent of map order.
			for _, m := range manifests[id] {
				candidate := FileEvidence{State: "missing", Reason: "All required audiobook media is recorded missing.", PresentFiles: m.present, RequiredFiles: m.required}
				switch {
				case m.present == m.required:
					candidate.State = "present"
					candidate.Reason = "Every required audiobook media file has matching committed import evidence."
				case m.present > 0 && m.missing > 0:
					candidate.State = "incomplete"
					candidate.Reason = "Required audiobook media is missing."
				case m.unknown > 0:
					candidate.State = "unknown"
					candidate.Reason = "Required audiobook media could not be verified against its import manifest."
				}
				if fileEvidenceRank(candidate) > fileEvidenceRank(e) || (candidate.State == e.State && (candidate.PresentFiles > e.PresentFiles || candidate.PresentFiles == e.PresentFiles && candidate.RequiredFiles > e.RequiredFiles)) {
					e = candidate
				}
			}
		}
		result[id] = e
	}
	return result, nil
}

func fileEvidenceRank(e FileEvidence) int {
	switch e.State {
	case "present":
		return 4
	case "incomplete":
		return 3
	case "unknown":
		return 2
	default:
		return 1
	}
}

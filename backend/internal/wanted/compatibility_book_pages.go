package wanted

import (
	"context"
	"database/sql"
	"time"
)

type CompatibilityBookPageQuery struct {
	Page, PageSize                int
	State, SortKey, SortDirection string
}
type CompatibilityBookPage struct {
	Books          []WantedItem
	Total, Unknown int
	Downloads      string
	StateCounts    map[string]int
}

func (s *Service) CompatibilityBookPage(ctx context.Context, q CompatibilityBookPageQuery) (CompatibilityBookPage, error) {
	result := CompatibilityBookPage{Books: []WantedItem{}, StateCounts: map[string]int{"missing": 0, "incomplete": 0, "unknown": 0, "downloading": 0, "cutoffUnmet": 0, "downloaded": 0, "unmonitored": 0}}
	if q.Page < 1 || q.Page > 1000000 || q.PageSize < 1 || q.PageSize > 1000 || (q.State != "missing" && q.State != "cutoffUnmet") || (q.SortDirection != "ascending" && q.SortDirection != "descending") {
		return result, ErrCompatibilityBookRequest
	}
	order := "lower(b.title) collate \"C\""
	switch q.SortKey {
	case "title":
	case "authorTitle":
		order = "lower(b.author_name) collate \"C\""
	case "releaseDate":
		order = "coalesce(b.release_date::text,'')"
	case "id":
		order = "librarry_book_compat_id(b.id)"
	default:
		return result, ErrCompatibilityBookRequest
	}
	direction := " asc"
	if q.SortDirection == "descending" {
		direction = " desc"
	}
	order += direction + ",b.id" + direction
	if !s.Available() {
		return result, sql.ErrConnDone
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, args, err := s.collectionSnapshot(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	result.Downloads = args[2].(string)
	args = append(args, q.State)
	rows, err := tx.QueryContext(ctx, bookCollectionSQL+`select derived_state,count(*),count(*) filter(where monitored and derived_state=$4) from stateful group by derived_state`, args...)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var state string
		var count, matching int
		if err = rows.Scan(&state, &count, &matching); err != nil {
			rows.Close()
			return result, err
		}
		result.StateCounts[state] = count
		result.Total += matching
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	result.Unknown = result.StateCounts["unknown"]
	query := bookCollectionSQL + `, paged as materialized (select * from stateful b where b.monitored and b.derived_state=$4 order by ` + order + ` limit $5 offset $6)
	 select ` + wantedDetailColumns + `,b.derived_state,b.file_state,b.file_reason,b.present_files,b.required_files
	 from paged b join wanted_items wi on wi.id=b.id left join works w on w.id=wi.work_id order by ` + order
	args = append(args, q.PageSize, int64(q.Page-1)*int64(q.PageSize))
	result.Books, err = readCompatibilityBooks(ctx, tx, query, args)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

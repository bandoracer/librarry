package wanted

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

var ErrAuthorReviewChanged = errors.New("author review changed or was already resolved differently; refresh before deciding")
var ErrAuthorReviewDecision = errors.New("author review requires a wanted or ignore action and a valid revision when supplied")

// ResolveAuthorReview commits the book, decision and history together. The row
// lock makes retries replay the first decision instead of changing its result.
func (s *Store) ResolveAuthorReview(ctx context.Context, id string, request AuthorMetadataReviewDecisionRequest) (AuthorMetadataReviewDecision, error) {
	var outcome AuthorMetadataReviewDecision
	action := strings.ToLower(strings.TrimSpace(request.Action))
	switch action {
	case "wanted", "mark_wanted", "mark-wanted":
		action = "wanted"
	case "ignore", "ignored", "skip":
		action = "ignored"
	default:
		return outcome, ErrAuthorReviewDecision
	}
	if request.Revision != "" {
		if raw, err := hex.DecodeString(request.Revision); err != nil || len(raw) != 32 {
			return outcome, ErrAuthorReviewDecision
		}
	}
	if !s.Configured() {
		return outcome, sql.ErrConnDone
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return outcome, err
	}
	defer tx.Rollback()
	review, err := scanAuthorMetadataReview(tx.QueryRowContext(ctx, `select `+authorReviewColumns+` from author_metadata_reviews where id::text=$1 for update`, strings.TrimSpace(id)))
	if err != nil {
		return outcome, err
	}
	outcome.Review = review
	if review.Status != "pending" {
		if review.Status != action {
			return outcome, ErrAuthorReviewChanged
		}
		outcome.Replayed = true
		if review.WantedID != "" {
			item, e := wantedInTransaction(ctx, tx, review.WantedID)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return outcome, e
			}
			if e == nil {
				outcome.WantedItem = &item
			}
		}
		return outcome, tx.Commit()
	}
	if request.Revision != "" && !strings.EqualFold(request.Revision, review.Revision) {
		return outcome, ErrAuthorReviewChanged
	}
	wantedID := ""
	entityType, entityID, message := "author_metadata_review", review.ID, "Ignored author metadata candidate for "+review.Title
	if action == "wanted" {
		item, e := s.createWantedInTransaction(ctx, tx, CreateRequest{Result: review.Result, Format: review.Format, RootFolderID: review.RootFolderID, QualityProfile: review.QualityProfile, Tags: review.Tags, OnlyIfUntracked: true})
		if e != nil {
			return outcome, e
		}
		outcome.WantedItem = &item
		outcome.AlreadyTracked = item.alreadyTracked
		wantedID = item.ID
		entityType, entityID, message = "wanted_item", item.ID, "Marked author metadata candidate wanted for "+item.Title
		if item.alreadyTracked {
			message = "Linked author metadata candidate to existing book; retained settings for " + item.Title
		}
	}
	outcome.Review, err = scanAuthorMetadataReview(tx.QueryRowContext(ctx, `update author_metadata_reviews set status=$2,decision=$2,wanted_item_id=nullif($3,'')::uuid,updated_at=now(),resolved_at=now() where id::text=$1 returning `+authorReviewColumns, review.ID, action, wantedID))
	if err != nil {
		return outcome, err
	}
	data, _ := json.Marshal(map[string]any{"reviewId": review.ID, "policy": review.Policy, "reason": review.Reason, "revision": review.Revision, "alreadyTracked": outcome.AlreadyTracked})
	if _, err = tx.ExecContext(ctx, `insert into history_events(event_type,entity_type,entity_id,severity,message,data) values($1,$2,$3,'info',$4,$5::jsonb)`, "author_metadata_review_"+action, entityType, entityID, message, string(data)); err != nil {
		return outcome, err
	}
	return outcome, tx.Commit()
}

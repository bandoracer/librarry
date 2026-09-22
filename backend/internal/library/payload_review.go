package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
)

var ErrImportReviewConflict = errors.New("import review conflict")

func (s *Service) queuePayloadReview(ctx context.Context, download acquisition.DownloadStatus, payload DownloadPayload, wantedID, reason string) (ImportReview, error) {
	metadata := map[string]any{"payload": payload, "downloadClient": download.Client, "payloadReview": true, "requiresWantedSelection": true}
	title, author, format := download.Name, "", "unknown"
	if wantedID != "" {
		item, err := s.lookupWanted(ctx, wantedID)
		if err != nil {
			return ImportReview{}, err
		}
		title, author, format = item.Title, item.AuthorName, item.Format
	}
	var size int64
	for _, f := range payload.Files {
		size += f.SizeBytes
		if format == "unknown" && (f.Format == "ebook" || f.Format == "audiobook") {
			format = f.Format
		}
	}
	review, err := s.store.CreateImportReview(ctx, ImportReview{SourcePath: payload.Root, DownloadID: download.ID, WantedID: wantedID, MediaFormat: format, Title: title, AuthorName: author, SizeBytes: size, Reason: reason, Status: "pending", Metadata: metadata})
	if err != nil {
		return review, err
	}
	// A waiting tick must not erase the conflict that originally requested review.
	if reason == "download awaits an explicit import review decision" && review.Reason != "" {
		reason = review.Reason
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return review, err
	}
	_, err = s.store.db.ExecContext(ctx, `update import_reviews set metadata=$2,reason=$3,size_bytes=$4,updated_at=now() where id=$1 and status='pending' and (metadata is distinct from $2::jsonb or reason<>$3 or size_bytes is distinct from $4)`, review.ID, string(raw), reason, size)
	if err != nil {
		return review, err
	}
	return s.store.GetImportReview(ctx, review.ID)
}

func reviewDownload(review ImportReview) (acquisition.DownloadStatus, error) {
	client, _ := review.Metadata["downloadClient"].(string)
	if client == "" || review.DownloadID == "" {
		return acquisition.DownloadStatus{}, errors.New("review download identity is missing")
	}
	return acquisition.DownloadStatus{Client: client, ID: review.DownloadID}, nil
}

type PayloadPreview struct {
	Operation   ImportOperation `json:"operation"`
	Fingerprint string          `json:"fingerprint"`
}

func payloadPlanFingerprint(op ImportOperation) (string, error) {
	raw, err := json.Marshal(op)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) PreviewPayloadReview(ctx context.Context, id string, request ReviewDecisionRequest) (PayloadPreview, error) {
	if !s.Available() {
		return PayloadPreview{}, errors.New("library persistence is unavailable")
	}
	review, err := s.store.GetImportReview(ctx, id)
	if err != nil {
		return PayloadPreview{}, err
	}
	if review.Status != "pending" || review.Metadata["payloadReview"] != true {
		return PayloadPreview{}, errors.New("review is not a pending payload mapping")
	}
	download, err := reviewDownload(review)
	if err != nil {
		return PayloadPreview{}, err
	}
	payload, err := s.inspectDownloadPayload(ctx, download)
	if err != nil {
		return PayloadPreview{}, err
	}
	op, _, err := s.planPayload(ctx, payload, ImportRequest{WantedID: firstNonEmpty(request.WantedID, review.WantedID), ImportMode: request.ImportMode, Move: request.Move, ConflictAction: request.ConflictAction, Overwrite: request.Overwrite}, request.Mapping, request.ConfirmIdentity)
	if err != nil {
		return PayloadPreview{}, err
	}
	fingerprint, err := payloadPlanFingerprint(op)
	return PayloadPreview{Operation: op, Fingerprint: fingerprint}, err
}

func (s *Service) resolvePayloadReview(ctx context.Context, review ImportReview, request ReviewDecisionRequest) (ReviewDecisionOutcome, error) {
	if !request.ConfirmIdentity || request.PreviewToken == "" {
		return ReviewDecisionOutcome{}, fmt.Errorf("%w: preview and confirm the file-to-book assignments before importing", ErrImportReviewConflict)
	}
	download, err := reviewDownload(review)
	if err != nil {
		return ReviewDecisionOutcome{}, err
	}
	op, err := s.store.operationForDownload(ctx, download.Client, download.ID)
	if errors.Is(err, sql.ErrNoRows) {
		preview, previewErr := s.PreviewPayloadReview(ctx, review.ID, request)
		if previewErr != nil {
			return ReviewDecisionOutcome{}, previewErr
		}
		if preview.Fingerprint != request.PreviewToken {
			return ReviewDecisionOutcome{}, fmt.Errorf("%w: payload, mapping or destination changed; refresh the import preview", ErrImportReviewConflict)
		}
		op = preview.Operation
		op.Metadata["reviewFingerprint"] = preview.Fingerprint
		op, err = s.store.planOperation(ctx, op)
	}
	if err != nil {
		return ReviewDecisionOutcome{}, err
	}
	if op.Metadata["reviewFingerprint"] != request.PreviewToken {
		return ReviewDecisionOutcome{}, fmt.Errorf("%w: another import plan already owns this download; inspect import recovery", ErrImportReviewConflict)
	}
	imported, err := s.runImportOperation(ctx, op)
	if err != nil {
		return ReviewDecisionOutcome{}, err
	}
	resolved, err := s.store.ResolveImportReview(ctx, review.ID, "imported", "import", imported.DestinationPath, op.WantedID)
	if err != nil {
		return ReviewDecisionOutcome{}, err
	}
	return ReviewDecisionOutcome{Review: resolved, Import: &imported}, nil
}

// A review disposition belongs to one client/download identity, independently
// of volatile status refreshed from the external client.
func (s *Store) payloadReviewDisposition(ctx context.Context, client, downloadID string) (string, error) {
	var status string
	err := s.db.QueryRowContext(ctx, `select status from import_reviews
		where download_id=$1 and lower(metadata->>'downloadClient')=lower($2)
		and metadata->>'payloadReview'='true' order by updated_at desc, id desc limit 1`, downloadID, client).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return status, err
}

func (s *Store) reopenPayloadReview(ctx context.Context, id string) (ImportReview, error) {
	result, err := s.db.ExecContext(ctx, `update import_reviews set status='pending',decision='',resolved_at=null,updated_at=now()
		where id=$1 and status in ('skipped','rejected') and metadata->>'payloadReview'='true'`, id)
	if err != nil {
		return ImportReview{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return ImportReview{}, err
	}
	if count != 1 {
		return ImportReview{}, errors.New("only a skipped or rejected payload review can be reopened")
	}
	return s.GetImportReview(ctx, id)
}

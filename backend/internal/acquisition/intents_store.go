package acquisition

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrAcquisitionBusy = errors.New("an acquisition for this book or release is already being processed")
var ErrAcquisitionUncertain = errors.New("download acceptance is uncertain; reconcile it in Activity before submitting again")
var ErrAcquisitionActive = errors.New("this book already has an active acquisition; resolve it in Activity first")
var ErrAcquisitionPersistence = errors.New("acquisition recovery requires database persistence")

// AcquisitionSelection is a sanitized snapshot of the decision actually submitted.
// It deliberately excludes provider URLs, client credentials and payload bodies.
type AcquisitionSelection struct {
	CurrentScore   *float64 `json:"currentScore,omitempty"`
	CutoffScore    *float64 `json:"cutoffScore,omitempty"`
	ReleaseID      string   `json:"releaseId"`
	Score          float64  `json:"score"`
	SourceID       string   `json:"sourceId"`
	Title          string   `json:"title"`
	Trigger        string   `json:"trigger"`
	Forced         bool     `json:"forced"`
	Paused         bool     `json:"paused"`
	RejectedReason string   `json:"rejectedReason"`
}

type AcquisitionIntent struct {
	Selection           AcquisitionSelection `json:"-"`
	BookkeepingRequired bool                 `json:"-"`
	BookkeepingAt       *time.Time           `json:"bookkeepingAt,omitempty"`
	ID                  string               `json:"id"`
	WantedID            string               `json:"wantedId,omitempty"`
	Format              string               `json:"format,omitempty"`
	Client              string               `json:"client"`
	Title               string               `json:"title"`
	State               string               `json:"state"`
	ExternalID          string               `json:"downloadId,omitempty"`
	LastError           string               `json:"lastError,omitempty"`
	Attempts            int                  `json:"attempts"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
	NextCheckAt         *time.Time           `json:"nextCheckAt,omitempty"`
	LeaseExpiresAt      *time.Time           `json:"leaseExpiresAt,omitempty"`
	ScopeKey            string               `json:"-"`
	RequestKey          string               `json:"-"`
	EndpointHash        string               `json:"-"`
	InfoHash            string               `json:"-"`
	Category            string               `json:"-"`
	LeaseToken          string               `json:"-"`
	Result              *DownloadStatus      `json:"-"`
}

type acquisitionIntentStore interface {
	ClaimAcquisition(context.Context, AcquisitionIntent) (AcquisitionIntent, bool, error)
	GetAcquisition(context.Context, string) (AcquisitionIntent, error)
	ListAcquisitions(context.Context) ([]AcquisitionIntent, error)
	ClaimAcquisitionRecovery(context.Context, string) (AcquisitionIntent, error)
	AcceptAcquisition(context.Context, AcquisitionIntent, DownloadStatus) error
	FinalizeAcquisition(context.Context, string) error
	UncertainAcquisition(context.Context, AcquisitionIntent, string) error
	ReleaseAcquisition(context.Context, string) error
}

const intentColumns = `id::text,coalesce(wanted_item_id::text,''),media_format,client,title,state,external_id,last_error,attempts,created_at,updated_at,next_check_at,lease_expires_at,scope_key,request_key,endpoint_hash,info_hash,category,coalesce(lease_token::text,''),result,selection,bookkeeping_required,bookkeeping_at`

type intentScanner interface{ Scan(...any) error }

func scanIntent(row intentScanner) (AcquisitionIntent, error) {
	var i AcquisitionIntent
	var raw, selection []byte
	err := row.Scan(&i.ID, &i.WantedID, &i.Format, &i.Client, &i.Title, &i.State, &i.ExternalID, &i.LastError, &i.Attempts, &i.CreatedAt, &i.UpdatedAt, &i.NextCheckAt, &i.LeaseExpiresAt, &i.ScopeKey, &i.RequestKey, &i.EndpointHash, &i.InfoHash, &i.Category, &i.LeaseToken, &raw, &selection, &i.BookkeepingRequired, &i.BookkeepingAt)
	if err == nil && len(raw) > 0 {
		err = json.Unmarshal(raw, &i.Result)
	}
	if err == nil {
		err = json.Unmarshal(selection, &i.Selection)
	}
	return i, err
}

func (s *SQLDownloadStore) GetAcquisition(ctx context.Context, id string) (AcquisitionIntent, error) {
	return scanIntent(s.db.QueryRowContext(ctx, `select `+intentColumns+` from acquisition_intents where id=$1`, id))
}
func (s *SQLDownloadStore) ListAcquisitions(ctx context.Context) ([]AcquisitionIntent, error) {
	rows, err := s.db.QueryContext(ctx, `select `+intentColumns+` from acquisition_intents where state in ('submitting','uncertain') or (state='accepted' and ((bookkeeping_required and bookkeeping_at is null) or not exists(select 1 from downloads d where d.client=acquisition_intents.client and d.external_id=acquisition_intents.external_id))) order by created_at,id limit 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []AcquisitionIntent{}
	for rows.Next() {
		i, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

// The transaction coordinates contenders only; no client IO runs under its lock.
func (s *SQLDownloadStore) ClaimAcquisition(ctx context.Context, candidate AcquisitionIntent) (AcquisitionIntent, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return candidate, false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,4))`, candidate.RequestKey); err != nil {
		return candidate, false, err
	}
	if _, err = tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,3))`, candidate.ScopeKey); err != nil {
		return candidate, false, err
	}
	existing, err := scanIntent(tx.QueryRowContext(ctx, `select `+intentColumns+` from acquisition_intents where scope_key=$1 and state<>'released' for update`, candidate.ScopeKey))
	if err == nil {
		if existing.State == "accepted" {
			// Explicit failed/deleted downloads release the slot. A completed import
			// permits a different upgrade release, but replaying the same request is a no-op.
			var terminal bool
			if err = tx.QueryRowContext(ctx, `select exists(select 1 from downloads where client=$1 and external_id=$2 and (state='removed' or failed_at is not null or (import_status='imported' and $3)))`, existing.Client, existing.ExternalID, existing.RequestKey != candidate.RequestKey).Scan(&terminal); err != nil {
				return existing, false, err
			}
			if terminal && (!existing.BookkeepingRequired || existing.BookkeepingAt != nil) {
				if _, err = tx.ExecContext(ctx, `update acquisition_intents set state='released',updated_at=now() where id=$1`, existing.ID); err != nil {
					return existing, false, err
				}
			} else {
				if existing.RequestKey != candidate.RequestKey {
					return existing, false, ErrAcquisitionActive
				}
				return existing, false, tx.Commit()
			}
		} else {
			// Expiry never authorizes another add: the previous process may have sent it.
			return existing, false, tx.Commit()
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return candidate, false, err
	}
	// An earlier raw grab may own this release under a different book scope.
	// Retire only an explicitly failed/deleted receipt; an unknown or active
	// download must still block a second submission.
	if _, err := tx.ExecContext(ctx, `update acquisition_intents a set state='released',updated_at=now() where a.request_key=$1 and a.state='accepted' and (not a.bookkeeping_required or a.bookkeeping_at is not null) and exists(select 1 from downloads d where d.client=a.client and d.external_id=a.external_id and (d.state='removed' or d.failed_at is not null))`, candidate.RequestKey); err != nil {
		return candidate, false, err
	}
	var otherRequest bool
	if err := tx.QueryRowContext(ctx, `select exists(select 1 from acquisition_intents where request_key=$1 and state<>'released')`, candidate.RequestKey).Scan(&otherRequest); err != nil {
		return candidate, false, err
	}
	if otherRequest {
		return candidate, false, ErrAcquisitionActive
	}
	if candidate.WantedID != "" {
		var status, format string
		var monitored bool
		if err := tx.QueryRowContext(ctx, `select status,wanted_format,monitored from wanted_items where id=$1 for update`, candidate.WantedID).Scan(&status, &format, &monitored); err != nil {
			return candidate, false, err
		}
		if status == "removed" || status == "ignored" || (candidate.Selection.Trigger != "" && candidate.Selection.Trigger != "manual" && !monitored) {
			return candidate, false, errors.New("this book is not eligible for acquisition")
		}
		candidate.Format = format
		var legacyActive bool
		if err := tx.QueryRowContext(ctx, `select exists(select 1 from downloads where $1=any(string_to_array(tags,',')) and state<>'removed' and failed_at is null and import_status<>'imported')`, "wanted:"+candidate.WantedID).Scan(&legacyActive); err != nil {
			return candidate, false, err
		}
		if legacyActive {
			return candidate, false, ErrAcquisitionActive
		}
	}
	if candidate.Selection.ReleaseID != "" {
		if candidate.WantedID == "" {
			return candidate, false, errors.New("release selection requires a wanted book")
		}
		err := tx.QueryRowContext(ctx, `select title,score,source_id,coalesce(rejected_reason,'') from releases where id=$1 and wanted_item_id=$2`, candidate.Selection.ReleaseID, candidate.WantedID).Scan(&candidate.Selection.Title, &candidate.Selection.Score, &candidate.Selection.SourceID, &candidate.Selection.RejectedReason)
		if err != nil {
			return candidate, false, fmt.Errorf("load selected release: %w", err)
		}
	}
	selection, err := json.Marshal(candidate.Selection)
	if err != nil {
		return candidate, false, err
	}
	inserted, err := scanIntent(tx.QueryRowContext(ctx, `insert into acquisition_intents(scope_key,request_key,wanted_item_id,media_format,client,endpoint_hash,info_hash,title,category,state,lease_token,lease_expires_at,selection)
 values($1,$2,nullif($3,'')::uuid,$4,$5,$6,$7,$8,$9,'submitting',gen_random_uuid(),clock_timestamp()+interval '2 minutes',$10::jsonb) returning `+intentColumns, candidate.ScopeKey, candidate.RequestKey, candidate.WantedID, candidate.Format, candidate.Client, candidate.EndpointHash, candidate.InfoHash, candidate.Title, candidate.Category, string(selection)))
	if err != nil {
		return candidate, false, err
	}
	return inserted, true, tx.Commit()
}

func (s *SQLDownloadStore) ClaimAcquisitionRecovery(ctx context.Context, id string) (AcquisitionIntent, error) {
	i, err := scanIntent(s.db.QueryRowContext(ctx, `update acquisition_intents set state='uncertain',lease_token=gen_random_uuid(),lease_expires_at=clock_timestamp()+interval '2 minutes',attempts=attempts+1,updated_at=now()
 where id=$1 and state in ('submitting','uncertain') and (lease_expires_at is null or lease_expires_at<clock_timestamp()) and (next_check_at is null or next_check_at<=clock_timestamp()) returning `+intentColumns, id))
	if errors.Is(err, sql.ErrNoRows) {
		return i, ErrAcquisitionBusy
	}
	return i, err
}

func (s *SQLDownloadStore) AcceptAcquisition(ctx context.Context, i AcquisitionIntent, status DownloadStatus) error {
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `update acquisition_intents set state='accepted',external_id=$3,result=$4::jsonb,last_error='',lease_token=null,lease_expires_at=null,next_check_at=null,updated_at=now() where id=$1 and lease_token=$2 and lease_expires_at>clock_timestamp() and state in ('submitting','uncertain')`, i.ID, i.LeaseToken, status.ID, string(raw))
	return checkIntentUpdate(result, err)
}
func (s *SQLDownloadStore) UncertainAcquisition(ctx context.Context, i AcquisitionIntent, message string) error {
	result, err := s.db.ExecContext(ctx, `update acquisition_intents set state='uncertain',last_error=$3,lease_token=null,lease_expires_at=null,next_check_at=clock_timestamp()+least(300,5*power(2,least(attempts-1,6))) * interval '1 second',updated_at=now() where id=$1 and lease_token=$2 and state in ('submitting','uncertain')`, i.ID, i.LeaseToken, message)
	return checkIntentUpdate(result, err)
}
func (s *SQLDownloadStore) ReleaseAcquisition(ctx context.Context, id string) error {
	// This is an explicit operator decision after inspection, never an empty-list inference.
	result, err := s.db.ExecContext(ctx, `update acquisition_intents set state='released',lease_token=null,lease_expires_at=null,last_error='Operator confirmed this acquisition may be retried',updated_at=now() where id=$1 and state in ('submitting','uncertain') and (lease_expires_at is null or lease_expires_at<clock_timestamp())`, id)
	return checkIntentUpdate(result, err)
}
func checkIntentUpdate(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrAcquisitionBusy
	}
	return nil
}

func intentError(i AcquisitionIntent, err error) error {
	return fmt.Errorf("acquisition %s: %w", i.ID, err)
}

// Client lists are observations. They cannot erase the durable book association
// when an adapter (notably SABnzbd) does not round-trip arbitrary tags.
func (s *SQLDownloadStore) restoreAcquisitionLinks(ctx context.Context, downloads []DownloadStatus) error {
	type identity struct {
		Client string `json:"client"`
		ID     string `json:"id"`
	}
	identities := make([]identity, 0, len(downloads))
	indices := map[string][]int{}
	for n, d := range downloads {
		identities = append(identities, identity{d.Client, d.ID})
		key := downloadStateKey(d.Client, d.ID)
		indices[key] = append(indices[key], n)
	}
	raw, err := json.Marshal(identities)
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `select a.id::text,a.client,a.external_id,coalesce(a.wanted_item_id::text,'') from acquisition_intents a join jsonb_to_recordset($1::jsonb) as d(client text,id text) on a.client=d.client and a.external_id=d.id where a.external_id<>'' and a.state in ('accepted','released')`, string(raw))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, client, external, wanted string
		if err := rows.Scan(&id, &client, &external, &wanted); err != nil {
			return err
		}
		for _, n := range indices[downloadStateKey(client, external)] {
			downloads[n].Tags = compactStrings(append(downloads[n].Tags, "librarry", "librarry-intent:"+id))
			if wanted != "" {
				downloads[n].Tags = compactStrings(append(downloads[n].Tags, "wanted:"+wanted))
			}
		}
	}
	return rows.Err()
}

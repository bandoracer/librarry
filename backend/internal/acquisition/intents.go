package acquisition

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var wantedTagID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func acquisitionHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func torrentIdentity(request DownloadRequest) string {
	value := strings.TrimSpace(request.InfoHash)
	if len(request.UploadData) > 0 {
		if hash, err := torrentInfoHashV1(request.UploadData); err == nil {
			return hash
		}
	}
	if u, err := url.Parse(request.ReleaseURL); err == nil && strings.EqualFold(u.Scheme, "magnet") {
		for _, xt := range u.Query()["xt"] {
			if strings.HasPrefix(strings.ToLower(xt), "urn:btih:") {
				value = xt[len("urn:btih:"):]
				break
			}
		}
	}
	if len(value) == 32 {
		if raw, err := base32.StdEncoding.DecodeString(strings.ToUpper(value)); err == nil {
			value = hex.EncodeToString(raw)
		}
	}
	if len(value) != 40 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return strings.ToLower(value)
}
func (s *integrationState) clientEndpoint(name string) string {
	switch name {
	case s.qbit.Name():
		return s.qbit.baseURL
	case s.trans.Name():
		return s.trans.baseURL
	case s.sab.Name():
		return s.sab.baseURL
	}
	return ""
}
func (s *integrationState) acquisitionCandidate(request DownloadRequest, client downloadClient) (AcquisitionIntent, error) {
	name := clientName(client)
	i := AcquisitionIntent{Client: name, Title: request.Title, Category: request.Category, InfoHash: torrentIdentity(request), EndpointHash: acquisitionHash(strings.TrimRight(s.clientEndpoint(name), "/"))}
	for _, tag := range request.Tags {
		if strings.HasPrefix(tag, "wanted:") {
			id := strings.TrimPrefix(tag, "wanted:")
			if !wantedTagID.MatchString(id) {
				return i, errors.New("wanted tag must identify a book")
			}
			id = strings.ToLower(id)
			if i.WantedID != "" && i.WantedID != id {
				return i, errors.New("one acquisition cannot target several wanted books")
			}
			i.WantedID = id
		}
	}
	if request.Category == s.ebookCategory() {
		i.Format = "ebook"
	} else if request.Category == s.audiobookCategory() {
		i.Format = "audiobook"
	}
	payload := i.InfoHash
	if payload == "" {
		payload = acquisitionHash(request.ReleaseURL + "\x00" + string(request.UploadData))
	}
	i.RequestKey = acquisitionHash(name + "\x00" + i.EndpointHash + "\x00" + payload)
	// A wanted row already represents its requested format. Keeping this key
	// independent of category and client prevents raw/manual bypasses and races
	// between workers choosing different clients or changing category settings.
	i.ScopeKey = acquisitionHash("release:" + i.RequestKey)
	if i.WantedID != "" {
		i.ScopeKey = acquisitionHash("wanted:" + i.WantedID)
	}
	return i, nil
}

func (s *integrationState) grabWithIntent(ctx context.Context, request DownloadRequest, client downloadClient, store acquisitionIntentStore) (DownloadStatus, error) {
	candidate, err := s.acquisitionCandidate(request, client)
	if err != nil {
		return DownloadStatus{}, err
	}
	i, claimed, err := store.ClaimAcquisition(ctx, candidate)
	if err != nil {
		return DownloadStatus{}, err
	}
	if !claimed {
		// Returning another release's receipt would let the wanted caller record
		// its newly selected release even though the older request was submitted.
		if i.RequestKey != candidate.RequestKey {
			return DownloadStatus{}, ErrAcquisitionActive
		}
		if i.State == "accepted" && i.Result != nil {
			status := *i.Result
			status.AcquisitionID = i.ID
			status.Deduplicated = true
			return s.replayAcceptedAcquisition(ctx, status)
		}
		return s.reconcileAcquisition(ctx, i.ID, "")
	}
	request.InfoHash = firstNonEmpty(i.InfoHash, request.InfoHash)
	request.Tags = append(compactStrings(request.Tags), "librarry-intent:"+i.ID)
	// Bound the send below the claim lifetime. Expiry still only permits a read
	// reconciliation: cancellation cannot prove that the client rejected an add.
	sendCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	status, err := client.Add(sendCtx, request)
	if err != nil {
		return DownloadStatus{}, s.markAcquisitionUncertain(ctx, store, i, "The client may have accepted the add request. Check its queue before retrying.")
	}
	status.AcquisitionID = i.ID
	if !acceptedClientIdentity(status) {
		// Older qBittorrent replies only say Ok. A URL hash is not a torrent ID.
		return s.reconcileClaimedAcquisition(ctx, store, i, "")
	}
	return s.persistAcquisitionAcceptance(ctx, store, i, status)
}
func acceptedClientIdentity(status DownloadStatus) bool {
	if status.Client == "qBittorrent" || status.Client == "Transmission" {
		return torrentIdentity(DownloadRequest{InfoHash: status.ID}) != ""
	}
	return status.Client == "SABnzbd" && strings.TrimSpace(status.ID) != ""
}
func (s *integrationState) persistAcquisitionAcceptance(ctx context.Context, store acquisitionIntentStore, i AcquisitionIntent, status DownloadStatus) (DownloadStatus, error) {
	status.AcquisitionID = i.ID
	if strings.HasPrefix(status.Name, "http://") || strings.HasPrefix(status.Name, "https://") || strings.HasPrefix(status.Name, "magnet:") {
		status.Name = "Download"
	}
	// A caller disconnect must not erase a known remote acceptance.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := store.AcceptAcquisition(saveCtx, i, status); err != nil {
		saveErr := s.markAcquisitionUncertain(saveCtx, store, i, "The client accepted the request but its receipt could not be saved. Reconcile before retrying.")
		return status, errors.Join(err, saveErr)
	}
	if err := s.storeDownloads(saveCtx, []DownloadStatus{status}); err != nil {
		return status, fmt.Errorf("download accepted; acquisition %s retained its receipt but download persistence needs retry: %w", i.ID, err)
	}
	return status, nil
}
func (s *integrationState) markAcquisitionUncertain(ctx context.Context, store acquisitionIntentStore, i AcquisitionIntent, message string) error {
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return errors.Join(intentError(i, ErrAcquisitionUncertain), store.UncertainAcquisition(saveCtx, i, message))
}

func (s *Service) AcquisitionRecovery(ctx context.Context) ([]AcquisitionIntent, error) {
	state := s.current.Load()
	store, ok := state.store.(acquisitionIntentStore)
	if !ok {
		return []AcquisitionIntent{}, nil
	}
	return store.ListAcquisitions(ctx)
}
func (s *Service) ReconcileAcquisition(ctx context.Context, id, downloadID string) (DownloadStatus, error) {
	return s.current.Load().reconcileAcquisition(ctx, id, downloadID)
}
func (s *Service) ReleaseAcquisition(ctx context.Context, id string) error {
	store, ok := s.current.Load().store.(acquisitionIntentStore)
	if !ok {
		return ErrAcquisitionPersistence
	}
	return store.ReleaseAcquisition(ctx, id)
}
func (s *integrationState) reconcileAcquisition(ctx context.Context, id, downloadID string) (DownloadStatus, error) {
	store, ok := s.store.(acquisitionIntentStore)
	if !ok {
		return DownloadStatus{}, ErrAcquisitionPersistence
	}
	i, err := store.GetAcquisition(ctx, id)
	if err != nil {
		return DownloadStatus{}, err
	}
	if i.State == "accepted" && i.Result != nil {
		status := *i.Result
		status.AcquisitionID = i.ID
		status.Deduplicated = true
		return s.replayAcceptedAcquisition(ctx, status)
	}
	i, err = store.ClaimAcquisitionRecovery(ctx, id)
	if err != nil {
		return DownloadStatus{}, err
	}
	return s.reconcileClaimedAcquisition(ctx, store, i, downloadID)
}
func (s *integrationState) reconcileClaimedAcquisition(ctx context.Context, store acquisitionIntentStore, i AcquisitionIntent, downloadID string) (DownloadStatus, error) {
	if i.EndpointHash != acquisitionHash(strings.TrimRight(s.clientEndpoint(i.Client), "/")) {
		return DownloadStatus{}, s.markAcquisitionUncertain(ctx, store, i, "Client address changed. Restore the original client connection before reconciling this acquisition.")
	}
	query := DownloadListQuery{Client: i.Client}
	if downloadID != "" {
		query.IDs = []string{downloadID}
	}
	var statuses []DownloadStatus
	var err error
	// Direct adapter reads deliberately bypass the cached/partial aggregate list.
	switch i.Client {
	case s.qbit.Name():
		statuses, err = s.qbit.List(ctx, query)
	case s.trans.Name():
		statuses, err = s.trans.List(ctx, query)
	case s.sab.Name():
		statuses, err = s.sab.List(ctx, query)
	default:
		err = ErrIntegrationNotConfigured
	}
	if err != nil {
		return DownloadStatus{}, s.markAcquisitionUncertain(ctx, store, i, "The original client could not be checked. No replacement download was submitted.")
	}
	matches := []DownloadStatus{}
	for _, status := range statuses {
		if status.Client != i.Client {
			continue
		}
		match := downloadID != "" && status.ID == downloadID
		if downloadID == "" {
			match = i.InfoHash != "" && strings.EqualFold(i.InfoHash, status.ID)
			for _, tag := range status.Tags {
				if tag == "librarry-intent:"+i.ID {
					match = true
				}
			}
		}
		if match {
			matches = append(matches, status)
		}
	}
	if len(matches) != 1 {
		return DownloadStatus{}, s.markAcquisitionUncertain(ctx, store, i, "No unique matching download was found. Inspect the client, then attach its exact ID or explicitly allow a new attempt.")
	}
	status := matches[0]
	status.Deduplicated = true
	// Preserve the authoritative association even where the client lacks tags (SAB).
	status.Tags = compactStrings(append(status.Tags, "librarry", "librarry-intent:"+i.ID))
	if i.WantedID != "" {
		status.Tags = compactStrings(append(status.Tags, "wanted:"+i.WantedID))
	}
	return s.persistAcquisitionAcceptance(ctx, store, i, status)
}

// Replaying a receipt must not reset a live/imported row to the original queued
// snapshot. Only repair a missing persistence write.
func (s *integrationState) replayAcceptedAcquisition(ctx context.Context, status DownloadStatus) (DownloadStatus, error) {
	rows, err := s.store.ListDownloads(ctx, DownloadListQuery{Client: status.Client, IDs: []string{status.ID}})
	if err != nil {
		return status, err
	}
	for _, row := range rows {
		if row.Client == status.Client && row.ID == status.ID {
			row.AcquisitionID = status.AcquisitionID
			row.Deduplicated = true
			return row, nil
		}
	}
	return status, s.storeDownloads(ctx, []DownloadStatus{status})
}

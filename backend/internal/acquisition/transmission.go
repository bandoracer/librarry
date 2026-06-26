package acquisition

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const transmissionRPCPath = "/transmission/rpc"

type TransmissionClient struct {
	baseURL   string
	username  string
	password  string
	sessionID string
	client    *http.Client
}

func NewTransmissionClient(baseURL string, username string, password string, client *http.Client) *TransmissionClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &TransmissionClient{
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		client:   client,
	}
}

func (c *TransmissionClient) Name() string { return "Transmission" }

func (c *TransmissionClient) Configured() bool {
	return c.baseURL != ""
}

func (c *TransmissionClient) Health(ctx context.Context) IntegrationHealth {
	if !c.Configured() {
		return IntegrationHealth{Name: c.Name(), Configured: false, Status: "missing_credentials", Message: "Set LIBRARRY_TRANSMISSION_URL."}
	}
	var payload struct {
		Version    string `json:"version"`
		RPCVersion int    `json:"rpc-version"`
	}
	if err := c.rpc(ctx, "session-get", map[string]any{}, &payload); err != nil {
		return IntegrationHealth{Name: c.Name(), Configured: true, Status: "error", Message: err.Error()}
	}
	message := "Ready"
	if payload.Version != "" {
		message = "Ready; version " + payload.Version
	}
	return IntegrationHealth{Name: c.Name(), Configured: true, Status: "ready", Message: message}
}

func (c *TransmissionClient) Add(ctx context.Context, request DownloadRequest) (DownloadStatus, error) {
	if !c.Configured() {
		return DownloadStatus{}, ErrIntegrationNotConfigured
	}
	if strings.TrimSpace(request.ReleaseURL) == "" && len(request.UploadData) == 0 {
		return DownloadStatus{}, errors.New("releaseUrl or torrent upload is required")
	}
	args := map[string]any{
		"paused": request.Paused,
	}
	if len(request.UploadData) > 0 {
		args["metainfo"] = base64.StdEncoding.EncodeToString(request.UploadData)
	} else {
		args["filename"] = request.ReleaseURL
	}
	if strings.TrimSpace(request.SavePath) != "" {
		args["download-dir"] = request.SavePath
	}
	var payload map[string]transmissionTorrentAdded
	if err := c.rpc(ctx, "torrent-add", args, &payload); err != nil {
		return DownloadStatus{}, err
	}
	torrent := payload["torrent-added"]
	if torrent.ID == 0 && torrent.HashString == "" {
		torrent = payload["torrent-duplicate"]
	}
	id := firstNonEmpty(torrent.HashString, strconv.Itoa(torrent.ID), request.InfoHash, releaseID(request.ReleaseURL))
	labels := transmissionLabels(request)
	if len(labels) > 0 && strings.TrimSpace(id) != "" {
		_ = c.rpc(ctx, "torrent-set", map[string]any{"ids": []string{id}, "labels": labels}, nil)
	}
	now := time.Now().UTC()
	return DownloadStatus{
		Client:     c.Name(),
		ID:         id,
		Name:       firstNonEmpty(torrent.Name, request.Title, request.ReleaseURL),
		State:      "queued",
		Progress:   0,
		SavePath:   request.SavePath,
		Category:   request.Category,
		Tags:       labels,
		LastSeenAt: &now,
	}, nil
}

func (c *TransmissionClient) List(ctx context.Context, query DownloadListQuery) ([]DownloadStatus, error) {
	if !c.Configured() {
		return nil, ErrIntegrationNotConfigured
	}
	args := map[string]any{
		"fields": transmissionStatusFields(),
	}
	if len(compactStrings(query.IDs)) > 0 {
		args["ids"] = compactStrings(query.IDs)
	}
	var payload struct {
		Torrents []transmissionTorrent `json:"torrents"`
	}
	if err := c.rpc(ctx, "torrent-get", args, &payload); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	statuses := make([]DownloadStatus, 0, len(payload.Torrents))
	for _, torrent := range payload.Torrents {
		status := torrent.DownloadStatus(now)
		if includeTransmissionStatus(status, query) {
			statuses = append(statuses, status)
		}
	}
	return statuses, nil
}

func (c *TransmissionClient) Details(ctx context.Context, id string) (DownloadDetails, error) {
	if !c.Configured() {
		return DownloadDetails{}, ErrIntegrationNotConfigured
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DownloadDetails{}, errors.New("download id is required")
	}
	args := map[string]any{
		"fields": append(transmissionStatusFields(),
			"dateCreated",
			"creator",
			"comment",
			"secondsDownloading",
			"secondsSeeding",
			"downloadLimit",
			"downloadLimited",
			"uploadLimit",
			"uploadLimited",
			"pieceCount",
			"pieceSize",
			"sizeWhenDone",
			"haveValid",
			"files",
			"fileStats",
			"trackers",
			"trackerStats",
			"peers",
		),
		"ids": []string{id},
	}
	var payload struct {
		Torrents []transmissionTorrentDetail `json:"torrents"`
	}
	if err := c.rpc(ctx, "torrent-get", args, &payload); err != nil {
		return DownloadDetails{}, err
	}
	if len(payload.Torrents) == 0 {
		return DownloadDetails{}, ErrDownloadNotFound
	}
	return payload.Torrents[0].DownloadDetails(time.Now().UTC()), nil
}

func (c *TransmissionClient) Action(ctx context.Context, request DownloadActionRequest) (DownloadActionResult, error) {
	if !c.Configured() {
		return DownloadActionResult{}, ErrIntegrationNotConfigured
	}
	ids := compactStrings(request.IDs)
	if len(ids) == 0 {
		return DownloadActionResult{}, errors.New("at least one download id is required")
	}
	action := normalizeAction(request.Action)
	args := map[string]any{"ids": ids}
	switch action {
	case DownloadActionStart:
		if err := c.rpc(ctx, "torrent-start", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionStop:
		if err := c.rpc(ctx, "torrent-stop", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionDelete:
		args["delete-local-data"] = request.DeleteFiles
		if err := c.rpc(ctx, "torrent-remove", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionRecheck:
		if err := c.rpc(ctx, "torrent-verify", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetLocation:
		location := strings.TrimSpace(request.SavePath)
		if location == "" {
			return DownloadActionResult{}, errors.New("savePath is required for setLocation")
		}
		args["location"] = location
		args["move"] = true
		if err := c.rpc(ctx, "torrent-set-location", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetDownloadLimit:
		if request.DownloadLimit < 0 {
			return DownloadActionResult{}, errors.New("downloadLimit must be zero or greater")
		}
		args["downloadLimited"] = request.DownloadLimit > 0
		args["downloadLimit"] = bytesPerSecondToKiB(request.DownloadLimit)
		if err := c.rpc(ctx, "torrent-set", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	case DownloadActionSetUploadLimit:
		if request.UploadLimit < 0 {
			return DownloadActionResult{}, errors.New("uploadLimit must be zero or greater")
		}
		args["uploadLimited"] = request.UploadLimit > 0
		args["uploadLimit"] = bytesPerSecondToKiB(request.UploadLimit)
		if err := c.rpc(ctx, "torrent-set", args, nil); err != nil {
			return DownloadActionResult{}, err
		}
	default:
		return DownloadActionResult{}, fmt.Errorf("unsupported Transmission download action %q", request.Action)
	}
	return DownloadActionResult{Action: action, IDs: ids, Applied: true}, nil
}

func (c *TransmissionClient) FileAction(ctx context.Context, request DownloadFileActionRequest) (DownloadFileActionResult, error) {
	if !c.Configured() {
		return DownloadFileActionResult{}, ErrIntegrationNotConfigured
	}
	id := strings.TrimSpace(request.DownloadID)
	if id == "" {
		return DownloadFileActionResult{}, errors.New("download id is required")
	}
	fileIDs := compactInts(request.IDs)
	if len(fileIDs) == 0 {
		return DownloadFileActionResult{}, errors.New("at least one file id is required")
	}
	action, priority, err := normalizeFilePriorityAction(request.Action, request.Priority)
	if err != nil {
		return DownloadFileActionResult{}, err
	}
	args := map[string]any{"ids": []string{id}}
	switch priority {
	case 0:
		args["files-unwanted"] = fileIDs
	case 1:
		args["files-wanted"] = fileIDs
		args["priority-normal"] = fileIDs
	case 6, 7:
		args["files-wanted"] = fileIDs
		args["priority-high"] = fileIDs
	default:
		return DownloadFileActionResult{}, fmt.Errorf("unsupported Transmission file priority %d", priority)
	}
	if err := c.rpc(ctx, "torrent-set", args, nil); err != nil {
		return DownloadFileActionResult{}, err
	}
	return DownloadFileActionResult{
		Action:     action,
		DownloadID: id,
		IDs:        fileIDs,
		Priority:   priority,
		Applied:    true,
	}, nil
}

func (c *TransmissionClient) rpc(ctx context.Context, method string, arguments map[string]any, target any) error {
	requestPayload := map[string]any{
		"method":    method,
		"arguments": arguments,
	}
	body, err := json.Marshal(requestPayload)
	if err != nil {
		return err
	}
	responseBody, err := c.doRPC(ctx, body, true)
	if err != nil {
		return err
	}
	var response struct {
		Result    string          `json:"result"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return fmt.Errorf("Transmission RPC decode failed: %w", err)
	}
	if response.Result != "" && response.Result != "success" {
		return fmt.Errorf("Transmission RPC %s failed: %s", method, response.Result)
	}
	if target != nil && len(response.Arguments) > 0 {
		if err := json.Unmarshal(response.Arguments, target); err != nil {
			return fmt.Errorf("Transmission RPC %s arguments decode failed: %w", method, err)
		}
	}
	return nil
}

func (c *TransmissionClient) doRPC(ctx context.Context, body []byte, retrySession bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		req.Header.Set("X-Transmission-Session-Id", c.sessionID)
	}
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict && retrySession {
		sessionID := strings.TrimSpace(resp.Header.Get("X-Transmission-Session-Id"))
		if sessionID == "" {
			return nil, errors.New("Transmission RPC returned 409 without session id")
		}
		c.sessionID = sessionID
		return c.doRPC(ctx, body, false)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Transmission RPC returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func (c *TransmissionClient) rpcURL() string {
	if strings.HasSuffix(c.baseURL, transmissionRPCPath) {
		return c.baseURL
	}
	return c.baseURL + transmissionRPCPath
}

type transmissionTorrentAdded struct {
	ID         int    `json:"id"`
	HashString string `json:"hashString"`
	Name       string `json:"name"`
}

type transmissionTorrent struct {
	ID                 int      `json:"id"`
	HashString         string   `json:"hashString"`
	Name               string   `json:"name"`
	Status             int      `json:"status"`
	PercentDone        float64  `json:"percentDone"`
	DownloadDir        string   `json:"downloadDir"`
	TotalSize          int64    `json:"totalSize"`
	DownloadedEver     int64    `json:"downloadedEver"`
	UploadedEver       int64    `json:"uploadedEver"`
	RateDownload       int64    `json:"rateDownload"`
	RateUpload         int64    `json:"rateUpload"`
	ETA                int64    `json:"eta"`
	UploadRatio        float64  `json:"uploadRatio"`
	PeersConnected     int      `json:"peersConnected"`
	PeersGettingFromUs int      `json:"peersGettingFromUs"`
	PeersSendingToUs   int      `json:"peersSendingToUs"`
	AddedDate          int64    `json:"addedDate"`
	DoneDate           int64    `json:"doneDate"`
	ActivityDate       int64    `json:"activityDate"`
	Error              int      `json:"error"`
	ErrorString        string   `json:"errorString"`
	Labels             []string `json:"labels"`
}

type transmissionTorrentDetail struct {
	transmissionTorrent
	DateCreated        int64                     `json:"dateCreated"`
	Creator            string                    `json:"creator"`
	Comment            string                    `json:"comment"`
	SecondsDownloading int64                     `json:"secondsDownloading"`
	SecondsSeeding     int64                     `json:"secondsSeeding"`
	DownloadLimit      int64                     `json:"downloadLimit"`
	DownloadLimited    bool                      `json:"downloadLimited"`
	UploadLimit        int64                     `json:"uploadLimit"`
	UploadLimited      bool                      `json:"uploadLimited"`
	PieceCount         int                       `json:"pieceCount"`
	PieceSize          int64                     `json:"pieceSize"`
	SizeWhenDone       int64                     `json:"sizeWhenDone"`
	HaveValid          int64                     `json:"haveValid"`
	Files              []transmissionFile        `json:"files"`
	FileStats          []transmissionFileStat    `json:"fileStats"`
	Trackers           []transmissionTracker     `json:"trackers"`
	TrackerStats       []transmissionTrackerStat `json:"trackerStats"`
	Peers              []transmissionPeer        `json:"peers"`
}

type transmissionFile struct {
	Name           string `json:"name"`
	Length         int64  `json:"length"`
	BytesCompleted int64  `json:"bytesCompleted"`
}

type transmissionFileStat struct {
	BytesCompleted int64 `json:"bytesCompleted"`
	Wanted         bool  `json:"wanted"`
	Priority       int   `json:"priority"`
}

type transmissionTracker struct {
	ID       int    `json:"id"`
	Announce string `json:"announce"`
	Scrape   string `json:"scrape"`
	Tier     int    `json:"tier"`
	SiteName string `json:"sitename"`
}

type transmissionTrackerStat struct {
	ID                    int    `json:"id"`
	Announce              string `json:"announce"`
	AnnounceState         int    `json:"announceState"`
	DownloadCount         int    `json:"downloadCount"`
	Host                  string `json:"host"`
	LastAnnouncePeerCount int    `json:"lastAnnouncePeerCount"`
	LastAnnounceResult    string `json:"lastAnnounceResult"`
	LastAnnounceSucceeded bool   `json:"lastAnnounceSucceeded"`
	LastAnnounceTimedOut  bool   `json:"lastAnnounceTimedOut"`
	LastScrapeResult      string `json:"lastScrapeResult"`
	LeecherCount          int    `json:"leecherCount"`
	Scrape                string `json:"scrape"`
	ScrapeState           int    `json:"scrapeState"`
	SeederCount           int    `json:"seederCount"`
	Tier                  int    `json:"tier"`
}

type transmissionPeer struct {
	Address            string  `json:"address"`
	ClientName         string  `json:"clientName"`
	ClientIsChoked     bool    `json:"clientIsChoked"`
	ClientIsInterested bool    `json:"clientIsInterested"`
	FlagStr            string  `json:"flagStr"`
	IsDownloadingFrom  bool    `json:"isDownloadingFrom"`
	IsEncrypted        bool    `json:"isEncrypted"`
	IsIncoming         bool    `json:"isIncoming"`
	IsUploadingTo      bool    `json:"isUploadingTo"`
	IsUTP              bool    `json:"isUTP"`
	PeerIsChoked       bool    `json:"peerIsChoked"`
	PeerIsInterested   bool    `json:"peerIsInterested"`
	Port               int     `json:"port"`
	Progress           float64 `json:"progress"`
	RateToClient       int64   `json:"rateToClient"`
	RateToPeer         int64   `json:"rateToPeer"`
}

func (t transmissionTorrent) DownloadStatus(now time.Time) DownloadStatus {
	progress := t.PercentDone
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	state := transmissionState(t.Status, progress)
	var failedAt *time.Time
	if t.Error != 0 || strings.TrimSpace(t.ErrorString) != "" {
		state = "error"
		value := now
		failedAt = &value
	}
	completedAt := unixPtr(t.DoneDate)
	if completedAt != nil && progress < 1 {
		completedAt = nil
	}
	downloaded := t.DownloadedEver
	if downloaded <= 0 && t.TotalSize > 0 {
		downloaded = int64(float64(t.TotalSize) * progress)
	}
	return DownloadStatus{
		Client:          "Transmission",
		ID:              firstNonEmpty(t.HashString, strconv.Itoa(t.ID)),
		Name:            t.Name,
		State:           state,
		Progress:        progress,
		SavePath:        t.DownloadDir,
		Category:        transmissionCategory(t.Labels),
		Tags:            compactStrings(t.Labels),
		SizeBytes:       t.TotalSize,
		DownloadedBytes: downloaded,
		UploadedBytes:   t.UploadedEver,
		DownloadRate:    t.RateDownload,
		UploadRate:      t.RateUpload,
		ETASeconds:      positiveInt64(t.ETA),
		Ratio:           t.UploadRatio,
		Seeders:         t.PeersGettingFromUs,
		Peers:           t.PeersConnected + t.PeersSendingToUs,
		AddedAt:         unixPtr(t.AddedDate),
		CompletedAt:     completedAt,
		LastActivityAt:  unixPtr(t.ActivityDate),
		LastSeenAt:      &now,
		FailureReason:   strings.TrimSpace(t.ErrorString),
		FailedAt:        failedAt,
	}
}

func (t transmissionTorrentDetail) DownloadDetails(now time.Time) DownloadDetails {
	status := t.transmissionTorrent.DownloadStatus(now)
	totalSize := firstPositiveInt64(t.SizeWhenDone, t.TotalSize, status.SizeBytes)
	pieceCount := t.PieceCount
	if pieceCount <= 0 && t.PieceSize > 0 && totalSize > 0 {
		pieceCount = int(math.Ceil(float64(totalSize) / float64(t.PieceSize)))
	}
	piecesHave := 0
	if pieceCount > 0 {
		piecesHave = int(math.Round(float64(pieceCount) * status.Progress))
	}
	properties := DownloadProperties{
		SavePath:           status.SavePath,
		CreationDate:       unixPtr(t.DateCreated),
		AdditionDate:       status.AddedAt,
		CompletionDate:     status.CompletedAt,
		TotalSizeBytes:     totalSize,
		TotalDownloaded:    t.DownloadedEver,
		TotalUploaded:      t.UploadedEver,
		DownloadLimit:      transmissionLimitBytes(t.DownloadLimit, t.DownloadLimited),
		UploadLimit:        transmissionLimitBytes(t.UploadLimit, t.UploadLimited),
		DownloadSpeed:      t.RateDownload,
		UploadSpeed:        t.RateUpload,
		ETASeconds:         positiveInt64(t.ETA),
		Ratio:              t.UploadRatio,
		Connections:        t.PeersConnected,
		TimeElapsedSeconds: t.SecondsDownloading,
		SeedingTimeSeconds: t.SecondsSeeding,
		PieceSizeBytes:     t.PieceSize,
		PiecesHave:         piecesHave,
		PiecesTotal:        pieceCount,
		CreatedBy:          strings.TrimSpace(t.Creator),
		Comment:            strings.TrimSpace(t.Comment),
	}
	return DownloadDetails{
		Status:     status,
		Properties: properties,
		Files:      t.DownloadFiles(),
		Trackers:   t.DownloadTrackers(),
		Peers:      t.DownloadPeers(),
	}
}

func (t transmissionTorrentDetail) DownloadFiles() []DownloadFile {
	files := make([]DownloadFile, 0, len(t.Files))
	for i, file := range t.Files {
		stat := transmissionFileStat{Wanted: true}
		if i < len(t.FileStats) {
			stat = t.FileStats[i]
		}
		files = append(files, DownloadFile{
			ID:        i,
			Name:      file.Name,
			SizeBytes: file.Length,
			Progress:  transmissionFileProgress(file, stat),
			Priority:  transmissionFilePriority(stat),
		})
	}
	return files
}

func (t transmissionTorrentDetail) DownloadTrackers() []DownloadTracker {
	if len(t.TrackerStats) > 0 {
		trackers := make([]DownloadTracker, 0, len(t.TrackerStats))
		for _, item := range t.TrackerStats {
			trackers = append(trackers, DownloadTracker{
				URL:        firstNonEmpty(item.Announce, item.Host, item.Scrape),
				StatusCode: item.AnnounceState,
				Status:     transmissionTrackerStatus(item),
				Tier:       item.Tier,
				Message:    transmissionTrackerMessage(item),
				Peers:      item.LastAnnouncePeerCount,
				Seeds:      item.SeederCount,
				Leeches:    item.LeecherCount,
				Downloads:  item.DownloadCount,
			})
		}
		return trackers
	}
	trackers := make([]DownloadTracker, 0, len(t.Trackers))
	for _, item := range t.Trackers {
		trackers = append(trackers, DownloadTracker{
			URL:        firstNonEmpty(item.Announce, item.Scrape, item.SiteName),
			StatusCode: 0,
			Status:     "unknown",
			Tier:       item.Tier,
		})
	}
	return trackers
}

func (t transmissionTorrentDetail) DownloadPeers() []DownloadPeer {
	peers := make([]DownloadPeer, 0, len(t.Peers))
	for _, item := range t.Peers {
		peers = append(peers, DownloadPeer{
			ID:           transmissionPeerID(item),
			IP:           item.Address,
			Port:         item.Port,
			Client:       strings.TrimSpace(item.ClientName),
			Connection:   transmissionPeerConnection(item),
			Flags:        strings.TrimSpace(item.FlagStr),
			Progress:     item.Progress,
			DownloadRate: item.RateToClient,
			UploadRate:   item.RateToPeer,
		})
	}
	return peers
}

func transmissionState(status int, progress float64) string {
	switch status {
	case 0:
		if progress >= 1 {
			return "completed"
		}
		return "stopped"
	case 1:
		return "queued"
	case 2:
		return "checking"
	case 3:
		return "queued"
	case 4:
		return "downloading"
	case 5:
		return "queued"
	case 6:
		return "seeding"
	default:
		return "unknown"
	}
}

func includeTransmissionStatus(status DownloadStatus, query DownloadListQuery) bool {
	if client := strings.TrimSpace(query.Client); client != "" && !strings.EqualFold(client, status.Client) {
		return false
	}
	if tag := strings.TrimSpace(query.Tag); tag != "" && !containsFold(status.Tags, tag) {
		return false
	}
	if category := strings.TrimSpace(query.Category); category != "" && !strings.EqualFold(status.Category, category) && !containsFold(status.Tags, category) {
		return false
	}
	return true
}

func containsFold(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
}

func transmissionCategory(labels []string) string {
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if strings.EqualFold(label, CategoryBooksEbook) || strings.EqualFold(label, CategoryBooksAudiobook) {
			return label
		}
	}
	return firstString(labels)
}

func transmissionLabels(request DownloadRequest) []string {
	seen := map[string]bool{}
	var labels []string
	for _, value := range append([]string{request.Category}, request.Tags...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		labels = append(labels, value)
	}
	return labels
}

func transmissionStatusFields() []string {
	return []string{
		"id", "hashString", "name", "status", "percentDone", "downloadDir",
		"totalSize", "downloadedEver", "uploadedEver", "rateDownload",
		"rateUpload", "eta", "uploadRatio", "peersConnected",
		"peersGettingFromUs", "peersSendingToUs", "addedDate", "doneDate",
		"activityDate", "error", "errorString", "labels",
	}
}

func unixPtr(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	t := time.Unix(value, 0).UTC()
	return &t
}

func positiveInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func bytesPerSecondToKiB(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(value) / 1024))
}

func transmissionLimitBytes(value int64, limited bool) int64 {
	if !limited || value <= 0 {
		return 0
	}
	return value * 1024
}

func transmissionFileProgress(file transmissionFile, stat transmissionFileStat) float64 {
	total := file.Length
	if total <= 0 {
		return 0
	}
	completed := stat.BytesCompleted
	if completed <= 0 {
		completed = file.BytesCompleted
	}
	progress := float64(completed) / float64(total)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

func transmissionFilePriority(stat transmissionFileStat) int {
	if !stat.Wanted {
		return 0
	}
	if stat.Priority > 0 {
		return 6
	}
	if stat.Priority < 0 {
		return -1
	}
	return 1
}

func transmissionTrackerStatus(stat transmissionTrackerStat) string {
	switch {
	case stat.LastAnnounceSucceeded:
		return "working"
	case stat.LastAnnounceTimedOut || strings.TrimSpace(stat.LastAnnounceResult) != "" || strings.TrimSpace(stat.LastScrapeResult) != "":
		return "not_working"
	case stat.AnnounceState == 1 || stat.ScrapeState == 1:
		return "not_contacted"
	case stat.AnnounceState == 2 || stat.ScrapeState == 2:
		return "updating"
	default:
		return "unknown"
	}
}

func transmissionTrackerMessage(stat transmissionTrackerStat) string {
	return firstNonEmpty(stat.LastAnnounceResult, stat.LastScrapeResult, stat.Host)
}

func transmissionPeerID(peer transmissionPeer) string {
	if peer.Address == "" {
		return ""
	}
	if peer.Port <= 0 {
		return peer.Address
	}
	return fmt.Sprintf("%s:%d", peer.Address, peer.Port)
}

func transmissionPeerConnection(peer transmissionPeer) string {
	var parts []string
	if peer.IsEncrypted {
		parts = append(parts, "encrypted")
	}
	if peer.IsIncoming {
		parts = append(parts, "incoming")
	} else {
		parts = append(parts, "outgoing")
	}
	if peer.IsUTP {
		parts = append(parts, "utp")
	}
	if peer.IsDownloadingFrom {
		parts = append(parts, "downloading")
	}
	if peer.IsUploadingTo {
		parts = append(parts, "uploading")
	}
	if peer.ClientIsChoked || peer.PeerIsChoked {
		parts = append(parts, "choked")
	}
	return strings.Join(parts, ", ")
}

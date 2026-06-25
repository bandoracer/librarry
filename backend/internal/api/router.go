package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/settings"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

type Dependencies struct {
	Logger   *slog.Logger
	Config   config.Config
	Metadata *metadata.Service
	Acquire  acquisitionService
	Wanted   wantedService
	Library  libraryService
	Compat   compatResourceService
}

type acquisitionService interface {
	Health(ctx context.Context) []acquisition.IntegrationHealth
	Bootstrap(ctx context.Context) (acquisition.BootstrapResult, error)
	Search(ctx context.Context, query acquisition.ReleaseSearchQuery) ([]acquisition.Release, error)
	Grab(ctx context.Context, request acquisition.DownloadRequest) (acquisition.DownloadStatus, error)
	Downloads(ctx context.Context, query acquisition.DownloadListQuery) ([]acquisition.DownloadStatus, error)
	DownloadDetails(ctx context.Context, id string, client string) (acquisition.DownloadDetails, error)
	DownloadAction(ctx context.Context, request acquisition.DownloadActionRequest) (acquisition.DownloadActionResult, error)
	DownloadFileAction(ctx context.Context, id string, request acquisition.DownloadFileActionRequest) (acquisition.DownloadFileActionResult, error)
	DownloadTrackerAction(ctx context.Context, id string, request acquisition.DownloadTrackerActionRequest) (acquisition.DownloadTrackerActionResult, error)
}

type wantedService interface {
	Create(ctx context.Context, request wanted.CreateRequest) (wanted.WantedItem, error)
	List(ctx context.Context, status string) ([]wanted.WantedItem, error)
	ListQualityProfiles(ctx context.Context) ([]wanted.QualityProfile, error)
	SaveQualityProfile(ctx context.Context, profile wanted.QualityProfile) (wanted.QualityProfile, error)
	SubscribeAuthor(ctx context.Context, request wanted.AuthorSubscribeRequest) (wanted.AuthorSubscription, error)
	ListAuthorSubscriptions(ctx context.Context, status string) ([]wanted.AuthorSubscription, error)
	MonitorAuthors(ctx context.Context, request wanted.AuthorMonitorRequest) (wanted.AuthorMonitorRun, error)
	SearchReleases(ctx context.Context, wantedID string, request wanted.SearchReleasesRequest) (wanted.SearchOutcome, error)
	ListReleases(ctx context.Context, wantedID string) (wanted.SearchOutcome, error)
	Grab(ctx context.Context, wantedID string, request wanted.GrabRequest) (acquisition.DownloadStatus, error)
	UpdateWanted(ctx context.Context, id string, request wanted.WantedUpdateRequest) (wanted.WantedItem, error)
	DeleteWanted(ctx context.Context, id string) error
	UpdateAuthorSubscription(ctx context.Context, id string, request wanted.AuthorUpdateRequest) (wanted.AuthorSubscription, error)
	DeleteAuthorSubscription(ctx context.Context, id string) error
	Monitor(ctx context.Context, request wanted.MonitorRequest) (wanted.MonitorRun, error)
	FeedSync(ctx context.Context, request wanted.FeedSyncRequest) (wanted.FeedSyncRun, error)
	RecoverFailedDownloads(ctx context.Context, request wanted.FailedDownloadRequest) (wanted.FailedDownloadRun, error)
	SearchUpgrades(ctx context.Context, request wanted.UpgradeRequest) (wanted.UpgradeRun, error)
	History(ctx context.Context, query wanted.HistoryQuery) ([]wanted.HistoryEvent, error)
}

type libraryService interface {
	ListFiles(ctx context.Context, query library.FileListQuery) ([]library.FileRecord, error)
	DeleteFiles(ctx context.Context, request library.DeleteFilesRequest) (library.DeleteFilesOutcome, error)
	PreviewRenameFiles(ctx context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error)
	RenameFiles(ctx context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error)
	ListImportReviews(ctx context.Context, query library.ReviewListQuery) ([]library.ImportReview, error)
	Scan(ctx context.Context, request library.ScanRequest) (library.ScanOutcome, error)
	Import(ctx context.Context, request library.ImportRequest) (library.ImportOutcome, error)
	ImportCompletedDownloads(ctx context.Context, downloads []acquisition.DownloadStatus, request library.CompletedImportRequest) (library.CompletedImportOutcome, error)
	ResolveImportReview(ctx context.Context, id string, request library.ReviewDecisionRequest) (library.ReviewDecisionOutcome, error)
}

type compatResourceService interface {
	ListRootFolders(ctx context.Context) ([]compatdata.RootFolder, error)
	CreateRootFolder(ctx context.Context, folder compatdata.RootFolder) (compatdata.RootFolder, error)
	DeleteRootFolder(ctx context.Context, id string) (bool, error)
	ListResources(ctx context.Context, resourceType string) ([]compatdata.Resource, error)
	GetResource(ctx context.Context, resourceType string, compatID int) (compatdata.Resource, bool, error)
	UpsertResource(ctx context.Context, resource compatdata.Resource) (compatdata.Resource, error)
	DeleteResource(ctx context.Context, resourceType string, compatID int) (bool, error)
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	handler := &handler{deps: deps}

	mux.HandleFunc("GET /ping", handler.compatPing)
	mux.HandleFunc("HEAD /ping", handler.compatPing)
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /api/v1/health", handler.compatHealth)
	mux.HandleFunc("GET /api/v1/system/status", handler.compatSystemStatus)
	mux.HandleFunc("GET /api/v1/system/routes", handler.compatSystemRoutes)
	mux.HandleFunc("GET /api/v1/system/routes/duplicate", handler.compatSystemDuplicateRoutes)
	mux.HandleFunc("GET /api/v1/system/backup", handler.compatSystemBackups)
	mux.HandleFunc("GET /api/v1/diskspace", handler.compatDiskspace)
	mux.HandleFunc("GET /api/v1/filesystem", handler.compatFilesystem)
	mux.HandleFunc("GET /api/v1/language", handler.compatLanguages)
	mux.HandleFunc("GET /api/v1/localization", handler.compatLocalization)
	mux.HandleFunc("GET /api/v1/localization/options", handler.compatLocalizationOptions)
	mux.HandleFunc("GET /api/v1/log", handler.compatLogs)
	mux.HandleFunc("GET /api/v1/log/file", handler.compatLogFiles)
	mux.HandleFunc("GET /api/v1/log/file/{filename}", handler.compatLogFile)
	mux.HandleFunc("GET /api/v1/update", handler.compatUpdates)
	mux.HandleFunc("GET /api/v1/config/naming", handler.compatNamingConfig)
	mux.HandleFunc("GET /api/v1/config/naming/{id}", handler.compatNamingConfig)
	mux.HandleFunc("PUT /api/v1/config/naming/{id}", handler.compatUpdateNamingConfig)
	mux.HandleFunc("GET /api/v1/config/naming/examples", handler.compatNamingConfigExamples)
	mux.HandleFunc("GET /api/v1/config/mediamanagement", handler.compatMediaManagementConfig)
	mux.HandleFunc("GET /api/v1/config/mediamanagement/{id}", handler.compatMediaManagementConfig)
	mux.HandleFunc("PUT /api/v1/config/mediamanagement/{id}", handler.compatUpdateMediaManagementConfig)
	mux.HandleFunc("GET /api/v1/config/host", handler.compatHostConfig)
	mux.HandleFunc("GET /api/v1/config/host/{id}", handler.compatHostConfig)
	mux.HandleFunc("PUT /api/v1/config/host/{id}", handler.compatUpdateHostConfig)
	mux.HandleFunc("GET /api/v1/config/ui", handler.compatUIConfig)
	mux.HandleFunc("GET /api/v1/config/ui/{id}", handler.compatUIConfig)
	mux.HandleFunc("PUT /api/v1/config/ui/{id}", handler.compatUpdateUIConfig)
	mux.HandleFunc("GET /api/v1/config/downloadclient", handler.compatDownloadClientConfig)
	mux.HandleFunc("GET /api/v1/config/downloadclient/{id}", handler.compatDownloadClientConfig)
	mux.HandleFunc("PUT /api/v1/config/downloadclient/{id}", handler.compatUpdateDownloadClientConfig)
	mux.HandleFunc("GET /api/v1/config/indexer", handler.compatIndexerConfig)
	mux.HandleFunc("GET /api/v1/config/indexer/{id}", handler.compatIndexerConfig)
	mux.HandleFunc("PUT /api/v1/config/indexer/{id}", handler.compatUpdateIndexerConfig)
	mux.HandleFunc("GET /api/v1/calendar", handler.compatCalendar)
	mux.HandleFunc("GET /api/v1/history", handler.compatHistory)
	mux.HandleFunc("GET /api/v1/history/since", handler.compatHistorySince)
	mux.HandleFunc("GET /api/v1/history/author", handler.compatHistoryAuthors)
	mux.HandleFunc("GET /api/v1/history/book", handler.compatHistoryBooks)
	mux.HandleFunc("GET /api/v1/parse", handler.compatParse)
	mux.HandleFunc("GET /api/v1/rootfolder", handler.compatRootFolders)
	mux.HandleFunc("GET /api/v1/rootfolder/{id}", handler.compatRootFolder)
	mux.HandleFunc("POST /api/v1/rootfolder", handler.compatCreateRootFolder)
	mux.HandleFunc("DELETE /api/v1/rootfolder/{id}", handler.compatDeleteRootFolder)
	mux.HandleFunc("GET /api/v1/queue", handler.compatQueue)
	mux.HandleFunc("GET /api/v1/queue/details", handler.compatQueueDetails)
	mux.HandleFunc("GET /api/v1/queue/status", handler.compatQueueStatus)
	mux.HandleFunc("DELETE /api/v1/queue/{id}", handler.compatDeleteQueue)
	mux.HandleFunc("DELETE /api/v1/queue/bulk", handler.compatDeleteQueueBulk)
	mux.HandleFunc("GET /api/v1/blocklist", handler.compatBlocklist)
	mux.HandleFunc("DELETE /api/v1/blocklist/bulk", handler.compatDeleteBlocklistBulk)
	mux.HandleFunc("DELETE /api/v1/blocklist/{id}", handler.compatDeleteBlocklist)
	mux.HandleFunc("GET /api/v1/blacklist", handler.compatBlocklist)
	mux.HandleFunc("DELETE /api/v1/blacklist/bulk", handler.compatDeleteBlocklistBulk)
	mux.HandleFunc("DELETE /api/v1/blacklist/{id}", handler.compatDeleteBlocklist)
	mux.HandleFunc("GET /api/v1/author/lookup", handler.compatAuthorLookup)
	mux.HandleFunc("GET /api/v1/author", handler.compatAuthors)
	mux.HandleFunc("POST /api/v1/author", handler.compatCreateAuthor)
	mux.HandleFunc("GET /api/v1/author/{id}", handler.compatAuthor)
	mux.HandleFunc("PUT /api/v1/author/{id}", handler.compatUpdateAuthor)
	mux.HandleFunc("DELETE /api/v1/author/{id}", handler.compatDeleteAuthor)
	mux.HandleFunc("GET /api/v1/book/lookup", handler.compatBookLookup)
	mux.HandleFunc("PUT /api/v1/book/monitor", handler.compatMonitorBooks)
	mux.HandleFunc("GET /api/v1/book", handler.compatBooks)
	mux.HandleFunc("POST /api/v1/book", handler.compatCreateBook)
	mux.HandleFunc("GET /api/v1/book/{id}", handler.compatBook)
	mux.HandleFunc("GET /api/v1/book/{id}/overview", handler.compatBookOverview)
	mux.HandleFunc("PUT /api/v1/book/{id}", handler.compatUpdateBook)
	mux.HandleFunc("DELETE /api/v1/book/{id}", handler.compatDeleteBook)
	mux.HandleFunc("GET /api/v1/bookfile", handler.compatBookFiles)
	mux.HandleFunc("GET /api/v1/bookfile/{id}", handler.compatBookFile)
	mux.HandleFunc("PUT /api/v1/bookfile/{id}", handler.compatUpdateBookFile)
	mux.HandleFunc("DELETE /api/v1/bookfile/bulk", handler.compatDeleteBookFileBulk)
	mux.HandleFunc("DELETE /api/v1/bookfile/{id}", handler.compatDeleteBookFile)
	mux.HandleFunc("GET /api/v1/rename", handler.compatRenamePreview)
	mux.HandleFunc("GET /api/v1/wanted/missing", handler.compatWantedMissing)
	mux.HandleFunc("GET /api/v1/qualityprofile", handler.compatQualityProfiles)
	mux.HandleFunc("GET /api/v1/delayprofile", handler.compatDelayProfiles)
	mux.HandleFunc("POST /api/v1/delayprofile", handler.compatCreateDelayProfile)
	mux.HandleFunc("GET /api/v1/delayprofile/{id}", handler.compatDelayProfile)
	mux.HandleFunc("PUT /api/v1/delayprofile/{id}", handler.compatUpdateDelayProfile)
	mux.HandleFunc("DELETE /api/v1/delayprofile/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/qualitydefinition", handler.compatQualityDefinitions)
	mux.HandleFunc("PUT /api/v1/qualitydefinition/{id}", handler.compatUpdateQualityDefinition)
	mux.HandleFunc("GET /api/v1/languageprofile", handler.compatLanguageProfiles)
	mux.HandleFunc("POST /api/v1/languageprofile", handler.compatCreateLanguageProfile)
	mux.HandleFunc("GET /api/v1/languageprofile/{id}", handler.compatLanguageProfile)
	mux.HandleFunc("PUT /api/v1/languageprofile/{id}", handler.compatUpdateLanguageProfile)
	mux.HandleFunc("DELETE /api/v1/languageprofile/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/metadataprofile", handler.compatMetadataProfiles)
	mux.HandleFunc("POST /api/v1/metadataprofile", handler.compatCreateMetadataProfile)
	mux.HandleFunc("GET /api/v1/metadataprofile/{id}", handler.compatMetadataProfile)
	mux.HandleFunc("PUT /api/v1/metadataprofile/{id}", handler.compatUpdateMetadataProfile)
	mux.HandleFunc("DELETE /api/v1/metadataprofile/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/metadata", handler.compatMetadataConsumers)
	mux.HandleFunc("GET /api/v1/metadata/schema", handler.compatMetadataSchema)
	mux.HandleFunc("POST /api/v1/metadata/test", handler.compatMetadataTest)
	mux.HandleFunc("POST /api/v1/metadata/testall", handler.compatMetadataTestAll)
	mux.HandleFunc("POST /api/v1/metadata/action/{name}", handler.compatResourceAction)
	mux.HandleFunc("PUT /api/v1/metadata/bulk", handler.compatResourceBulkUpdate)
	mux.HandleFunc("DELETE /api/v1/metadata/bulk", handler.compatResourceBulkDelete)
	mux.HandleFunc("POST /api/v1/metadata", handler.compatCreateMetadataConsumer)
	mux.HandleFunc("GET /api/v1/metadata/{id}", handler.compatMetadataConsumer)
	mux.HandleFunc("PUT /api/v1/metadata/{id}", handler.compatUpdateMetadataConsumer)
	mux.HandleFunc("DELETE /api/v1/metadata/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/customformat", handler.compatCustomFormats)
	mux.HandleFunc("POST /api/v1/customformat", handler.compatCreateCustomFormat)
	mux.HandleFunc("GET /api/v1/customformat/{id}", handler.compatCustomFormat)
	mux.HandleFunc("PUT /api/v1/customformat/{id}", handler.compatUpdateCustomFormat)
	mux.HandleFunc("DELETE /api/v1/customformat/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/tag", handler.compatTags)
	mux.HandleFunc("POST /api/v1/tag", handler.compatCreateTag)
	mux.HandleFunc("GET /api/v1/tag/{id}", handler.compatTag)
	mux.HandleFunc("PUT /api/v1/tag/{id}", handler.compatUpdateTag)
	mux.HandleFunc("DELETE /api/v1/tag/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/restriction", handler.compatRestrictions)
	mux.HandleFunc("POST /api/v1/restriction", handler.compatCreateRestriction)
	mux.HandleFunc("GET /api/v1/restriction/{id}", handler.compatRestriction)
	mux.HandleFunc("PUT /api/v1/restriction/{id}", handler.compatUpdateRestriction)
	mux.HandleFunc("DELETE /api/v1/restriction/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/notification", handler.compatNotifications)
	mux.HandleFunc("GET /api/v1/notification/schema", handler.compatNotificationSchema)
	mux.HandleFunc("POST /api/v1/notification/test", handler.compatNotificationTest)
	mux.HandleFunc("POST /api/v1/notification/testall", handler.compatNotificationTestAll)
	mux.HandleFunc("POST /api/v1/notification/action/{name}", handler.compatResourceAction)
	mux.HandleFunc("PUT /api/v1/notification/bulk", handler.compatResourceBulkUpdate)
	mux.HandleFunc("DELETE /api/v1/notification/bulk", handler.compatResourceBulkDelete)
	mux.HandleFunc("POST /api/v1/notification", handler.compatCreateNotification)
	mux.HandleFunc("GET /api/v1/notification/{id}", handler.compatNotification)
	mux.HandleFunc("PUT /api/v1/notification/{id}", handler.compatUpdateNotification)
	mux.HandleFunc("DELETE /api/v1/notification/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/importlist", handler.compatImportLists)
	mux.HandleFunc("GET /api/v1/importlist/schema", handler.compatImportListSchema)
	mux.HandleFunc("POST /api/v1/importlist/test", handler.compatImportListTest)
	mux.HandleFunc("POST /api/v1/importlist/testall", handler.compatImportListTestAll)
	mux.HandleFunc("POST /api/v1/importlist/action/{name}", handler.compatResourceAction)
	mux.HandleFunc("PUT /api/v1/importlist/bulk", handler.compatResourceBulkUpdate)
	mux.HandleFunc("DELETE /api/v1/importlist/bulk", handler.compatResourceBulkDelete)
	mux.HandleFunc("POST /api/v1/importlist", handler.compatCreateImportList)
	mux.HandleFunc("GET /api/v1/importlist/{id}", handler.compatImportList)
	mux.HandleFunc("PUT /api/v1/importlist/{id}", handler.compatUpdateImportList)
	mux.HandleFunc("DELETE /api/v1/importlist/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/importlistexclusion", handler.compatImportListExclusions)
	mux.HandleFunc("POST /api/v1/importlistexclusion", handler.compatCreateImportListExclusion)
	mux.HandleFunc("GET /api/v1/importlistexclusion/{id}", handler.compatImportListExclusion)
	mux.HandleFunc("PUT /api/v1/importlistexclusion/{id}", handler.compatUpdateImportListExclusion)
	mux.HandleFunc("DELETE /api/v1/importlistexclusion/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/remotepathmapping", handler.compatRemotePathMappings)
	mux.HandleFunc("POST /api/v1/remotepathmapping", handler.compatCreateRemotePathMapping)
	mux.HandleFunc("GET /api/v1/remotepathmapping/{id}", handler.compatRemotePathMapping)
	mux.HandleFunc("PUT /api/v1/remotepathmapping/{id}", handler.compatUpdateRemotePathMapping)
	mux.HandleFunc("DELETE /api/v1/remotepathmapping/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/downloadclient", handler.compatDownloadClients)
	mux.HandleFunc("GET /api/v1/downloadclient/schema", handler.compatDownloadClientSchema)
	mux.HandleFunc("POST /api/v1/downloadclient/test", handler.compatDownloadClientTest)
	mux.HandleFunc("POST /api/v1/downloadclient/testall", handler.compatDownloadClientTestAll)
	mux.HandleFunc("POST /api/v1/downloadclient/action/{name}", handler.compatResourceAction)
	mux.HandleFunc("PUT /api/v1/downloadclient/bulk", handler.compatResourceBulkUpdate)
	mux.HandleFunc("DELETE /api/v1/downloadclient/bulk", handler.compatResourceBulkDelete)
	mux.HandleFunc("POST /api/v1/downloadclient", handler.compatCreateDownloadClient)
	mux.HandleFunc("GET /api/v1/downloadclient/{id}", handler.compatDownloadClient)
	mux.HandleFunc("PUT /api/v1/downloadclient/{id}", handler.compatUpdateDownloadClient)
	mux.HandleFunc("DELETE /api/v1/downloadclient/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/indexer", handler.compatIndexers)
	mux.HandleFunc("GET /api/v1/indexer/schema", handler.compatIndexerSchema)
	mux.HandleFunc("POST /api/v1/indexer/test", handler.compatIndexerTest)
	mux.HandleFunc("POST /api/v1/indexer/testall", handler.compatIndexerTestAll)
	mux.HandleFunc("POST /api/v1/indexer/action/{name}", handler.compatResourceAction)
	mux.HandleFunc("PUT /api/v1/indexer/bulk", handler.compatResourceBulkUpdate)
	mux.HandleFunc("DELETE /api/v1/indexer/bulk", handler.compatResourceBulkDelete)
	mux.HandleFunc("POST /api/v1/indexer", handler.compatCreateIndexer)
	mux.HandleFunc("GET /api/v1/indexer/{id}", handler.compatIndexer)
	mux.HandleFunc("PUT /api/v1/indexer/{id}", handler.compatUpdateIndexer)
	mux.HandleFunc("DELETE /api/v1/indexer/{id}", handler.compatDeleteResource)
	mux.HandleFunc("GET /api/v1/release", handler.compatReleases)
	mux.HandleFunc("POST /api/v1/release", handler.compatGrabRelease)
	mux.HandleFunc("GET /api/v1/manualimport", handler.compatManualImport)
	mux.HandleFunc("POST /api/v1/manualimport", handler.compatCreateManualImport)
	mux.HandleFunc("GET /api/v1/command", handler.compatCommands)
	mux.HandleFunc("POST /api/v1/command", handler.compatCreateCommand)
	mux.HandleFunc("GET /api/v1/system/task", handler.compatSystemTasks)
	mux.HandleFunc("GET /api/v1/system/task/{id}", handler.compatSystemTask)
	mux.HandleFunc("GET /api/v1/providers/health", handler.providerHealth)
	mux.HandleFunc("GET /api/v1/providers/diagnostics", handler.providerDiagnostics)
	mux.HandleFunc("GET /api/v1/search", handler.search)
	mux.HandleFunc("POST /api/v1/settings/validate", handler.validateSettings)
	mux.HandleFunc("GET /api/v1/integrations/health", handler.integrationHealth)
	mux.HandleFunc("POST /api/v1/integrations/bootstrap", handler.integrationBootstrap)
	mux.HandleFunc("POST /api/v1/releases/search", handler.releaseSearch)
	mux.HandleFunc("POST /api/v1/grabs", handler.grab)
	mux.HandleFunc("GET /api/v1/downloads", handler.downloads)
	mux.HandleFunc("GET /api/v1/downloads/{id}", handler.downloadDetails)
	mux.HandleFunc("POST /api/v1/downloads/actions", handler.downloadAction)
	mux.HandleFunc("POST /api/v1/downloads/{id}/files/actions", handler.downloadFileAction)
	mux.HandleFunc("POST /api/v1/downloads/{id}/trackers/actions", handler.downloadTrackerAction)
	mux.HandleFunc("POST /api/v1/downloads/rebalance", handler.rebalanceDownloads)
	mux.HandleFunc("POST /api/v1/downloads/recover-failed", handler.recoverFailedDownloads)
	mux.HandleFunc("GET /api/v1/quality-profiles", handler.qualityProfiles)
	mux.HandleFunc("POST /api/v1/quality-profiles", handler.saveQualityProfile)
	mux.HandleFunc("GET /api/v1/authors", handler.authorSubscriptions)
	mux.HandleFunc("POST /api/v1/authors", handler.subscribeAuthor)
	mux.HandleFunc("POST /api/v1/authors/monitor", handler.monitorAuthors)
	mux.HandleFunc("GET /api/v1/wanted", handler.listWanted)
	mux.HandleFunc("POST /api/v1/wanted", handler.createWanted)
	mux.HandleFunc("POST /api/v1/wanted/monitor", handler.monitorWanted)
	mux.HandleFunc("POST /api/v1/wanted/feed-sync", handler.feedSyncWanted)
	mux.HandleFunc("POST /api/v1/wanted/upgrades", handler.upgradeWanted)
	mux.HandleFunc("POST /api/v1/wanted/{id}/search", handler.searchWantedReleases)
	mux.HandleFunc("GET /api/v1/wanted/{id}/releases", handler.listWantedReleases)
	mux.HandleFunc("POST /api/v1/wanted/{id}/grab", handler.grabWanted)
	mux.HandleFunc("GET /api/v1/librarry/history", handler.history)
	mux.HandleFunc("GET /api/v1/library/files", handler.libraryFiles)
	mux.HandleFunc("DELETE /api/v1/library/files/{id}", handler.deleteLibraryFile)
	mux.HandleFunc("POST /api/v1/library/files/delete", handler.deleteLibraryFiles)
	mux.HandleFunc("POST /api/v1/library/files/rename/preview", handler.previewRenameLibraryFiles)
	mux.HandleFunc("POST /api/v1/library/files/rename", handler.renameLibraryFiles)
	mux.HandleFunc("GET /api/v1/library/import-reviews", handler.importReviews)
	mux.HandleFunc("POST /api/v1/library/scan", handler.scanLibrary)
	mux.HandleFunc("POST /api/v1/library/import", handler.importLibraryFile)
	mux.HandleFunc("POST /api/v1/library/import-completed", handler.importCompletedDownloads)
	mux.HandleFunc("POST /api/v1/library/import-reviews/{id}/resolve", handler.resolveImportReview)

	return withCORS(deps.Config.WebOrigin, mux)
}

type handler struct {
	deps Dependencies
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"service":   "librarry",
		"checkedAt": time.Now().UTC(),
	})
}

func (h *handler) providerHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": h.deps.Metadata.Health(r.Context()),
	})
}

func (h *handler) providerDiagnostics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": h.deps.Metadata.Diagnostics(r.Context()),
	})
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	queryText := strings.TrimSpace(r.URL.Query().Get("query"))
	if queryText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query is required"})
		return
	}

	query := metadata.Query{
		Query:  queryText,
		Type:   metadata.SearchType(defaultString(r.URL.Query().Get("type"), string(metadata.SearchTypeBook))),
		Format: metadata.MediaFormat(defaultString(r.URL.Query().Get("format"), string(metadata.FormatAny))),
		Limit:  10,
	}
	outcome := h.deps.Metadata.SearchDetailed(r.Context(), query)
	if len(outcome.ProviderErrors) > 0 {
		h.deps.Logger.Warn("search completed with provider errors", "errors", outcome.ProviderErrors)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"query":          outcome.Query,
		"results":        outcome.Results,
		"providerErrors": outcome.ProviderErrors,
	})
}

func (h *handler) validateSettings(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input settings.Settings
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid settings payload"})
		return
	}
	writeJSON(w, http.StatusOK, settings.Validate(input))
}

func (h *handler) integrationHealth(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusOK, map[string]any{"integrations": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"integrations": h.deps.Acquire.Health(r.Context())})
}

func (h *handler) integrationBootstrap(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	result, err := h.deps.Acquire.Bootstrap(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) releaseSearch(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var query acquisition.ReleaseSearchQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid release search payload"})
		return
	}
	if strings.TrimSpace(query.Query) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query is required"})
		return
	}
	releases, err := h.deps.Acquire.Search(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": releases})
}

func (h *handler) grab(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid grab payload"})
		return
	}
	status, err := h.deps.Acquire.Grab(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) downloads(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	statuses, err := h.deps.Acquire.Downloads(r.Context(), acquisition.DownloadListQuery{
		Client:   r.URL.Query().Get("client"),
		Tag:      r.URL.Query().Get("tag"),
		Category: r.URL.Query().Get("category"),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"downloads": statuses})
}

func (h *handler) downloadDetails(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "download id is required"})
		return
	}
	details, err := h.deps.Acquire.DownloadDetails(r.Context(), id, r.URL.Query().Get("client"))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, acquisition.ErrDownloadNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, acquisition.ErrDownloadDetailsUnsupported) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, details)
}

func (h *handler) downloadAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download action payload"})
		return
	}
	result, err := h.deps.Acquire.DownloadAction(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) downloadFileAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "download id is required"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadFileActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download file action payload"})
		return
	}
	result, err := h.deps.Acquire.DownloadFileAction(r.Context(), id, request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, acquisition.ErrDownloadNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, acquisition.ErrDownloadDetailsUnsupported) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) downloadTrackerAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "download id is required"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadTrackerActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download tracker action payload"})
		return
	}
	result, err := h.deps.Acquire.DownloadTrackerAction(r.Context(), id, request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, acquisition.ErrDownloadNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, acquisition.ErrDownloadDetailsUnsupported) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type downloadRebalanceRequest struct {
	MaxActive    int    `json:"maxActive"`
	Client       string `json:"client,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Category     string `json:"category,omitempty"`
	DryRun       bool   `json:"dryRun,omitempty"`
	StopOverflow bool   `json:"stopOverflow,omitempty"`
}

type downloadRebalancePlan struct {
	MaxActive     int                          `json:"maxActive"`
	ActiveCount   int                          `json:"activeCount"`
	PausedCount   int                          `json:"pausedCount"`
	CompleteCount int                          `json:"completeCount"`
	FailedCount   int                          `json:"failedCount"`
	StartIDs      []string                     `json:"startIds"`
	StopIDs       []string                     `json:"stopIds"`
	Applied       bool                         `json:"applied"`
	Message       string                       `json:"message"`
	Downloads     []acquisition.DownloadStatus `json:"downloads,omitempty"`
}

func (h *handler) rebalanceDownloads(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request downloadRebalanceRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.MaxActive <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "maxActive must be greater than zero"})
		return
	}
	if strings.TrimSpace(request.Tag) == "" && strings.TrimSpace(request.Client) == "" && strings.TrimSpace(request.Category) == "" {
		request.Tag = "librarry"
	}
	query := acquisition.DownloadListQuery{
		Client:   request.Client,
		Tag:      request.Tag,
		Category: request.Category,
	}
	downloads, err := h.deps.Acquire.Downloads(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	plan := planDownloadRebalance(downloads, request.MaxActive, request.StopOverflow)
	plan.MaxActive = request.MaxActive

	if !request.DryRun {
		if len(plan.StopIDs) > 0 {
			if _, err := h.deps.Acquire.DownloadAction(r.Context(), acquisition.DownloadActionRequest{Action: acquisition.DownloadActionStop, IDs: plan.StopIDs}); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "plan": plan})
				return
			}
		}
		if len(plan.StartIDs) > 0 {
			if _, err := h.deps.Acquire.DownloadAction(r.Context(), acquisition.DownloadActionRequest{Action: acquisition.DownloadActionStart, IDs: plan.StartIDs}); err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "plan": plan})
				return
			}
		}
		plan.Applied = true
		refreshed, refreshErr := h.deps.Acquire.Downloads(r.Context(), query)
		if refreshErr == nil {
			plan.Downloads = refreshed
		}
	}
	if plan.Message == "" {
		switch {
		case len(plan.StartIDs) == 0 && len(plan.StopIDs) == 0:
			plan.Message = "queue already matches active-download limit"
		case request.DryRun:
			plan.Message = "queue rebalance plan created"
		default:
			plan.Message = "queue rebalance applied"
		}
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *handler) recoverFailedDownloads(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.FailedDownloadRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Trigger == "" {
		request.Trigger = "manual"
	}
	run, err := h.deps.Wanted.RecoverFailedDownloads(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) qualityProfiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	profiles, err := h.deps.Wanted.ListQualityProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}

func (h *handler) saveQualityProfile(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var profile wanted.QualityProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid quality profile payload"})
		return
	}
	saved, err := h.deps.Wanted.SaveQualityProfile(r.Context(), profile)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *handler) authorSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	subscriptions, err := h.deps.Wanted.ListAuthorSubscriptions(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authors": subscriptions})
}

func (h *handler) subscribeAuthor(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.AuthorSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author subscription payload"})
		return
	}
	subscription, err := h.deps.Wanted.SubscribeAuthor(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, subscription)
}

func (h *handler) monitorAuthors(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.AuthorMonitorRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Trigger == "" {
		request.Trigger = "manual"
	}
	run, err := h.deps.Wanted.MonitorAuthors(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) listWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	items, err := h.deps.Wanted.List(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wanted": items})
}

func (h *handler) createWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted payload"})
		return
	}
	item, err := h.deps.Wanted.Create(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) searchWantedReleases(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.SearchReleasesRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	outcome, err := h.deps.Wanted.SearchReleases(r.Context(), r.PathValue("id"), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) listWantedReleases(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	outcome, err := h.deps.Wanted.ListReleases(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) grabWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.GrabRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	status, err := h.deps.Wanted.Grab(r.Context(), r.PathValue("id"), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) monitorWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.MonitorRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Trigger == "" {
		request.Trigger = "manual"
	}
	run, err := h.deps.Wanted.Monitor(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) feedSyncWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.FeedSyncRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Trigger == "" {
		request.Trigger = "manual"
	}
	run, err := h.deps.Wanted.FeedSync(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) upgradeWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.UpgradeRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	if request.Trigger == "" {
		request.Trigger = "manual"
	}
	run, err := h.deps.Wanted.SearchUpgrades(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "run": run})
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *handler) history(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.deps.Wanted.History(r.Context(), wanted.HistoryQuery{Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *handler) libraryFiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	files, err := h.deps.Library.ListFiles(r.Context(), library.FileListQuery{
		Format: r.URL.Query().Get("format"),
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (h *handler) deleteLibraryFile(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "file id is required"})
		return
	}
	outcome, err := h.deps.Library.DeleteFiles(r.Context(), library.DeleteFilesRequest{
		IDs:         []string{id},
		DeleteFiles: parseBoolDefault(r.URL.Query().Get("deleteFiles"), false),
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) deleteLibraryFiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.DeleteFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid library file delete payload"})
		return
	}
	outcome, err := h.deps.Library.DeleteFiles(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) previewRenameLibraryFiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.RenameFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid library rename preview payload"})
		return
	}
	outcome, err := h.deps.Library.PreviewRenameFiles(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) renameLibraryFiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.RenameFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid library rename payload"})
		return
	}
	outcome, err := h.deps.Library.RenameFiles(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "outcome": outcome})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) importReviews(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reviews, err := h.deps.Library.ListImportReviews(r.Context(), library.ReviewListQuery{
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (h *handler) scanLibrary(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.ScanRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	outcome, err := h.deps.Library.Scan(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) importLibraryFile(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid library import payload"})
		return
	}
	outcome, err := h.deps.Library.Import(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) importCompletedDownloads(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.CompletedImportRequest
	if r.Body != http.NoBody {
		_ = json.NewDecoder(r.Body).Decode(&request)
	}
	downloads, err := h.deps.Acquire.Downloads(r.Context(), acquisition.DownloadListQuery{
		IDs: request.DownloadIDs,
		Tag: "librarry",
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	outcome, err := h.deps.Library.ImportCompletedDownloads(r.Context(), downloads, request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) resolveImportReview(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "review id is required"})
		return
	}
	defer r.Body.Close()
	var request library.ReviewDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid review decision payload"})
		return
	}
	outcome, err := h.deps.Library.ResolveImportReview(r.Context(), id, request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func planDownloadRebalance(downloads []acquisition.DownloadStatus, maxActive int, stopOverflow bool) downloadRebalancePlan {
	var active []acquisition.DownloadStatus
	var paused []acquisition.DownloadStatus
	plan := downloadRebalancePlan{}
	for _, download := range downloads {
		switch {
		case isFailedDownloadState(download):
			plan.FailedCount++
		case isCompleteDownloadState(download):
			plan.CompleteCount++
		case isPausedDownloadState(download):
			plan.PausedCount++
			paused = append(paused, download)
		case isActiveDownloadState(download):
			plan.ActiveCount++
			active = append(active, download)
		}
	}
	if plan.ActiveCount < maxActive {
		sortStartCandidates(paused)
		slots := maxActive - plan.ActiveCount
		for _, download := range paused {
			if slots <= 0 {
				break
			}
			if strings.TrimSpace(download.ID) == "" {
				continue
			}
			plan.StartIDs = append(plan.StartIDs, download.ID)
			slots--
		}
		return plan
	}
	if plan.ActiveCount > maxActive && stopOverflow {
		sortKeepActiveCandidates(active)
		for _, download := range active[maxActive:] {
			if strings.TrimSpace(download.ID) != "" {
				plan.StopIDs = append(plan.StopIDs, download.ID)
			}
		}
	}
	return plan
}

func sortStartCandidates(downloads []acquisition.DownloadStatus) {
	sort.SliceStable(downloads, func(i, j int) bool {
		if downloads[i].Progress != downloads[j].Progress {
			return downloads[i].Progress > downloads[j].Progress
		}
		return earlierDownload(downloads[i], downloads[j])
	})
}

func sortKeepActiveCandidates(downloads []acquisition.DownloadStatus) {
	sort.SliceStable(downloads, func(i, j int) bool {
		if downloads[i].Progress != downloads[j].Progress {
			return downloads[i].Progress > downloads[j].Progress
		}
		return earlierDownload(downloads[i], downloads[j])
	})
}

func earlierDownload(a acquisition.DownloadStatus, b acquisition.DownloadStatus) bool {
	if a.AddedAt != nil && b.AddedAt != nil && !a.AddedAt.Equal(*b.AddedAt) {
		return a.AddedAt.Before(*b.AddedAt)
	}
	return strings.ToLower(a.Name) < strings.ToLower(b.Name)
}

func isActiveDownloadState(download acquisition.DownloadStatus) bool {
	state := normalizedDownloadState(download.State)
	if state == "" {
		return false
	}
	if isCompleteDownloadState(download) || isFailedDownloadState(download) || isPausedDownloadState(download) {
		return false
	}
	return strings.Contains(state, "download") ||
		strings.Contains(state, "stalled") ||
		strings.Contains(state, "meta") ||
		strings.Contains(state, "queued") ||
		strings.Contains(state, "checking") ||
		strings.Contains(state, "forced") ||
		strings.Contains(state, "moving") ||
		strings.Contains(state, "allocating")
}

func isPausedDownloadState(download acquisition.DownloadStatus) bool {
	state := normalizedDownloadState(download.State)
	if isCompleteDownloadState(download) || isFailedDownloadState(download) {
		return false
	}
	return strings.Contains(state, "pause") ||
		strings.Contains(state, "stop") ||
		state == "idle"
}

func isCompleteDownloadState(download acquisition.DownloadStatus) bool {
	state := normalizedDownloadState(download.State)
	return download.CompletedAt != nil ||
		download.Progress >= 1 ||
		strings.Contains(state, "complete") ||
		strings.Contains(state, "upload") ||
		strings.Contains(state, "seed")
}

func isFailedDownloadState(download acquisition.DownloadStatus) bool {
	state := normalizedDownloadState(download.State)
	return strings.Contains(state, "error") ||
		strings.Contains(state, "missing") ||
		strings.Contains(state, "failed") ||
		strings.TrimSpace(download.FailureReason) != ""
}

func normalizedDownloadState(state string) string {
	state = strings.ToLower(strings.TrimSpace(state))
	state = strings.ReplaceAll(state, "_", "")
	state = strings.ReplaceAll(state, "-", "")
	state = strings.ReplaceAll(state, " ", "")
	return state
}

func withCORS(webOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (webOrigin == "*" || origin == webOrigin || strings.HasPrefix(origin, "http://127.0.0.1:517")) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

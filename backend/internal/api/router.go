package api

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/auth"
	"github.com/bandoracer/librarry/backend/internal/backups"
	compatdata "github.com/bandoracer/librarry/backend/internal/compat"
	"github.com/bandoracer/librarry/backend/internal/config"
	"github.com/bandoracer/librarry/backend/internal/importlists"
	"github.com/bandoracer/librarry/backend/internal/integrationsettings"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/metadata"
	"github.com/bandoracer/librarry/backend/internal/notify"
	"github.com/bandoracer/librarry/backend/internal/scheduler"
	"github.com/bandoracer/librarry/backend/internal/settings"
	"github.com/bandoracer/librarry/backend/internal/tags"
	"github.com/bandoracer/librarry/backend/internal/wanted"
)

const maxGrabUploadBytes = 64 << 20

type Dependencies struct {
	Logger      *slog.Logger
	Config      config.Config
	Metadata    *metadata.Service
	Acquire     acquisitionService
	Wanted      wantedService
	Library     libraryService
	Compat      compatResourceService
	Notify      *notify.Service
	Scheduler   *scheduler.Registry
	Health      *HealthEvaluator
	Auth        *auth.Service
	ImportLists *importlists.Service
	Tags        *tags.Store
	Backups     *backups.Service
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
	DownloadResources(ctx context.Context, client string) (acquisition.DownloadResources, error)
	DownloadPreferences(ctx context.Context, client string) (acquisition.DownloadPreferences, error)
	UpdateDownloadPreferences(ctx context.Context, request acquisition.DownloadPreferencesUpdate) (acquisition.DownloadPreferences, error)
	DownloadCategoryAction(ctx context.Context, request acquisition.DownloadCategoryActionRequest) (acquisition.DownloadResourceActionResult, error)
	DownloadTagAction(ctx context.Context, request acquisition.DownloadTagActionRequest) (acquisition.DownloadResourceActionResult, error)
	ClearDownloadFailure(ctx context.Context, id string) error
}

type configurableAcquisitionService interface {
	IntegrationConfig() acquisition.IntegrationConfig
	Reconfigure(config acquisition.IntegrationConfig)
}

type wantedService interface {
	Create(ctx context.Context, request wanted.CreateRequest) (wanted.WantedItem, error)
	List(ctx context.Context, status string) ([]wanted.WantedItem, error)
	ListQualityProfiles(ctx context.Context) ([]wanted.QualityProfile, error)
	SaveQualityProfile(ctx context.Context, profile wanted.QualityProfile) (wanted.QualityProfile, error)
	DeleteQualityProfile(ctx context.Context, idOrName string) error
	ListReleaseProfiles(ctx context.Context) ([]wanted.ReleaseProfile, error)
	SaveReleaseProfile(ctx context.Context, profile wanted.ReleaseProfile) (wanted.ReleaseProfile, error)
	DeleteReleaseProfile(ctx context.Context, id string) error
	ListQualityDefinitions(ctx context.Context) ([]wanted.QualityDefinition, error)
	UpdateQualityDefinitions(ctx context.Context, definitions []wanted.QualityDefinition) ([]wanted.QualityDefinition, error)
	SubscribeAuthor(ctx context.Context, request wanted.AuthorSubscribeRequest) (wanted.AuthorSubscription, error)
	ListAuthorSubscriptions(ctx context.Context, status string) ([]wanted.AuthorSubscription, error)
	MonitorAuthors(ctx context.Context, request wanted.AuthorMonitorRequest) (wanted.AuthorMonitorRun, error)
	ListAuthorMetadataReviews(ctx context.Context, query wanted.AuthorMetadataReviewQuery) ([]wanted.AuthorMetadataReview, error)
	ResolveAuthorMetadataReview(ctx context.Context, id string, request wanted.AuthorMetadataReviewDecisionRequest) (wanted.AuthorMetadataReviewDecision, error)
	AcquisitionQueue(ctx context.Context, query wanted.AcquisitionQueueQuery) (wanted.AcquisitionQueue, error)
	SearchReleases(ctx context.Context, wantedID string, request wanted.SearchReleasesRequest) (wanted.SearchOutcome, error)
	ListReleases(ctx context.Context, wantedID string) (wanted.SearchOutcome, error)
	Grab(ctx context.Context, wantedID string, request wanted.GrabRequest) (acquisition.DownloadStatus, error)
	UpdateWanted(ctx context.Context, id string, request wanted.WantedUpdateRequest) (wanted.WantedItem, error)
	DeleteWanted(ctx context.Context, id string) error
	ClearWantedManualOverrides(ctx context.Context, id string, fields []string) (wanted.WantedItem, error)
	UpdateAuthorSubscription(ctx context.Context, id string, request wanted.AuthorUpdateRequest) (wanted.AuthorSubscription, error)
	DeleteAuthorSubscription(ctx context.Context, id string) error
	Monitor(ctx context.Context, request wanted.MonitorRequest) (wanted.MonitorRun, error)
	FeedSync(ctx context.Context, request wanted.FeedSyncRequest) (wanted.FeedSyncRun, error)
	RecoverFailedDownloads(ctx context.Context, request wanted.FailedDownloadRequest) (wanted.FailedDownloadRun, error)
	SearchUpgrades(ctx context.Context, request wanted.UpgradeRequest) (wanted.UpgradeRun, error)
	History(ctx context.Context, query wanted.HistoryQuery) ([]wanted.HistoryEvent, error)
	AnnotateDownloads(ctx context.Context, downloads []acquisition.DownloadStatus) []acquisition.DownloadStatus
	AnnotateWantedStates(ctx context.Context, items []wanted.WantedItem) []wanted.WantedItem
	ListCutoffUnmet(ctx context.Context) ([]wanted.WantedItem, error)
	ListCalendar(ctx context.Context, start time.Time, end time.Time, includeUnmonitored bool) ([]wanted.WantedItem, error)
	BulkUpdateWanted(ctx context.Context, request wanted.WantedBulkRequest) ([]wanted.WantedBulkResult, error)
	ListBlocklist(ctx context.Context, limit int) ([]wanted.BlocklistEntry, error)
	BlocklistDownload(ctx context.Context, request wanted.BlocklistDownloadRequest) (wanted.BlocklistEntry, error)
	DeleteBlocklistEntry(ctx context.Context, id string) error
	ClearBlocklist(ctx context.Context, ids []string) (int, error)
	MarkDownloadFailedManually(ctx context.Context, request wanted.ManualFailRequest) (wanted.ManualFailOutcome, error)
}

type metadataProvenanceService interface {
	MetadataProvenance(ctx context.Context, id string) (wanted.MetadataProvenance, error)
	MetadataReviewQueue(ctx context.Context) (wanted.MetadataReviewQueue, error)
	ApplyMetadataCorrection(ctx context.Context, id string, request wanted.MetadataCorrectionRequest) (wanted.MetadataProvenance, error)
	ApplyMetadataCorrections(ctx context.Context, id string, request wanted.MetadataCorrectionBatchRequest) (wanted.MetadataProvenance, error)
	ConfirmMetadataReviewCanonical(ctx context.Context, request wanted.MetadataReviewConfirmRequest) (wanted.MetadataReviewConfirmOutcome, error)
}

type libraryService interface {
	ListFiles(ctx context.Context, query library.FileListQuery) ([]library.FileRecord, error)
	TrackFile(ctx context.Context, file library.FileRecord) (library.FileRecord, error)
	UpdateFile(ctx context.Context, file library.FileRecord) (library.FileRecord, error)
	DeleteFiles(ctx context.Context, request library.DeleteFilesRequest) (library.DeleteFilesOutcome, error)
	PreviewRenameFiles(ctx context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error)
	RenameFiles(ctx context.Context, request library.RenameFilesRequest) (library.RenameFilesOutcome, error)
	RefreshCalibreConversions(ctx context.Context, request library.CalibreConversionRefreshRequest) (library.CalibreConversionRefreshOutcome, error)
	ListImportReviews(ctx context.Context, query library.ReviewListQuery) ([]library.ImportReview, error)
	Scan(ctx context.Context, request library.ScanRequest) (library.ScanOutcome, error)
	Import(ctx context.Context, request library.ImportRequest) (library.ImportOutcome, error)
	ImportCompletedDownloads(ctx context.Context, downloads []acquisition.DownloadStatus, request library.CompletedImportRequest) (library.CompletedImportOutcome, error)
	ResolveImportReview(ctx context.Context, id string, request library.ReviewDecisionRequest) (library.ReviewDecisionOutcome, error)
}

type configurableLibraryService interface {
	Config() library.Config
	Reconfigure(config library.Config)
}

type rootFolderLibraryService interface {
	ListRootFolders(ctx context.Context) ([]library.RootFolder, error)
	CreateRootFolder(ctx context.Context, folder library.RootFolder) (library.RootFolder, error)
	UpdateRootFolder(ctx context.Context, id string, folder library.RootFolder) (library.RootFolder, error)
	DeleteRootFolder(ctx context.Context, id string) error
	SyncDefaultRootFolders(ctx context.Context, config library.Config) error
}

type remotePathMappingLibraryService interface {
	ListRemotePathMappings(ctx context.Context) ([]library.RemotePathMapping, error)
	CreateRemotePathMapping(ctx context.Context, mapping library.RemotePathMapping) (library.RemotePathMapping, error)
	UpdateRemotePathMapping(ctx context.Context, id string, mapping library.RemotePathMapping) (library.RemotePathMapping, error)
	DeleteRemotePathMapping(ctx context.Context, id string) error
}

type configurableWantedSearchLanguage interface {
	SetDefaultSearchLanguage(language string)
}

type compatResourceService interface {
	ListRootFolders(ctx context.Context) ([]compatdata.RootFolder, error)
	CreateRootFolder(ctx context.Context, folder compatdata.RootFolder) (compatdata.RootFolder, error)
	UpdateRootFolder(ctx context.Context, id string, folder compatdata.RootFolder) (compatdata.RootFolder, bool, error)
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
	mux.HandleFunc("PUT /api/v1/rootfolder/{id}", handler.compatUpdateRootFolder)
	mux.HandleFunc("DELETE /api/v1/rootfolder/{id}", handler.compatDeleteRootFolder)
	mux.HandleFunc("GET /api/v1/queue", handler.compatQueue)
	mux.HandleFunc("GET /api/v1/queue/details", handler.compatQueueDetails)
	mux.HandleFunc("GET /api/v1/queue/status", handler.compatQueueStatus)
	mux.HandleFunc("POST /api/v1/queue/grab/bulk", handler.compatGrabQueueBulk)
	mux.HandleFunc("POST /api/v1/queue/grab/{id}", handler.compatGrabQueue)
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
	mux.HandleFunc("PUT /api/v1/author/editor", handler.compatAuthorEditor)
	mux.HandleFunc("DELETE /api/v1/author/editor", handler.compatDeleteAuthorEditor)
	mux.HandleFunc("GET /api/v1/author/{id}", handler.compatAuthor)
	mux.HandleFunc("PUT /api/v1/author/{id}", handler.compatUpdateAuthor)
	mux.HandleFunc("DELETE /api/v1/author/{id}", handler.compatDeleteAuthor)
	mux.HandleFunc("GET /api/v1/book/lookup", handler.compatBookLookup)
	mux.HandleFunc("PUT /api/v1/book/monitor", handler.compatMonitorBooks)
	mux.HandleFunc("GET /api/v1/book", handler.compatBooks)
	mux.HandleFunc("POST /api/v1/book", handler.compatCreateBook)
	mux.HandleFunc("PUT /api/v1/book/editor", handler.compatBookEditor)
	mux.HandleFunc("DELETE /api/v1/book/editor", handler.compatDeleteBookEditor)
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
	mux.HandleFunc("GET /api/v1/retag", handler.compatRetagPreview)
	mux.HandleFunc("POST /api/v1/retag", handler.compatRetag)
	mux.HandleFunc("GET /api/v1/wanted/missing", handler.compatWantedMissing)
	mux.HandleFunc("GET /api/v1/wanted/missing/{id}", handler.compatWantedMissingItem)
	mux.HandleFunc("GET /api/v1/wanted/cutoff", handler.compatWantedCutoff)
	mux.HandleFunc("GET /api/v1/wanted/cutoff/{id}", handler.compatWantedCutoffItem)
	mux.HandleFunc("GET /api/v1/qualityprofile", handler.compatQualityProfiles)
	mux.HandleFunc("POST /api/v1/qualityprofile", handler.compatCreateQualityProfile)
	mux.HandleFunc("GET /api/v1/qualityprofile/{id}", handler.compatQualityProfile)
	mux.HandleFunc("PUT /api/v1/qualityprofile/{id}", handler.compatUpdateQualityProfile)
	mux.HandleFunc("DELETE /api/v1/qualityprofile/{id}", handler.compatDeleteQualityProfile)
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
	mux.HandleFunc("GET /api/v1/command/{id}", handler.compatCommand)
	mux.HandleFunc("DELETE /api/v1/command/{id}", handler.compatDeleteCommand)
	mux.HandleFunc("GET /api/v1/system/task", handler.compatSystemTasks)
	mux.HandleFunc("GET /api/v1/system/task/{id}", handler.compatSystemTask)
	mux.HandleFunc("GET /api/v1/system/tasks", handler.systemTasks)
	mux.HandleFunc("POST /api/v1/system/tasks/{id}/run", handler.runSystemTask)
	mux.HandleFunc("GET /api/v1/system/health", handler.systemHealth)
	mux.HandleFunc("GET /api/v1/system/diskspace", handler.systemDiskspace)
	mux.HandleFunc("GET /api/v1/notifications", handler.listNotificationTargets)
	mux.HandleFunc("POST /api/v1/notifications", handler.createNotificationTarget)
	mux.HandleFunc("PUT /api/v1/notifications/{id}", handler.updateNotificationTarget)
	mux.HandleFunc("DELETE /api/v1/notifications/{id}", handler.deleteNotificationTarget)
	mux.HandleFunc("POST /api/v1/notifications/{id}/test", handler.testNotificationTarget)
	mux.HandleFunc("GET /api/v1/providers/health", handler.providerHealth)
	mux.HandleFunc("GET /api/v1/providers/diagnostics", handler.providerDiagnostics)
	mux.HandleFunc("GET /api/v1/readiness", handler.readiness)
	mux.HandleFunc("GET /api/v1/search", handler.search)
	mux.HandleFunc("GET /api/v1/readarr/compatibility", handler.readarrCompatibility)
	mux.HandleFunc("POST /api/v1/readarr/import/preview", handler.previewReadarrImport)
	mux.HandleFunc("POST /api/v1/readarr/import", handler.applyReadarrImport)
	mux.HandleFunc("POST /api/v1/settings/validate", handler.validateSettings)
	mux.HandleFunc("GET /api/v1/integrations/health", handler.integrationHealth)
	mux.HandleFunc("GET /api/v1/integrations/config", handler.integrationConfig)
	mux.HandleFunc("PUT /api/v1/integrations/config", handler.updateIntegrationConfig)
	mux.HandleFunc("POST /api/v1/integrations/bootstrap", handler.integrationBootstrap)
	mux.HandleFunc("POST /api/v1/releases/search", handler.releaseSearch)
	mux.HandleFunc("POST /api/v1/grabs", handler.grab)
	mux.HandleFunc("GET /api/v1/downloads", handler.downloads)
	mux.HandleFunc("GET /api/v1/downloads/{id}", handler.downloadDetails)
	mux.HandleFunc("GET /api/v1/downloads/resources", handler.downloadResources)
	mux.HandleFunc("GET /api/v1/downloads/preferences", handler.downloadPreferences)
	mux.HandleFunc("PUT /api/v1/downloads/preferences", handler.updateDownloadPreferences)
	mux.HandleFunc("POST /api/v1/downloads/categories/actions", handler.downloadCategoryAction)
	mux.HandleFunc("POST /api/v1/downloads/tags/actions", handler.downloadTagAction)
	mux.HandleFunc("POST /api/v1/downloads/actions", handler.downloadAction)
	mux.HandleFunc("POST /api/v1/downloads/{id}/files/actions", handler.downloadFileAction)
	mux.HandleFunc("POST /api/v1/downloads/{id}/trackers/actions", handler.downloadTrackerAction)
	mux.HandleFunc("POST /api/v1/downloads/rebalance", handler.rebalanceDownloads)
	mux.HandleFunc("POST /api/v1/downloads/recover-failed", handler.recoverFailedDownloads)
	mux.HandleFunc("POST /api/v1/downloads/mark-failed", handler.markDownloadFailed)
	mux.HandleFunc("GET /api/v1/quality-profiles", handler.qualityProfiles)
	mux.HandleFunc("POST /api/v1/quality-profiles", handler.saveQualityProfile)
	mux.HandleFunc("GET /api/v1/release-profiles", handler.releaseProfiles)
	mux.HandleFunc("POST /api/v1/release-profiles", handler.createReleaseProfile)
	mux.HandleFunc("PUT /api/v1/release-profiles/{id}", handler.updateReleaseProfile)
	mux.HandleFunc("DELETE /api/v1/release-profiles/{id}", handler.deleteReleaseProfile)
	mux.HandleFunc("GET /api/v1/quality-definitions", handler.qualityDefinitions)
	mux.HandleFunc("PUT /api/v1/quality-definitions", handler.updateQualityDefinitions)
	mux.HandleFunc("GET /api/v1/metadata-profiles", handler.metadataProfiles)
	mux.HandleFunc("POST /api/v1/metadata-profiles", handler.createMetadataProfile)
	mux.HandleFunc("PUT /api/v1/metadata-profiles/{id}", handler.updateMetadataProfile)
	mux.HandleFunc("DELETE /api/v1/metadata-profiles/{id}", handler.deleteMetadataProfile)
	mux.HandleFunc("GET /api/v1/authors", handler.authorSubscriptions)
	mux.HandleFunc("POST /api/v1/authors", handler.subscribeAuthor)
	mux.HandleFunc("PATCH /api/v1/authors/{id}", handler.updateAuthorSubscription)
	mux.HandleFunc("PUT /api/v1/authors/{id}", handler.updateAuthorSubscription)
	mux.HandleFunc("DELETE /api/v1/authors/{id}", handler.deleteAuthorSubscription)
	mux.HandleFunc("POST /api/v1/authors/monitor", handler.monitorAuthors)
	mux.HandleFunc("GET /api/v1/authors/metadata/review", handler.authorMetadataReviews)
	mux.HandleFunc("POST /api/v1/authors/metadata/review/{id}/resolve", handler.resolveAuthorMetadataReview)
	mux.HandleFunc("GET /api/v1/wanted", handler.listWanted)
	mux.HandleFunc("POST /api/v1/wanted", handler.createWanted)
	mux.HandleFunc("POST /api/v1/wanted/bulk", handler.bulkUpdateWanted)
	mux.HandleFunc("PUT /api/v1/wanted/{id}", handler.updateWanted)
	mux.HandleFunc("PATCH /api/v1/wanted/{id}", handler.updateWanted)
	mux.HandleFunc("DELETE /api/v1/wanted/{id}", handler.deleteWanted)
	mux.HandleFunc("GET /api/v1/wanted/metadata/review", handler.wantedMetadataReview)
	mux.HandleFunc("POST /api/v1/wanted/metadata/review/confirm-canonical", handler.confirmWantedMetadataReviewCanonical)
	mux.HandleFunc("GET /api/v1/wanted/metadata/{id}", handler.wantedMetadata)
	mux.HandleFunc("POST /api/v1/wanted/metadata/{id}/apply", handler.applyWantedMetadataCorrection)
	mux.HandleFunc("POST /api/v1/wanted/metadata/{id}/apply-bulk", handler.applyWantedMetadataCorrections)
	mux.HandleFunc("DELETE /api/v1/wanted/{id}/overrides/{field}", handler.clearWantedOverride)
	mux.HandleFunc("GET /api/v1/acquisition/queue", handler.acquisitionQueue)
	mux.HandleFunc("POST /api/v1/wanted/monitor", handler.monitorWanted)
	mux.HandleFunc("POST /api/v1/wanted/feed-sync", handler.feedSyncWanted)
	mux.HandleFunc("POST /api/v1/wanted/upgrades", handler.upgradeWanted)
	mux.HandleFunc("POST /api/v1/wanted/{id}/search", handler.searchWantedReleases)
	mux.HandleFunc("GET /api/v1/wanted/releases/{id}", handler.listWantedReleases)
	mux.HandleFunc("POST /api/v1/wanted/{id}/grab", handler.grabWanted)
	mux.HandleFunc("GET /api/v1/librarry/calendar", handler.librarryCalendar)
	mux.HandleFunc("GET /feed/v1/calendar.ics", handler.calendarFeed)
	mux.HandleFunc("GET /api/v1/auth/status", handler.authStatus)
	mux.HandleFunc("POST /api/v1/login", handler.login)
	mux.HandleFunc("POST /api/v1/logout", handler.logout)
	mux.HandleFunc("PUT /api/v1/auth/config", handler.updateAuthConfig)
	mux.HandleFunc("GET /api/v1/import-lists", handler.listImportLists)
	mux.HandleFunc("POST /api/v1/import-lists", handler.createImportList)
	mux.HandleFunc("GET /api/v1/import-lists/exclusions", handler.listImportListExclusions)
	mux.HandleFunc("POST /api/v1/import-lists/exclusions", handler.createImportListExclusion)
	mux.HandleFunc("DELETE /api/v1/import-lists/exclusions/{id}", handler.deleteImportListExclusion)
	mux.HandleFunc("PUT /api/v1/import-lists/{id}", handler.updateImportList)
	mux.HandleFunc("DELETE /api/v1/import-lists/{id}", handler.deleteImportList)
	mux.HandleFunc("POST /api/v1/import-lists/{id}/sync", handler.syncImportList)
	mux.HandleFunc("GET /api/v1/tags", handler.listTags)
	mux.HandleFunc("POST /api/v1/tags", handler.createTag)
	mux.HandleFunc("PUT /api/v1/tags/{id}", handler.updateTag)
	mux.HandleFunc("DELETE /api/v1/tags/{id}", handler.deleteTag)
	mux.HandleFunc("GET /api/v1/librarry/backups", handler.listBackups)
	mux.HandleFunc("POST /api/v1/librarry/backups", handler.createBackup)
	mux.HandleFunc("DELETE /api/v1/librarry/backups/{name}", handler.deleteBackup)
	mux.HandleFunc("GET /api/v1/librarry/history", handler.history)
	mux.HandleFunc("GET /api/v1/librarry/blocklist", handler.blocklist)
	mux.HandleFunc("POST /api/v1/librarry/blocklist", handler.createBlocklistEntry)
	mux.HandleFunc("POST /api/v1/librarry/blocklist/clear", handler.clearBlocklist)
	mux.HandleFunc("DELETE /api/v1/librarry/blocklist/{id}", handler.deleteBlocklistEntry)
	mux.HandleFunc("GET /api/v1/library/config", handler.libraryConfig)
	mux.HandleFunc("PUT /api/v1/library/config", handler.updateLibraryConfig)
	mux.HandleFunc("GET /api/v1/library/root-folders", handler.libraryRootFolders)
	mux.HandleFunc("POST /api/v1/library/root-folders", handler.createLibraryRootFolder)
	mux.HandleFunc("PUT /api/v1/library/root-folders/{id}", handler.updateLibraryRootFolder)
	mux.HandleFunc("DELETE /api/v1/library/root-folders/{id}", handler.deleteLibraryRootFolder)
	mux.HandleFunc("GET /api/v1/library/remote-path-mappings", handler.libraryRemotePathMappings)
	mux.HandleFunc("POST /api/v1/library/remote-path-mappings", handler.createLibraryRemotePathMapping)
	mux.HandleFunc("PUT /api/v1/library/remote-path-mappings/{id}", handler.updateLibraryRemotePathMapping)
	mux.HandleFunc("DELETE /api/v1/library/remote-path-mappings/{id}", handler.deleteLibraryRemotePathMapping)
	mux.HandleFunc("GET /api/v1/library/files", handler.libraryFiles)
	mux.HandleFunc("DELETE /api/v1/library/files/{id}", handler.deleteLibraryFile)
	mux.HandleFunc("POST /api/v1/library/files/delete", handler.deleteLibraryFiles)
	mux.HandleFunc("POST /api/v1/library/files/rename/preview", handler.previewRenameLibraryFiles)
	mux.HandleFunc("POST /api/v1/library/files/rename", handler.renameLibraryFiles)
	mux.HandleFunc("POST /api/v1/library/calibre/conversions/refresh", handler.refreshCalibreConversions)
	mux.HandleFunc("GET /api/v1/library/import-reviews", handler.importReviews)
	mux.HandleFunc("POST /api/v1/library/import-reviews/resolve-bulk", handler.resolveImportReviewsBulk)
	mux.HandleFunc("POST /api/v1/library/scan", handler.scanLibrary)
	mux.HandleFunc("POST /api/v1/library/import", handler.importLibraryFile)
	mux.HandleFunc("POST /api/v1/library/import-completed", handler.importCompletedDownloads)
	mux.HandleFunc("POST /api/v1/library/import-reviews/{id}/resolve", handler.resolveImportReview)

	return withCORS(deps.Config.WebOrigin, withAuth(deps.Config.APIKey, deps.Auth, mux))
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

type readinessReport struct {
	Status      string          `json:"status"`
	Summary     string          `json:"summary"`
	Steps       []readinessStep `json:"steps"`
	GeneratedAt time.Time       `json:"generatedAt"`
}

type readinessStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Required    bool   `json:"required"`
	Message     string `json:"message"`
	ActionLabel string `json:"actionLabel,omitempty"`
	TargetView  string `json:"targetView,omitempty"`
}

func (h *handler) readiness(w http.ResponseWriter, r *http.Request) {
	report := h.readinessReport(r.Context())
	writeJSON(w, http.StatusOK, report)
}

func (h *handler) readinessReport(ctx context.Context) readinessReport {
	steps := []readinessStep{
		h.databaseReadinessStep(),
		h.metadataReadinessStep(ctx),
		h.libraryReadinessStep(ctx),
		h.indexerReadinessStep(ctx),
		h.downloadClientReadinessStep(ctx),
		h.qualityProfileReadinessStep(ctx),
	}

	blocked := 0
	warnings := 0
	for _, step := range steps {
		switch step.Status {
		case "blocked":
			if step.Required {
				blocked++
			} else {
				warnings++
			}
		case "warning":
			warnings++
		}
	}

	status := "ready"
	summary := "Readarr workflow is ready for metadata search, monitoring, release decisions, and imports."
	if blocked > 0 {
		status = "blocked"
		summary = strconv.Itoa(blocked) + " required setup step"
		if blocked != 1 {
			summary += "s"
		}
		summary += " must be completed before the Readarr workflow is usable."
	} else if warnings > 0 {
		status = "warning"
		summary = "Core Readarr workflow is usable; " + strconv.Itoa(warnings) + " setup step"
		if warnings != 1 {
			summary += "s"
		}
		summary += " need attention."
	}

	return readinessReport{
		Status:      status,
		Summary:     summary,
		Steps:       steps,
		GeneratedAt: time.Now().UTC(),
	}
}

func (h *handler) databaseReadinessStep() readinessStep {
	if databaseType(h.deps.Config.DatabaseURL) != "none" {
		return readinessStep{
			ID:       "database",
			Title:    "Postgres persistence",
			Status:   "ready",
			Required: true,
			Message:  "Database-backed wanted queues, history, root folders, and compatibility resources are enabled.",
		}
	}
	return readinessStep{
		ID:          "database",
		Title:       "Postgres persistence",
		Status:      "blocked",
		Required:    true,
		Message:     "Set LIBRARRY_DATABASE_URL and restart the API to persist Readarr workflow state.",
		ActionLabel: "Open settings",
		TargetView:  "settings",
	}
}

func (h *handler) metadataReadinessStep(ctx context.Context) readinessStep {
	if h.deps.Metadata == nil {
		return readinessStep{
			ID:          "metadata",
			Title:       "Metadata providers",
			Status:      "blocked",
			Required:    true,
			Message:     "No metadata service is available.",
			ActionLabel: "Providers",
			TargetView:  "providers",
		}
	}
	health := h.deps.Metadata.Health(ctx)
	ready := 0
	configured := 0
	for _, provider := range health {
		if provider.Configured {
			configured++
		}
		if provider.Status == "ready" {
			ready++
		}
	}
	if ready > 0 {
		status := "ready"
		message := strconv.Itoa(ready) + "/" + strconv.Itoa(len(health)) + " providers are ready for lookup and import evidence."
		if configured < len(health) {
			status = "warning"
			message += " Add Hardcover for richer series and edition metadata."
		}
		return readinessStep{
			ID:          "metadata",
			Title:       "Metadata providers",
			Status:      status,
			Required:    true,
			Message:     message,
			ActionLabel: "Providers",
			TargetView:  "providers",
		}
	}
	return readinessStep{
		ID:          "metadata",
		Title:       "Metadata providers",
		Status:      "blocked",
		Required:    true,
		Message:     "Configure at least one metadata provider before monitoring authors or books.",
		ActionLabel: "Providers",
		TargetView:  "providers",
	}
}

func (h *handler) libraryReadinessStep(ctx context.Context) readinessStep {
	config, err := h.effectiveLibraryConfig(ctx)
	if err != nil {
		return readinessStep{
			ID:          "library",
			Title:       "Library roots",
			Status:      "blocked",
			Required:    true,
			Message:     err.Error(),
			ActionLabel: "Library settings",
			TargetView:  "settings",
		}
	}
	config = library.NormalizeConfig(config)
	ebookRoot := strings.TrimSpace(config.EbookRoot)
	audiobookRoot := strings.TrimSpace(config.AudiobookRoot)
	switch {
	case ebookRoot != "" && audiobookRoot != "":
		return readinessStep{
			ID:       "library",
			Title:    "Library roots",
			Status:   "ready",
			Required: true,
			Message:  "Ebook and audiobook import roots are configured.",
		}
	case ebookRoot != "" || audiobookRoot != "":
		return readinessStep{
			ID:          "library",
			Title:       "Library roots",
			Status:      "warning",
			Required:    true,
			Message:     "One library root is configured. Add the other root when managing both ebooks and audiobooks.",
			ActionLabel: "Library settings",
			TargetView:  "settings",
		}
	default:
		return readinessStep{
			ID:          "library",
			Title:       "Library roots",
			Status:      "blocked",
			Required:    true,
			Message:     "Configure ebook and audiobook roots so imports and rename previews know where books belong.",
			ActionLabel: "Library settings",
			TargetView:  "settings",
		}
	}
}

func (h *handler) indexerReadinessStep(ctx context.Context) readinessStep {
	health := h.integrationHealthList(ctx)
	for _, integration := range health {
		if integration.Name != "Prowlarr" {
			continue
		}
		if integration.Status == "ready" {
			return readinessStep{
				ID:       "indexer",
				Title:    "Prowlarr indexer",
				Status:   "ready",
				Required: true,
				Message:  integration.Message,
			}
		}
		status := "blocked"
		if integration.Configured {
			status = "warning"
		}
		return readinessStep{
			ID:          "indexer",
			Title:       "Prowlarr indexer",
			Status:      status,
			Required:    true,
			Message:     integration.Message,
			ActionLabel: "Integrations",
			TargetView:  "settings",
		}
	}
	return readinessStep{
		ID:          "indexer",
		Title:       "Prowlarr indexer",
		Status:      "blocked",
		Required:    true,
		Message:     "Configure Prowlarr so wanted searches and feed sync can find book releases.",
		ActionLabel: "Integrations",
		TargetView:  "settings",
	}
}

func (h *handler) downloadClientReadinessStep(ctx context.Context) readinessStep {
	health := h.integrationHealthList(ctx)
	var configured []string
	var ready []string
	for _, integration := range health {
		if integration.Name == "Prowlarr" {
			continue
		}
		if integration.Configured {
			configured = append(configured, integration.Name)
		}
		if integration.Status == "ready" {
			ready = append(ready, integration.Name)
		}
	}
	if len(ready) > 0 {
		return readinessStep{
			ID:       "download-client",
			Title:    "Download client",
			Status:   "ready",
			Required: true,
			Message:  strings.Join(ready, ", ") + " ready for approved book releases.",
		}
	}
	status := "blocked"
	message := "Configure qBittorrent, Transmission, or SABnzbd so approved releases can be sent to a client."
	if len(configured) > 0 {
		status = "warning"
		message = strings.Join(configured, ", ") + " configured but not ready. Check credentials and network access."
	}
	return readinessStep{
		ID:          "download-client",
		Title:       "Download client",
		Status:      status,
		Required:    true,
		Message:     message,
		ActionLabel: "Integrations",
		TargetView:  "settings",
	}
}

func (h *handler) qualityProfileReadinessStep(ctx context.Context) readinessStep {
	if h.deps.Wanted == nil {
		return readinessStep{
			ID:          "quality-profiles",
			Title:       "Quality profiles",
			Status:      "warning",
			Required:    false,
			Message:     "Wanted service is unavailable; release decisions will use built-in defaults only.",
			ActionLabel: "Profiles",
			TargetView:  "settings",
		}
	}
	profiles, err := h.deps.Wanted.ListQualityProfiles(ctx)
	if err != nil {
		return readinessStep{
			ID:          "quality-profiles",
			Title:       "Quality profiles",
			Status:      "warning",
			Required:    false,
			Message:     err.Error(),
			ActionLabel: "Profiles",
			TargetView:  "settings",
		}
	}
	if len(profiles) == 0 {
		return readinessStep{
			ID:          "quality-profiles",
			Title:       "Quality profiles",
			Status:      "warning",
			Required:    false,
			Message:     "No quality profiles are configured. Add profiles to tune release scoring and upgrades.",
			ActionLabel: "Profiles",
			TargetView:  "settings",
		}
	}
	return readinessStep{
		ID:       "quality-profiles",
		Title:    "Quality profiles",
		Status:   "ready",
		Required: false,
		Message:  strconv.Itoa(len(profiles)) + " release policy profiles are available.",
	}
}

func (h *handler) integrationHealthList(ctx context.Context) []acquisition.IntegrationHealth {
	if h.deps.Acquire == nil {
		return nil
	}
	return h.deps.Acquire.Health(ctx)
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	queryText := strings.TrimSpace(r.URL.Query().Get("query"))
	if queryText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query is required"})
		return
	}

	query := metadata.Query{
		Query:             queryText,
		Type:              metadata.SearchType(defaultString(r.URL.Query().Get("type"), string(metadata.SearchTypeBook))),
		Format:            metadata.MediaFormat(defaultString(r.URL.Query().Get("format"), string(metadata.FormatAny))),
		PreferredLanguage: h.searchLanguageForRequest(r),
		Limit:             10,
		ProviderKey:       r.URL.Query().Get("providerKey"),
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

func (h *handler) integrationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.effectiveIntegrationConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":  integrationsettings.ToSettings(config),
		"persisted": h.deps.Compat != nil,
	})
}

func (h *handler) updateIntegrationConfig(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var patch integrationsettings.Patch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid integration config payload"})
		return
	}
	current, err := h.effectiveIntegrationConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	next := integrationsettings.ApplyPatch(current, patch)
	if h.deps.Compat != nil {
		if err := integrationsettings.SaveResources(r.Context(), h.deps.Compat, next); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
	}
	if configurable, ok := h.deps.Acquire.(configurableAcquisitionService); ok {
		configurable.Reconfigure(next)
	}
	response := map[string]any{
		"settings":  integrationsettings.ToSettings(next),
		"persisted": h.deps.Compat != nil,
	}
	if h.deps.Acquire != nil {
		response["integrations"] = h.deps.Acquire.Health(r.Context())
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *handler) effectiveIntegrationConfig(ctx context.Context) (acquisition.IntegrationConfig, error) {
	if configurable, ok := h.deps.Acquire.(configurableAcquisitionService); ok {
		return configurable.IntegrationConfig(), nil
	}
	base := acquisition.IntegrationConfig{
		ProwlarrURL:       h.deps.Config.ProwlarrURL,
		ProwlarrAPIKey:    h.deps.Config.ProwlarrAPIKey,
		QBittorrentURL:    h.deps.Config.QBittorrentURL,
		QBittorrentUser:   h.deps.Config.QBittorrentUser,
		QBittorrentPass:   h.deps.Config.QBittorrentPass,
		TransmissionURL:   h.deps.Config.TransmissionURL,
		TransmissionUser:  h.deps.Config.TransmissionUser,
		TransmissionPass:  h.deps.Config.TransmissionPass,
		SABnzbdURL:        h.deps.Config.SABnzbdURL,
		SABnzbdAPIKey:     h.deps.Config.SABnzbdAPIKey,
		SABnzbdUser:       h.deps.Config.SABnzbdUser,
		SABnzbdPass:       h.deps.Config.SABnzbdPass,
		EbookCategory:     h.deps.Config.EbookCategory,
		AudiobookCategory: h.deps.Config.AudiobookCategory,
		BookTorrentRoot:   h.deps.Config.BookTorrentRoot,
	}
	return integrationsettings.FromResources(ctx, h.deps.Compat, base)
}

type libraryConfigSettings struct {
	EbookLibraryRoot       string `json:"ebookLibraryRoot"`
	AudiobookLibraryRoot   string `json:"audiobookLibraryRoot"`
	NamingAuthorFolder     string `json:"namingAuthorFolder"`
	NamingBookFolder       string `json:"namingBookFolder"`
	NamingFileName         string `json:"namingFileName"`
	NamingSpaceReplacement string `json:"namingSpaceReplacement"`
	// RenameBooks controls whether imports apply the naming templates
	// (default true; when false imports keep the source basename inside the
	// author folder). Omitting the key on PUT leaves the stored value alone.
	RenameBooks            *bool  `json:"renameBooks,omitempty"`
	StandardSearchLanguage string `json:"standardSearchLanguage"`
	// RecycleBin is read-only surface: it is configured through
	// LIBRARRY_RECYCLE_BIN, not through this endpoint.
	RecycleBin string `json:"recycleBin"`
}

func (h *handler) libraryConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.effectiveLibraryConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":  libraryConfigSettingsFromConfig(config),
		"persisted": h.deps.Compat != nil,
	})
}

func (h *handler) updateLibraryConfig(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var settings libraryConfigSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid library config payload"})
		return
	}
	current, err := h.effectiveLibraryConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	next, err := applyLibraryConfigSettings(current, settings)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if h.deps.Compat != nil {
		if _, err := h.saveLibraryRootFolders(r.Context(), next); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		if err := h.saveLibraryNamingConfig(r.Context(), next); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
	}
	if rootService, ok := h.deps.Library.(rootFolderLibraryService); ok {
		// The legacy two-root fields map onto the per-format default native
		// root folders.
		if err := rootService.SyncDefaultRootFolders(r.Context(), next); err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
	}
	if configurable, ok := h.deps.Library.(configurableLibraryService); ok {
		configurable.Reconfigure(next)
	}
	if configurable, ok := h.deps.Wanted.(configurableWantedSearchLanguage); ok {
		configurable.SetDefaultSearchLanguage(next.StandardSearchLanguage)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":  libraryConfigSettingsFromConfig(next),
		"persisted": h.deps.Compat != nil,
	})
}

func (h *handler) effectiveLibraryConfig(ctx context.Context) (library.Config, error) {
	if configurable, ok := h.deps.Library.(configurableLibraryService); ok {
		return configurable.Config(), nil
	}
	renameBooks := h.deps.Config.RenameBooks
	base := library.Config{
		EbookRoot:                  h.deps.Config.EbookLibraryRoot,
		AudiobookRoot:              h.deps.Config.AudiobookLibraryRoot,
		NamingAuthorFolderTemplate: h.deps.Config.NamingAuthorFolder,
		NamingBookFolderTemplate:   h.deps.Config.NamingBookFolder,
		NamingFileNameTemplate:     h.deps.Config.NamingFileName,
		NamingSpaceReplacement:     h.deps.Config.NamingSpaceReplacement,
		RenameBooks:                &renameBooks,
		StandardSearchLanguage:     h.deps.Config.StandardSearchLanguage,
	}
	if h.deps.Compat == nil {
		return library.NormalizeConfig(base), nil
	}
	roots, err := h.deps.Compat.ListRootFolders(ctx)
	if err != nil {
		return library.Config{}, err
	}
	config := library.ConfigWithRootFolders(base, roots)
	resource, ok, err := h.deps.Compat.GetResource(ctx, "config-naming", 1)
	if err != nil {
		return library.Config{}, err
	}
	if ok {
		config = library.ConfigWithNamingRecord(config, resource.Payload)
	}
	return config, nil
}

func (h *handler) searchLanguageForRequest(r *http.Request) string {
	language := strings.TrimSpace(defaultString(r.URL.Query().Get("language"), r.URL.Query().Get("preferredLanguage")))
	if language != "" {
		return normalizeStandardSearchLanguage(language)
	}
	config, err := h.effectiveLibraryConfig(r.Context())
	if err != nil {
		return normalizeStandardSearchLanguage(h.deps.Config.StandardSearchLanguage)
	}
	return normalizeStandardSearchLanguage(config.StandardSearchLanguage)
}

func libraryConfigSettingsFromConfig(config library.Config) libraryConfigSettings {
	config = library.NormalizeConfig(config)
	renameBooks := config.RenameBooksEnabled()
	return libraryConfigSettings{
		EbookLibraryRoot:       config.EbookRoot,
		AudiobookLibraryRoot:   config.AudiobookRoot,
		NamingAuthorFolder:     config.NamingAuthorFolderTemplate,
		NamingBookFolder:       config.NamingBookFolderTemplate,
		NamingFileName:         config.NamingFileNameTemplate,
		NamingSpaceReplacement: config.NamingSpaceReplacement,
		RenameBooks:            &renameBooks,
		StandardSearchLanguage: config.StandardSearchLanguage,
		RecycleBin:             config.RecycleBin,
	}
}

func applyLibraryConfigSettings(current library.Config, settings libraryConfigSettings) (library.Config, error) {
	next := library.NormalizeConfig(current)
	ebookRoot := strings.TrimSpace(settings.EbookLibraryRoot)
	audiobookRoot := strings.TrimSpace(settings.AudiobookLibraryRoot)
	if ebookRoot == "" {
		return library.Config{}, errors.New("ebook library root is required")
	}
	if audiobookRoot == "" {
		return library.Config{}, errors.New("audiobook library root is required")
	}
	authorFolder := strings.TrimSpace(settings.NamingAuthorFolder)
	bookFolder := strings.TrimSpace(settings.NamingBookFolder)
	fileName := strings.TrimSpace(settings.NamingFileName)
	if authorFolder == "" {
		return library.Config{}, errors.New("author folder naming template is required")
	}
	if bookFolder == "" {
		return library.Config{}, errors.New("book folder naming template is required")
	}
	if fileName == "" {
		return library.Config{}, errors.New("file naming template is required")
	}
	next.EbookRoot = ebookRoot
	next.AudiobookRoot = audiobookRoot
	next.NamingAuthorFolderTemplate = authorFolder
	next.NamingBookFolderTemplate = bookFolder
	next.NamingFileNameTemplate = fileName
	next.NamingSpaceReplacement = strings.TrimSpace(settings.NamingSpaceReplacement)
	if settings.RenameBooks != nil {
		renameBooks := *settings.RenameBooks
		next.RenameBooks = &renameBooks
	}
	next.StandardSearchLanguage = normalizeStandardSearchLanguage(settings.StandardSearchLanguage)
	return next, nil
}

func normalizeStandardSearchLanguage(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return "English"
	}
	switch strings.ToLower(language) {
	case "any", "all", "none", "no preference":
		return "Any"
	case "en", "eng", "english":
		return "English"
	default:
		return language
	}
}

func (h *handler) saveLibraryRootFolders(ctx context.Context, config library.Config) ([]compatdata.RootFolder, error) {
	if h.deps.Compat == nil {
		return nil, nil
	}
	existing, err := h.deps.Compat.ListRootFolders(ctx)
	if err != nil {
		return nil, err
	}
	config = library.NormalizeConfig(config)
	saved := make([]compatdata.RootFolder, 0, 2)
	for _, target := range []struct {
		format string
		name   string
		path   string
	}{
		{format: "ebook", name: "Ebooks", path: config.EbookRoot},
		{format: "audiobook", name: "Audiobooks", path: config.AudiobookRoot},
	} {
		folder := compatdata.RootFolder{
			Name:        target.name,
			Path:        target.path,
			MediaFormat: target.format,
			Metadata:    libraryRootMetadata(nil),
		}
		if root, ok := libraryRootFolderForFormat(existing, target.format); ok {
			folder.Metadata = libraryRootMetadata(root.Metadata)
			updated, found, err := h.deps.Compat.UpdateRootFolder(ctx, root.ID, folder)
			if err != nil {
				return nil, err
			}
			if found {
				saved = append(saved, updated)
				continue
			}
		}
		created, err := h.deps.Compat.CreateRootFolder(ctx, folder)
		if err != nil {
			return nil, err
		}
		saved = append(saved, created)
	}
	return saved, nil
}

func (h *handler) saveLibraryNamingConfig(ctx context.Context, config library.Config) error {
	if h.deps.Compat == nil {
		return nil
	}
	config = library.NormalizeConfig(config)
	_, err := h.deps.Compat.UpsertResource(ctx, compatdata.Resource{
		ResourceType: "config-naming",
		CompatID:     1,
		Name:         "naming config",
		Payload:      h.libraryNamingPayload(config),
	})
	return err
}

func (h *handler) libraryNamingPayload(config library.Config) map[string]any {
	config = library.NormalizeConfig(config)
	return map[string]any{
		"id":                             1,
		"renameBooks":                    config.RenameBooksEnabled(),
		"replaceIllegalCharacters":       true,
		"colonReplacementFormat":         "delete",
		"standardBookFormat":             config.NamingFileNameTemplate,
		"authorFolderFormat":             config.NamingAuthorFolderTemplate,
		"bookFolderFormat":               config.NamingBookFolderTemplate,
		"includeAuthorName":              strings.Contains(config.NamingFileNameTemplate, "{Author}"),
		"includeBookTitle":               strings.Contains(config.NamingFileNameTemplate, "{Title}"),
		"includeQuality":                 false,
		"replaceSpaces":                  config.NamingSpaceReplacement != "",
		"replaceSpacesWith":              config.NamingSpaceReplacement,
		"multiAuthorStyle":               "standard",
		"librarryAuthorFolderTemplate":   config.NamingAuthorFolderTemplate,
		"librarryBookFolderTemplate":     config.NamingBookFolderTemplate,
		"librarryFileNameTemplate":       config.NamingFileNameTemplate,
		"librarryStandardSearchLanguage": config.StandardSearchLanguage,
		"standardSearchLanguage":         config.StandardSearchLanguage,
	}
}

func libraryRootFolderForFormat(roots []compatdata.RootFolder, format string) (compatdata.RootFolder, bool) {
	for _, root := range roots {
		if !payloadBoolDefault(root.Metadata, "librarryLibraryRoot", false) || !libraryRootFolderMatchesFormat(root, format) {
			continue
		}
		return root, true
	}
	for _, root := range roots {
		if libraryRootFolderMatchesFormat(root, format) {
			return root, true
		}
	}
	return compatdata.RootFolder{}, false
}

func libraryRootFolderMatchesFormat(root compatdata.RootFolder, format string) bool {
	mediaFormat := strings.ToLower(strings.TrimSpace(root.MediaFormat))
	if mediaFormat == format {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(root.Name))
	if format == "ebook" {
		return mediaFormat == "books" || strings.Contains(name, "ebook")
	}
	return format == "audiobook" && strings.Contains(name, "audio")
}

func libraryRootMetadata(existing map[string]any) map[string]any {
	metadata := map[string]any{}
	for key, value := range existing {
		metadata[key] = value
	}
	metadata["source"] = "librarry-settings"
	metadata["updatedBy"] = "native-library-settings"
	metadata["librarryLibraryRoot"] = true
	return metadata
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
	if len(query.Languages) == 0 {
		if language := h.searchLanguageForRequest(r); language != "" && !strings.EqualFold(language, "Any") {
			query.Languages = []string{language}
		}
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
	request, err := decodeGrabRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid grab payload"})
		return
	}
	status, err := h.deps.Acquire.Grab(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	h.notifyDownloadGrab(r.Context(), "native-grab", status, "")
	writeJSON(w, http.StatusOK, status)
}

func decodeGrabRequest(r *http.Request) (acquisition.DownloadRequest, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "multipart/form-data") {
		return decodeMultipartGrabRequest(r)
	}
	var request acquisition.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return acquisition.DownloadRequest{}, err
	}
	return request, nil
}

func decodeMultipartGrabRequest(r *http.Request) (acquisition.DownloadRequest, error) {
	if err := r.ParseMultipartForm(maxGrabUploadBytes); err != nil {
		return acquisition.DownloadRequest{}, err
	}
	request := acquisition.DownloadRequest{
		Client:     strings.TrimSpace(firstFormValue(r, "client")),
		ReleaseURL: strings.TrimSpace(firstFormValue(r, "releaseUrl", "releaseURL", "url")),
		InfoHash:   strings.TrimSpace(firstFormValue(r, "infoHash")),
		Title:      strings.TrimSpace(firstFormValue(r, "title")),
		Protocol:   strings.TrimSpace(firstFormValue(r, "protocol")),
		Category:   strings.TrimSpace(firstFormValue(r, "category")),
		SavePath:   strings.TrimSpace(firstFormValue(r, "savePath")),
		Paused:     formBoolValue(r, "paused"),
		Tags:       formListValue(r, "tags"),
	}
	if name := strings.TrimSpace(firstFormValue(r, "uploadName", "fileName", "filename")); name != "" {
		request.UploadName = name
	}
	for _, field := range []string{"file", "torrent", "torrents", "upload"} {
		file, header, err := r.FormFile(field)
		if err != nil {
			if errors.Is(err, http.ErrMissingFile) {
				continue
			}
			return acquisition.DownloadRequest{}, err
		}
		defer file.Close()
		data, err := readLimitedUpload(file, maxGrabUploadBytes)
		if err != nil {
			return acquisition.DownloadRequest{}, err
		}
		request.UploadData = data
		if request.UploadName == "" && header != nil {
			request.UploadName = header.Filename
		}
		if request.Protocol == "" {
			request.Protocol = "torrent"
		}
		break
	}
	return request, nil
}

func firstFormValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.FormValue(key)); value != "" {
			return value
		}
	}
	return ""
}

func formBoolValue(r *http.Request, key string) bool {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func formListValue(r *http.Request, key string) []string {
	if r.MultipartForm == nil {
		return nil
	}
	values := r.MultipartForm.Value[key]
	var items []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	}
	return items
}

func readLimitedUpload(file io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("upload is too large")
	}
	return data, nil
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
	if h.deps.Wanted != nil {
		statuses = h.deps.Wanted.AnnotateDownloads(r.Context(), statuses)
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
	if h.deps.Wanted != nil {
		annotated := h.deps.Wanted.AnnotateDownloads(r.Context(), []acquisition.DownloadStatus{details.Status})
		details.Status = annotated[0]
	}
	writeJSON(w, http.StatusOK, details)
}

func (h *handler) downloadResources(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	resources, err := h.deps.Acquire.DownloadResources(r.Context(), r.URL.Query().Get("client"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resources)
}

func (h *handler) downloadPreferences(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	preferences, err := h.deps.Acquire.DownloadPreferences(r.Context(), r.URL.Query().Get("client"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, preferences)
}

func (h *handler) updateDownloadPreferences(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadPreferencesUpdate
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download preferences payload"})
		return
	}
	if request.Client == "" {
		request.Client = r.URL.Query().Get("client")
	}
	preferences, err := h.deps.Acquire.UpdateDownloadPreferences(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, preferences)
}

func (h *handler) downloadCategoryAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadCategoryActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download category action payload"})
		return
	}
	result, err := h.deps.Acquire.DownloadCategoryAction(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) downloadTagAction(w http.ResponseWriter, r *http.Request) {
	if h.deps.Acquire == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "acquisition service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request acquisition.DownloadTagActionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid download tag action payload"})
		return
	}
	result, err := h.deps.Acquire.DownloadTagAction(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
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
	h.notifyFailedDownloads(r.Context(), "failed-download-recovery", run)
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

func (h *handler) releaseProfiles(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	profiles, err := h.deps.Wanted.ListReleaseProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if profiles == nil {
		profiles = []wanted.ReleaseProfile{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}

func (h *handler) createReleaseProfile(w http.ResponseWriter, r *http.Request) {
	h.saveReleaseProfile(w, r, "")
}

func (h *handler) updateReleaseProfile(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "release profile id is required"})
		return
	}
	h.saveReleaseProfile(w, r, id)
}

func (h *handler) saveReleaseProfile(w http.ResponseWriter, r *http.Request, id string) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	profile := wanted.ReleaseProfile{Enabled: true}
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid release profile payload"})
		return
	}
	if id != "" {
		profile.ID = id
	}
	saved, err := h.deps.Wanted.SaveReleaseProfile(r.Context(), profile)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": saved})
}

func (h *handler) deleteReleaseProfile(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "release profile id is required"})
		return
	}
	if err := h.deps.Wanted.DeleteReleaseProfile(r.Context(), id); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *handler) qualityDefinitions(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	definitions, err := h.deps.Wanted.ListQualityDefinitions(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if definitions == nil {
		definitions = []wanted.QualityDefinition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"definitions": definitions})
}

func (h *handler) updateQualityDefinitions(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid quality definitions payload"})
		return
	}
	// Accept both a bare array and a {"definitions": [...]} envelope.
	var definitions []wanted.QualityDefinition
	if err := json.Unmarshal(body, &definitions); err != nil {
		var envelope struct {
			Definitions []wanted.QualityDefinition `json:"definitions"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.Definitions == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid quality definitions payload"})
			return
		}
		definitions = envelope.Definitions
	}
	saved, err := h.deps.Wanted.UpdateQualityDefinitions(r.Context(), definitions)
	if err != nil {
		if strings.HasPrefix(err.Error(), "unknown quality") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if saved == nil {
		saved = []wanted.QualityDefinition{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"definitions": saved})
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
	if subscriptions == nil {
		subscriptions = []wanted.AuthorSubscription{}
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
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "metadata profile") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, subscription)
}

func (h *handler) updateAuthorSubscription(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author update payload"})
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author update payload"})
		return
	}
	// Tags accept labels (or legacy integer ids), so they decode separately
	// from the typed struct.
	tagsRaw, hasTags := raw["tags"]
	delete(raw, "tags")
	stripped, err := json.Marshal(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author update payload"})
		return
	}
	var request wanted.AuthorUpdateRequest
	if err := json.Unmarshal(stripped, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author update payload"})
		return
	}
	if hasTags {
		labels, err := decodeTagLabels(tagsRaw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author update payload"})
			return
		}
		request.Tags = h.resolveTagLabels(r.Context(), labels)
		request.TagsSet = true
	}
	if _, ok := raw["allowedLanguages"]; ok {
		request.AllowedLanguagesSet = true
	}
	if _, ok := raw["mustNotContain"]; ok {
		request.MustNotContainSet = true
	}
	subscription, err := h.deps.Wanted.UpdateAuthorSubscription(r.Context(), r.PathValue("id"), request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "metadata profile") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, subscription)
}

func (h *handler) deleteAuthorSubscription(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "author subscription id is required"})
		return
	}
	if err := h.deps.Wanted.DeleteAuthorSubscription(r.Context(), id); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *handler) authorMetadataReviews(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reviews, err := h.deps.Wanted.ListAuthorMetadataReviews(r.Context(), wanted.AuthorMetadataReviewQuery{
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (h *handler) resolveAuthorMetadataReview(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "review id is required"})
		return
	}
	defer r.Body.Close()
	var request wanted.AuthorMetadataReviewDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid author metadata review decision payload"})
		return
	}
	outcome, err := h.deps.Wanted.ResolveAuthorMetadataReview(r.Context(), id, request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) listWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	var items []wanted.WantedItem
	var err error
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("view")), "cutoff-unmet") {
		items, err = h.deps.Wanted.ListCutoffUnmet(r.Context())
	} else {
		items, err = h.deps.Wanted.List(r.Context(), r.URL.Query().Get("status"))
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if items == nil {
		items = []wanted.WantedItem{}
	}
	items = h.deps.Wanted.AnnotateWantedStates(r.Context(), items)
	writeJSON(w, http.StatusOK, map[string]any{"wanted": items})
}

func (h *handler) bulkUpdateWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.WantedBulkRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted bulk payload"})
		return
	}
	if len(request.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one wanted id is required"})
		return
	}
	results, err := h.deps.Wanted.BulkUpdateWanted(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if results == nil {
		results = []wanted.WantedBulkResult{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
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
		// Root-folder validation (unknown id, format mismatch) is caller
		// error, not an upstream failure.
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "root folder") {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) updateWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wanted item id is required"})
		return
	}
	defer r.Body.Close()
	request, err := decodeWantedUpdateRequest(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted update payload"})
		return
	}
	if request.TagsSet {
		request.Tags = h.resolveTagLabels(r.Context(), request.Tags)
	}
	item, err := h.deps.Wanted.UpdateWanted(r.Context(), id, request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) deleteWanted(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wanted item id is required"})
		return
	}
	if err := h.deps.Wanted.DeleteWanted(r.Context(), id); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) clearWantedOverride(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	field := strings.TrimSpace(r.PathValue("field"))
	if id == "" || field == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wanted item id and override field are required"})
		return
	}
	item, err := h.deps.Wanted.ClearWantedManualOverrides(r.Context(), id, []string{field})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) wantedMetadata(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	provenanceService, ok := h.deps.Wanted.(metadataProvenanceService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted metadata provenance is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wanted item id is required"})
		return
	}
	provenance, err := provenanceService.MetadataProvenance(r.Context(), id)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, provenance)
}

func (h *handler) wantedMetadataReview(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	provenanceService, ok := h.deps.Wanted.(metadataProvenanceService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted metadata review is unavailable"})
		return
	}
	queue, err := provenanceService.MetadataReviewQueue(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, queue)
}

func (h *handler) confirmWantedMetadataReviewCanonical(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	provenanceService, ok := h.deps.Wanted.(metadataProvenanceService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted metadata review is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.MetadataReviewConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted metadata review confirmation payload"})
		return
	}
	outcome, err := provenanceService.ConfirmMetadataReviewCanonical(r.Context(), request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) applyWantedMetadataCorrection(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	provenanceService, ok := h.deps.Wanted.(metadataProvenanceService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted metadata correction is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wanted item id is required"})
		return
	}
	defer r.Body.Close()
	var request wanted.MetadataCorrectionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted metadata correction payload"})
		return
	}
	provenance, err := provenanceService.ApplyMetadataCorrection(r.Context(), id, request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, provenance)
}

func (h *handler) applyWantedMetadataCorrections(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	provenanceService, ok := h.deps.Wanted.(metadataProvenanceService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted metadata correction is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "wanted item id is required"})
		return
	}
	defer r.Body.Close()
	var request wanted.MetadataCorrectionBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid wanted metadata correction payload"})
		return
	}
	provenance, err := provenanceService.ApplyMetadataCorrections(r.Context(), id, request)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, provenance)
}

func decodeWantedUpdateRequest(body io.Reader) (wanted.WantedUpdateRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return wanted.WantedUpdateRequest{}, err
	}
	var request wanted.WantedUpdateRequest
	if value, ok := raw["title"]; ok {
		if err := json.Unmarshal(value, &request.Title); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
	}
	if value, ok := raw["authorName"]; ok {
		if err := json.Unmarshal(value, &request.AuthorName); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
	}
	if value, ok := raw["coverUrl"]; ok {
		if err := json.Unmarshal(value, &request.CoverURL); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
	}
	if value, ok := raw["qualityProfile"]; ok {
		if err := json.Unmarshal(value, &request.QualityProfile); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
	}
	if value, ok := raw["status"]; ok {
		if err := json.Unmarshal(value, &request.Status); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
	}
	if value, ok := raw["monitored"]; ok {
		var monitored bool
		if err := json.Unmarshal(value, &monitored); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
		request.Monitored = &monitored
	}
	if value, ok := raw["tags"]; ok {
		labels, err := decodeTagLabels(value)
		if err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
		request.Tags = labels
		request.TagsSet = true
	}
	if value, ok := raw["rootFolderId"]; ok {
		var rootFolderID string
		if err := json.Unmarshal(value, &rootFolderID); err != nil {
			return wanted.WantedUpdateRequest{}, err
		}
		request.RootFolderID = &rootFolderID
	}
	return request, nil
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

func (h *handler) acquisitionQueue(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	queue, err := h.deps.Wanted.AcquisitionQueue(r.Context(), wanted.AcquisitionQueueQuery{
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, queue)
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
	h.notifyDownloadGrab(r.Context(), "wanted-grab", status, r.PathValue("id"))
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
	h.notifyMonitorGrabs(r.Context(), "wanted-monitor", run)
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
	h.notifyFeedGrabs(r.Context(), "feed-sync", run)
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
	h.notifyUpgradeGrabs(r.Context(), "upgrade-search", run)
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

func (h *handler) blocklist(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	entries, err := h.deps.Wanted.ListBlocklist(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []wanted.BlocklistEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": entries})
}

func (h *handler) createBlocklistEntry(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.BlocklistDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid blocklist payload"})
		return
	}
	if strings.TrimSpace(request.DownloadID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "download id is required"})
		return
	}
	entry, err := h.deps.Wanted.BlocklistDownload(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *handler) deleteBlocklistEntry(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "blocklist id is required"})
		return
	}
	if err := h.deps.Wanted.DeleteBlocklistEntry(r.Context(), id); err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (h *handler) clearBlocklist(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request struct {
		IDs []string `json:"ids"`
	}
	if r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid blocklist clear payload"})
			return
		}
	}
	removed, err := h.deps.Wanted.ClearBlocklist(r.Context(), request.IDs)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (h *handler) markDownloadFailed(w http.ResponseWriter, r *http.Request) {
	if h.deps.Wanted == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wanted service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request wanted.ManualFailRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid mark-failed payload"})
		return
	}
	if strings.TrimSpace(request.ID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "download id is required"})
		return
	}
	outcome, err := h.deps.Wanted.MarkDownloadFailedManually(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) rootFolderService(w http.ResponseWriter) (rootFolderLibraryService, bool) {
	service, ok := h.deps.Library.(rootFolderLibraryService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library root folder service is unavailable"})
		return nil, false
	}
	return service, true
}

func (h *handler) libraryRootFolders(w http.ResponseWriter, r *http.Request) {
	service, ok := h.rootFolderService(w)
	if !ok {
		return
	}
	folders, err := service.ListRootFolders(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	redacted := make([]library.RootFolder, 0, len(folders))
	for _, folder := range folders {
		redacted = append(redacted, library.RedactRootFolderSecrets(folder))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rootFolders": redacted})
}

func (h *handler) createLibraryRootFolder(w http.ResponseWriter, r *http.Request) {
	service, ok := h.rootFolderService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var folder library.RootFolder
	if err := json.NewDecoder(r.Body).Decode(&folder); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid root folder payload"})
		return
	}
	if strings.TrimSpace(folder.Path) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "root folder path is required"})
		return
	}
	created, err := service.CreateRootFolder(r.Context(), folder)
	if err != nil {
		writeJSON(w, rootFolderErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rootFolder": library.RedactRootFolderSecrets(created)})
}

func (h *handler) updateLibraryRootFolder(w http.ResponseWriter, r *http.Request) {
	service, ok := h.rootFolderService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "root folder id is required"})
		return
	}
	defer r.Body.Close()
	var folder library.RootFolder
	if err := json.NewDecoder(r.Body).Decode(&folder); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid root folder payload"})
		return
	}
	if strings.TrimSpace(folder.Path) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "root folder path is required"})
		return
	}
	updated, err := service.UpdateRootFolder(r.Context(), id, folder)
	if err != nil {
		writeJSON(w, rootFolderErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rootFolder": library.RedactRootFolderSecrets(updated)})
}

func (h *handler) deleteLibraryRootFolder(w http.ResponseWriter, r *http.Request) {
	service, ok := h.rootFolderService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "root folder id is required"})
		return
	}
	if err := service.DeleteRootFolder(r.Context(), id); err != nil {
		writeJSON(w, rootFolderErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func rootFolderErrorStatus(err error) int {
	var conflict *library.ConflictError
	switch {
	case errors.As(err, &conflict):
		return http.StatusConflict
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	case strings.Contains(err.Error(), "is required"), strings.Contains(err.Error(), "must be"):
		return http.StatusBadRequest
	default:
		return http.StatusBadGateway
	}
}

func (h *handler) remotePathMappingService(w http.ResponseWriter) (remotePathMappingLibraryService, bool) {
	service, ok := h.deps.Library.(remotePathMappingLibraryService)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "remote path mapping service is unavailable"})
		return nil, false
	}
	return service, true
}

func (h *handler) libraryRemotePathMappings(w http.ResponseWriter, r *http.Request) {
	service, ok := h.remotePathMappingService(w)
	if !ok {
		return
	}
	mappings, err := service.ListRemotePathMappings(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if mappings == nil {
		mappings = []library.RemotePathMapping{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"mappings": mappings})
}

func (h *handler) createLibraryRemotePathMapping(w http.ResponseWriter, r *http.Request) {
	service, ok := h.remotePathMappingService(w)
	if !ok {
		return
	}
	defer r.Body.Close()
	var mapping library.RemotePathMapping
	if err := json.NewDecoder(r.Body).Decode(&mapping); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid remote path mapping payload"})
		return
	}
	created, err := service.CreateRemotePathMapping(r.Context(), mapping)
	if err != nil {
		writeJSON(w, rootFolderErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mapping": created})
}

func (h *handler) updateLibraryRemotePathMapping(w http.ResponseWriter, r *http.Request) {
	service, ok := h.remotePathMappingService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "remote path mapping id is required"})
		return
	}
	defer r.Body.Close()
	var mapping library.RemotePathMapping
	if err := json.NewDecoder(r.Body).Decode(&mapping); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid remote path mapping payload"})
		return
	}
	updated, err := service.UpdateRemotePathMapping(r.Context(), id, mapping)
	if err != nil {
		writeJSON(w, rootFolderErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mapping": updated})
}

func (h *handler) deleteLibraryRemotePathMapping(w http.ResponseWriter, r *http.Request) {
	service, ok := h.remotePathMappingService(w)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "remote path mapping id is required"})
		return
	}
	if err := service.DeleteRemotePathMapping(r.Context(), id); err != nil {
		writeJSON(w, rootFolderErrorStatus(err), map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
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
	if files == nil {
		files = []library.FileRecord{}
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

func (h *handler) refreshCalibreConversions(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.CalibreConversionRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid Calibre conversion refresh payload"})
		return
	}
	outcome, err := h.deps.Library.RefreshCalibreConversions(r.Context(), request)
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
	if outcome.Imported {
		h.notifyReleaseImport(r.Context(), "library-import", outcome)
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
	h.notifyCompletedImports(r.Context(), "completed-download-import", outcome)
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
	if outcome.Import != nil && outcome.Import.Imported {
		h.notifyReviewImport(r.Context(), "import-review", outcome)
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) resolveImportReviewsBulk(w http.ResponseWriter, r *http.Request) {
	if h.deps.Library == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "library service is unavailable"})
		return
	}
	defer r.Body.Close()
	var request library.ReviewBulkDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid bulk review decision payload"})
		return
	}
	ids, err := h.bulkImportReviewIDs(r.Context(), request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "at least one review id is required"})
		return
	}
	decision := library.ReviewDecisionRequest{
		Action:         request.Action,
		WantedID:       request.WantedID,
		Format:         request.Format,
		Move:           request.Move,
		ImportMode:     request.ImportMode,
		ConflictAction: request.ConflictAction,
		Overwrite:      request.Overwrite,
	}
	outcome := library.ReviewBulkDecisionOutcome{Requested: len(ids)}
	for _, id := range ids {
		result := library.ReviewBulkDecisionResult{ID: id}
		reviewOutcome, reviewErr := h.deps.Library.ResolveImportReview(r.Context(), id, decision)
		if reviewErr != nil {
			result.Status = "error"
			result.Message = reviewErr.Error()
			outcome.Errored++
			outcome.Results = append(outcome.Results, result)
			continue
		}
		result.Status = reviewOutcome.Review.Status
		result.Outcome = &reviewOutcome
		outcome.Resolved++
		switch strings.ToLower(strings.TrimSpace(reviewOutcome.Review.Status)) {
		case "imported":
			outcome.Imported++
		case "skipped":
			outcome.Skipped++
		case "rejected":
			outcome.Rejected++
		}
		if reviewOutcome.Import != nil && reviewOutcome.Import.Imported {
			h.notifyReviewImport(r.Context(), "import-review-bulk", reviewOutcome)
		}
		outcome.Results = append(outcome.Results, result)
	}
	writeJSON(w, http.StatusOK, outcome)
}

func (h *handler) bulkImportReviewIDs(ctx context.Context, request library.ReviewBulkDecisionRequest) ([]string, error) {
	ids := compactStringValues(append(append([]string{}, request.IDs...), request.ReviewIDs...))
	if len(ids) > 0 {
		return ids, nil
	}
	limit := request.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	reviews, err := h.deps.Library.ListImportReviews(ctx, library.ReviewListQuery{
		Status: defaultString(request.Status, "pending"),
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	ids = make([]string, 0, len(reviews))
	for _, review := range reviews {
		if strings.TrimSpace(review.ID) != "" {
			ids = append(ids, review.ID)
		}
	}
	return ids, nil
}

func compactStringValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	compact := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		compact = append(compact, trimmed)
	}
	return compact
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authExemptPath(path string) bool {
	switch path {
	case "/healthz", "/ping":
		return true
	default:
		return false
	}
}

func validAPIKey(r *http.Request, expected string) bool {
	for _, candidate := range requestAPIKeyCandidates(r) {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

func requestAPIKeyCandidates(r *http.Request) []string {
	query := r.URL.Query()
	candidates := []string{
		r.Header.Get("X-Api-Key"),
		query.Get("apikey"),
		query.Get("apiKey"),
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth != "" {
		fields := strings.Fields(auth)
		if len(fields) == 2 && (strings.EqualFold(fields[0], "Bearer") || strings.EqualFold(fields[0], "ApiKey")) {
			candidates = append(candidates, fields[1])
		}
	}
	compact := candidates[:0]
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			compact = append(compact, strings.TrimSpace(candidate))
		}
	}
	return compact
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

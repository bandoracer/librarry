package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/buildinfo"
	"github.com/bandoracer/librarry/backend/internal/library"
)

// This export deliberately uses an allowlist, not redaction of arbitrary config,
// logs, provider payloads or task errors. New secret-bearing fields in those
// types must never become part of a support bundle implicitly.
type supportReport struct {
	FormatVersion int                  `json:"formatVersion"`
	GeneratedAt   time.Time            `json:"generatedAt"`
	Build         map[string]any       `json:"build"`
	Database      supportDatabase      `json:"database"`
	Configuration map[string]any       `json:"configuration"`
	Providers     []supportProvider    `json:"providers"`
	Integrations  []supportIntegration `json:"integrations"`
	Roots         []supportRoot        `json:"roots"`
	Tasks         []supportTask        `json:"tasks"`
	Sections      map[string]string    `json:"sections"`
	Notes         []string             `json:"notes"`
}

type supportDatabase struct {
	Status              string    `json:"status"`
	CheckedAt           time.Time `json:"checkedAt"`
	SchemaAtStartup     string    `json:"schemaAtStartup,omitempty"`
	ServerVersionNumber *int      `json:"serverVersionNumber,omitempty"`
}

type supportProvider struct {
	Name          string     `json:"name"`
	Configured    bool       `json:"configured"`
	Status        string     `json:"status"`
	EvidenceScope string     `json:"evidenceScope"`
	Version       string     `json:"version"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
	LastSuccessAt *time.Time `json:"lastSuccessAt,omitempty"`
	RetryAfter    *time.Time `json:"retryAfter,omitempty"`
	Reachable     *bool      `json:"reachable,omitempty"`
	Authenticated *bool      `json:"authenticated,omitempty"`
}

type supportIntegration struct {
	Configured         *bool      `json:"configured,omitempty"`
	LastCheckedAt      *time.Time `json:"lastCheckedAt,omitempty"`
	LastSuccessAt      *time.Time `json:"lastSuccessAt,omitempty"`
	LastVersionAt      *time.Time `json:"lastVersionAt,omitempty"`
	RetryAfter         *time.Time `json:"retryAfter,omitempty"`
	Freshness          string     `json:"freshness,omitempty"`
	ObservedStatus     string     `json:"observedStatus,omitempty"`
	Reachable          *bool      `json:"reachable,omitempty"`
	Authenticated      *bool      `json:"authenticated,omitempty"`
	Name               string     `json:"name"`
	EndpointConfigured bool       `json:"endpointConfigured"`
	Status             string     `json:"status"`
	Version            string     `json:"version"`
}

type supportRoot struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checkedAt"`
}

type supportTask struct {
	ID                 string     `json:"id"`
	Enabled            bool       `json:"enabled"`
	Available          bool       `json:"available"`
	Running            bool       `json:"running"`
	Interval           string     `json:"interval"`
	RunState           string     `json:"runState"`
	HasError           bool       `json:"hasError"`
	Errors             int        `json:"errors"`
	UnreviewedFailures int        `json:"unreviewedFailures"`
	LastRunAt          *time.Time `json:"lastRunAt,omitempty"`
	LastFinishedAt     *time.Time `json:"lastFinishedAt,omitempty"`
	LastSuccessAt      *time.Time `json:"lastSuccessAt,omitempty"`
	NextRunAt          *time.Time `json:"nextRunAt,omitempty"`
	DurationMS         *int64     `json:"durationMs,omitempty"`
}

func (h *handler) databaseObservation(ctx context.Context) supportDatabase {
	result := supportDatabase{Status: "not_configured", SchemaAtStartup: h.deps.SchemaMigration}
	if h.deps.Database != nil {
		checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		result.Status = "unavailable"
		if h.deps.Database.PingContext(checkCtx) == nil {
			result.Status = "ready"
		}
	} else if strings.TrimSpace(h.deps.Config.DatabaseURL) != "" {
		result.Status = "unavailable"
	}
	result.CheckedAt = time.Now().UTC()
	return result
}

// Liveness is /healthz. This public probe intentionally reveals only whether the
// API can reach its persistence dependency, not paths, credentials or topology.
// External integration degradation must not turn into a container restart loop.
func (h *handler) operationalReadiness(w http.ResponseWriter, r *http.Request) {
	observation := h.databaseObservation(r.Context())
	status, code := "not_ready", http.StatusServiceUnavailable
	if observation.Status == "ready" {
		status, code = "ready", http.StatusOK
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, code, map[string]any{"status": status, "checkedAt": observation.CheckedAt})
}

func (h *handler) supportDiagnostics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	report := h.supportReport(ctx)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="librarry-support.json"`)
	writeJSON(w, http.StatusOK, report)
}

func (h *handler) supportReport(ctx context.Context) supportReport {
	cfg := h.deps.Config
	report := supportReport{
		FormatVersion: 1, GeneratedAt: time.Now().UTC(),
		Build:    map[string]any{"version": buildinfo.Version, "commit": buildinfo.Commit, "builtAt": buildinfo.BuildTime, "dirty": buildinfo.Dirty, "goVersion": runtime.Version(), "os": runtime.GOOS, "architecture": runtime.GOARCH, "imageDigest": "unknown"},
		Database: h.databaseObservation(ctx),
		Configuration: map[string]any{
			"authMethod": effectiveAuthMethod(h.deps.Auth), "apiKeyConfigured": strings.TrimSpace(cfg.APIKey) != "",
			"monitorAutoGrab": cfg.MonitorAutoGrab, "feedAutoGrab": cfg.FeedSyncAutoGrab,
			"failedDownloadAutoGrab": cfg.FailedDownloadAutoGrab, "upgradeAutoGrab": cfg.UpgradeSearchAutoGrab,
			"failedDownloadRemove": cfg.FailedDownloadRemove, "failedDownloadDeleteFiles": cfg.FailedDownloadDeleteFiles,
			"completedRemoveEnabled": cfg.CompletedRemoveEnabled,
			"completedImportMode":    supportEnum(cfg.CompletedImportMode, "hardlinkOrCopy", "hardlink", "copy"),
			"backupRetention":        cfg.BackupRetention,
		},
		Providers: []supportProvider{}, Integrations: []supportIntegration{}, Roots: []supportRoot{}, Tasks: []supportTask{},
		Sections: map[string]string{"providers": "unavailable", "integrations": "unavailable", "library": "unavailable", "tasks": "unavailable"},
		Notes: []string{
			"Generated time is the export time, not the time of the last successful operation. Provider and integration observations are process-local and disappear on restart.",
			"Readiness checks database connectivity only. Directory presence does not prove mount identity, write permission, free space or media integrity.",
			"Root labels are anonymous within this export. Paths, names, URLs, usernames, credentials, book metadata, notification targets and free-text errors are omitted.",
			"Task configuration and next run describe this API instance; persisted history can include peer instances. Unknown times stay omitted.",
			"No provider or download-client request is made. Remote versions and reachability without recorded evidence remain unknown; the image manifest digest is not available inside the process.",
			"Configuration is a selected effective snapshot, not a backup. Worker intervals and enable flags are in tasks; naming templates and other private free-text settings are omitted.",
		},
	}
	if report.Database.Status == "ready" {
		versionCtx, cancel := context.WithTimeout(ctx, time.Second)
		var version int
		if h.deps.Database.QueryRowContext(versionCtx, "select current_setting('server_version_num')::integer").Scan(&version) == nil {
			report.Database.ServerVersionNumber = &version
		}
		cancel()
	}
	if h.deps.Metadata != nil {
		report.Sections["providers"] = "available"
		for _, health := range h.deps.Metadata.Health(ctx) {
			name := supportEnum(health.Name, "Hardcover", "Open Library", "Google Books", "Local OPF")
			if name == "unknown" {
				continue
			}
			report.Providers = append(report.Providers, supportProvider{
				Name: name, Configured: health.Configured,
				Status:        supportEnum(health.Status, "ready", "configured", "missing_credentials", "invalid_credentials", "forbidden", "rate_limited", "unavailable", "degraded"),
				EvidenceScope: "process", Version: "unknown", LastCheckedAt: health.LastCheckedAt, LastSuccessAt: health.LastSuccessAt,
				RetryAfter: health.RetryAfter, Reachable: health.Reachable, Authenticated: health.Authenticated,
			})
		}
	}
	integration, integrationErr := h.effectiveIntegrationConfig(ctx)
	var observations []acquisition.IntegrationHealth
	if snapshots, ok := h.deps.Acquire.(interface {
		HealthSnapshot() (acquisition.IntegrationConfig, []acquisition.IntegrationHealth)
	}); ok {
		integration, observations = snapshots.HealthSnapshot()
		integrationErr = nil
	}
	if integrationErr == nil {
		report.Sections["integrations"] = "available"
		for _, item := range []struct{ name, endpoint string }{
			{"prowlarr", integration.ProwlarrURL}, {"qbittorrent", integration.QBittorrentURL},
			{"transmission", integration.TransmissionURL}, {"sabnzbd", integration.SABnzbdURL},
		} {
			entry := supportIntegration{Name: item.name, EndpointConfigured: strings.TrimSpace(item.endpoint) != "", Status: "unknown", Version: "unknown"}
			for _, observation := range observations {
				if !strings.EqualFold(observation.Name, item.name) {
					continue
				}
				configured := observation.Configured
				entry.Configured = &configured
				entry.Status = supportEnum(observation.Status, "configured", "missing_credentials", "ready", "degraded", "invalid_credentials", "rate_limited", "unavailable", "stale")
				entry.ObservedStatus = supportEnum(observation.ObservedStatus, "ready", "degraded", "invalid_credentials", "rate_limited", "unavailable")
				entry.Freshness = supportEnum(observation.Freshness, "never_checked", "fresh", "stale")
				entry.LastCheckedAt = observation.LastCheckedAt
				entry.LastSuccessAt = observation.LastSuccessAt
				entry.LastVersionAt = observation.LastVersionAt
				entry.RetryAfter = observation.RetryAfter
				entry.Reachable = observation.Reachable
				entry.Authenticated = observation.Authenticated
				if supportVersionPattern.MatchString(observation.Version) {
					entry.Version = observation.Version
				}
			}
			report.Integrations = append(report.Integrations, entry)
		}
		report.Roots = append(report.Roots, observeSupportRoot(ctx, "downloads", "downloads", integration.BookTorrentRoot))
	}
	if config, err := h.effectiveLibraryConfig(ctx); err == nil {
		report.Sections["library"] = "available"
		report.Configuration["renameBooks"] = config.RenameBooksEnabled()
		// Language is free text in legacy settings. Report only known policy values.
		report.Configuration["standardSearchLanguage"] = supportEnum(config.StandardSearchLanguage, "English", "any")
		paths := []struct{ role, path string }{{"ebook", config.EbookRoot}, {"audiobook", config.AudiobookRoot}}
		if roots, ok := h.deps.Library.(interface {
			RootLocations(context.Context) ([]library.RootLocation, error)
		}); ok {
			folders, err := roots.RootLocations(ctx)
			if err != nil {
				report.Sections["library"] = "partial"
			} else if len(folders) > 0 {
				paths = nil
				for _, folder := range folders {
					paths = append(paths, struct{ role, path string }{supportEnum(folder.MediaFormat, "ebook", "audiobook"), folder.Path})
				}
			}
		}
		for index, root := range paths {
			report.Roots = append(report.Roots, observeSupportRoot(ctx, fmt.Sprintf("library-%d", index+1), root.role, root.path))
		}
	}
	report.Roots = append(report.Roots, observeSupportRoot(ctx, "backups", "backups", cfg.BackupDir))
	if h.deps.Scheduler != nil {
		if tasks, err := h.deps.Scheduler.TasksContext(ctx); err == nil {
			report.Sections["tasks"] = "available"
			for _, task := range tasks {
				id := supportEnum(task.ID, "history-maintenance", "notification-delivery", "library-scan", "wanted-monitor", "author-monitor", "feed-sync", "failed-download-recovery", "upgrade-search", "calibre-refresh", "completed-import", "import-list-sync", "backup", "health-check")
				if id == "unknown" {
					continue
				}
				interval := "unknown"
				if duration, err := time.ParseDuration(task.Interval); err == nil {
					interval = duration.String()
				}
				report.Tasks = append(report.Tasks, supportTask{
					ID: id, Enabled: task.Enabled, Available: task.Available, Running: task.Running, Interval: interval,
					RunState: supportEnum(task.RunState, "running", "completed", "degraded", "failed", "interrupted"),
					HasError: task.LastError != "", Errors: task.Details.Errors, UnreviewedFailures: task.UnreviewedFailures,
					LastRunAt: task.LastRunAt, LastFinishedAt: task.LastFinishedAt, LastSuccessAt: task.LastSuccessAt,
					NextRunAt: task.NextRunAt, DurationMS: task.DurationMS,
				})
			}
		}
	}
	return report
}

func supportEnum(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return "unknown"
}

func observeSupportRoot(ctx context.Context, id, role, path string) supportRoot {
	result := supportRoot{ID: id, Role: role, Status: "not_configured"}
	if strings.TrimSpace(path) != "" {
		info, err := supportStat(ctx, path)
		switch {
		case err == context.DeadlineExceeded || err == context.Canceled:
			result.Status = "timed_out"
		case os.IsNotExist(err):
			result.Status = "missing"
		case err != nil:
			result.Status = "unavailable"
		case !info.IsDir():
			result.Status = "not_directory"
		default:
			result.Status = "directory_present"
		}
	}
	result.CheckedAt = time.Now().UTC()
	return result
}

// A stalled NAS stat must not hold an HTTP response forever or create unbounded
// goroutines. At most four filesystem checks may remain in the kernel at once.
var supportStatSlots = make(chan struct{}, 4)

func supportStat(ctx context.Context, path string) (os.FileInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	select {
	case supportStatSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	type result struct {
		info os.FileInfo
		err  error
	}
	done := make(chan result, 1)
	go func() {
		defer func() { <-supportStatSlots }()
		info, err := os.Stat(path)
		done <- result{info, err}
	}()
	select {
	case got := <-done:
		return got.info, got.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var supportVersionPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){1,3}$`)

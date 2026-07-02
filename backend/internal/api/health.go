package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/bandoracer/librarry/backend/internal/acquisition"
	"github.com/bandoracer/librarry/backend/internal/library"
	"github.com/bandoracer/librarry/backend/internal/notify"
)

// HealthCheck is one evaluated health rule. Every evaluated rule is returned
// (including ok ones) so the UI can filter by severity.
type HealthCheck struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Name     string `json:"name"`
	Message  string `json:"message"`
}

const (
	healthSeverityOK      = "ok"
	healthSeverityWarning = "warning"
	healthSeverityError   = "error"

	lowDiskErrorBytes   = int64(1) << 30 // 1 GiB
	lowDiskWarningBytes = int64(5) << 30 // 5 GiB
)

// healthInputs is the snapshot the pure check rules run against.
type healthInputs struct {
	databaseConfigured       bool
	integrationsAvailable    bool
	integrations             []acquisition.IntegrationHealth
	roots                    []healthRoot
	disks                    []library.DiskSpace
	completedImportEnabled   bool
	qualityProfilesAvailable bool
	qualityProfileCount      int
	qualityProfileError      string
}

type healthRoot struct {
	Path       string
	Label      string
	Accessible bool
}

// HealthEvaluator runs the health rules (on the 5m registry task and on
// demand) and dispatches healthIssue notifications when a check transitions
// from ok to warning/error.
type HealthEvaluator struct {
	handler *handler
	notify  *notify.Service

	mu        sync.Mutex
	lastState map[string]string
}

func NewHealthEvaluator(deps Dependencies) *HealthEvaluator {
	return &HealthEvaluator{
		handler:   &handler{deps: deps},
		notify:    deps.Notify,
		lastState: map[string]string{},
	}
}

// Evaluate runs every check, dispatches ok-to-bad transition notifications,
// and returns the full check list.
func (e *HealthEvaluator) Evaluate(ctx context.Context) []HealthCheck {
	checks := evaluateHealthChecks(e.handler.healthInputs(ctx))
	e.mu.Lock()
	nextState, events := healthTransitions(e.lastState, checks)
	e.lastState = nextState
	e.mu.Unlock()
	if e.notify != nil && len(events) > 0 {
		e.notify.DispatchAll(ctx, events)
	}
	return checks
}

// healthTransitions computes the next last-state map and the healthIssue
// events for checks that newly turned bad (unknown or ok before, warning or
// error now).
func healthTransitions(lastState map[string]string, checks []HealthCheck) (map[string]string, []notify.Event) {
	nextState := make(map[string]string, len(checks))
	var events []notify.Event
	for _, check := range checks {
		nextState[check.ID] = check.Severity
		if check.Severity == healthSeverityOK {
			continue
		}
		previous, seen := lastState[check.ID]
		if seen && previous != healthSeverityOK {
			continue
		}
		events = append(events, notify.HealthIssueEvent(check.Name, check.Severity, check.Message))
	}
	return nextState, events
}

// healthInputs gathers the live snapshot the pure rules evaluate.
func (h *handler) healthInputs(ctx context.Context) healthInputs {
	inputs := healthInputs{
		databaseConfigured:     databaseType(h.deps.Config.DatabaseURL) != "none",
		completedImportEnabled: h.deps.Config.CompletedImportEnabled,
	}
	if h.deps.Acquire != nil {
		inputs.integrationsAvailable = true
		inputs.integrations = h.deps.Acquire.Health(ctx)
	}
	paths := h.monitoredRootPaths(ctx)
	for _, path := range paths {
		root := healthRoot{Path: path.Path, Label: path.Label}
		if info, err := os.Stat(path.Path); err == nil && info.IsDir() {
			root.Accessible = true
		}
		inputs.roots = append(inputs.roots, root)
	}
	inputs.disks = library.DiskSpaces(paths)
	if h.deps.Wanted != nil {
		inputs.qualityProfilesAvailable = true
		profiles, err := h.deps.Wanted.ListQualityProfiles(ctx)
		if err != nil {
			inputs.qualityProfileError = err.Error()
		} else {
			inputs.qualityProfileCount = len(profiles)
		}
	}
	return inputs
}

// monitoredRootPaths lists the labelled library roots: native root folders
// when configured, else the effective two-root config.
func (h *handler) monitoredRootPaths(ctx context.Context) []library.DiskPath {
	seen := map[string]bool{}
	var paths []library.DiskPath
	appendPath := func(path string, label string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, library.DiskPath{Path: path, Label: label})
	}
	if rootService, ok := h.deps.Library.(rootFolderLibraryService); ok {
		if folders, err := rootService.ListRootFolders(ctx); err == nil {
			for _, folder := range folders {
				label := folder.Name
				if label == "" {
					label = folder.MediaFormat
				}
				appendPath(folder.Path, label)
			}
		}
	}
	if len(paths) > 0 {
		return paths
	}
	config, err := h.effectiveLibraryConfig(ctx)
	if err != nil {
		return paths
	}
	config = library.NormalizeConfig(config)
	appendPath(config.EbookRoot, "Ebooks")
	appendPath(config.AudiobookRoot, "Audiobooks")
	return paths
}

// evaluateHealthChecks applies the health rules to a snapshot. Pure so the
// state matrix is testable.
func evaluateHealthChecks(inputs healthInputs) []HealthCheck {
	checks := []HealthCheck{databaseHealthCheck(inputs)}
	checks = append(checks, indexerHealthCheck(inputs))
	checks = append(checks, downloadClientHealthCheck(inputs))
	checks = append(checks, rootFolderHealthChecks(inputs)...)
	checks = append(checks, completedImportHealthCheck(inputs))
	checks = append(checks, diskSpaceHealthChecks(inputs)...)
	checks = append(checks, qualityProfileHealthCheck(inputs))
	return checks
}

func databaseHealthCheck(inputs healthInputs) HealthCheck {
	check := HealthCheck{ID: "database", Name: "Database persistence"}
	if inputs.databaseConfigured {
		check.Severity = healthSeverityOK
		check.Message = "Postgres persistence is configured."
		return check
	}
	check.Severity = healthSeverityWarning
	check.Message = "LIBRARRY_DATABASE_URL is not set; wanted queues, history, and settings are not persisted."
	return check
}

func indexerHealthCheck(inputs healthInputs) HealthCheck {
	check := HealthCheck{ID: "indexer", Name: "Indexer"}
	for _, integration := range inputs.integrations {
		if integration.Name != "Prowlarr" {
			continue
		}
		if !integration.Configured {
			break
		}
		if integration.Status == "ready" {
			check.Severity = healthSeverityOK
			check.Message = integration.Message
			return check
		}
		check.Severity = healthSeverityError
		check.Message = "Prowlarr is unreachable: " + integration.Message
		return check
	}
	check.Severity = healthSeverityError
	check.Message = "No indexer is configured. Configure Prowlarr so searches can find releases."
	return check
}

func downloadClientHealthCheck(inputs healthInputs) HealthCheck {
	check := HealthCheck{ID: "download-client", Name: "Download client"}
	configured := 0
	var readyNames []string
	var configuredNames []string
	for _, integration := range inputs.integrations {
		if integration.Name == "Prowlarr" {
			continue
		}
		if integration.Configured {
			configured++
			configuredNames = append(configuredNames, integration.Name)
		}
		if integration.Status == "ready" {
			readyNames = append(readyNames, integration.Name)
		}
	}
	switch {
	case len(readyNames) > 0:
		check.Severity = healthSeverityOK
		check.Message = joinNames(readyNames) + " ready for grabs."
	case configured > 0:
		check.Severity = healthSeverityError
		check.Message = joinNames(configuredNames) + " configured but unreachable. Check credentials and network access."
	default:
		check.Severity = healthSeverityError
		check.Message = "No download client is configured. Configure qBittorrent, Transmission, or SABnzbd."
	}
	return check
}

func rootFolderHealthChecks(inputs healthInputs) []HealthCheck {
	if len(inputs.roots) == 0 {
		return []HealthCheck{{
			ID:       "root-folders",
			Name:     "Root folders",
			Severity: healthSeverityError,
			Message:  "No root folders are configured. Imports have nowhere to place books.",
		}}
	}
	checks := make([]HealthCheck, 0, len(inputs.roots))
	for _, root := range inputs.roots {
		check := HealthCheck{
			ID:   "root-folder:" + root.Path,
			Name: "Root folder " + root.Path,
		}
		if root.Accessible {
			check.Severity = healthSeverityOK
			check.Message = "Root folder is accessible."
		} else {
			check.Severity = healthSeverityError
			check.Message = "Root folder is missing or not a directory."
		}
		checks = append(checks, check)
	}
	return checks
}

func completedImportHealthCheck(inputs healthInputs) HealthCheck {
	check := HealthCheck{ID: "completed-import", Name: "Completed download handling"}
	if inputs.completedImportEnabled {
		check.Severity = healthSeverityOK
		check.Message = "Finished downloads are imported automatically."
		return check
	}
	check.Severity = healthSeverityWarning
	check.Message = "Completed download import is disabled; finished downloads will wait for manual import."
	return check
}

func diskSpaceHealthChecks(inputs healthInputs) []HealthCheck {
	checks := make([]HealthCheck, 0, len(inputs.disks))
	for _, disk := range inputs.disks {
		check := HealthCheck{
			ID:   "disk-space:" + disk.Path,
			Name: "Disk space " + disk.Path,
		}
		free := formatBytes(disk.FreeBytes)
		switch {
		case disk.FreeBytes < lowDiskErrorBytes:
			check.Severity = healthSeverityError
			check.Message = "Only " + free + " free (below 1 GiB)."
		case disk.FreeBytes < lowDiskWarningBytes:
			check.Severity = healthSeverityWarning
			check.Message = "Only " + free + " free (below 5 GiB)."
		default:
			check.Severity = healthSeverityOK
			check.Message = free + " free."
		}
		checks = append(checks, check)
	}
	return checks
}

func qualityProfileHealthCheck(inputs healthInputs) HealthCheck {
	check := HealthCheck{ID: "quality-profiles", Name: "Quality profiles"}
	switch {
	case !inputs.qualityProfilesAvailable:
		check.Severity = healthSeverityWarning
		check.Message = "Wanted service is unavailable; release decisions use built-in defaults."
	case inputs.qualityProfileError != "":
		check.Severity = healthSeverityWarning
		check.Message = inputs.qualityProfileError
	case inputs.qualityProfileCount == 0:
		check.Severity = healthSeverityWarning
		check.Message = "No quality profiles are configured. Release scoring uses built-in defaults."
	default:
		check.Severity = healthSeverityOK
		check.Message = strconv.Itoa(inputs.qualityProfileCount) + " quality profiles available."
	}
	return check
}

func joinNames(names []string) string {
	joined := ""
	for i, name := range names {
		if i > 0 {
			joined += ", "
		}
		joined += name
	}
	return joined
}

func formatBytes(value int64) string {
	const gib = int64(1) << 30
	const mib = int64(1) << 20
	switch {
	case value >= gib:
		return strconv.FormatFloat(float64(value)/float64(gib), 'f', 1, 64) + " GiB"
	case value >= mib:
		return strconv.FormatFloat(float64(value)/float64(mib), 'f', 1, 64) + " MiB"
	default:
		return strconv.FormatInt(value, 10) + " B"
	}
}

func (h *handler) systemHealth(w http.ResponseWriter, r *http.Request) {
	var checks []HealthCheck
	if h.deps.Health != nil {
		checks = h.deps.Health.Evaluate(r.Context())
	} else {
		checks = evaluateHealthChecks(h.healthInputs(r.Context()))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"checks":      checks,
		"generatedAt": time.Now().UTC(),
	})
}

import {
  Activity,
  BookOpen,
  CheckCircle2,
  Clock3,
  Database,
  Download,
  FileCheck2,
  FileSearch,
  FolderSearch,
  HardDriveDownload,
  History as HistoryIcon,
  Library,
  Pause,
  Play,
  RadioTower,
  RefreshCw,
  Search,
  Settings,
  SlidersHorizontal,
  Trash2,
  TrendingUp,
  UploadCloud
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  createWanted,
  fetchIntegrationHealth,
  fetchDownloads,
  fetchHistory,
  fetchLibraryFiles,
  fetchLibraryImportReviews,
  fetchProviderHealth,
  fetchWanted,
  grabWanted,
  grabRelease,
  importCompletedDownloads,
  importLibraryFile,
  recoverFailedDownloads,
  resolveLibraryImportReview,
  runUpgradeSearch,
  runWantedFeedSync,
  runWantedMonitor,
  runDownloadAction,
  scanLibrary,
  searchMetadata,
  searchReleases,
  searchWantedReleases,
  type DownloadAction,
  type CompletedImportOutcome,
  type DownloadStatus,
  type FailedDownloadRun,
  type FeedSyncRun,
  type HistoryEvent,
  type ImportReview,
  type IntegrationHealth,
  type LibraryFile,
  type LibraryImportOutcome,
  type LibraryScanOutcome,
  type MonitorRun,
  type ProviderHealth,
  type ReleaseDecision,
  type Release,
  type SearchResult,
  type UpgradeRun,
  type WantedItem
} from "./lib/api";
import { seedProviders, seedResults } from "./lib/seed";

const navItems = [
  { label: "Dashboard", icon: Activity },
  { label: "Search", icon: Search },
  { label: "Wanted", icon: Clock3 },
  { label: "Imports", icon: UploadCloud },
  { label: "Providers", icon: Database },
  { label: "Settings", icon: Settings }
];

export function App() {
  const [providers, setProviders] = useState<ProviderHealth[]>(seedProviders);
  const [results, setResults] = useState<SearchResult[]>(seedResults);
  const [integrations, setIntegrations] = useState<IntegrationHealth[]>([]);
  const [releases, setReleases] = useState<Release[]>([]);
  const [wantedItems, setWantedItems] = useState<WantedItem[]>([]);
  const [wantedReleases, setWantedReleases] = useState<ReleaseDecision[]>([]);
  const [downloads, setDownloads] = useState<DownloadStatus[]>([]);
  const [historyEvents, setHistoryEvents] = useState<HistoryEvent[]>([]);
  const [libraryFiles, setLibraryFiles] = useState<LibraryFile[]>([]);
  const [importReviews, setImportReviews] = useState<ImportReview[]>([]);
  const [libraryScan, setLibraryScan] = useState<LibraryScanOutcome | null>(null);
  const [libraryImport, setLibraryImport] = useState<LibraryImportOutcome | null>(null);
  const [completedImport, setCompletedImport] = useState<CompletedImportOutcome | null>(null);
  const [monitorRun, setMonitorRun] = useState<MonitorRun | null>(null);
  const [feedSyncRun, setFeedSyncRun] = useState<FeedSyncRun | null>(null);
  const [failedDownloadRun, setFailedDownloadRun] = useState<FailedDownloadRun | null>(null);
  const [upgradeRun, setUpgradeRun] = useState<UpgradeRun | null>(null);
  const [downloadStatus, setDownloadStatus] = useState<DownloadStatus | null>(null);
  const [selectedID, setSelectedID] = useState(seedResults[0]?.work.id ?? "");
  const [selectedWantedID, setSelectedWantedID] = useState("");
  const [query, setQuery] = useState("Project Hail Mary");
  const [importPath, setImportPath] = useState("");
  const [format, setFormat] = useState("any");
  const [apiState, setAPIState] = useState<"checking" | "live" | "offline">("checking");
  const [isSearching, setIsSearching] = useState(false);
  const [isSearchingReleases, setIsSearchingReleases] = useState(false);
  const [isMarkingWanted, setIsMarkingWanted] = useState(false);
  const [isSearchingWanted, setIsSearchingWanted] = useState(false);
  const [isRunningMonitor, setIsRunningMonitor] = useState(false);
  const [isRunningFeedSync, setIsRunningFeedSync] = useState(false);
  const [isRunningUpgrade, setIsRunningUpgrade] = useState(false);
  const [isScanningLibrary, setIsScanningLibrary] = useState(false);
  const [isImportingLibrary, setIsImportingLibrary] = useState(false);
  const [isImportingCompleted, setIsImportingCompleted] = useState(false);
  const [isRecoveringFailed, setIsRecoveringFailed] = useState(false);
  const [isRefreshingDownloads, setIsRefreshingDownloads] = useState(false);
  const [reviewActionID, setReviewActionID] = useState("");
  const [downloadActionID, setDownloadActionID] = useState("");
  const [releaseError, setReleaseError] = useState("");
  const [wantedError, setWantedError] = useState("");
  const [monitorError, setMonitorError] = useState("");
  const [feedError, setFeedError] = useState("");
  const [upgradeError, setUpgradeError] = useState("");
  const [historyError, setHistoryError] = useState("");
  const [libraryError, setLibraryError] = useState("");
  const [downloadError, setDownloadError] = useState("");

  useEffect(() => {
    Promise.all([fetchProviderHealth(), fetchIntegrationHealth()])
      .then(([nextProviders, nextIntegrations]) => {
        setProviders(nextProviders);
        setIntegrations(nextIntegrations);
        setAPIState("live");
      })
      .catch(() => {
        setAPIState("offline");
      });
    fetchDownloads()
      .then(setDownloads)
      .catch((error) => {
        setDownloadError(error instanceof Error ? error.message : "Download refresh failed");
      });
    fetchWanted()
      .then((items) => {
        setWantedItems(items);
        setSelectedWantedID(items[0]?.id ?? "");
      })
      .catch((error) => {
        setWantedError(error instanceof Error ? error.message : "Wanted refresh failed");
      });
    fetchHistory()
      .then(setHistoryEvents)
      .catch((error) => {
        setHistoryError(error instanceof Error ? error.message : "History refresh failed");
      });
    fetchLibraryFiles()
      .then(setLibraryFiles)
      .catch((error) => {
        setLibraryError(error instanceof Error ? error.message : "Library refresh failed");
      });
    fetchLibraryImportReviews()
      .then(setImportReviews)
      .catch((error) => {
        setLibraryError(error instanceof Error ? error.message : "Import review refresh failed");
      });
  }, []);

  const selected = useMemo(
    () => results.find((result) => result.work.id === selectedID) ?? results[0],
    [results, selectedID]
  );
  const selectedWanted = useMemo(
    () => wantedItems.find((item) => item.id === selectedWantedID) ?? wantedItems[0],
    [wantedItems, selectedWantedID]
  );

  async function runSearch() {
    if (!query.trim()) return;
    setIsSearching(true);
    try {
      const nextResults = await searchMetadata(query, format);
      setResults(nextResults);
      setSelectedID(nextResults[0]?.work.id ?? "");
      setAPIState("live");
    } catch {
      setAPIState("offline");
    } finally {
      setIsSearching(false);
    }
  }

  async function runReleaseSearch() {
    const releaseQuery = selected?.work.title ?? query;
    if (!releaseQuery.trim()) return;
    setIsSearchingReleases(true);
    setReleaseError("");
    try {
      const nextReleases = await searchReleases(releaseQuery, selected?.edition?.format ?? format);
      setReleases(nextReleases);
    } catch (error) {
      setReleaseError(error instanceof Error ? error.message : "Release search failed");
    } finally {
      setIsSearchingReleases(false);
    }
  }

  async function runGrab(release: Release) {
    setReleaseError("");
    try {
      const status = await grabRelease(release, selected?.edition?.format ?? format);
      setDownloadStatus(status);
      await refreshDownloads();
    } catch (error) {
      setReleaseError(error instanceof Error ? error.message : "Grab failed");
    }
  }

  async function markSelectedWanted() {
    if (!selected) return;
    setIsMarkingWanted(true);
    setWantedError("");
    try {
      const item = await createWanted(selected, selected.edition?.format ?? format);
      setWantedItems((current) => mergeWanted(current, [item]));
      setSelectedWantedID(item.id);
      setAPIState("live");
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Mark wanted failed");
    } finally {
      setIsMarkingWanted(false);
    }
  }

  async function runWantedReleaseSearch(item = selectedWanted) {
    if (!item) return;
    setIsSearchingWanted(true);
    setWantedError("");
    try {
      const outcome = await searchWantedReleases(item.id);
      setWantedItems((current) => mergeWanted(current, [outcome.wantedItem]));
      setWantedReleases(outcome.releases);
      setSelectedWantedID(item.id);
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted release search failed");
    } finally {
      setIsSearchingWanted(false);
    }
  }

  async function grabWantedRelease(release?: ReleaseDecision) {
    const item = selectedWanted;
    if (!item) return;
    setWantedError("");
    try {
      const status = await grabWanted(item.id, release?.id);
      setDownloadStatus(status);
      await refreshDownloads();
      await refreshWantedAndHistory();
    } catch (error) {
      setWantedError(error instanceof Error ? error.message : "Wanted grab failed");
    }
  }

  async function runMonitor(options: { force?: boolean; autoGrab?: boolean }) {
    setIsRunningMonitor(true);
    setMonitorError("");
    try {
      const run = await runWantedMonitor({
        force: options.force ?? false,
        autoGrab: options.autoGrab ?? false,
        paused: true
      });
      setMonitorRun(run);
      setWantedItems((current) => mergeWanted(current, run.items?.map((item) => item.wantedItem) ?? []));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
    } catch (error) {
      setMonitorError(error instanceof Error ? error.message : "Wanted monitor failed");
    } finally {
      setIsRunningMonitor(false);
    }
  }

  async function runFeedSync(options: { autoGrab?: boolean }) {
    setIsRunningFeedSync(true);
    setFeedError("");
    try {
      const run = await runWantedFeedSync({
        format,
        autoGrab: options.autoGrab ?? false,
        paused: true
      });
      setFeedSyncRun(run);
      setWantedItems((current) => mergeWanted(current, run.matches?.map((match) => match.wantedItem) ?? []));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setFeedError(error instanceof Error ? error.message : "Feed sync failed");
    } finally {
      setIsRunningFeedSync(false);
    }
  }

  async function runUpgrades(options: { autoGrab?: boolean }) {
    setIsRunningUpgrade(true);
    setUpgradeError("");
    try {
      const run = await runUpgradeSearch({
        autoGrab: options.autoGrab ?? false,
        paused: true,
        force: true
      });
      setUpgradeRun(run);
      setWantedItems((current) => mergeWanted(current, run.items?.map((item) => item.wantedItem) ?? []));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setUpgradeError(error instanceof Error ? error.message : "Upgrade search failed");
    } finally {
      setIsRunningUpgrade(false);
    }
  }

  async function refreshWantedAndHistory() {
    setHistoryError("");
    setWantedError("");
    try {
      const [nextWanted, nextHistory] = await Promise.all([fetchWanted(), fetchHistory()]);
      setWantedItems(nextWanted);
      setHistoryEvents(nextHistory);
      setSelectedWantedID((current) => current || nextWanted[0]?.id || "");
    } catch (error) {
      const message = error instanceof Error ? error.message : "Refresh failed";
      setHistoryError(message);
      setWantedError(message);
    }
  }

  async function runLibraryScan(nextFormat = format) {
    setIsScanningLibrary(true);
    setLibraryError("");
    try {
      const outcome = await scanLibrary(nextFormat);
      setLibraryScan(outcome);
      setLibraryFiles((current) => mergeLibraryFiles(current, outcome.files));
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Library scan failed");
    } finally {
      setIsScanningLibrary(false);
    }
  }

  async function runLibraryImport() {
    if (!importPath.trim()) return;
    setIsImportingLibrary(true);
    setLibraryError("");
    try {
      const outcome = await importLibraryFile({
        sourcePath: importPath.trim(),
        wantedId: selectedWanted?.id,
        format: selectedWanted?.format ?? (format === "any" ? "ebook" : format),
        move: false
      });
      setLibraryImport(outcome);
      setLibraryFiles((current) => mergeLibraryFiles(current, [outcome.file]));
      const nextWanted = await fetchWanted();
      setWantedItems(nextWanted);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Library import failed");
    } finally {
      setIsImportingLibrary(false);
    }
  }

  async function runCompletedImport(download?: DownloadStatus) {
    setIsImportingCompleted(true);
    setLibraryError("");
    try {
      const outcome = await importCompletedDownloads({
        downloadIds: download ? [download.id] : [],
        move: false,
        limit: 50
      });
      setCompletedImport(outcome);
      const importedFiles = outcome.results.flatMap((result) => (result.import ? [result.import.file] : []));
      if (importedFiles.length) {
        setLibraryFiles((current) => mergeLibraryFiles(current, importedFiles));
      }
      const [reviews] = await Promise.all([fetchLibraryImportReviews(), refreshDownloads(), refreshWantedAndHistory()]);
      setImportReviews(reviews);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Completed import failed");
    } finally {
      setIsImportingCompleted(false);
    }
  }

  async function runResolveImportReview(review: ImportReview, action: "import" | "skip" | "reject") {
    const actionID = `${review.id}:${action}`;
    setReviewActionID(actionID);
    setLibraryError("");
    try {
      const nextFormat = review.mediaFormat === "unknown" ? (selectedWanted?.format ?? (format === "any" ? "ebook" : format)) : review.mediaFormat;
      const outcome = await resolveLibraryImportReview(review.id, {
        action,
        wantedId: action === "import" ? (selectedWanted?.id ?? review.wantedId) : review.wantedId,
        format: nextFormat,
        move: false
      });
      if (outcome.import) {
        setLibraryImport(outcome.import);
        setLibraryFiles((current) => mergeLibraryFiles(current, [outcome.import!.file]));
      }
      setImportReviews((current) => current.filter((item) => item.id !== review.id));
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setLibraryError(error instanceof Error ? error.message : "Import review update failed");
    } finally {
      setReviewActionID("");
    }
  }

  async function runFailedRecovery(download?: DownloadStatus, options: { autoGrab?: boolean; removeFailed?: boolean; force?: boolean } = {}) {
    const actionID = download ? `${download.id}:recover` : "";
    setIsRecoveringFailed(true);
    if (actionID) {
      setDownloadActionID(actionID);
    }
    setDownloadError("");
    try {
      const run = await recoverFailedDownloads({
        downloadIds: download ? [download.id] : [],
        autoGrab: options.autoGrab ?? false,
        paused: true,
        removeFailed: options.removeFailed ?? false,
        force: options.force ?? Boolean(download)
      });
      setFailedDownloadRun(run);
      await Promise.all([refreshDownloads(), refreshWantedAndHistory()]);
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Failed-download recovery failed");
    } finally {
      setIsRecoveringFailed(false);
      if (actionID) {
        setDownloadActionID("");
      }
    }
  }

  async function refreshDownloads() {
    setIsRefreshingDownloads(true);
    setDownloadError("");
    try {
      const nextDownloads = await fetchDownloads();
      setDownloads(nextDownloads);
      setAPIState("live");
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download refresh failed");
    } finally {
      setIsRefreshingDownloads(false);
    }
  }

  async function applyDownloadAction(action: DownloadAction, download: DownloadStatus, deleteFiles = false) {
    setDownloadActionID(`${download.id}:${action}`);
    setDownloadError("");
    try {
      const result = await runDownloadAction(action, [download.id], { deleteFiles });
      if (result.downloads?.length) {
        setDownloads((current) => mergeDownloads(current, result.downloads ?? []));
      } else if (action === "delete") {
        setDownloads((current) => current.filter((item) => item.id !== download.id));
      } else {
        await refreshDownloads();
      }
    } catch (error) {
      setDownloadError(error instanceof Error ? error.message : "Download action failed");
    } finally {
      setDownloadActionID("");
    }
  }

  const readyCount = providers.filter((provider) => provider.status === "ready").length;
  const integrationReadyCount = integrations.filter((integration) => integration.status === "ready").length;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark">
            <Library size={22} />
          </div>
          <div>
            <strong>Librarry</strong>
            <span>Metadata ops</span>
          </div>
        </div>

        <nav className="nav-list" aria-label="Primary navigation">
          {navItems.map((item) => (
            <button className={item.label === "Search" ? "nav-item active" : "nav-item"} key={item.label}>
              <item.icon size={17} />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-footer">
          <span className={apiState === "live" ? "status-dot ready" : "status-dot muted"} />
          <span>{apiState === "live" ? "API connected" : apiState === "checking" ? "Checking API" : "Demo data"}</span>
        </div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div>
            <h1>Metadata search</h1>
            <p>Resolve works, editions, identifiers, and provider confidence before acquisition.</p>
          </div>
          <div className="topbar-status">
            <CheckCircle2 size={18} />
            <span>{readyCount}/{providers.length} providers ready</span>
          </div>
        </header>

        <section className="search-strip" aria-label="Metadata search controls">
          <div className="search-input">
            <Search size={18} />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") runSearch();
              }}
              placeholder="Search title, author, series, or ISBN"
            />
          </div>
          <div className="segmented" role="group" aria-label="Format">
            {["any", "ebook", "audiobook"].map((option) => (
              <button
                className={format === option ? "selected" : ""}
                key={option}
                onClick={() => setFormat(option)}
                type="button"
              >
                {option}
              </button>
            ))}
          </div>
          <button className="primary-action" onClick={runSearch} type="button">
            <FileSearch size={17} />
            <span>{isSearching ? "Searching" : "Search"}</span>
          </button>
        </section>

        <section className="provider-grid" aria-label="Provider health">
          {providers.map((provider) => (
            <article className="provider-tile" key={provider.name}>
              <div className="provider-header">
                <span className={provider.status === "ready" ? "status-dot ready" : "status-dot warn"} />
                <strong>{provider.name}</strong>
              </div>
              <p>{provider.message}</p>
              <span className="provider-state">{provider.status.replace(/_/g, " ")}</span>
            </article>
          ))}
        </section>

        <section className="integration-strip" aria-label="Acquisition integrations">
          <div className="integration-summary">
            <HardDriveDownload size={18} />
            <span>{integrationReadyCount}/{integrations.length || 2} acquisition integrations ready</span>
          </div>
          {integrations.map((integration) => (
            <div className="integration-pill" key={integration.name}>
              <span className={integration.status === "ready" ? "status-dot ready" : "status-dot warn"} />
              <strong>{integration.name}</strong>
              <span>{integration.message}</span>
            </div>
          ))}
        </section>

        <div className="content-grid">
          <section className="results-panel" aria-label="Search results">
            <div className="panel-heading">
              <div>
                <h2>Candidate matches</h2>
                <p>{results.length} normalized results sorted by confidence and provider rank.</p>
              </div>
              <button className="icon-button" type="button" aria-label="Filter results">
                <SlidersHorizontal size={18} />
              </button>
            </div>

            <div className="result-table" role="table">
              <div className="table-row table-head" role="row">
                <span>Title</span>
                <span>Format</span>
                <span>Source</span>
                <span>Confidence</span>
                <span>Action</span>
              </div>
              {results.map((result) => (
                <button
                  className={result.work.id === selected?.work.id ? "table-row result-row selected" : "table-row result-row"}
                  key={`${result.provider}-${result.work.id}`}
                  onClick={() => setSelectedID(result.work.id)}
                  role="row"
                  type="button"
                >
                  <span className="title-cell">
                    <BookOpen size={16} />
                    <span>
                      <strong>{result.work.title}</strong>
                      <small>{result.work.authors?.[0]?.name ?? "Unknown author"}</small>
                    </span>
                  </span>
                  <span>{result.edition?.format ?? "any"}</span>
                  <span>{result.provider}</span>
                  <span>
                    <em className={`confidence ${result.confidence}`}>{result.confidence}</em>
                  </span>
                  <span className="row-action">Review</span>
                </button>
              ))}
            </div>
          </section>

          <aside className="detail-panel" aria-label="Selected book details">
            {selected ? (
              <>
                <div className="cover-frame">
                  {selected.work.coverUrl ? <img src={selected.work.coverUrl} alt="" /> : <BookOpen size={42} />}
                </div>
                <h2>{selected.work.title}</h2>
                <p className="detail-author">{selected.work.authors?.[0]?.name ?? "Unknown author"}</p>

                <dl className="detail-list">
                  <div>
                    <dt>Provider</dt>
                    <dd>{selected.provider}</dd>
                  </div>
                  <div>
                    <dt>First published</dt>
                    <dd>{selected.work.firstPublishYear ?? "Unknown"}</dd>
                  </div>
                  <div>
                    <dt>Format</dt>
                    <dd>{selected.edition?.format ?? "Any"}</dd>
                  </div>
                  <div>
                    <dt>Identifiers</dt>
                    <dd>{selected.edition?.isbns?.slice(0, 2).join(", ") || "None"}</dd>
                  </div>
                  <div>
                    <dt>Matched on</dt>
                    <dd>{selected.matchedOn.join(", ")}</dd>
                  </div>
                  <div>
                    <dt>Manual override</dt>
                    <dd>None</dd>
                  </div>
                </dl>

                <div className="detail-actions">
                  <button className="secondary-action" onClick={markSelectedWanted} disabled={isMarkingWanted} type="button">
                    <HardDriveDownload size={17} />
                    <span>{isMarkingWanted ? "Marking" : "Mark wanted"}</span>
                  </button>
                  <button className="secondary-action" onClick={runReleaseSearch} type="button">
                    <Download size={17} />
                    {isSearchingReleases ? "Searching releases" : "Search releases"}
                  </button>
                </div>
              </>
            ) : (
              <div className="empty-detail">Select a result.</div>
            )}
          </aside>
        </div>

        <section className="release-panel" aria-label="Release search results">
          <div className="panel-heading">
            <div>
              <h2>Release search</h2>
              <p>
                {releases.length
                  ? `${releases.length} Prowlarr releases ready for paused qBittorrent grab.`
                  : "Search releases from the selected metadata match."}
              </p>
            </div>
            {downloadStatus ? <span className="download-state">Queued: {downloadStatus.category}</span> : null}
          </div>
          {releaseError ? <div className="inline-error">{releaseError}</div> : null}
          <div className="release-list">
            {releases.map((release) => (
              <article className="release-row" key={release.id}>
                <div>
                  <strong>{release.title}</strong>
                  <span>
                    {release.indexer} · {release.protocol} · {formatBytes(release.sizeBytes ?? 0)} · {release.seeders ?? 0} seeders
                  </span>
                </div>
                <button className="secondary-action compact" onClick={() => runGrab(release)} type="button">
                  Grab paused
                </button>
              </article>
            ))}
          </div>
        </section>

        <section className="wanted-panel" aria-label="Wanted queue">
          <div className="panel-heading">
            <div>
              <h2>Wanted queue</h2>
              <p>
                {wantedItems.length
                  ? `${wantedItems.length} wanted books ready for release evaluation.`
                  : "Mark a metadata result wanted to start Readarr-style acquisition planning."}
              </p>
            </div>
            <button className="secondary-action compact" disabled={!selectedWanted || isSearchingWanted} onClick={() => runWantedReleaseSearch()} type="button">
              <FileSearch size={16} />
              {isSearchingWanted ? "Searching" : "Search wanted"}
            </button>
          </div>
          {wantedError ? <div className="inline-error">{wantedError}</div> : null}
          <div className="wanted-grid">
            <div className="wanted-list">
              {wantedItems.map((item) => (
                <button
                  className={item.id === selectedWanted?.id ? "wanted-item selected" : "wanted-item"}
                  key={item.id}
                  onClick={() => {
                    setSelectedWantedID(item.id);
                    setWantedReleases([]);
                  }}
                  type="button"
                >
                  <span>
                    <strong>{item.title}</strong>
                    <small>{item.authorName || "Unknown author"}</small>
                  </span>
                  <em>{item.format}</em>
                </button>
              ))}
            </div>
            <div className="wanted-release-list">
              {wantedReleases.length ? (
                wantedReleases.map((release) => (
                  <article className={release.approved ? "wanted-release approved" : "wanted-release rejected"} key={release.id}>
                    <div>
                      <div className="wanted-release-title">
                        <strong>{release.title}</strong>
                        <span>{release.approved ? "Approved" : "Rejected"}</span>
                      </div>
                      <p>
                        {release.indexer} · score {release.score.toFixed(1)} · {formatBytes(release.sizeBytes ?? 0)} · {release.seeders ?? 0} seeders
                      </p>
                      {release.rejectedReason ? <small>{release.rejectedReason}</small> : null}
                    </div>
                    <button className="secondary-action compact" disabled={!release.approved} onClick={() => grabWantedRelease(release)} type="button">
                      Grab paused
                    </button>
                  </article>
                ))
              ) : (
                <div className="wanted-empty">
                  {selectedWanted ? "Search wanted releases to evaluate candidates." : "No wanted item selected."}
                </div>
              )}
            </div>
          </div>
        </section>

        <section className="library-panel" aria-label="Library import">
          <div className="panel-heading">
            <div>
              <h2>Library</h2>
              <p>
                {libraryScan
                  ? `${libraryScan.upserted} files indexed from ${libraryScan.roots.length} roots.`
                  : `${libraryFiles.length} tracked files from library scans and imports.`}
              </p>
            </div>
            <div className="library-actions">
              <button className="secondary-action compact" disabled={isScanningLibrary} onClick={() => runLibraryScan("ebook")} type="button">
                <FolderSearch size={16} />
                Ebooks
              </button>
              <button className="secondary-action compact" disabled={isScanningLibrary} onClick={() => runLibraryScan("audiobook")} type="button">
                <FolderSearch size={16} />
                Audio
              </button>
              <button className="secondary-action compact" disabled={isScanningLibrary} onClick={() => runLibraryScan("any")} type="button">
                <RefreshCw size={16} />
                All
              </button>
              <button className="secondary-action compact" disabled={isImportingCompleted} onClick={() => runCompletedImport()} type="button">
                <HardDriveDownload size={16} />
                Completed
              </button>
            </div>
          </div>
          {libraryError ? <div className="inline-error">{libraryError}</div> : null}
          <div className="library-import-row">
            <div className="library-import-input">
              <FileCheck2 size={17} />
              <input
                value={importPath}
                onChange={(event) => setImportPath(event.target.value)}
                placeholder="Source file path to import into the selected wanted book"
              />
            </div>
            <button className="primary-action" disabled={!importPath.trim() || isImportingLibrary} onClick={runLibraryImport} type="button">
              <UploadCloud size={17} />
              {isImportingLibrary ? "Importing" : "Import"}
            </button>
          </div>
          {libraryImport ? (
            <div className="library-import-result">
              <strong>{libraryImport.file.title || "Imported file"}</strong>
              <span>{libraryImport.destinationPath}</span>
            </div>
          ) : null}
          {completedImport ? (
            <div className="library-import-result">
              <strong>
                Completed import: {completedImport.imported} imported, {completedImport.reviewQueued} review, {completedImport.skipped} skipped,{" "}
                {completedImport.errored} errors
              </strong>
              <span>{completedImport.checked} downloads checked from the Librarry queue.</span>
            </div>
          ) : null}
          <div className="library-grid">
            {[
              ["Tracked", libraryFiles.length],
              ["Imported", libraryFiles.filter((file) => file.importStatus === "imported").length],
              ["Review", importReviews.length],
              ["Ebooks", libraryFiles.filter((file) => file.mediaFormat === "ebook").length],
              ["Audiobooks", libraryFiles.filter((file) => file.mediaFormat === "audiobook").length],
              ["Scanned", libraryScan?.scanned ?? 0],
              ["Skipped", libraryScan?.skipped ?? 0]
            ].map(([label, value]) => (
              <div className="library-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          {importReviews.length ? (
            <div className="import-review-list">
              {importReviews.slice(0, 6).map((review) => (
                <article className="import-review-row" key={review.id}>
                  <div>
                    <strong>{review.title || review.sourcePath.split("/").pop() || "Pending import"}</strong>
                    <span>
                      {review.authorName || "Unknown author"} · {review.mediaFormat} · {formatBytes(review.sizeBytes ?? 0)} · {review.reason}
                    </span>
                    <small>{review.sourcePath}</small>
                  </div>
                  <div className="import-review-actions">
                    <button
                      className="secondary-action compact"
                      disabled={Boolean(reviewActionID)}
                      onClick={() => runResolveImportReview(review, "import")}
                      type="button"
                    >
                      <CheckCircle2 size={16} />
                      {reviewActionID === `${review.id}:import` ? "Importing" : "Import"}
                    </button>
                    <button
                      className="secondary-action compact danger-outline"
                      disabled={Boolean(reviewActionID)}
                      onClick={() => runResolveImportReview(review, "skip")}
                      type="button"
                    >
                      <Trash2 size={16} />
                      Skip
                    </button>
                  </div>
                </article>
              ))}
            </div>
          ) : null}
          <div className="library-file-list">
            {libraryFiles.slice(0, 8).map((file) => (
              <article className="library-file-row" key={file.id || file.path}>
                <div>
                  <strong>{file.title || file.path.split("/").pop()}</strong>
                  <span>
                    {file.authorName || "Unknown author"} · {file.mediaFormat} · {file.importStatus} · {formatBytes(file.sizeBytes ?? 0)}
                  </span>
                  <small>{file.path}</small>
                </div>
                <em>{file.extension || "file"}</em>
              </article>
            ))}
          </div>
        </section>

        <section className="monitor-panel" aria-label="Wanted monitor">
          <div className="panel-heading">
            <div>
              <h2>Monitor</h2>
              <p>
                {monitorRun
                  ? `${monitorRun.status}: checked ${monitorRun.wantedChecked}, approved ${monitorRun.approvedCount}, grabbed ${monitorRun.grabbedCount}.`
                  : "Run Readarr-style wanted monitoring across due items or force a full wanted scan."}
              </p>
            </div>
            <div className="monitor-actions">
              <button className="secondary-action compact" disabled={isRunningMonitor} onClick={() => runMonitor({ force: false })} type="button">
                <RadioTower size={16} />
                Due scan
              </button>
              <button className="secondary-action compact" disabled={isRunningMonitor} onClick={() => runMonitor({ force: true })} type="button">
                <RefreshCw size={16} />
                Force scan
              </button>
              <button className="secondary-action compact danger-outline" disabled={isRunningMonitor} onClick={() => runMonitor({ force: true, autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                Scan + grab paused
              </button>
              <button className="secondary-action compact" disabled={isRunningFeedSync} onClick={() => runFeedSync({})} type="button">
                <RadioTower size={16} />
                Feed
              </button>
              <button className="secondary-action compact danger-outline" disabled={isRunningFeedSync} onClick={() => runFeedSync({ autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                Feed + grab
              </button>
              <button className="secondary-action compact" disabled={isRunningUpgrade} onClick={() => runUpgrades({})} type="button">
                <TrendingUp size={16} />
                Upgrades
              </button>
              <button className="secondary-action compact danger-outline" disabled={isRunningUpgrade} onClick={() => runUpgrades({ autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                Upgrade + grab
              </button>
            </div>
          </div>
          {monitorError ? <div className="inline-error">{monitorError}</div> : null}
          {feedError ? <div className="inline-error">{feedError}</div> : null}
          {upgradeError ? <div className="inline-error">{upgradeError}</div> : null}
          <div className="monitor-grid">
            {[
              ["Wanted checked", monitorRun?.wantedChecked ?? 0],
              ["Releases found", monitorRun?.releasesFound ?? 0],
              ["Approved", monitorRun?.approvedCount ?? 0],
              ["Rejected", monitorRun?.rejectedCount ?? 0],
              ["Grabbed", monitorRun?.grabbedCount ?? 0],
              ["Errors", monitorRun?.errorCount ?? 0]
            ].map(([label, value]) => (
              <div className="monitor-metric" key={label}>
                <span>{label}</span>
                <strong>{value}</strong>
              </div>
            ))}
          </div>
          {monitorRun?.items?.length ? (
            <div className="monitor-results">
              {monitorRun.items.map((item) => (
                <article className={item.error ? "monitor-result error" : "monitor-result"} key={item.wantedItem.id}>
                  <strong>{item.wantedItem.title}</strong>
                  <span>
                    {item.releasesFound} releases · {item.approvedCount} approved · {item.rejectedCount} rejected
                  </span>
                  {item.grabbedDownload ? <em>Queued {item.grabbedDownload.category}</em> : null}
                  {item.error ? <small>{item.error}</small> : null}
                </article>
              ))}
            </div>
          ) : null}
          {upgradeRun ? (
            <div className="upgrade-results">
              <div className="upgrade-summary">
                <strong>{upgradeRun.status}</strong>
                <span>
                  {upgradeRun.wantedChecked} checked · {upgradeRun.releasesFound} releases · {upgradeRun.upgradeCount} upgrades · {upgradeRun.grabbedCount} grabbed
                </span>
              </div>
              {upgradeRun.items?.slice(0, 8).map((item) => (
                <article className={item.upgradeRelease ? "upgrade-row approved" : item.error ? "upgrade-row rejected" : "upgrade-row"} key={item.wantedItem.id}>
                  <div>
                    <strong>{item.wantedItem.title}</strong>
                    <span>
                      current {item.currentScore.toFixed(1)} / cutoff {item.cutoffScore.toFixed(1)}
                      {item.upgradeRelease ? ` · ${item.upgradeRelease.title} (${item.upgradeRelease.score.toFixed(1)})` : item.error ? ` · ${item.error}` : " · no upgrade"}
                    </span>
                  </div>
                  {item.grabbedDownload ? <em>Queued</em> : null}
                </article>
              ))}
            </div>
          ) : null}
          {feedSyncRun ? (
            <div className="feed-sync-results">
              <div className="feed-sync-summary">
                <strong>{feedSyncRun.status}</strong>
                <span>
                  {feedSyncRun.releasesSeen} feed releases · {feedSyncRun.matchedCount} matches · {feedSyncRun.approvedCount} approved · {feedSyncRun.grabbedCount} grabbed
                </span>
              </div>
              {feedSyncRun.matches?.slice(0, 8).map((match) => (
                <article className={match.release.approved ? "feed-sync-row approved" : "feed-sync-row rejected"} key={`${match.wantedItem.id}-${match.release.id || match.release.sourceId}`}>
                  <div>
                    <strong>{match.release.title}</strong>
                    <span>
                      {match.wantedItem.title} · score {match.release.score.toFixed(1)} · {match.release.approved ? "approved" : match.release.rejectedReason || "rejected"}
                    </span>
                  </div>
                  {match.grabbedDownload ? <em>Queued</em> : null}
                </article>
              ))}
            </div>
          ) : null}
        </section>

        <section className="downloads-panel" aria-label="Download manager">
          <div className="panel-heading">
            <div>
              <h2>Downloads</h2>
              <p>
                {downloads.length
                  ? `${downloads.length} qBittorrent items tracked with Librarry tags.`
                  : "No Librarry-tagged downloads are currently visible."}
              </p>
            </div>
            <div className="download-toolbar">
              <button className="secondary-action compact" onClick={refreshDownloads} type="button">
                <RefreshCw size={16} />
                {isRefreshingDownloads ? "Refreshing" : "Refresh"}
              </button>
              <button className="secondary-action compact" disabled={isImportingCompleted || downloads.length === 0} onClick={() => runCompletedImport()} type="button">
                <UploadCloud size={16} />
                {isImportingCompleted ? "Importing" : "Import done"}
              </button>
              <button className="secondary-action compact" disabled={isRecoveringFailed || downloads.length === 0} onClick={() => runFailedRecovery(undefined, { autoGrab: true })} type="button">
                <HardDriveDownload size={16} />
                {isRecoveringFailed ? "Recovering" : "Recover failed"}
              </button>
            </div>
          </div>
          {downloadError ? <div className="inline-error">{downloadError}</div> : null}
          {failedDownloadRun ? (
            <div className="failed-download-results">
              <div className="failed-download-summary">
                <strong>{failedDownloadRun.status}</strong>
                <span>
                  {failedDownloadRun.downloadsChecked} checked · {failedDownloadRun.failedCount} failed · {failedDownloadRun.replacementsFound} replacements · {failedDownloadRun.grabbedCount} grabbed · {failedDownloadRun.removedCount} removed
                </span>
              </div>
              {failedDownloadRun.items?.slice(0, 6).map((item) => (
                <article className={item.error ? "failed-download-row error" : "failed-download-row"} key={`${item.download.id}-${item.failureReason}`}>
                  <div>
                    <strong>{item.download.name || item.download.id}</strong>
                    <span>
                      {item.failureReason}
                      {item.replacementRelease ? ` · replacement ${item.replacementRelease.title}` : ""}
                    </span>
                  </div>
                  {item.replacementDownload ? <em>Queued</em> : item.error ? <em>Error</em> : null}
                </article>
              ))}
            </div>
          ) : null}
          <div className="download-list">
            {downloads.map((download) => {
              const busy = downloadActionID.startsWith(`${download.id}:`);
              return (
                <article className="download-row" key={download.id}>
                  <div className="download-main">
                    <div className="download-title-line">
                      <strong>{download.name || download.id}</strong>
                      <span className={`download-badge ${stateTone(download.state)}`}>{download.state}</span>
                    </div>
                    <div className="download-meta">
                      <span>{download.category || "uncategorized"}</span>
                      <span>{formatBytes(download.downloadedBytes ?? 0)} / {formatBytes(download.sizeBytes ?? 0)}</span>
                      <span>{formatSpeed(download.downloadRate ?? 0)} down</span>
                      <span>{formatSpeed(download.uploadRate ?? 0)} up</span>
                      <span>ratio {(download.ratio ?? 0).toFixed(2)}</span>
                      <span>{download.seeders ?? 0} seeders</span>
                      <span>import {download.importStatus || "pending"}</span>
                      {download.retryCount ? <span>{download.retryCount} retries</span> : null}
                    </div>
                    {download.importError ? <div className="download-import-error">{download.importError}</div> : null}
                    {download.failureReason ? <div className="download-import-error">{download.failureReason}</div> : null}
                    <div className="progress-track" aria-label={`Download progress ${Math.round((download.progress ?? 0) * 100)} percent`}>
                      <span style={{ width: `${Math.max(0, Math.min(100, (download.progress ?? 0) * 100))}%` }} />
                    </div>
                    <div className="download-path">{download.savePath}</div>
                  </div>
                  <div className="download-actions">
                    <button className="icon-button" disabled={busy} onClick={() => applyDownloadAction("start", download)} type="button" aria-label="Start download" title="Start">
                      <Play size={16} />
                    </button>
                    <button className="icon-button" disabled={busy} onClick={() => applyDownloadAction("stop", download)} type="button" aria-label="Stop download" title="Stop">
                      <Pause size={16} />
                    </button>
                    <button className="icon-button" disabled={busy} onClick={() => applyDownloadAction("recheck", download)} type="button" aria-label="Recheck download" title="Recheck">
                      <RefreshCw size={16} />
                    </button>
                    <button className="icon-button" disabled={busy} onClick={() => applyDownloadAction("increasePriority", download)} type="button" aria-label="Increase priority" title="Increase priority">
                      <span className="priority-glyph">+</span>
                    </button>
                    <button className="icon-button" disabled={busy || isImportingCompleted} onClick={() => runCompletedImport(download)} type="button" aria-label="Import completed download" title="Import completed">
                      <UploadCloud size={16} />
                    </button>
                    <button className="icon-button" disabled={busy || isRecoveringFailed} onClick={() => runFailedRecovery(download, { autoGrab: true, force: true })} type="button" aria-label="Retry failed download" title="Retry failed download">
                      <HardDriveDownload size={16} />
                    </button>
                    <button className="icon-button danger" disabled={busy} onClick={() => applyDownloadAction("delete", download, false)} type="button" aria-label="Remove download" title="Remove without deleting files">
                      <Trash2 size={16} />
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        </section>

        <section className="history-panel" aria-label="Activity history">
          <div className="panel-heading">
            <div>
              <h2>History</h2>
              <p>
                {historyEvents.length
                  ? `${historyEvents.length} recent monitor and grab events.`
                  : "No monitor or grab history recorded yet."}
              </p>
            </div>
            <button className="secondary-action compact" onClick={refreshWantedAndHistory} type="button">
              <HistoryIcon size={16} />
              Refresh
            </button>
          </div>
          {historyError ? <div className="inline-error">{historyError}</div> : null}
          <div className="history-list">
            {historyEvents.map((event) => (
              <article className={`history-row ${event.severity}`} key={event.id}>
                <div>
                  <strong>{event.message}</strong>
                  <span>{event.eventType.replace(/_/g, " ")} · {formatDateTime(event.createdAt)}</span>
                </div>
                <em>{event.severity}</em>
              </article>
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}

function formatBytes(bytes: number) {
  if (!bytes) return "unknown size";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 ? 0 : 1)} ${units[unit]}`;
}

function formatSpeed(bytesPerSecond: number) {
  if (!bytesPerSecond) return "0 B/s";
  return `${formatBytes(bytesPerSecond)}/s`;
}

function formatDateTime(value: string) {
  if (!value) return "unknown time";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "unknown time";
  return date.toLocaleString();
}

function mergeDownloads(current: DownloadStatus[], next: DownloadStatus[]) {
  const byID = new Map(current.map((download) => [download.id, download]));
  for (const download of next) {
    byID.set(download.id, download);
  }
  return Array.from(byID.values());
}

function mergeWanted(current: WantedItem[], next: WantedItem[]) {
  const byID = new Map(current.map((item) => [item.id, item]));
  for (const item of next) {
    byID.set(item.id, item);
  }
  return Array.from(byID.values()).sort((a, b) => {
    const aTime = Date.parse(a.createdAt || "");
    const bTime = Date.parse(b.createdAt || "");
    return bTime - aTime;
  });
}

function mergeLibraryFiles(current: LibraryFile[], next: LibraryFile[]) {
  const byPath = new Map(current.map((file) => [file.path, file]));
  for (const file of next) {
    byPath.set(file.path, file);
  }
  return Array.from(byPath.values()).sort((a, b) => {
    const aTime = Date.parse(a.updatedAt || "");
    const bTime = Date.parse(b.updatedAt || "");
    return bTime - aTime;
  });
}

function stateTone(state: string) {
  const normalized = state.toLowerCase();
  if (normalized.includes("error") || normalized.includes("missing")) return "error";
  if (normalized.includes("stop") || normalized.includes("pause")) return "paused";
  if (normalized.includes("upload") || normalized.includes("seed")) return "seeding";
  if (normalized.includes("download") || normalized.includes("meta")) return "active";
  return "idle";
}

import {
  Activity,
  BookOpen,
  CheckCircle2,
  Clock3,
  Database,
  Download,
  FileSearch,
  HardDriveDownload,
  Library,
  Search,
  Settings,
  SlidersHorizontal,
  UploadCloud
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  fetchIntegrationHealth,
  fetchProviderHealth,
  grabRelease,
  searchMetadata,
  searchReleases,
  type DownloadStatus,
  type IntegrationHealth,
  type ProviderHealth,
  type Release,
  type SearchResult
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
  const [downloadStatus, setDownloadStatus] = useState<DownloadStatus | null>(null);
  const [selectedID, setSelectedID] = useState(seedResults[0]?.work.id ?? "");
  const [query, setQuery] = useState("Project Hail Mary");
  const [format, setFormat] = useState("any");
  const [apiState, setAPIState] = useState<"checking" | "live" | "offline">("checking");
  const [isSearching, setIsSearching] = useState(false);
  const [isSearchingReleases, setIsSearchingReleases] = useState(false);
  const [releaseError, setReleaseError] = useState("");

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
  }, []);

  const selected = useMemo(
    () => results.find((result) => result.work.id === selectedID) ?? results[0],
    [results, selectedID]
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
    } catch (error) {
      setReleaseError(error instanceof Error ? error.message : "Grab failed");
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
                  <button className="secondary-action" type="button">
                    <HardDriveDownload size={17} />
                    Mark wanted
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
